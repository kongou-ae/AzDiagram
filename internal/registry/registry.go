// Package registry maps Azure resource types to display metadata such as
// short names, categories, and icon badge colours.
package registry

import (
	"strings"

	"github.com/kongou-ae/AzDiagram/internal/model"
)

// entry holds display metadata for one resource type.
type entry struct {
	ShortName string
	Category  model.ResourceCategory
	Color     string // CSS hex colour for the icon badge background
}

// table maps a normalised resource type prefix to its metadata.
// Keys are lowercase and may be partial prefixes (longest match wins).
var table = map[string]entry{
	// ── Compute ──────────────────────────────────────────────────────────
	"microsoft.compute/virtualmachinescalesets":  {"VMSS", model.CategoryCompute, "#6366F1"},
	"microsoft.compute/virtualmachines":          {"VM", model.CategoryCompute, "#6366F1"},
	"microsoft.web/serverfarms":                  {"Plan", model.CategoryCompute, "#6366F1"},
	"microsoft.app/containerapps":                {"CA", model.CategoryCompute, "#6366F1"},
	"microsoft.containerservice/managedclusters": {"AKS", model.CategoryCompute, "#6366F1"},
	"microsoft.containerregistry/registries":     {"ACR", model.CategoryCompute, "#6366F1"},
	// App Service / Functions differentiated at runtime by Kind
	"microsoft.web/sites": {"App", model.CategoryCompute, "#6366F1"},

	// ── Networking ────────────────────────────────────────────────────────
	"microsoft.network/virtualnetworks/subnets": {"Subnet", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/virtualnetworks":         {"VNet", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/loadbalancers":           {"LB", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/applicationgateways":     {"AppGW", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/virtualnetworkgateways":  {"VPN GW", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/networksecuritygroups":   {"NSG", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/publicipaddresses":       {"PIP", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/networkinterfaces":       {"NIC", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/privateendpoints":        {"PE", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/azurefirewalls":          {"FW", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/bastionhosts":            {"Bastion", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/frontdoors":              {"FD", model.CategoryNetworking, "#2563EB"},
	"microsoft.network/trafficmanagerprofiles":  {"TM", model.CategoryNetworking, "#2563EB"},
	"microsoft.cdn/profiles":                    {"CDN", model.CategoryNetworking, "#2563EB"},

	// ── Storage ───────────────────────────────────────────────────────────
	"microsoft.storage/storageaccounts": {"SA", model.CategoryStorage, "#0891B2"},

	// ── Database ──────────────────────────────────────────────────────────
	"microsoft.sql/servers/databases":           {"SQL DB", model.CategoryDatabase, "#7C3AED"},
	"microsoft.sql/servers":                     {"SQL", model.CategoryDatabase, "#7C3AED"},
	"microsoft.documentdb/databaseaccounts":     {"CosmosDB", model.CategoryDatabase, "#7C3AED"},
	"microsoft.cache/redis":                     {"Redis", model.CategoryDatabase, "#7C3AED"},
	"microsoft.dbforpostgresql/flexibleservers": {"PgSQL", model.CategoryDatabase, "#7C3AED"},
	"microsoft.dbformysql/flexibleservers":      {"MySQL", model.CategoryDatabase, "#7C3AED"},
	"microsoft.synapse/workspaces":              {"Synapse", model.CategoryDatabase, "#7C3AED"},

	// ── AI ──────────────────────────────────────────────────────────────────
	"microsoft.cognitiveservices/accounts": {"AOAI", model.CategoryCompute, "#8B5CF6"},

	// ── Security ──────────────────────────────────────────────────────────
	"microsoft.keyvault/vaults":                        {"KV", model.CategorySecurity, "#047857"},
	"microsoft.managedidentity/userassignedidentities": {"MI", model.CategorySecurity, "#047857"},

	// ── Integration ───────────────────────────────────────────────────────
	"microsoft.servicebus/namespaces":  {"SB", model.CategoryIntegration, "#D97706"},
	"microsoft.eventhub/namespaces":    {"EH", model.CategoryIntegration, "#D97706"},
	"microsoft.apimanagement/service":  {"APIM", model.CategoryIntegration, "#D97706"},
	"microsoft.logic/workflows":        {"Logic", model.CategoryIntegration, "#D97706"},
	"microsoft.eventgrid/topics":       {"EG", model.CategoryIntegration, "#D97706"},
	"microsoft.eventgrid/systemtopics": {"EG", model.CategoryIntegration, "#D97706"},

	// ── Monitoring ────────────────────────────────────────────────────────
	"microsoft.operationalinsights/workspaces": {"LAW", model.CategoryMonitoring, "#7E57C2"},
	"microsoft.insights/components":            {"AppIns", model.CategoryMonitoring, "#7E57C2"},
	"microsoft.insights/activitylogalerts":     {"Alert", model.CategoryMonitoring, "#7E57C2"},
	"microsoft.dashboard/grafana":              {"Grafana", model.CategoryMonitoring, "#7E57C2"},
}

// Lookup returns display metadata for the given resource type.
// The type should already be normalised to lowercase.
func Lookup(resourceType string) entry {
	rt := strings.ToLower(strings.TrimSpace(resourceType))

	// Exact match first.
	if e, ok := table[rt]; ok {
		return e
	}

	// Prefix / longest-match fallback (handles unversioned child types etc.).
	best := ""
	for k := range table {
		if strings.HasPrefix(rt, k) && len(k) > len(best) {
			best = k
		}
	}
	if best != "" {
		return table[best]
	}

	// Unknown resource: derive a short name from the last path segment.
	parts := strings.Split(rt, "/")
	short := parts[len(parts)-1]
	if len(short) > 6 {
		short = strings.ToUpper(short[:3])
	} else {
		short = strings.ToUpper(short)
	}
	return entry{short, model.CategoryOther, "#78909C"}
}

// Annotate enriches a resource with metadata from the registry.
func Annotate(r *model.Resource) {
	e := Lookup(r.Type)

	// Differentiate Function App vs regular App Service.
	if strings.Contains(r.Type, "microsoft.web/sites") {
		if strings.Contains(r.Kind, "function") {
			e.ShortName = "Func"
			e.Color = "#D97706"
		}
	}

	r.Category = e.Category
	r.ShortName = e.ShortName
	r.IconColor = e.Color
}
