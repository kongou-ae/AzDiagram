// Package renderer – iconloader.go
// Loads official Microsoft Azure Architecture icons from a local directory.
//
// Download the official icon pack from:
//
//	https://learn.microsoft.com/en-us/azure/architecture/icons/
//
// After extracting the ZIP, pass the root folder to --icons-dir.
// The loader walks the directory tree recursively and indexes all .svg files.
// When generating a diagram, each resource type is matched against a keyword
// table to find the correct icon file.
package renderer

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kongou-ae/AzDiagram/internal/model"
)

// officialIconKeywords maps a normalised resource type → ordered list of
// filename keywords (lowercased). The first keyword that matches a file in the
// icons directory is used.
var officialIconKeywords = map[string][]string{
	// ── Compute ───────────────────────────────────────────────────────────
	"microsoft.compute/virtualmachinescalesets":  {"vm-scale-sets"},
	"microsoft.compute/virtualmachines":          {"virtual-machine"},
	"microsoft.web/serverfarms":                  {"app-service-plans"},
	"microsoft.app/containerapps":                {"container-apps"},
	"microsoft.containerservice/managedclusters": {"kubernetes-services"},
	"microsoft.containerregistry/registries":     {"container-registries"},
	"microsoft.web/sites":                        {"app-services"},

	// ── Networking ────────────────────────────────────────────────────────
	"microsoft.network/virtualnetworks":        {"virtual-networks"},
	"microsoft.network/loadbalancers":          {"load-balancers"},
	"microsoft.network/applicationgateways":    {"application-gateways"},
	"microsoft.network/virtualnetworkgateways": {"virtual-network-gateways"},
	"microsoft.network/networksecuritygroups":  {"network-security-groups"},
	"microsoft.network/publicipaddresses":      {"public-ip-addresses"},
	"microsoft.network/networkinterfaces":      {"network-interfaces"},
	"microsoft.network/privateendpoints":       {"private-endpoint"},
	"microsoft.network/privatednszones":        {"dns-private-zones", "dns-zones", "private-dns"},
	"microsoft.network/privatednszones/virtualnetworklinks": {"virtual-network", "virtual-networks"},
	"microsoft.network/azurefirewalls":         {"firewalls"},
	"microsoft.network/bastionhosts":           {"bastions"},
	"microsoft.network/frontdoors":             {"front-door"},
	"microsoft.network/trafficmanagerprofiles": {"traffic-manager-profiles"},
	"microsoft.cdn/profiles":                   {"cdn-profiles", "content-delivery-networks"},
	"microsoft.network/routetables":            {"route-tables"},

	// ── Storage ───────────────────────────────────────────────────────────
	"microsoft.storage/storageaccounts": {"storage-accounts"},

	// ── Database ──────────────────────────────────────────────────────────
	"microsoft.sql/servers":                     {"sql-server"},
	"microsoft.sql/servers/databases":           {"azure-sql"},
	"microsoft.documentdb/databaseaccounts":     {"azure-cosmos-db"},
	"microsoft.cache/redis":                     {"cache-redis"},
	"microsoft.dbforpostgresql/flexibleservers": {"azure-database-for-postgresql"},
	"microsoft.dbformysql/flexibleservers":      {"azure-database-for-mysql"},
	"microsoft.synapse/workspaces":              {"azure-synapse-analytics"},

	// ── AI ──────────────────────────────────────────────────────────────────
	"microsoft.cognitiveservices/accounts": {"azure-openai"},

	// ── Security ──────────────────────────────────────────────────────────
	"microsoft.keyvault/vaults":                        {"key-vaults"},
	"microsoft.managedidentity/userassignedidentities": {"managed-identities"},

	// ── Integration ───────────────────────────────────────────────────────
	"microsoft.servicebus/namespaces":  {"service-bus"},
	"microsoft.eventhub/namespaces":    {"event-hubs"},
	"microsoft.apimanagement/service":  {"api-management-services"},
	"microsoft.logic/workflows":        {"logic-apps"},
	"microsoft.eventgrid/topics":       {"event-grid-domains"},
	"microsoft.eventgrid/systemtopics": {"event-grid"},

	// ── Monitoring ────────────────────────────────────────────────────────
	"microsoft.operationalinsights/workspaces": {"log-analytics-workspaces"},
	"microsoft.insights/components":            {"application-insights"},
	"microsoft.dashboard/grafana":              {"grafana"},
}

// officialIconNumbers overrides icon selection by specifying the exact 5-digit
// number prefix of the icon filename for a given resource type (lowercased).
// When a number is set here, it takes priority over keyword matching.
// Leave an entry blank ("") to fall back to keyword matching for that type.
//
// Example: "10132" selects "10132-icon-service-SQL-Server.svg".
var officialIconNumbers = map[string]string{
	// ── Compute ───────────────────────────────────────────────────────────
	// "microsoft.compute/virtualmachines": "00000",

	// ── Networking ────────────────────────────────────────────────────────

	// ── Storage ───────────────────────────────────────────────────────────

	// ── AI ──────────────────────────────────────────────────────────────────
	"microsoft.cognitiveservices/accounts": "03438", // 03438-icon-service-Cognitive-Services.svg

	// ── Database ──────────────────────────────────────────────────────────
	"microsoft.sql/servers":           "10132", // 10132-icon-service-SQL-Server.svg
	"microsoft.sql/servers/databases": "10130", // 10130-icon-service-SQL-Database.svg

	// ── Security ──────────────────────────────────────────────────────────

	// ── Monitoring ────────────────────────────────────────────────────────
}

// functionAppKeywords is used when microsoft.web/sites has kind=functionapp.
var functionAppKeywords = []string{"function-apps"}

// ── IconRegistry ─────────────────────────────────────────────────────────────

// IconRegistry indexes official Azure SVG icons found in a local directory.
type IconRegistry struct {
	// index maps lowercase base filename (without extension) → absolute path.
	index map[string]string
	// sortedKeys holds the keys of index in ascending alphabetical order so that
	// iteration is deterministic and identical resources always pick the same file.
	sortedKeys []string
}

// NewIconRegistry walks dir recursively and indexes every .svg file found.
// Returns an empty (non-nil) registry if dir is empty string.
func NewIconRegistry(dir string) (*IconRegistry, error) {
	reg := &IconRegistry{index: make(map[string]string)}
	if dir == "" {
		return reg, nil
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries gracefully
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.EqualFold(filepath.Ext(name), ".svg") {
			return nil
		}
		// Key: lowercase filename without extension.
		base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
		reg.index[base] = path
		return nil
	})
	// Sort keys once so every subsequent lookup iterates in stable order.
	reg.sortedKeys = make([]string, 0, len(reg.index))
	for k := range reg.index {
		reg.sortedKeys = append(reg.sortedKeys, k)
	}
	sort.Strings(reg.sortedKeys)
	return reg, err
}

// Count returns the number of indexed icon files.
func (reg *IconRegistry) Count() int {
	if reg == nil {
		return 0
	}
	return len(reg.index)
}

// Lookup finds the official icon for r and returns its inner SVG content,
// the viewBox string, and whether a match was found.
//
// All id= attributes in the returned content are namespaced with a prefix
// derived from r.SymbolicName to prevent ID collisions in the document.
func (reg *IconRegistry) Lookup(r *model.Resource) (svgContent, viewBox string, ok bool) {
	if reg == nil || len(reg.index) == 0 {
		return "", "", false
	}

	rt := strings.ToLower(strings.TrimSpace(r.Type))

	// ── Number-based override (highest priority) ──────────────────────────
	if num, ok2 := officialIconNumbers[rt]; ok2 && num != "" {
		for _, base := range reg.sortedKeys {
			if strings.HasPrefix(base, num+"-") {
				content, vb := loadSVGContent(reg.index[base])
				if content != "" {
					ns := "ic-" + sanitizeNS(r.SymbolicName)
					content = namespaceIDs(content, ns)
					return content, vb, true
				}
			}
		}
		// Number specified but file not found; fall through to keyword matching.
	}

	// ── Keyword-based matching ────────────────────────────────────────────

	// Exact type lookup first.
	keywords, found := officialIconKeywords[rt]
	if !found {
		// Longest-prefix match for child resource types (e.g. servers/databases).
		best := ""
		for k := range officialIconKeywords {
			if strings.HasPrefix(rt, k) && len(k) > len(best) {
				best = k
			}
		}
		if best == "" {
			return "", "", false
		}
		keywords = officialIconKeywords[best]
	}

	// Function Apps get a different icon than plain App Service.
	if rt == "microsoft.web/sites" && strings.Contains(strings.ToLower(r.Kind), "function") {
		keywords = functionAppKeywords
	}

	for _, kw := range keywords {
		// Find all matching filenames, then pick the best one:
		// shortest basename that contains the keyword (avoids "(Classic)", "VM",
		// "Stretch", etc. variants being selected over the canonical icon).
		bestBase := ""
		for _, base := range reg.sortedKeys {
			if !strings.Contains(base, kw) {
				continue
			}
			if bestBase == "" || len(base) < len(bestBase) {
				bestBase = base
			}
		}
		if bestBase == "" {
			continue
		}
		content, vb := loadSVGContent(reg.index[bestBase])
		if content == "" {
			continue
		}
		// Namespace IDs to prevent collisions when multiple icons are
		// embedded in the same SVG document.
		ns := "ic-" + sanitizeNS(r.SymbolicName)
		content = namespaceIDs(content, ns)
		return content, vb, true
	}
	return "", "", false
}

// sanitizeNS produces a safe namespace prefix from a Bicep symbolic name.
func sanitizeNS(s string) string {
	var b strings.Builder
	for _, ch := range s {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9':
			b.WriteRune(ch)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// ── SVG content extraction ────────────────────────────────────────────────────

var (
	reViewBoxAttr = regexp.MustCompile(`(?i)viewBox\s*=\s*"([^"]+)"`)
	reSVGOpenTag  = regexp.MustCompile(`(?i)<svg(?:\s[^>]*)?>`)
	reSVGCloseTag = regexp.MustCompile(`(?i)</svg\s*>`)
)

// loadSVGContent reads path and extracts the content between the outer <svg>
// tags plus the viewBox attribute value.
func loadSVGContent(path string) (inner, viewBox string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	s := string(data)

	// Extract viewBox (default to a common Azure icon size).
	viewBox = "0 0 18 18"
	if m := reViewBoxAttr.FindStringSubmatch(s); m != nil {
		viewBox = m[1]
	}

	// Strip XML / DOCTYPE declarations before the root element.
	if idx := strings.LastIndex(s, "?>"); idx != -1 {
		s = s[idx+2:]
	}

	// Extract inner content: text between end of opening <svg> tag and start
	// of closing </svg> tag.
	openIdx := reSVGOpenTag.FindStringIndex(s)
	if openIdx == nil {
		return "", ""
	}
	inner = s[openIdx[1]:]

	if closeIdx := reSVGCloseTag.FindStringIndex(inner); closeIdx != nil {
		inner = inner[:closeIdx[0]]
	}

	return strings.TrimSpace(inner), viewBox
}

// ── ID namespace scoping ─────────────────────────────────────────────────────

var (
	reIDAttr    = regexp.MustCompile(`\bid="([^"]+)"`)
	reURLRef    = regexp.MustCompile(`url\(#([^)]+)\)`)
	reHrefHash  = regexp.MustCompile(`href="#([^"]+)"`)
	reXlinkHref = regexp.MustCompile(`xlink:href="#([^"]+)"`)
)

// namespaceIDs rewrites all id definitions and their url(#)/href="#" references
// to use the given namespace prefix, preventing ID collisions.
func namespaceIDs(content, ns string) string {
	content = reIDAttr.ReplaceAllString(content, `id="`+ns+`-$1"`)
	content = reURLRef.ReplaceAllString(content, `url(#`+ns+`-$1)`)
	content = reHrefHash.ReplaceAllString(content, `href="#`+ns+`-$1"`)
	content = reXlinkHref.ReplaceAllString(content, `xlink:href="#`+ns+`-$1"`)
	return content
}
