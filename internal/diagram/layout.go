package diagram

import (
	"math"
	"sort"

	"github.com/kongou-ae/AzDiagram/internal/model"
)

// ── Dimension constants (pixels) ─────────────────────────────────────────────

const (
	// Resource card dimensions.
	cardW   = 140.0
	cardH   = 60.0
	nicRowH = 22.0 // extra height added to a card that has attached NIC(s)
	nsgBarH = 22.0 // extra height appended to a subnet that has an attached NSG

	// Spacing between cards within a container.
	cardGapX = 10.0
	cardGapY = 10.0

	// Maximum cards per row inside a subnet.
	maxCardsPerRow = 3

	// Subnet padding (inside VNet).
	subnetPadTop    = 30.0 // space for the subnet label
	subnetPadSide   = 8.0
	subnetPadBottom = 8.0
	subnetGapX      = 12.0 // horizontal gap between subnets
	subnetGapY      = 12.0 // vertical gap between subnets (kept for reference)

	// VNet padding.
	vnetPadTop    = 30.0 // space for VNet label (18px label + 12px gap before first subnet)
	vnetPadSide   = 10.0
	vnetPadBottom = 10.0

	// Standalone resource grid padding.
	gridPadTop  = 20.0
	gridPadSide = 20.0
	gridGapX    = 20.0
	gridGapY    = 20.0
	maxGridCols = 3

	// Gap between the VNet section and the standalone grid.
	sectionGapX = 40.0

	// Standalone child resources (parent: relationship): gap between child and parent.
	standaloneChildGapX = 20.0
	standaloneChildGapY = 8.0

	// Gap between VNet boxes.
	vnetGapX = 40.0
	vnetGapY = 20.0 // vertical gap between stacked spoke VNets

	// Gap between VNet bottom and DNS Zone cards below it.
	dnsZoneGapY = 20.0

	// Gap between the last item below a VNet (DNS zones or VNet bottom) and
	// an Azure Bastion Developer card placed below the VNet.
	bastionDevGapY = 20.0

	// Resource-group / subscription padding.
	rgPadTop    = 24.0 // space for label
	rgPadSide   = 10.0
	rgPadBottom = 10.0

	subPadTop    = 16.0
	subPadSide   = 30.0
	subPadBottom = 30.0
)

// Layout calculates positions for every element in the diagram and sets
// diagram.Width / diagram.Height.
func Layout(d *model.Diagram) {
	// ── 0. Count peerings per VNet and sort hub-first ─────────────────────
	peeringCount := make(map[string]int)
	for _, p := range d.VNetPeerings {
		peeringCount[p.Local.Resource.SymbolicName]++
		peeringCount[p.Remote.Resource.SymbolicName]++
	}
	sort.SliceStable(d.VNets, func(i, j int) bool {
		return peeringCount[d.VNets[i].Resource.SymbolicName] >
			peeringCount[d.VNets[j].Resource.SymbolicName]
	})

	// ── 1. Identify hub and its direct spoke symbols ─────────────────────
	// Primary:  the VNet with the most peerings (peeringCount > 1).
	// Fallback: if no VNet has more than one peering, pick the first VNet
	//           that contains AzureFirewallSubnet or GatewaySubnet.
	var hubVC *model.VNetContainer
	if len(d.VNets) > 0 && peeringCount[d.VNets[0].Resource.SymbolicName] > 1 {
		hubVC = d.VNets[0]
	}
	if hubVC == nil {
		for _, vc := range d.VNets {
			for _, sc := range vc.Subnets {
				if sc.Name == "AzureFirewallSubnet" || sc.Name == "GatewaySubnet" {
					hubVC = vc
					break
				}
			}
			if hubVC != nil {
				break
			}
		}
	}
	spokeSymbols := make(map[string]bool)
	if hubVC != nil {
		hubSym := hubVC.Resource.SymbolicName
		for _, p := range d.VNetPeerings {
			if p.Local.Resource.SymbolicName == hubSym {
				spokeSymbols[p.Remote.Resource.SymbolicName] = true
			} else if p.Remote.Resource.SymbolicName == hubSym {
				spokeSymbols[p.Local.Resource.SymbolicName] = true
			}
		}
	}

	// ── 2. Partition VNets into hub / spoke / other ───────────────────────
	var spokeVCs, otherVCs []*model.VNetContainer
	for _, vc := range d.VNets {
		if vc == hubVC {
			continue
		}
		if spokeSymbols[vc.Resource.SymbolicName] {
			spokeVCs = append(spokeVCs, vc)
		} else {
			otherVCs = append(otherVCs, vc)
		}
	}

	// ── 3. Layout hub at origin (vertical: subnets stacked to keep hub compact) ─
	if hubVC != nil {
		layoutVNet(hubVC, 0, 0, true)
	}

	// ── 4. Layout spokes stacked vertically to the right of hub ──────────
	spokeStartX := 0.0
	if hubVC != nil {
		spokeStartX = hubVC.Width + vnetGapX
	}
	curSpokeY := 0.0
	var maxSpokeW float64
	for _, vc := range spokeVCs {
		layoutVNet(vc, spokeStartX, curSpokeY, false)
		// Include DNS Zone card height so the next spoke doesn't overlap.
		curSpokeY += vc.Height + dnsZonesBlockHeight(vc) + bastionDevBlockHeight(vc) + vnetGapY
		maxSpokeW = math.Max(maxSpokeW, vc.Width)
	}

	// ── 5. Layout remaining VNets horizontally after spoke column ─────────
	curOtherX := spokeStartX + maxSpokeW
	if len(otherVCs) > 0 && (hubVC != nil || len(spokeVCs) > 0) {
		curOtherX += vnetGapX
	}
	for _, vc := range otherVCs {
		layoutVNet(vc, curOtherX, 0, false)
		curOtherX += vc.Width + vnetGapX
	}

	// ── 5.5. Position DNS Zone cards below their associated VNets ─────────
	// Each DNS zone is placed below its VNet, wrapping into multiple rows so
	// the total width never exceeds the VNet width.  Each row is centred under
	// the VNet independently.
	for _, vc := range d.VNets {
		if len(vc.DNSZones) == 0 {
			continue
		}
		// How many cards fit in one row without exceeding the VNet width?
		maxPerRow := int((vc.Width + cardGapX) / (cardW + cardGapX))
		if maxPerRow < 1 {
			maxPerRow = 1
		}
		zoneBaseY := vc.Y + vc.Height + dnsZoneGapY
		for i, z := range vc.DNSZones {
			row := i / maxPerRow
			col := i % maxPerRow
			// Count zones in this specific row to centre it.
			rowStart := row * maxPerRow
			rowEnd := rowStart + maxPerRow
			if rowEnd > len(vc.DNSZones) {
				rowEnd = len(vc.DNSZones)
			}
			rowCount := rowEnd - rowStart
			rowW := float64(rowCount)*cardW + float64(rowCount-1)*cardGapX
			rowStartX := vc.X + vc.Width/2 - rowW/2
			z.X = rowStartX + float64(col)*(cardW+cardGapX)
			z.Y = zoneBaseY + float64(row)*(cardH+dnsZoneGapY)
			z.Width = cardW
			z.Height = cardH
		}
	}

	// ── 5.6. Position Bastion Developer card below its VNet (below DNS zones if any) ────
	for _, vc := range d.VNets {
		if vc.BastionDev == nil {
			continue
		}
		baseY := vc.Y + vc.Height + dnsZonesBlockHeight(vc) + bastionDevGapY
		vc.BastionDev.X = vc.X + vc.Width/2 - cardW/2
		vc.BastionDev.Y = baseY
		vc.BastionDev.Width = cardW
		vc.BastionDev.Height = cardH
	}

	// ── 6. Compute total VNet width for standalone grid placement ─────────
	vnetTotalW := 0.0
	for _, vc := range d.VNets {
		vnetTotalW = math.Max(vnetTotalW, vc.X+vc.Width)
	}

	// ── 6.5. Position linked services (PE targets) outside their VNet ────
	// Each linked service is placed at the VNet's right edge + gap,
	// at the same Y as its corresponding PE card.
	// In horizontal (spoke) VNets multiple subnets may share the same PE Y;
	// we track the last placed Y per VNet and push down to avoid overlaps.
	// Child resources (e.g. sqlDb with parent: sqlServer) are placed to the
	// right of the linked service with standaloneChildGapX spacing.
	const externalServiceGap = 40.0
	linkedServiceMaxX := vnetTotalW
	for _, vc := range d.VNets {
		lastLinkedServiceBottomY := math.Inf(-1)
		for _, sc := range vc.Subnets {
			for _, pair := range sc.PEPairs {
				if pair.LinkedService != nil {
					targetY := pair.PE.Y
					if targetY < lastLinkedServiceBottomY+standaloneChildGapY {
						targetY = lastLinkedServiceBottomY + standaloneChildGapY
					}
					pair.LinkedService.X = vc.X + vc.Width + externalServiceGap
					pair.LinkedService.Y = targetY
					pair.LinkedService.Width = cardW
					pair.LinkedService.Height = cardH
					lastLinkedServiceBottomY = targetY + cardH
					childX := pair.LinkedService.X + cardW + standaloneChildGapX
					for j, child := range pair.LinkedService.ChildResources {
						child.X = childX
						child.Y = pair.LinkedService.Y + float64(j)*(cardH+standaloneChildGapY)
						child.Width = cardW
						child.Height = cardH
					}
					childEndX := pair.LinkedService.X + cardW
					if len(pair.LinkedService.ChildResources) > 0 {
						childEndX = childX + cardW
						lastLinkedServiceBottomY = math.Max(lastLinkedServiceBottomY,
							pair.LinkedService.Y+float64(len(pair.LinkedService.ChildResources))*(cardH+standaloneChildGapY))
					}
					linkedServiceMaxX = math.Max(linkedServiceMaxX, childEndX)
				}
			}
		}
	}

	// ── 7. Layout standalone resource grid ───────────────────────────────
	// Start after the rightmost linked service (or VNet if none).
	standaloneX := 0.0
	if linkedServiceMaxX > 0 {
		standaloneX = linkedServiceMaxX + sectionGapX
	}
	gridH := layoutGrid(d.StandaloneResources, standaloneX, 0, maxGridCols)

	// Position child resources to the right of their parent.
	for _, r := range d.StandaloneResources {
		for j, child := range r.ChildResources {
			child.X = r.X + r.Width + standaloneChildGapX
			child.Y = r.Y + float64(j)*(cardH+standaloneChildGapY)
			child.Width = cardW
			child.Height = cardH
		}
	}
	_ = gridH

	// ── 8. Compute inner content bounding box ─────────────────────────────
	innerW, innerH := contentBounds(d, vnetTotalW, standaloneX)

	// ── 9. Add a small margin around the whole diagram ───────────────────
	const diagramMargin = 20.0
	applyOffset(d, diagramMargin, diagramMargin)
	d.Width = innerW + 2*diagramMargin
	d.Height = innerH + 2*diagramMargin
}

// bastionDevBlockHeight returns the vertical space consumed by a Bastion Developer
// card placed below a VNet (including the gap). Returns 0 when vc has no BastionDev.
func bastionDevBlockHeight(vc *model.VNetContainer) float64 {
	if vc.BastionDev == nil {
		return 0
	}
	return bastionDevGapY + cardH
}

// dnsZonesBlockHeight returns the total vertical space consumed by DNS Zone cards
// placed below vc (including the initial gap from the VNet bottom).
// Returns 0 when vc has no DNS zones.  vc.Width must already be set.
func dnsZonesBlockHeight(vc *model.VNetContainer) float64 {
	if len(vc.DNSZones) == 0 {
		return 0
	}
	maxPerRow := int((vc.Width + cardGapX) / (cardW + cardGapX))
	if maxPerRow < 1 {
		maxPerRow = 1
	}
	rows := (len(vc.DNSZones) + maxPerRow - 1) / maxPerRow
	// dnsZoneGapY (gap before first row) + rows×cardH + (rows-1)×dnsZoneGapY (inter-row gaps)
	return float64(rows) * (cardH + dnsZoneGapY)
}

// layoutVNet positions all subnets (and their resources) within a VNet container.
// When vertical is true, subnets are stacked vertically and equalised to the same
// width (hub style). When vertical is false, subnets are arranged horizontally and
// each subnet keeps its natural width so PEs stay at the right edge (spoke style).
// All coordinates produced are absolute (relative to diagram origin, not the VNet).
func layoutVNet(vc *model.VNetContainer, startX, startY float64, vertical bool) {
	var innerW, innerH float64

	if vertical {
		curY := startY + vnetPadTop
		var maxSubnetW float64
		for _, sc := range vc.Subnets {
			layoutSubnet(sc, startX+vnetPadSide, curY)
			curY += sc.Height + subnetGapY
			if !isFixedWidthSubnet(sc.Name) {
				maxSubnetW = math.Max(maxSubnetW, sc.Width)
			}
		}
		innerW = maxSubnetW + 2*vnetPadSide
		innerH = (curY - subnetGapY) - startY + vnetPadBottom
		// Equalise widths and reposition PEs at the new uniform right edge.
		// Fixed-width subnets (Azure platform subnets) keep their natural width.
		for _, sc := range vc.Subnets {
			if !isFixedWidthSubnet(sc.Name) {
				sc.Width = maxSubnetW
			}
			repositionSubnetPEs(sc)
		}
	} else {
		// Separate subnets into non-PE (horizontal left) and PE/VNetInt (vertical right).
		var nonPESubs, peSubs []*model.SubnetContainer
		for _, sc := range vc.Subnets {
			if len(sc.PEPairs) > 0 || sc.IsVNetInt {
				peSubs = append(peSubs, sc)
			} else {
				nonPESubs = append(nonPESubs, sc)
			}
		}

		// Layout non-PE subnets horizontally.
		curX := startX + vnetPadSide
		var nonPEMaxH float64
		for _, sc := range nonPESubs {
			layoutSubnet(sc, curX, startY+vnetPadTop)
			curX += sc.Width + subnetGapX
			nonPEMaxH = math.Max(nonPEMaxH, sc.Height)
		}
		// X where PE column starts (right of non-PE subnets).
		peStartX := curX
		if len(nonPESubs) == 0 {
			peStartX = startX + vnetPadSide
		}

		// Reorder peSubs so that each PE subnet is immediately followed by its paired
		// VNet integration subnet (LinkedVNetIntSubnetName), regardless of Bicep order.
		// Algorithm: process PE subnets (those with PEPairs) first in their original
		// relative order, inserting their paired VNetInt subnet right after each one.
		// VNetInt-only subnets without a PE pair are appended at the end.
		{
			// Build a lookup from subnet name to SubnetContainer.
			subnetByName := make(map[string]*model.SubnetContainer, len(peSubs))
			for _, sc := range peSubs {
				subnetByName[sc.Name] = sc
			}
			placed := make(map[string]bool, len(peSubs))
			ordered := make([]*model.SubnetContainer, 0, len(peSubs))
			// First pass: process PE subnets (have PEPairs) in their original relative order.
			for _, sc := range peSubs {
				if len(sc.PEPairs) == 0 {
					continue // skip VNetInt-only subnets; handled below
				}
				if placed[sc.Name] {
					continue
				}
				ordered = append(ordered, sc)
				placed[sc.Name] = true
				// Insert paired VNetInt subnet immediately after.
				if sc.LinkedVNetIntSubnetName != "" {
					if sc2, ok := subnetByName[sc.LinkedVNetIntSubnetName]; ok && !placed[sc2.Name] {
						ordered = append(ordered, sc2)
						placed[sc2.Name] = true
					}
				}
			}
			// Second pass: append any remaining subnets (VNetInt-only without a PE pair, or unmatched PE subnets).
			for _, sc := range peSubs {
				if !placed[sc.Name] {
					ordered = append(ordered, sc)
				}
			}
			peSubs = ordered
		}

		// Layout PE subnets vertically (stacked).
		curY := startY + vnetPadTop
		var peMaxW float64
		for _, sc := range peSubs {
			layoutSubnet(sc, peStartX, curY)
			curY += sc.Height + subnetGapY
			peMaxW = math.Max(peMaxW, sc.Width)
		}
		// Equalise PE subnet widths so PEs align, and reposition PE cards.
		for _, sc := range peSubs {
			sc.Width = peMaxW
			repositionSubnetPEs(sc)
		}

		var totalW, totalH float64
		if len(peSubs) > 0 {
			totalW = (peStartX - startX) + peMaxW + vnetPadSide
			peColH := (curY - subnetGapY) - (startY + vnetPadTop)
			totalH = math.Max(nonPEMaxH, peColH) + vnetPadTop + vnetPadBottom
		} else {
			if len(nonPESubs) > 0 {
				totalW = (curX - subnetGapX) - startX + vnetPadSide
			} else {
				totalW = cardW + 2*vnetPadSide
			}
			totalH = nonPEMaxH + vnetPadTop + vnetPadBottom
		}
		if totalH < cardH+vnetPadTop+vnetPadBottom {
			totalH = cardH + vnetPadTop + vnetPadBottom
		}
		innerW = totalW
		innerH = totalH
	}

	vc.X, vc.Y = startX, startY
	vc.Width = innerW
	vc.Height = innerH

	// Position the VNet resource card itself (centred at the top label area).
	vc.Resource.X = startX + innerW/2 - cardW/2
	vc.Resource.Y = startY
	vc.Resource.Width = cardW
	vc.Resource.Height = cardH
}

// effectiveCardH returns the rendered height of a resource card.
// Cards with attached NICs or PIPs get extra height for the inline row.
func effectiveCardH(r *model.Resource) float64 {
	h := cardH
	if len(r.AttachedNICs) > 0 {
		h += nicRowH
	}
	if len(r.AttachedPIPs) > 0 {
		h += nicRowH // same row height constant reused for PIP row
	}
	h += float64(len(r.AttachedNicNSGs)) * nsgBarH
	return h
}

// layoutSubnet positions resources within a subnet box.
// Regular resources occupy the left side; PE cards sit on the right edge.
// Linked services (PE targets) are positioned outside the VNet by layoutVNet.
func layoutSubnet(sc *model.SubnetContainer, startX, startY float64) {
	n := len(sc.Resources)
	nPairs := len(sc.PEPairs)

	// Regular resource grid dimensions.
	rowH := cardH
	for _, r := range sc.Resources {
		if h := effectiveCardH(r); h > rowH {
			rowH = h
		}
	}

	hasLB := sc.AnchoredLB != nil
	var regularContentW, regularContentH float64
	if n > 0 {
		cols := n
		if cols > maxCardsPerRow {
			cols = maxCardsPerRow
		}
		if hasLB {
			cols = 1
		}
		rows := (n + cols - 1) / cols
		regularContentW = float64(cols)*cardW + float64(cols-1)*cardGapX
		regularContentH = float64(rows)*rowH + float64(rows-1)*cardGapY
	}
	// When an LB is anchored, prepend a LB column to the left.
	if hasLB {
		regularContentW += cardW + cardGapX
	}

	// PE column height (PEs stacked vertically on the right edge).
	peColH := 0.0
	if nPairs > 0 {
		peColH = float64(nPairs)*cardH + float64(nPairs-1)*cardGapY
	}

	// Overall content width: regular content + gap + PE column (when both present).
	var contentW float64
	switch {
	case nPairs > 0 && n > 0:
		contentW = regularContentW + cardGapX + cardW
	case nPairs > 0:
		contentW = cardW
	default:
		contentW = regularContentW
	}
	if contentW == 0 {
		contentW = cardW // minimum
	}

	// Height is the maximum of regular grid and PE column.
	contentH := regularContentH
	if peColH > contentH {
		contentH = peColH
	}
	if contentH == 0 {
		contentH = cardH // minimum
	}

	sc.Width = contentW + 2*subnetPadSide
	sc.Height = contentH + subnetPadTop + subnetPadBottom
	if sc.AttachedNSG != nil {
		sc.Height += nsgBarH
	}
	if sc.AttachedRT != nil {
		sc.Height += nsgBarH
	}
	// VNet Integration subnets are dedicated delegation subnets; render at half height.
	if sc.IsVNetInt {
		sc.Height /= 2
	}

	// Position PE cards on the RIGHT edge of the subnet.
	// (After layoutVNet equalises widths, repositionSubnetPEs will update these.)
	peX := startX + sc.Width - subnetPadSide - cardW
	pairY := startY + subnetPadTop
	for _, pair := range sc.PEPairs {
		pair.PE.X = peX
		pair.PE.Y = pairY
		pair.PE.Width = cardW
		pair.PE.Height = cardH
		// LinkedService is positioned outside the VNet — see Layout().
		pairY += cardH + cardGapY
	}

	// Position resources. When an LB is anchored, resources are in a single
	// column to the RIGHT of the LB column.
	if n > 0 {
		cols := 1
		if !hasLB {
			cols = n
			if cols > maxCardsPerRow {
				cols = maxCardsPerRow
			}
		}
		lbColOffset := 0.0
		if hasLB {
			lbColOffset = cardW + cardGapX
		}
		for i, r := range sc.Resources {
			col := i % cols
			row := i / cols
			r.X = startX + subnetPadSide + lbColOffset + float64(col)*(cardW+cardGapX)
			r.Y = startY + subnetPadTop + float64(row)*(rowH+cardGapY)
			r.Width = cardW
			r.Height = effectiveCardH(r)
		}
	}
	// Position anchored LB: left column.
	// - Odd VM count:  align LB centre with the middle VM's centre.
	// - Even VM count: align LB centre with the midpoint of the whole VM column.
	if hasLB {
		lb := sc.AnchoredLB
		lb.X = startX + subnetPadSide
		lb.Width = cardW
		lb.Height = cardH

		if n%2 == 1 {
			// Odd: centre on the middle VM (index n/2).
			midVM := sc.Resources[n/2]
			lb.Y = midVM.Y + effectiveCardH(midVM)/2 - cardH/2
		} else {
			// Even: centre on the midpoint of the total VM column height.
			vmColH := 0.0
			if n > 0 {
				for _, r := range sc.Resources {
					vmColH += effectiveCardH(r)
				}
				vmColH += float64(n-1) * cardGapY
			}
			if vmColH == 0 {
				vmColH = cardH
			}
			contentStartY := startY + subnetPadTop
			lb.Y = contentStartY + vmColH/2 - cardH/2
		}
	}

	sc.X, sc.Y = startX, startY
}

// repositionSubnetPEs re-places PE cards on the right edge of sc after the
// subnet width has been equalised by layoutVNet.
func repositionSubnetPEs(sc *model.SubnetContainer) {
	if len(sc.PEPairs) == 0 {
		return
	}
	peX := sc.X + sc.Width - subnetPadSide - cardW
	pairY := sc.Y + subnetPadTop
	for _, pair := range sc.PEPairs {
		pair.PE.X = peX
		pair.PE.Y = pairY
		pairY += cardH + cardGapY
	}
}

// isFixedWidthSubnet returns true for Azure platform-reserved subnet names that
// should keep their natural (content-driven) width and not be stretched to match
// the widest subnet in the VNet.
func isFixedWidthSubnet(name string) bool {
	switch name {
	case "AzureFirewallSubnet", "AzureBastionSubnet", "GatewaySubnet":
		return true
	}
	return false
}

// layoutGrid arranges a flat slice of resources in a grid starting at (ox, oy).
// Returns the total height consumed.
func layoutGrid(resources []*model.Resource, ox, oy float64, cols int) float64 {
	if len(resources) == 0 {
		return 0
	}
	for i, r := range resources {
		col := i % cols
		row := i / cols
		r.X = ox + gridPadSide + float64(col)*(cardW+gridGapX)
		r.Y = oy + gridPadTop + float64(row)*(cardH+gridGapY)
		r.Width = cardW
		r.Height = cardH
	}
	rows := (len(resources) + cols - 1) / cols
	return gridPadTop + float64(rows)*cardH + float64(rows-1)*gridGapY
}

// contentBounds computes the bounding box of all positioned elements
// (before the resource-group / subscription padding is added).
func contentBounds(d *model.Diagram, vnetTotalW, standaloneStartX float64) (w, h float64) {
	maxX, maxY := 0.0, 0.0

	for _, vc := range d.VNets {
		maxX = math.Max(maxX, vc.X+vc.Width)
		maxY = math.Max(maxY, vc.Y+vc.Height)
		// Include DNS zones placed below the VNet.
		for _, z := range vc.DNSZones {
			maxX = math.Max(maxX, z.X+z.Width)
			maxY = math.Max(maxY, z.Y+z.Height)
		}
		// Include Bastion Developer card placed below the VNet.
		if vc.BastionDev != nil {
			maxX = math.Max(maxX, vc.BastionDev.X+vc.BastionDev.Width)
			maxY = math.Max(maxY, vc.BastionDev.Y+vc.BastionDev.Height)
		}
		// Include linked services positioned outside the VNet.
		for _, sc := range vc.Subnets {
			for _, pair := range sc.PEPairs {
				if pair.LinkedService != nil {
					maxX = math.Max(maxX, pair.LinkedService.X+pair.LinkedService.Width)
					maxY = math.Max(maxY, pair.LinkedService.Y+pair.LinkedService.Height)
					for _, child := range pair.LinkedService.ChildResources {
						maxX = math.Max(maxX, child.X+child.Width)
						maxY = math.Max(maxY, child.Y+child.Height)
					}
				}
			}
		}
	}
	for _, r := range d.StandaloneResources {
		maxX = math.Max(maxX, r.X+r.Width)
		maxY = math.Max(maxY, r.Y+r.Height)
		for _, child := range r.ChildResources {
			maxX = math.Max(maxX, child.X+child.Width)
			maxY = math.Max(maxY, child.Y+child.Height)
		}
	}
	// AnchoredLBs are now inside subnets; their bounds are covered by VNet bounds.

	// Ensure a minimum canvas size.
	if maxX < 400 {
		maxX = 400
	}
	if maxY < 200 {
		maxY = 200
	}
	return maxX, maxY
}

// applyOffset shifts every positioned element by (dx, dy).
func applyOffset(d *model.Diagram, dx, dy float64) {
	for _, vc := range d.VNets {
		vc.X += dx
		vc.Y += dy
		vc.Resource.X += dx
		vc.Resource.Y += dy
		for _, z := range vc.DNSZones {
			z.X += dx
			z.Y += dy
		}
		if vc.BastionDev != nil {
			vc.BastionDev.X += dx
			vc.BastionDev.Y += dy
		}
		for _, sc := range vc.Subnets {
			sc.X += dx
			sc.Y += dy
			for _, r := range sc.Resources {
				r.X += dx
				r.Y += dy
			}
			for _, pair := range sc.PEPairs {
				pair.PE.X += dx
				pair.PE.Y += dy
				if pair.LinkedService != nil {
					pair.LinkedService.X += dx
					pair.LinkedService.Y += dy
					for _, child := range pair.LinkedService.ChildResources {
						child.X += dx
						child.Y += dy
					}
				}
			}
		}
	}
	for _, r := range d.StandaloneResources {
		r.X += dx
		r.Y += dy
		for _, child := range r.ChildResources {
			child.X += dx
			child.Y += dy
		}
	}
	// AnchoredLBs are positioned inside subnets via SubnetContainer.AnchoredLB;
	// their offset is applied as part of the subnet resources loop above.
	for _, vc := range d.VNets {
		for _, sc := range vc.Subnets {
			if sc.AnchoredLB != nil {
				sc.AnchoredLB.X += dx
				sc.AnchoredLB.Y += dy
			}
		}
	}
}
