// Package diagram builds and lays out a Diagram from a flat list of resources.
package diagram

import (
	"sort"
	"strings"

	"github.com/kongou-ae/AzDiagram/internal/model"
	"github.com/kongou-ae/AzDiagram/internal/registry"
)

// Build converts a flat slice of resources (from the parser) into a fully
// assembled Diagram ready for layout and rendering.
func Build(resources []*model.Resource) *model.Diagram {
	// Annotate every resource with registry metadata.
	for _, r := range resources {
		registry.Annotate(r)
	}

	// Build a symbol → resource lookup map.
	bySymbol := make(map[string]*model.Resource, len(resources))
	for _, r := range resources {
		bySymbol[r.SymbolicName] = r
	}

	// Identify VNet resources (but not child subnet resources).
	var vnetResources []*model.Resource
	vnetSet := make(map[string]bool)
	for _, r := range resources {
		if isVNet(r.Type) {
			vnetResources = append(vnetResources, r)
			vnetSet[r.SymbolicName] = true
		}
	}

	// Build VNetContainers from VNet resources.
	vnetContainers := make([]*model.VNetContainer, 0, len(vnetResources))
	for _, vr := range vnetResources {
		vc := &model.VNetContainer{Resource: vr}
		for _, sd := range vr.Subnets {
			vc.Subnets = append(vc.Subnets, &model.SubnetContainer{
				Name:          sd.Name,
				AddressPrefix: sd.AddressPrefix,
				NSGSymbol:     sd.NSGSymbol,
				RTSymbol:      sd.RTSymbol,
			})
		}
		// If no explicit subnets were found, add a placeholder "default" subnet
		// so the container still renders as a VNet box.
		if len(vc.Subnets) == 0 {
			vc.Subnets = append(vc.Subnets, &model.SubnetContainer{Name: "default"})
		}
		vnetContainers = append(vnetContainers, vc)
	}

	// Pass 0: resolve NSGSymbol → AttachedNSG and RTSymbol → AttachedRT for each SubnetContainer.
	for _, vc := range vnetContainers {
		for _, sc := range vc.Subnets {
			if sc.NSGSymbol != "" {
				if nsg, ok := bySymbol[sc.NSGSymbol]; ok {
					sc.AttachedNSG = nsg
				}
			}
			if sc.RTSymbol != "" {
				if rt, ok := bySymbol[sc.RTSymbol]; ok {
					sc.AttachedRT = rt
				}
			}
		}
	}

	// Assign non-VNet resources to subnets or to the standalone list.
	//
	// Strategy:
	//   Pass 1 – identify which VNet/subnet each NIC belongs to (don't add yet).
	//   Pass 2 – for each compute resource, attach its NIC dependencies inline
	//             (r.AttachedNICs) and add the compute resource to the subnet.
	//   Pass 3 – add any remaining un-attached NICs to their subnet directly.
	//   All other resources → StandaloneResources.

	nicToVNet := make(map[string]*model.VNetContainer)     // NIC symName → VNet
	nicToSubnet := make(map[string]*model.SubnetContainer) // NIC symName → subnet
	nicConsumed := make(map[string]bool)                   // NIC attached to a VM

	// Pass 1: identify NIC → VNet/subnet mapping.
	for _, r := range resources {
		if isVNet(r.Type) || !isNIC(r.Type) {
			continue
		}
		for _, dep := range r.Dependencies {
			if vnetSet[dep] {
				vc := vnetContainerBySymbol(vnetContainers, dep)
				if vc != nil && len(vc.Subnets) > 0 {
					nicToVNet[r.SymbolicName] = vc
					// Use SubnetRef to find the correct subnet; fall back to first subnet.
					targetSC := vc.Subnets[0]
					if r.SubnetRef != nil {
						for _, sc := range vc.Subnets {
							if strings.EqualFold(sc.Name, r.SubnetRef.SubnetName) {
								targetSC = sc
								break
							}
						}
					}
					nicToSubnet[r.SymbolicName] = targetSC
				}
				break
			}
		}
	}

	// Collect already-assigned symbolic names.
	assigned := make(map[string]bool)
	for _, vc := range vnetContainers {
		assigned[vc.Resource.SymbolicName] = true
		// Mark subnet-attached NSGs and Route Tables as assigned so they don't appear as standalone cards.
		for _, sc := range vc.Subnets {
			if sc.AttachedNSG != nil {
				assigned[sc.AttachedNSG.SymbolicName] = true
			}
			if sc.AttachedRT != nil {
				assigned[sc.AttachedRT.SymbolicName] = true
			}
		}
	}

	// Pass 1b: resources with an explicit ipConfigurations subnet reference
	// (e.g. Azure Firewall → AzureFirewallSubnet, VPN GW → GatewaySubnet,
	//  Bastion → AzureBastionSubnet, internal Load Balancer → app-subnet).
	// NICs and compute resources are excluded here; they are handled by Pass 2/3.
	for _, r := range resources {
		if assigned[r.SymbolicName] || r.SubnetRef == nil {
			continue
		}
		// NICs are handled by Pass 1 / Pass 3; compute resources by Pass 2.
		if isNIC(r.Type) || isCompute(r.Type) {
			continue
		}
		vc := vnetContainerBySymbol(vnetContainers, r.SubnetRef.VNetSymbol)
		if vc == nil {
			continue
		}
		for _, sc := range vc.Subnets {
			if strings.EqualFold(sc.Name, r.SubnetRef.SubnetName) {
				sc.Resources = append(sc.Resources, r)
				assigned[r.SymbolicName] = true
				break
			}
		}
	}

	// Pass 1c: attach PIP resources inline to their Firewall / VPN GW / Bastion.
	// A PIP is "consumed" when the network resource lists it as a dependency.
	// Exception: Azure Firewall management PIPs are placed in the management subnet
	// as standalone cards instead of being attached inline.
	pipConsumed := make(map[string]bool)
	for _, r := range resources {
		if !isPIPConsumer(r.Type) {
			continue
		}
		for _, dep := range r.Dependencies {
			if pip, ok := bySymbol[dep]; ok && isPIP(pip.Type) {
				// Skip the management PIP — it will be placed in the mgmt subnet below.
				if dep == r.MgmtPIPSymbol {
					continue
				}
				r.AttachedPIPs = append(r.AttachedPIPs, pip)
				pipConsumed[dep] = true
				assigned[dep] = true
			}
		}
	}

	// Pass 1c-2: place a proxy card (with management PIP attached) into the management subnet.
	// The proxy card represents the Firewall's management interface without duplicating the icon.
	for _, r := range resources {
		if r.MgmtSubnetRef == nil || r.MgmtPIPSymbol == "" {
			continue
		}
		pip, ok := bySymbol[r.MgmtPIPSymbol]
		if !ok || assigned[pip.SymbolicName] {
			continue
		}
		vc := vnetContainerBySymbol(vnetContainers, r.MgmtSubnetRef.VNetSymbol)
		if vc == nil {
			continue
		}
		for _, sc := range vc.Subnets {
			if strings.EqualFold(sc.Name, r.MgmtSubnetRef.SubnetName) {
				proxy := &model.Resource{
					SymbolicName: r.SymbolicName + "__mgmt_proxy__",
					Type:         r.Type,
					DisplayName:  r.DisplayName,
					Category:     r.Category,
					ShortName:    r.ShortName,
					TypeLabel:    r.TypeLabel,
					IconColor:    r.IconColor,
					IsMgmtProxy:  true,
					AttachedPIPs: []*model.Resource{pip},
				}
				sc.Resources = append(sc.Resources, proxy)
				pipConsumed[pip.SymbolicName] = true
				assigned[pip.SymbolicName] = true
				break
			}
		}
	}

	// Pass 1d: form PE pairs.
	// PEs are already placed into subnets by Pass 1b. Move them from
	// sc.Resources into sc.PEPairs and pull their linked service into the pair.
	peLinkedConsumed := make(map[string]bool)
	for _, vc := range vnetContainers {
		for _, sc := range vc.Subnets {
			var remaining []*model.Resource
			for _, r := range sc.Resources {
				if !isPE(r.Type) {
					remaining = append(remaining, r)
					continue
				}
				pair := &model.PEPair{PE: r}
				if r.LinkedServiceSymbol != "" {
					if linked, ok := bySymbol[r.LinkedServiceSymbol]; ok {
						// If the linked service references a parent (e.g. App Service → App Service Plan),
						// promote the parent as the displayed linked service and add the child to it.
						if linked.ParentSymbol != "" {
							if parent, ok2 := bySymbol[linked.ParentSymbol]; ok2 {
								parent.ChildResources = append(parent.ChildResources, linked)
								assigned[r.LinkedServiceSymbol] = true
								peLinkedConsumed[r.LinkedServiceSymbol] = true
								pair.LinkedService = parent
								peLinkedConsumed[parent.SymbolicName] = true
								assigned[parent.SymbolicName] = true
							} else {
								pair.LinkedService = linked
								peLinkedConsumed[r.LinkedServiceSymbol] = true
								assigned[r.LinkedServiceSymbol] = true
							}
						} else {
							pair.LinkedService = linked
							peLinkedConsumed[r.LinkedServiceSymbol] = true
							assigned[r.LinkedServiceSymbol] = true
						}
					}
				}
				sc.PEPairs = append(sc.PEPairs, pair)
			}
			sc.Resources = remaining
		}
	}
	for _, r := range resources {
		if assigned[r.SymbolicName] || !isCompute(r.Type) {
			continue
		}
		var targetSC *model.SubnetContainer
		for _, dep := range r.Dependencies {
			if sc, ok := nicToSubnet[dep]; ok {
				if nic, exists := bySymbol[dep]; exists {
					r.AttachedNICs = append(r.AttachedNICs, nic)
					nicConsumed[dep] = true
				}
				if targetSC == nil {
					targetSC = sc
				}
			}
		}
		// Collect distinct NSGs from attached NICs and mark them as assigned.
		nsgSeen := make(map[string]bool)
		for _, nic := range r.AttachedNICs {
			if nic.NICNSGSymbol != "" && !nsgSeen[nic.NICNSGSymbol] {
				if nsgRes, ok := bySymbol[nic.NICNSGSymbol]; ok {
					r.AttachedNicNSGs = append(r.AttachedNicNSGs, nsgRes)
					assigned[nic.NICNSGSymbol] = true
					nsgSeen[nic.NICNSGSymbol] = true
				}
			}
		}
		if targetSC != nil {
			targetSC.Resources = append(targetSC.Resources, r)
			assigned[r.SymbolicName] = true
		}
	}

	// Pass 3: NICs not consumed by a VM go into their subnet directly.
	for nicSym, sc := range nicToSubnet {
		assigned[nicSym] = true // always mark as assigned (NIC is either inline or in subnet)
		if !nicConsumed[nicSym] {
			nic := bySymbol[nicSym]
			sc.Resources = append(sc.Resources, nic)
		}
	}

	// Collect edges – skip any edge whose target is a NIC that was merged into
	// a VM card, since those NICs have no independent position on the canvas.
	edgeSet := make(map[[2]string]struct{})
	for _, r := range resources {
		for _, dep := range r.Dependencies {
			if _, ok := bySymbol[dep]; !ok {
				continue
			}
			// Drop VM → consumed-NIC edges (NIC is drawn inside the VM card).
			if nicConsumed[dep] {
				continue
			}
			// Drop PIP → consumed-PIP edges (PIP is drawn inside the parent card).
			if pipConsumed[dep] {
				continue
			}
			// Drop PE → linked-service edges (shown as visual pair in subnet).
			if peLinkedConsumed[dep] {
				continue
			}
			// Drop BastionDev → VNet edge (connection rendered as dedicated solid line).
			if r.BastionDevVNetSymbol != "" && dep == r.BastionDevVNetSymbol {
				continue
			}
			key := [2]string{r.SymbolicName, dep}
			edgeSet[key] = struct{}{}
		}
	}
	edges := make([]*model.Edge, 0, len(edgeSet))
	for k := range edgeSet {
		edges = append(edges, &model.Edge{From: k[0], To: k[1]})
	}
	// Sort edges for deterministic SVG output.
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	// Build VNet peering relationships.
	// A peering resource has SubnetRef.SubnetName == "__peering__" and its
	// SymbolicName is of the form "<localVNetSym>/<peeringName>" or simply
	// depends on the local VNet symbol. We detect the local VNet by finding
	// which VNet the peering resource was declared as a child of (first
	// dependency that is a VNet).
	var peerings []*model.VNetPeering
	peeringAdded := make(map[[2]string]bool)
	for _, r := range resources {
		if r.SubnetRef == nil || r.SubnetRef.SubnetName != "__peering__" {
			continue
		}
		assigned[r.SymbolicName] = true
		remoteSym := r.SubnetRef.VNetSymbol
		// Find the local VNet: first VNet dependency that is NOT the remote VNet.
		// (Reverse peering entries list the remote VNet first in dependsOn, so we
		// must skip it to avoid creating a self-peering.)
		var localVC *model.VNetContainer
		for _, dep := range r.Dependencies {
			if dep == remoteSym {
				continue
			}
			if vc := vnetContainerBySymbol(vnetContainers, dep); vc != nil {
				localVC = vc
				break
			}
		}
		remoteVC := vnetContainerBySymbol(vnetContainers, remoteSym)
		if localVC == nil || remoteVC == nil {
			continue
		}
		// Add only one entry per pair (deduplicate hub↔spoke duplicates).
		a, b := localVC.Resource.SymbolicName, remoteVC.Resource.SymbolicName
		if a > b {
			a, b = b, a
		}
		key := [2]string{a, b}
		if !peeringAdded[key] {
			peeringAdded[key] = true
			peerings = append(peerings, &model.VNetPeering{Local: localVC, Remote: remoteVC})
		}
	}

	// Build Private DNS Zone ↔ VNet links.
	// A virtualNetworkLinks resource is converted into a DNSLinkConnection
	// between its parent (Private DNS Zone) and the target VNet container.
	// The link resource itself is not rendered as a card.
	var dnsLinks []*model.DNSLinkConnection
	dnsLinkAdded := make(map[[2]string]bool)
	for _, r := range resources {
		if r.Type != "microsoft.network/privatednszones/virtualnetworklinks" {
			continue
		}
		assigned[r.SymbolicName] = true
		zone, okZ := bySymbol[r.ParentSymbol]
		if !okZ || zone == nil {
			continue
		}
		if r.LinkedServiceSymbol == "" {
			continue
		}
		vc := vnetContainerBySymbol(vnetContainers, r.LinkedServiceSymbol)
		if vc == nil {
			continue
		}
		key := [2]string{zone.SymbolicName, vc.Resource.SymbolicName}
		if dnsLinkAdded[key] {
			continue
		}
		dnsLinkAdded[key] = true
		dnsLinks = append(dnsLinks, &model.DNSLinkConnection{Zone: zone, VNet: vc})
		// Place the zone below its primary VNet (first association wins).
		// Subsequent links to other VNets are still drawn as connector lines.
		if !assigned[zone.SymbolicName] {
			assigned[zone.SymbolicName] = true
			vc.DNSZones = append(vc.DNSZones, zone)
		}
	}

	// Place Azure Bastion Developer SKU resources below their referenced VNet.
	for _, r := range resources {
		if r.BastionDevVNetSymbol == "" || assigned[r.SymbolicName] {
			continue
		}
		vc := vnetContainerBySymbol(vnetContainers, r.BastionDevVNetSymbol)
		if vc == nil {
			continue
		}
		vc.BastionDev = r
		assigned[r.SymbolicName] = true
	}

	// All remaining non-VNet resources are standalone.
	var standalone []*model.Resource
	for _, r := range resources {
		if !assigned[r.SymbolicName] {
			standalone = append(standalone, r)
		}
	}

	// Absorb child resources (those with parent: declaration) into their
	// parent's ChildResources list. The parent may be a standalone resource
	// or a LinkedService rendered outside a VNet (e.g. sqlDb → sqlServer).
	parentableSet := make(map[string]bool, len(standalone))
	for _, r := range standalone {
		parentableSet[r.SymbolicName] = true
	}
	// Also treat LinkedService resources as valid parents.
	for _, vc := range vnetContainers {
		for _, sc := range vc.Subnets {
			for _, pair := range sc.PEPairs {
				if pair.LinkedService != nil {
					parentableSet[pair.LinkedService.SymbolicName] = true
				}
			}
		}
	}
	childAbsorbed := make(map[string]bool)
	for _, r := range standalone {
		if r.ParentSymbol == "" || !parentableSet[r.ParentSymbol] {
			continue
		}
		parent := bySymbol[r.ParentSymbol]
		parent.ChildResources = append(parent.ChildResources, r)
		childAbsorbed[r.SymbolicName] = true
	}
	if len(childAbsorbed) > 0 {
		filtered := standalone[:0]
		for _, r := range standalone {
			if !childAbsorbed[r.SymbolicName] {
				filtered = append(filtered, r)
			}
		}
		standalone = filtered
		// Drop child→parent edges (relationship is shown as a visual connector).
		filteredEdges := edges[:0]
		for _, e := range edges {
			if childAbsorbed[e.From] {
				continue
			}
			filteredEdges = append(filteredEdges, e)
		}
		edges = filteredEdges
	}

	// Build LB connections: for each VM with attached NICs, collect LB associations.
	var lbConnections []*model.LBConnection
	lbAdded := make(map[[2]string]bool)
	for _, r := range resources {
		if !isCompute(r.Type) {
			continue
		}
		lbSyms := make(map[string]bool)
		for _, nic := range r.AttachedNICs {
			for _, lbSym := range nic.LBSymbols {
				lbSyms[lbSym] = true
			}
		}
		for lbSym := range lbSyms {
			lb, ok := bySymbol[lbSym]
			if !ok {
				continue
			}
			key := [2]string{r.SymbolicName, lbSym}
			if !lbAdded[key] {
				lbAdded[key] = true
				lbConnections = append(lbConnections, &model.LBConnection{From: r, To: lb})
			}
		}
	}
	// Also handle NICs not consumed by a VM.
	for _, r := range resources {
		if !isNIC(r.Type) || nicConsumed[r.SymbolicName] {
			continue
		}
		for _, lbSym := range r.LBSymbols {
			lb, ok := bySymbol[lbSym]
			if !ok {
				continue
			}
			key := [2]string{r.SymbolicName, lbSym}
			if !lbAdded[key] {
				lbAdded[key] = true
				lbConnections = append(lbConnections, &model.LBConnection{From: r, To: lb})
			}
		}
	}

	// Extract anchored LBs from subnets and standalone.
	// Any LB that appears in LBConnections is repositioned to the left of its VMs.
	lbAnchoredMap := make(map[string]*model.Resource)
	for _, c := range lbConnections {
		lbAnchoredMap[c.To.SymbolicName] = c.To
	}
	// Attach LBs directly onto the subnet containing their VMs.
	// Build: lbSym → first subnet that contains a connected VM.
	lbToSubnet := make(map[string]*model.SubnetContainer)
	for _, c := range lbConnections {
		vmSym := c.From.SymbolicName
		lbSym := c.To.SymbolicName
		if _, already := lbToSubnet[lbSym]; already {
			continue
		}
		for _, vc := range vnetContainers {
			for _, sc := range vc.Subnets {
				for _, r := range sc.Resources {
					if r.SymbolicName == vmSym {
						lbToSubnet[lbSym] = sc
					}
				}
			}
		}
	}
	for lbSym, sc := range lbToSubnet {
		if lb, ok := bySymbol[lbSym]; ok {
			sc.AnchoredLB = lb
		}
	}
	// Remove LBs from subnet Resources.
	for _, vc := range vnetContainers {
		for _, sc := range vc.Subnets {
			var remaining []*model.Resource
			for _, r := range sc.Resources {
				if lbAnchoredMap[r.SymbolicName] == nil {
					remaining = append(remaining, r)
				}
			}
			sc.Resources = remaining
		}
	}
	// Remove from standalone; collect into anchoredLBs.
	var anchoredLBs []*model.Resource
	var newStandalone []*model.Resource
	for _, r := range standalone {
		if lbAnchoredMap[r.SymbolicName] != nil {
			anchoredLBs = append(anchoredLBs, r)
		} else {
			newStandalone = append(newStandalone, r)
		}
	}
	standalone = newStandalone
	// Any anchored LB that was in a subnet (not in standalone) also needs to be listed.
	for sym, lb := range lbAnchoredMap {
		found := false
		for _, r := range anchoredLBs {
			if r.SymbolicName == sym {
				found = true
				break
			}
		}
		if !found {
			anchoredLBs = append(anchoredLBs, lb)
		}
	}

	// Build VNet integration connections: App Service → subnet it is VNet-integrated into.
	var vnetIntConns []*model.VNetIntConnection
	// Build a lookup: (vnetSymbol, subnetName) → SubnetContainer
	type subnetKey struct{ vnet, subnet string }
	subnetByKey := make(map[subnetKey]*model.SubnetContainer)
	for _, vc := range vnetContainers {
		for _, sc := range vc.Subnets {
			subnetByKey[subnetKey{vc.Resource.SymbolicName, sc.Name}] = sc
		}
	}
	// Build a VNet-symbol → VNetContainer lookup for pairing logic below.
	vnetBySymbol := make(map[string]*model.VNetContainer, len(vnetContainers))
	for _, vc := range vnetContainers {
		vnetBySymbol[vc.Resource.SymbolicName] = vc
	}

	for _, r := range resources {
		if r.VNetIntSubnet == nil {
			continue
		}
		key := subnetKey{r.VNetIntSubnet.VNetSymbol, r.VNetIntSubnet.SubnetName}
		vnetIntSC, ok := subnetByKey[key]
		if !ok {
			continue
		}
		vnetIntSC.IsVNetInt = true
		vnetIntConns = append(vnetIntConns, &model.VNetIntConnection{App: r, Subnet: vnetIntSC})

		// Find the PE subnet in the same VNet whose linked service is this App Service
		// (directly or as a child of its parent plan) and record the pairing.
		if vc, ok2 := vnetBySymbol[r.VNetIntSubnet.VNetSymbol]; ok2 {
			for _, sc := range vc.Subnets {
				for _, pair := range sc.PEPairs {
					ls := pair.LinkedService
					if ls == nil {
						continue
					}
					matched := ls.SymbolicName == r.SymbolicName
					if !matched {
						for _, child := range ls.ChildResources {
							if child.SymbolicName == r.SymbolicName {
								matched = true
								break
							}
						}
					}
					if matched {
						sc.LinkedVNetIntSubnetName = vnetIntSC.Name
						break
					}
				}
			}
		}
	}

	return &model.Diagram{
		Resources:           resources,
		ResourcesBySymbol:   bySymbol,
		Edges:               edges,
		VNets:               vnetContainers,
		VNetPeerings:        peerings,
		StandaloneResources: standalone,
		LBConnections:       lbConnections,
		AnchoredLBs:         anchoredLBs,
		VNetIntConnections:  vnetIntConns,
		DNSLinks:            dnsLinks,
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

func isVNet(t string) bool {
	return strings.Contains(t, "microsoft.network/virtualnetworks") &&
		!strings.Contains(t, "/subnets") &&
		!strings.Contains(t, "/virtualnetworkpeerings")
}

func isNIC(t string) bool {
	return strings.Contains(t, "microsoft.network/networkinterfaces")
}

func isPIP(t string) bool {
	return strings.Contains(t, "microsoft.network/publicipaddresses")
}

// isPIPConsumer returns true for resource types that directly reference a PIP
// (Firewall, VPN Gateway, Bastion) and should display it inline.
func isPIPConsumer(t string) bool {
	return strings.Contains(t, "microsoft.network/azurefirewalls") ||
		strings.Contains(t, "microsoft.network/virtualnetworkgateways") ||
		strings.Contains(t, "microsoft.network/bastionhosts")
}

func isPE(t string) bool {
	return strings.Contains(t, "microsoft.network/privateendpoints")
}

func isCompute(t string) bool {
	return strings.Contains(t, "microsoft.compute/virtualmachines") ||
		strings.Contains(t, "microsoft.compute/virtualmachinescalesets") ||
		strings.Contains(t, "microsoft.web/sites") ||
		strings.Contains(t, "microsoft.app/containerapps") ||
		strings.Contains(t, "microsoft.containerservice/managedclusters")
}

func vnetContainerBySymbol(containers []*model.VNetContainer, sym string) *model.VNetContainer {
	for _, vc := range containers {
		if vc.Resource.SymbolicName == sym {
			return vc
		}
	}
	return nil
}
