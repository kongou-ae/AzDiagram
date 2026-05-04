// Package model defines the core data structures used throughout AzDiagram.
package model

// ResourceCategory is the visual grouping category for an Azure resource.
type ResourceCategory string

const (
	CategoryNetworking  ResourceCategory = "networking"
	CategoryCompute     ResourceCategory = "compute"
	CategoryStorage     ResourceCategory = "storage"
	CategoryDatabase    ResourceCategory = "database"
	CategorySecurity    ResourceCategory = "security"
	CategoryIntegration ResourceCategory = "integration"
	CategoryMonitoring  ResourceCategory = "monitoring"
	CategoryOther       ResourceCategory = "other"
)

// Resource represents a parsed Azure resource from a Bicep or ARM template.
type Resource struct {
	// SymbolicName is the identifier used in the Bicep source (e.g. "myVNet").
	// For ARM JSON, it is generated from the type and index.
	SymbolicName string

	// Type is the full Azure resource type in lowercase (e.g. "microsoft.network/virtualnetworks").
	Type string

	// APIVersion is the resource API version string.
	APIVersion string

	// DisplayName is the resource name value extracted from the template (best-effort).
	DisplayName string

	// Kind holds the "kind" property (e.g. "functionapp" for App Service).
	Kind string

	// Dependencies lists the symbolic names of resources this resource depends on.
	Dependencies []string

	// AttachedNICs holds NIC resources that are logically part of this VM/compute resource.
	// They are rendered inline inside the same card rather than as separate cards.
	AttachedNICs []*Resource

	// AttachedPIPs holds Public IP resources directly associated with this resource
	// (Firewall, VPN Gateway, Bastion). They are rendered inline at the bottom of the card.
	AttachedPIPs []*Resource

	// LinkedServiceSymbol is the Bicep symbolic name of the service this Private Endpoint connects to.
	// Only populated for microsoft.network/privateendpoints.
	LinkedServiceSymbol string

	// ParentSymbol is the Bicep symbolic name declared with `parent:` on this resource.
	// Only populated for child resources (e.g. Microsoft.Sql/servers/databases).
	ParentSymbol string

	// ChildResources holds child resources (those with `parent:` pointing to this resource)
	// that are rendered to the left of this card with a solid connector line.
	ChildResources []*Resource

	// LBSymbols holds the Bicep symbolic names of Load Balancers this NIC is associated with
	// via loadBalancerBackendAddressPools.
	LBSymbols []string

	// NICNSGSymbol is the Bicep symbolic name of the NSG attached to this NIC resource.
	// Only populated for microsoft.network/networkinterfaces.
	NICNSGSymbol string

	// AttachedNicNSGs holds NSG resources collected from this VM's attached NICs.
	// Rendered as NSG bars at the bottom of the VM card, below the NIC/PIP rows.
	AttachedNicNSGs []*Resource

	// SubnetRef, if non-nil, explicitly identifies the VNet+subnet this resource belongs to.
	// Populated from ipConfigurations[].properties.subnet.id in the Bicep source.
	SubnetRef *SubnetRefDef

	// MgmtSubnetRef, if non-nil, identifies the management subnet for Azure Firewall
	// (managementIpConfiguration.properties.subnet.id). The management PIP is placed
	// in this subnet as a standalone card; the Firewall icon is NOT duplicated there.
	MgmtSubnetRef *SubnetRefDef

	// MgmtPIPSymbol is the Bicep symbolic name of the PIP used in managementIpConfiguration.
	// This PIP is placed in MgmtSubnetRef rather than attached inline to the Firewall card.
	MgmtPIPSymbol string

	// IsMgmtProxy marks a synthetic resource created to represent the Firewall's management
	// interface in AzureFirewallManagementSubnet. The card renders as a plain white box
	// (no Firewall icon) with the management PIP attached below.
	IsMgmtProxy bool

	// BastionDevVNetSymbol is set for Azure Bastion Developer SKU resources.
	// It holds the Bicep symbolic name of the VNet referenced via virtualNetwork.id.
	// These resources are placed below their VNet container rather than inside a subnet.
	BastionDevVNetSymbol string

	// VNetIntSubnet, if non-nil, identifies the subnet this App Service is VNet-integrated into.
	// Populated from virtualNetworkSubnetId in the Bicep source.
	VNetIntSubnet *SubnetRefDef

	// Subnets holds subnet definitions for Microsoft.Network/virtualNetworks resources.
	Subnets []SubnetDef

	// ---- Metadata populated by the registry ----

	// Category is the visual category used for colour and grouping.
	Category ResourceCategory

	// ShortName is the abbreviated label shown on the icon badge (e.g. "VM", "SA").
	ShortName string

	// TypeLabel is the resource type segment after the provider prefix (e.g. "virtualMachines").
	TypeLabel string

	// IconColor is the CSS hex colour for the icon badge background.
	IconColor string

	// ---- Layout positions set by the layout engine ----

	X, Y          float64
	Width, Height float64
}

// VNetPeering represents a peering link between two VNet containers.
type VNetPeering struct {
	Local  *VNetContainer // this side of the peering
	Remote *VNetContainer // the other side
}

type SubnetDef struct {
	Name          string
	AddressPrefix string
	// NSGSymbol is the Bicep symbolic name of the NSG associated with this subnet.
	NSGSymbol string
	// RTSymbol is the Bicep symbolic name of the Route Table associated with this subnet.
	RTSymbol string
}

// SubnetRefDef identifies a specific subnet within a named VNet.
type SubnetRefDef struct {
	VNetSymbol string // Bicep symbolic name of the parent VNet
	SubnetName string // exact subnet name (e.g. "AzureFirewallSubnet")
}

// Diagram is the fully assembled model that the renderer consumes.
type Diagram struct {
	// All resources in declaration order.
	Resources []*Resource

	// ResourcesBySymbol provides O(1) lookup by symbolic name.
	ResourcesBySymbol map[string]*Resource

	// Directed dependency edges.
	Edges []*Edge

	// VNets that appear as hierarchical containers.
	VNets []*VNetContainer

	// VNetPeerings holds peering relationships between VNets for rendering.
	VNetPeerings []*VNetPeering

	// Resources that live outside any VNet container.
	StandaloneResources []*Resource

	// LBConnections holds VM-to-LoadBalancer connections derived from NIC backend pool refs.
	LBConnections []*LBConnection

	// VNetIntConnections holds App Service → subnet VNet integration connections.
	VNetIntConnections []*VNetIntConnection

	// DNSLinks holds Private DNS Zone ↔ VNet links, rendered as solid lines.
	DNSLinks []*DNSLinkConnection

	// AnchoredLBs holds Load Balancers repositioned above their connected VMs.
	// They are removed from their original subnet/standalone list by the builder.
	AnchoredLBs []*Resource

	// Canvas dimensions set by the layout engine.
	Width, Height float64
}

// LBConnection represents a visual solid-line connection from a VM to a Load Balancer.
type LBConnection struct {
	From *Resource // VM (or NIC)
	To   *Resource // Load Balancer
}

// VNetIntConnection represents a VNet integration connection from an App Service
// to the subnet it is integrated into.
type VNetIntConnection struct {
	App    *Resource        // Microsoft.Web/sites resource
	Subnet *SubnetContainer // target subnet
}

// DNSLinkConnection represents a Private DNS Zone ↔ VNet link, derived from a
// Microsoft.Network/privateDnsZones/virtualNetworkLinks resource.
type DNSLinkConnection struct {
	Zone *Resource      // Microsoft.Network/privateDnsZones resource (the parent zone)
	VNet *VNetContainer // target VNet container
}

// Edge represents a directed dependency arrow between two resources.
type Edge struct {
	// From and To are symbolic names.
	From, To string
}

// PEPair holds a Private Endpoint and the service it connects to,
// rendered as a side-by-side pair within a subnet.
type PEPair struct {
	PE            *Resource
	LinkedService *Resource // may be nil if target not resolved
}

// VNetContainer holds a VNet resource together with its subnets.
type VNetContainer struct {
	Resource *Resource

	// Subnets derived from the VNet's properties.subnets.
	Subnets []*SubnetContainer

	// DNSZones holds Private DNS Zone resources whose VNet link targets this VNet.
	// They are rendered directly below the VNet container.
	DNSZones []*Resource

	// BastionDev holds an Azure Bastion Developer SKU resource that references this VNet
	// via virtualNetwork.id (no AzureBastionSubnet required). It is rendered directly
	// below the VNet container (below DNSZones if any) with a solid connector line.
	BastionDev *Resource

	// Layout bounding box.
	X, Y          float64
	Width, Height float64
}

// SubnetContainer represents one subnet within a VNet, potentially holding
// resources that are assigned to it.
type SubnetContainer struct {
	Name          string
	AddressPrefix string
	NSGSymbol     string // Bicep symbolic name of the associated NSG (from SubnetDef)
	RTSymbol      string // Bicep symbolic name of the associated Route Table (from SubnetDef)

	// AnchoredLB holds a Load Balancer that is placed to the left of this
	// subnet's VM column. When set, resources are forced into one vertical column.
	AnchoredLB *Resource

	// Resources assigned to this subnet (populated by the builder).
	Resources []*Resource

	// AttachedNSG is the NSG associated with this subnet, rendered as a badge
	// in the subnet's bottom bar (analogous to NIC inside a VM card).
	AttachedNSG *Resource

	// AttachedRT is the Route Table associated with this subnet, rendered as a
	// badge below the NSG bar (or at the bottom if no NSG).
	AttachedRT *Resource

	// PEPairs holds Private Endpoint + linked service pairs placed in the subnet.
	// PEs are rendered at the left of the subnet, their linked service to the right.
	PEPairs []*PEPair

	// IsVNetInt marks this subnet as a VNet integration target (delegated subnet).
	// It is placed on the right side of the VNet alongside PE subnets.
	IsVNetInt bool

	// LinkedVNetIntSubnetName, when set on a PE subnet, is the name of the VNet integration
	// subnet that pairs with it (same App Service uses both). The layout engine places them
	// adjacent to each other in the right column regardless of Bicep declaration order.
	LinkedVNetIntSubnetName string

	// Layout bounding box relative to the parent VNet.
	X, Y          float64
	Width, Height float64
}
