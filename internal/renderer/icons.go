// Package renderer contains the SVG icon library for Azure services.
// Each icon is a 32×32 SVG fragment (white shapes on transparent background)
// intended to be placed inside a coloured badge rectangle.
package renderer

import (
	"strings"

	"github.com/kongou-ae/AzDiagram/internal/model"
)

// iconShapes maps resource category + short name to a 32×32 white SVG fragment.
// The viewBox of the enclosing <svg> is "0 0 32 32".
var iconShapes = map[string]string{

	// ── Compute ──────────────────────────────────────────────────────────

	// Virtual Machine – monitor outline
	"VM": `<rect x="4" y="6" width="24" height="15" rx="2" fill="none" stroke="white" stroke-width="2"/>` +
		`<rect x="13" y="21" width="6" height="3" fill="white"/>` +
		`<rect x="10" y="24" width="12" height="2" rx="1" fill="white"/>`,

	// VM Scale Set – two overlapping monitors
	"VMSS": `<rect x="3" y="8" width="14" height="10" rx="1" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<rect x="15" y="5" width="14" height="10" rx="1" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<line x1="22" y1="15" x2="22" y2="19" stroke="white" stroke-width="1.5"/>` +
		`<line x1="18" y1="19" x2="26" y2="19" stroke="white" stroke-width="1.5"/>`,

	// App Service – browser window
	"App": `<rect x="4" y="6" width="24" height="20" rx="2" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="4" y1="12" x2="28" y2="12" stroke="white" stroke-width="1.5"/>` +
		`<circle cx="8" cy="9" r="1.5" fill="white"/>` +
		`<circle cx="13" cy="9" r="1.5" fill="white"/>` +
		`<line x1="8" y1="18" x2="24" y2="18" stroke="white" stroke-width="1.5"/>` +
		`<line x1="8" y1="22" x2="18" y2="22" stroke="white" stroke-width="1.5"/>`,

	// Function App – lightning bolt
	"Func": `<polygon points="18,4 10,18 16,18 14,29 22,15 16,15" fill="white"/>`,

	// App Service Plan – ruler/gauge
	"Plan": `<rect x="5" y="10" width="22" height="12" rx="2" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="10" y1="10" x2="10" y2="7" stroke="white" stroke-width="1.5"/>` +
		`<line x1="16" y1="10" x2="16" y2="7" stroke="white" stroke-width="1.5"/>` +
		`<line x1="22" y1="10" x2="22" y2="7" stroke="white" stroke-width="1.5"/>`,

	// AKS – ship-wheel hexagon
	"AKS": `<polygon points="16,4 25,9 25,22 16,28 7,22 7,9" fill="none" stroke="white" stroke-width="2"/>` +
		`<circle cx="16" cy="16" r="3" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<line x1="16" y1="13" x2="16" y2="4" stroke="white" stroke-width="1.5"/>` +
		`<line x1="16" y1="19" x2="16" y2="28" stroke="white" stroke-width="1.5"/>` +
		`<line x1="13.4" y1="14.5" x2="7" y2="9" stroke="white" stroke-width="1.5"/>` +
		`<line x1="18.6" y1="14.5" x2="25" y2="9" stroke="white" stroke-width="1.5"/>` +
		`<line x1="13.4" y1="17.5" x2="7" y2="22" stroke="white" stroke-width="1.5"/>` +
		`<line x1="18.6" y1="17.5" x2="25" y2="22" stroke="white" stroke-width="1.5"/>`,

	// Container Apps – grid of boxes
	"CA": `<rect x="4" y="4" width="10" height="10" rx="1" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<rect x="18" y="4" width="10" height="10" rx="1" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<rect x="4" y="18" width="10" height="10" rx="1" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<rect x="18" y="18" width="10" height="10" rx="1" fill="none" stroke="white" stroke-width="1.5"/>`,

	// ACR – lens / camera circle
	"ACR": `<circle cx="16" cy="16" r="10" fill="none" stroke="white" stroke-width="2"/>` +
		`<circle cx="16" cy="16" r="5" fill="none" stroke="white" stroke-width="2"/>` +
		`<circle cx="16" cy="16" r="2" fill="white"/>`,

	// ── Networking ────────────────────────────────────────────────────────

	// VNet – hub-and-spoke network
	"VNet": `<circle cx="16" cy="11" r="3" fill="white"/>` +
		`<circle cx="7" cy="22" r="2.5" fill="white"/>` +
		`<circle cx="25" cy="22" r="2.5" fill="white"/>` +
		`<line x1="16" y1="14" x2="7" y2="20" stroke="white" stroke-width="1.5"/>` +
		`<line x1="16" y1="14" x2="25" y2="20" stroke="white" stroke-width="1.5"/>` +
		`<line x1="9" y1="22" x2="23" y2="22" stroke="white" stroke-width="1.5"/>`,

	// Subnet – dashed inner rectangle
	"Subnet": `<rect x="5" y="5" width="22" height="22" rx="2" fill="none" stroke="white" stroke-width="2" stroke-dasharray="4,2"/>`,

	// Load Balancer – triangles pointing inward
	"LB": `<polygon points="5,11 14,6 14,16" fill="white" opacity="0.9"/>` +
		`<polygon points="27,11 18,6 18,16" fill="white" opacity="0.9"/>` +
		`<rect x="14" y="9" width="4" height="4" fill="white"/>` +
		`<line x1="5" y1="22" x2="27" y2="22" stroke="white" stroke-width="1.5"/>` +
		`<line x1="5" y1="25" x2="27" y2="25" stroke="white" stroke-width="1.5"/>`,

	// Application Gateway – shield with down-arrow
	"AppGW": `<path d="M16,3 L26,7 L26,18 C26,23 21,27 16,29 C11,27 6,23 6,18 L6,7 Z" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="16" y1="10" x2="16" y2="22" stroke="white" stroke-width="2"/>` +
		`<polyline points="11,18 16,24 21,18" fill="none" stroke="white" stroke-width="2"/>`,

	// VPN Gateway – diamond
	"VPN GW": `<polygon points="16,4 28,16 16,28 4,16" fill="none" stroke="white" stroke-width="2"/>` +
		`<polygon points="16,10 22,16 16,22 10,16" fill="none" stroke="white" stroke-width="1.5"/>`,

	// NSG – shield with checkmark
	"NSG": `<path d="M16,4 L26,8 L26,17 C26,22 21,27 16,29 C11,27 6,22 6,17 L6,8 Z" fill="none" stroke="white" stroke-width="2"/>` +
		`<polyline points="11,17 14,20 21,13" fill="none" stroke="white" stroke-width="2"/>`,

	// Public IP – globe
	"PIP": `<circle cx="16" cy="16" r="11" fill="none" stroke="white" stroke-width="2"/>` +
		`<ellipse cx="16" cy="16" rx="5" ry="11" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<line x1="5" y1="11" x2="27" y2="11" stroke="white" stroke-width="1.5"/>` +
		`<line x1="5" y1="21" x2="27" y2="21" stroke="white" stroke-width="1.5"/>`,

	// NIC – ethernet port
	"NIC": `<rect x="8" y="5" width="16" height="16" rx="2" fill="none" stroke="white" stroke-width="2"/>` +
		`<rect x="11" y="8" width="4" height="5" rx="1" fill="white"/>` +
		`<rect x="17" y="8" width="4" height="5" rx="1" fill="white"/>` +
		`<line x1="16" y1="21" x2="16" y2="27" stroke="white" stroke-width="2"/>` +
		`<line x1="10" y1="27" x2="22" y2="27" stroke="white" stroke-width="1.5"/>`,

	// Private Endpoint – circle with lock
	"PE": `<circle cx="16" cy="12" r="6" fill="none" stroke="white" stroke-width="2"/>` +
		`<rect x="11" y="16" width="10" height="8" rx="1" fill="none" stroke="white" stroke-width="2"/>` +
		`<circle cx="16" cy="20" r="1.5" fill="white"/>`,

	// Azure Firewall – flame
	"FW": `<path d="M16,28 C9,24 7,18 9,12 C11,7 14,5 14,5 C14,9 16,10 18,8 C20,6 20,3 18,2 C22,4 25,9 23,16 C22,20 19,23 19,25 C19,27 17,28 16,28 Z" fill="none" stroke="white" stroke-width="2"/>`,

	// Bastion – castle battlements
	"Bastion": `<rect x="6" y="13" width="20" height="13" rx="1" fill="none" stroke="white" stroke-width="2"/>` +
		`<rect x="6" y="7" width="5" height="8" rx="1" fill="none" stroke="white" stroke-width="2"/>` +
		`<rect x="14" y="7" width="5" height="8" rx="1" fill="none" stroke="white" stroke-width="2"/>` +
		`<rect x="22" y="7" width="5" height="8" rx="1" fill="none" stroke="white" stroke-width="2"/>` +
		`<rect x="13" y="20" width="6" height="6" rx="1" fill="none" stroke="white" stroke-width="1.5"/>`,

	// Front Door – globe with arrow
	"FD": `<circle cx="16" cy="16" r="10" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="6" y1="16" x2="26" y2="16" stroke="white" stroke-width="1.5"/>` +
		`<polyline points="20,12 24,16 20,20" fill="none" stroke="white" stroke-width="1.5"/>`,

	// Traffic Manager – traffic circle
	"TM": `<circle cx="16" cy="16" r="10" fill="none" stroke="white" stroke-width="2"/>` +
		`<circle cx="16" cy="16" r="3" fill="white"/>` +
		`<line x1="16" y1="6" x2="16" y2="13" stroke="white" stroke-width="1.5"/>` +
		`<line x1="26" y1="16" x2="19" y2="16" stroke="white" stroke-width="1.5"/>`,

	// CDN – globe with speed lines
	"CDN": `<circle cx="16" cy="16" r="9" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="3" y1="11" x2="9" y2="13" stroke="white" stroke-width="1.5"/>` +
		`<line x1="2" y1="16" x2="7" y2="16" stroke="white" stroke-width="1.5"/>` +
		`<line x1="3" y1="21" x2="9" y2="19" stroke="white" stroke-width="1.5"/>`,

	// ── Storage ───────────────────────────────────────────────────────────

	// Storage Account – stacked bars (like a storage rack)
	"SA": `<rect x="4" y="7" width="24" height="5" rx="1" fill="white" opacity="0.9"/>` +
		`<rect x="4" y="14" width="24" height="5" rx="1" fill="white" opacity="0.7"/>` +
		`<rect x="4" y="21" width="24" height="5" rx="1" fill="white" opacity="0.5"/>`,

	// ── Database ──────────────────────────────────────────────────────────

	// SQL – database cylinder
	"SQL": `<ellipse cx="16" cy="9" rx="10" ry="3" fill="none" stroke="white" stroke-width="2"/>` +
		`<ellipse cx="16" cy="23" rx="10" ry="3" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="6" y1="9" x2="6" y2="23" stroke="white" stroke-width="2"/>` +
		`<line x1="26" y1="9" x2="26" y2="23" stroke="white" stroke-width="2"/>`,

	// SQL DB – cylinder with lines
	"SQL DB": `<ellipse cx="16" cy="9" rx="10" ry="3" fill="none" stroke="white" stroke-width="2"/>` +
		`<ellipse cx="16" cy="23" rx="10" ry="3" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="6" y1="9" x2="6" y2="23" stroke="white" stroke-width="2"/>` +
		`<line x1="26" y1="9" x2="26" y2="23" stroke="white" stroke-width="2"/>` +
		`<line x1="8" y1="14" x2="24" y2="14" stroke="white" stroke-width="1"/>` +
		`<line x1="8" y1="18" x2="24" y2="18" stroke="white" stroke-width="1"/>`,

	// Cosmos DB – orbits
	"CosmosDB": `<ellipse cx="16" cy="16" rx="12" ry="5" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<ellipse cx="16" cy="16" rx="5" ry="12" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<circle cx="16" cy="16" r="3" fill="white"/>`,

	// Redis – database + lightning bolt
	"Redis": `<ellipse cx="16" cy="9" rx="9" ry="3" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<ellipse cx="16" cy="22" rx="9" ry="3" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<line x1="7" y1="9" x2="7" y2="22" stroke="white" stroke-width="1.5"/>` +
		`<line x1="25" y1="9" x2="25" y2="22" stroke="white" stroke-width="1.5"/>` +
		`<polygon points="19,10 15,16 18,16 16,23 21,16 18,16" fill="white"/>`,

	// PostgreSQL – elephant silhouette (simplified as database)
	"PgSQL": `<ellipse cx="16" cy="9" rx="10" ry="3" fill="none" stroke="white" stroke-width="2"/>` +
		`<ellipse cx="16" cy="23" rx="10" ry="3" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="6" y1="9" x2="6" y2="23" stroke="white" stroke-width="2"/>` +
		`<line x1="26" y1="9" x2="26" y2="23" stroke="white" stroke-width="2"/>`,

	// MySQL – database
	"MySQL": `<ellipse cx="16" cy="9" rx="10" ry="3" fill="none" stroke="white" stroke-width="2"/>` +
		`<ellipse cx="16" cy="23" rx="10" ry="3" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="6" y1="9" x2="6" y2="23" stroke="white" stroke-width="2"/>` +
		`<line x1="26" y1="9" x2="26" y2="23" stroke="white" stroke-width="2"/>`,

	// Synapse – data warehouse (stacked triangles)
	"Synapse": `<polygon points="16,4 26,22 6,22" fill="none" stroke="white" stroke-width="2"/>` +
		`<polygon points="16,10 22,22 10,22" fill="none" stroke="white" stroke-width="1.5"/>`,

	// ── Security ──────────────────────────────────────────────────────────

	// Key Vault – padlock
	"KV": `<rect x="8" y="15" width="16" height="12" rx="2" fill="none" stroke="white" stroke-width="2"/>` +
		`<path d="M11,15 L11,10 A5,5 0 0 1 21,10 L21,15" fill="none" stroke="white" stroke-width="2"/>` +
		`<circle cx="16" cy="20" r="2" fill="white"/>` +
		`<rect x="15" y="22" width="2" height="3" fill="white"/>`,

	// Managed Identity – person silhouette
	"MI": `<circle cx="16" cy="10" r="5" fill="none" stroke="white" stroke-width="2"/>` +
		`<path d="M6,28 C6,21 26,21 26,28" fill="none" stroke="white" stroke-width="2"/>`,

	// ── Integration ───────────────────────────────────────────────────────

	// Service Bus – message bubbles
	"SB": `<rect x="4" y="6" width="17" height="11" rx="2" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<path d="M5,17 L5,20 L9,17" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<rect x="11" y="14" width="17" height="11" rx="2" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<path d="M27,25 L27,27.5 L23,25" fill="none" stroke="white" stroke-width="1.5"/>`,

	// Event Hub – funnel / filter
	"EH": `<path d="M4,7 L28,7 L20,18 L20,27 L12,27 L12,18 Z" fill="none" stroke="white" stroke-width="2"/>`,

	// APIM – gear
	"APIM": `<circle cx="16" cy="16" r="5" fill="none" stroke="white" stroke-width="2"/>` +
		`<circle cx="16" cy="16" r="2" fill="white"/>` +
		`<rect x="14.5" y="3" width="3" height="5" rx="1" fill="white"/>` +
		`<rect x="14.5" y="24" width="3" height="5" rx="1" fill="white"/>` +
		`<rect x="3" y="14.5" width="5" height="3" rx="1" fill="white"/>` +
		`<rect x="24" y="14.5" width="5" height="3" rx="1" fill="white"/>` +
		`<rect x="7" y="7" width="3" height="3" rx="0.5" transform="rotate(45,8.5,8.5)" fill="white"/>` +
		`<rect x="22" y="7" width="3" height="3" rx="0.5" transform="rotate(45,23.5,8.5)" fill="white"/>` +
		`<rect x="7" y="22" width="3" height="3" rx="0.5" transform="rotate(45,8.5,23.5)" fill="white"/>` +
		`<rect x="22" y="22" width="3" height="3" rx="0.5" transform="rotate(45,23.5,23.5)" fill="white"/>`,

	// Logic Apps – flowchart
	"Logic": `<rect x="10" y="4" width="12" height="7" rx="2" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="16" y1="11" x2="16" y2="15" stroke="white" stroke-width="2"/>` +
		`<path d="M7,15 L25,15 L25,21 L20,21 L20,25 L16,29 L12,25 L12,21 L7,21 Z" fill="none" stroke="white" stroke-width="1.5"/>`,

	// Event Grid – grid with dots
	"EG": `<line x1="4" y1="10" x2="28" y2="10" stroke="white" stroke-width="1.5"/>` +
		`<line x1="4" y1="16" x2="28" y2="16" stroke="white" stroke-width="1.5"/>` +
		`<line x1="4" y1="22" x2="28" y2="22" stroke="white" stroke-width="1.5"/>` +
		`<circle cx="10" cy="10" r="2" fill="white"/>` +
		`<circle cx="22" cy="16" r="2" fill="white"/>` +
		`<circle cx="16" cy="22" r="2" fill="white"/>`,

	// ── Monitoring ────────────────────────────────────────────────────────

	// Log Analytics – bar chart
	"LAW": `<line x1="4" y1="28" x2="28" y2="28" stroke="white" stroke-width="2"/>` +
		`<line x1="4" y1="28" x2="4" y2="6" stroke="white" stroke-width="2"/>` +
		`<rect x="7" y="20" width="5" height="8" rx="1" fill="white" opacity="0.9"/>` +
		`<rect x="14" y="14" width="5" height="14" rx="1" fill="white" opacity="0.8"/>` +
		`<rect x="21" y="9" width="5" height="19" rx="1" fill="white" opacity="0.7"/>`,

	// Application Insights – lightbulb
	"AppIns": `<circle cx="16" cy="12" r="7" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="13" y1="19" x2="13" y2="22" stroke="white" stroke-width="1.5"/>` +
		`<line x1="19" y1="19" x2="19" y2="22" stroke="white" stroke-width="1.5"/>` +
		`<rect x="13" y="22" width="6" height="3" rx="1.5" fill="none" stroke="white" stroke-width="1.5"/>` +
		`<rect x="14" y="25" width="4" height="2" rx="1" fill="white"/>`,

	// Alert – bell
	"Alert": `<path d="M16,3 A7,7 0 0 1 23,10 L25,22 L7,22 L9,10 A7,7 0 0 1 16,3 Z" fill="none" stroke="white" stroke-width="2"/>` +
		`<line x1="7" y1="22" x2="25" y2="22" stroke="white" stroke-width="2"/>` +
		`<path d="M13,22 A3,3 0 0 0 19,22" fill="none" stroke="white" stroke-width="2"/>`,

	// Grafana – G letter in circle
	"Grafana": `<circle cx="16" cy="16" r="11" fill="none" stroke="white" stroke-width="2"/>` +
		`<path d="M22,13 L16,13 A5,5 0 1 0 16,21 L22,21 L22,17 L18,17" fill="none" stroke="white" stroke-width="2"/>`,
}

// IconSVG returns a 32×32 white SVG fragment for the given resource.
// If no specific icon is found for the short name, a generic text-label
// fallback is returned instead.
func IconSVG(r *model.Resource) string {
	key := strings.TrimSpace(r.ShortName)
	if shape, ok := iconShapes[key]; ok {
		return shape
	}
	// Generic fallback: first two uppercase letters
	abbr := abbreviate(r.ShortName)
	return `<text x="16" y="21" font-family="'Segoe UI',Arial,sans-serif" font-size="13" font-weight="bold" fill="white" text-anchor="middle">` + abbr + `</text>`
}

// abbreviate returns up to two uppercase letters for use as a generic icon label.
func abbreviate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return "?"
	}
	if len(s) <= 2 {
		return strings.ToUpper(s)
	}
	return strings.ToUpper(s[:2])
}
