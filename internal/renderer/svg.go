// Package renderer generates the SVG document from a laid-out Diagram.
package renderer

import (
	"fmt"
	"math"
	"strings"

	"github.com/kongou-ae/AzDiagram/internal/model"
)

// ── Colour palette ────────────────────────────────────────────────────────────

const (
	colorSubscriptionFill   = "#F0F6FF"
	colorSubscriptionStroke = "#0078D4"

	colorRGFill   = "#FFFFFF"
	colorRGStroke = "#212121"

	colorVNetFill   = "#E3EFF9"
	colorVNetStroke = "#1565C0"

	colorSubnetFill   = "#F0F6FF"
	colorSubnetStroke = "#0078D4"

	colorCardFill   = "#FFFFFF"
	colorCardStroke = "#CBD5E1"
	colorCardShadow = "#00000018"

	colorEdge     = "#64748B"
	colorEdgeHead = "#334155"

	colorLabel      = "#212121"
	colorLabelLight = "#212121"

	// Font stack matching Azure docs.
	fontFamily = "'Segoe UI', 'Helvetica Neue', Arial, sans-serif"
)

// globalIconRegistry holds the optional official Azure icon registry.
// Set once via SetIconRegistry before calling Render.
var globalIconRegistry *IconRegistry

// SetIconRegistry configures the renderer to use official Azure icons.
// Pass nil to revert to the built-in fallback icon shapes.
func SetIconRegistry(reg *IconRegistry) {
	globalIconRegistry = reg
}

// Render generates a complete SVG document string from a laid-out Diagram.
func Render(d *model.Diagram) string {
	var sb strings.Builder

	w := d.Width
	h := d.Height

	// ── SVG header ──────────────────────────────────────────────────────────
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&sb,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f">`+"\n",
		w, h, w, h)

	// Defs: arrowhead marker + drop-shadow filter.
	sb.WriteString(`<defs>
  <marker id="arrow" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">
    <polygon points="0 0, 10 3.5, 0 7" fill="` + colorEdgeHead + `"/>
  </marker>
  <filter id="shadow" x="-5%" y="-5%" width="110%" height="110%">
    <feDropShadow dx="1" dy="2" stdDeviation="3" flood-color="#00000020"/>
  </filter>
</defs>` + "\n")

	// Background.
	fmt.Fprintf(&sb,
		`<rect width="%.0f" height="%.0f" fill="%s"/>`, w, h, "#F8FAFC")
	sb.WriteString("\n")

	// ── VNet Peering lines ──────────────────────────────────────────────
	// Rendered before resource cards so they appear underneath.
	for i, p := range d.VNetPeerings {
		renderVNetPeering(&sb, p.Local, p.Remote, i)
	}

	// ── VNet containers ──────────────────────────────────────────────────────
	for _, vc := range d.VNets {
		renderVNet(&sb, vc)
	}

	// ── Standalone resource cards ─────────────────────────────────────────────
	for _, r := range d.StandaloneResources {
		// Render child resources (parent: relationship) to the left first.
		for _, child := range r.ChildResources {
			renderCard(&sb, child)
		}
		renderCard(&sb, r)
	}
	// Draw solid connectors between standalone parent and its child resources.
	for _, r := range d.StandaloneResources {
		for _, child := range r.ChildResources {
			// Horizontal solid line: parent's right edge → child's left edge, at mid-height.
			x1 := r.X + r.Width
			x2 := child.X
			midY := r.Y + r.Height/2
			fmt.Fprintf(&sb,
				`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1.5"/>`,
				x1, midY, x2, midY, colorEdge)
			sb.WriteByte('\n')
		}
	}

	// ── Edges ────────────────────────────────────────────────────────────────
	// Build a set of resources that are rendered as standalone cards or subnet
	// cards (i.e. have a real layout position). VNet container resources and
	// any resource whose Width is still 0 (never positioned) are excluded so
	// we don't draw arrows toward the top-left corner of the canvas.
	vnetSymbols := make(map[string]struct{}, len(d.VNets))
	for _, vc := range d.VNets {
		vnetSymbols[vc.Resource.SymbolicName] = struct{}{}
	}
	isRendered := func(r *model.Resource) bool {
		if r == nil {
			return false
		}
		if _, isVNet := vnetSymbols[r.SymbolicName]; isVNet {
			return false
		}
		return r.Width > 0
	}
	for _, e := range d.Edges {
		from := d.ResourcesBySymbol[e.From]
		to := d.ResourcesBySymbol[e.To]
		if isRendered(from) && isRendered(to) {
			renderEdge(&sb, from, to)
		}
	}

	// ── LB Connections (solid straight lines LB→VM) ────────────────────────
	for _, lbc := range d.LBConnections {
		lb := lbc.To
		vm := lbc.From
		if lb.Width == 0 || vm.Width == 0 {
			continue
		}
		// LB right edge → VM left edge.
		x1 := lb.X + lb.Width
		y1 := lb.Y + lb.Height/2
		x2 := vm.X
		y2 := vm.Y + vm.Height/2
		fmt.Fprintf(&sb,
			`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1.5"/>`+"\n",
			x1, y1, x2, y2, colorEdge)
	}

	// ── VNet Integration Connections (dashed lines: App Service bottom → subnet right edge) ────
	for _, vic := range d.VNetIntConnections {
		app := vic.App
		sc := vic.Subnet
		if app.Width == 0 || sc.Width == 0 {
			continue
		}
		// Start: bottom-centre of the App Service card.
		x1 := app.X + app.Width/2
		y1 := app.Y + app.Height
		// End: right edge of the subnet, vertically centred.
		x2 := sc.X + sc.Width
		y2 := sc.Y + sc.Height/2
		// L-shaped path: go down from app bottom, then horizontally to subnet right edge.
		path := fmt.Sprintf("M %.1f,%.1f V %.1f H %.1f", x1, y1, y2, x2)
		fmt.Fprintf(&sb,
			`<path d="%s" fill="none" stroke="%s" stroke-width="1.5"/>`+"\n",
			path, colorEdge)
		// "VNet Integration" label at the bend point.
		lx := (x1 + x2) / 2
		ly := y2 - 3
		const vniLabelW = 78.0
		const vniLabelH = 12.0
		fmt.Fprintf(&sb,
			`<rect x="%.1f" y="%.1f" width="%.0f" height="%.0f" rx="2" fill="%s" opacity="0.85"/>`+"\n",
			lx-vniLabelW/2, ly-vniLabelH+2, vniLabelW, vniLabelH, "#F8FAFC")
		fmt.Fprintf(&sb,
			`<text x="%.1f" y="%.1f" font-family=%q font-size="9" fill="%s" font-style="italic" text-anchor="middle">VNet Integration</text>`+"\n",
			lx, ly, fontFamily, colorEdge)
	}

	sb.WriteString("</svg>\n")
	return sb.String()
}

// ── Container renderers ───────────────────────────────────────────────────────

func renderSubscription(sb *strings.Builder, x, y, w, h float64) {
	fmt.Fprintf(sb,
		`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="12" fill="%s" stroke="%s" stroke-width="2" stroke-dasharray="8,4"/>`,
		x, y, w, h, colorSubscriptionFill, colorSubscriptionStroke)
	sb.WriteString("\n")
	// Label icon + text.
	fmt.Fprintf(sb,
		`<text x="%.1f" y="%.1f" font-family=%q font-size="12" fill="%s" font-weight="500">&#9632; Subscription</text>`+"\n",
		x+12, y+18, fontFamily, colorSubscriptionStroke)
}

func renderResourceGroup(sb *strings.Builder, x, y, w, h float64) {
	fmt.Fprintf(sb,
		`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="8" fill="%s" stroke="%s" stroke-width="1.5"/>`,
		x, y, w, h, colorRGFill, colorRGStroke)
	sb.WriteString("\n")
	fmt.Fprintf(sb,
		`<text x="%.1f" y="%.1f" font-family=%q font-size="12" fill="%s" font-weight="500">&#9632; Resource Group</text>`+"\n",
		x+10, y+16, fontFamily, colorRGStroke)
}

// renderVNetPeering draws a horizontal bidirectional dashed line between two
// VNets. In the Hub & Spoke layout the hub is to the LEFT and the spoke is to
// the RIGHT, so the line runs from the hub's right edge (at the spoke's
// vertical midpoint) to the spoke's left edge.
func renderVNetPeering(sb *strings.Builder, a, b *model.VNetContainer, idx int) {
	// left = hub (smaller X), right = spoke (larger X)
	left, right := a, b
	if b.X < a.X {
		left, right = b, a
	}

	// Connection points: right edge of hub at hub's midpoint,
	// left edge of spoke at spoke's midpoint.
	x1 := left.X + left.Width
	y1 := left.Y + left.Height/2
	x2 := right.X
	y2 := right.Y + right.Height/2

	// Draw an L-shaped connector: horizontal from hub right edge, then vertical,
	// then horizontal to spoke left edge. Midpoint X is halfway between.
	mx := (x1 + x2) / 2
	path := fmt.Sprintf("M %.1f,%.1f H %.1f V %.1f H %.1f", x1, y1, mx, y2, x2)
	fmt.Fprintf(sb,
		`<path d="%s" fill="none" stroke="%s" stroke-width="1.5"/>`+"\n",
		path, colorVNetStroke)

	// "Peering" label at the midpoint of the vertical segment (unique Y per peering).
	// This avoids labels clustering at the hub midpoint and overlapping with VNet boxes.
	labelX := mx
	labelY := (y1+y2)/2 + 4 // +4 = text baseline offset
	const peerLabelW = 38.0
	const peerLabelH = 12.0
	fmt.Fprintf(sb,
		`<rect x="%.1f" y="%.1f" width="%.0f" height="%.0f" rx="2" fill="%s" opacity="0.85"/>`,
		labelX-peerLabelW/2, labelY-peerLabelH+2, peerLabelW, peerLabelH, colorVNetFill)
	sb.WriteByte('\n')
	fmt.Fprintf(sb,
		`<text x="%.1f" y="%.1f" font-family=%q font-size="9" fill="%s" font-style="italic" text-anchor="middle">Peering</text>`+"\n",
		labelX, labelY, fontFamily, colorVNetStroke)
}

func renderVNet(sb *strings.Builder, vc *model.VNetContainer) {
	// VNet bounding box.
	fmt.Fprintf(sb,
		`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="8" fill="%s" stroke="%s" stroke-width="1.5"/>`,
		vc.X, vc.Y, vc.Width, vc.Height, colorVNetFill, colorVNetStroke)
	sb.WriteString("\n")

	// VNet label (top-left inside the box).
	labelX := vc.X + 12
	labelY := vc.Y + 18
	fmt.Fprintf(sb,
		`<text x="%.1f" y="%.1f" font-family=%q font-size="11" fill="%s" font-weight="600">VNet: %s</text>`+"\n",
		labelX, labelY, fontFamily, colorVNetStroke, escapeXML(vc.Resource.DisplayName))

	// Subnets inside the VNet.
	for _, sc := range vc.Subnets {
		renderSubnet(sb, sc, vc)
	}
}

func renderSubnet(sb *strings.Builder, sc *model.SubnetContainer, vc *model.VNetContainer) {
	// Subnet bounding box — sc.X, sc.Y are already absolute (set by layoutVNet).
	absX := sc.X
	absY := sc.Y

	fmt.Fprintf(sb,
		`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="6" fill="%s" stroke="%s" stroke-width="1" stroke-dasharray="5,3"/>`,
		absX, absY, sc.Width, sc.Height, colorSubnetFill, colorSubnetStroke)
	sb.WriteString("\n")

	// Subnet label (2 lines: name on first, address prefix on second).
	fmt.Fprintf(sb,
		`<text x="%.1f" y="%.1f" font-family=%q font-size="10" fill="%s">Subnet: %s</text>`+"\n",
		absX+8, absY+14, fontFamily, colorSubnetStroke, escapeXML(sc.Name))
	if sc.AddressPrefix != "" {
		fmt.Fprintf(sb,
			`<text x="%.1f" y="%.1f" font-family=%q font-size="9" fill="%s">%s</text>`+"\n",
			absX+8, absY+26, fontFamily, colorSubnetStroke, escapeXML(sc.AddressPrefix))
	}

	// Anchored LB (left column) — rendered before VM cards so it appears behind connections.
	if sc.AnchoredLB != nil {
		renderCard(sb, sc.AnchoredLB)
	}
	// Resources inside the subnet.
	for _, r := range sc.Resources {
		// The resource positions are set in absolute diagram coordinates by the layout engine.
		renderCard(sb, r)
	}

	// ── PE pairs: PE on left, linked service on right ──────────────────────
	for _, pair := range sc.PEPairs {
		renderCard(sb, pair.PE)
		if pair.LinkedService != nil {
			renderCard(sb, pair.LinkedService)
			// Solid connector between PE right edge and linked service left edge.
			x1 := pair.PE.X + pair.PE.Width
			x2 := pair.LinkedService.X
			midY := pair.PE.Y + pair.PE.Height/2
			fmt.Fprintf(sb,
				`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1.5"/>`+"\n",
				x1, midY, x2, midY, colorEdge)
			// Render child resources of the linked service (e.g. sqlDb → sqlServer).
			for _, child := range pair.LinkedService.ChildResources {
				renderCard(sb, child)
			}
			// Solid connectors: linked service right edge → each child left edge.
			for _, child := range pair.LinkedService.ChildResources {
				cx1 := pair.LinkedService.X + pair.LinkedService.Width
				cx2 := child.X
				cMidY := pair.LinkedService.Y + pair.LinkedService.Height/2
				fmt.Fprintf(sb,
					`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="1.5"/>`+"\n",
					cx1, cMidY, cx2, cMidY, colorEdge)
			}
		}
	}

	// ── Attached NSG bar ──────────────────────────────────────────────────
	if sc.AttachedNSG != nil {
		nsg := sc.AttachedNSG
		// When a RT bar also follows, the NSG bar sits one row higher.
		barY := absY + sc.Height - nsgBarH
		if sc.AttachedRT != nil {
			barY = absY + sc.Height - nsgBarH*2
		}
		// Divider line.
		fmt.Fprintf(sb,
			`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="0.5" stroke-dasharray="3,2"/>`+"\n",
			absX+4, barY, absX+sc.Width-4, barY, colorSubnetStroke)
		// Small NSG icon.
		const nsgIconSz = 14.0
		iconX := absX + 6
		iconY := barY + (nsgBarH-nsgIconSz)/2
		if content, vb, ok := globalIconRegistry.Lookup(nsg); ok {
			fmt.Fprintf(sb,
				`<svg x="%.1f" y="%.1f" width="%.0f" height="%.0f" viewBox="%s" preserveAspectRatio="xMidYMid meet">%s</svg>`+"\n",
				iconX, iconY, nsgIconSz, nsgIconSz, vb, content)
		}
		// NSG display name.
		label := truncate(nsg.DisplayName, 22)
		fmt.Fprintf(sb,
			`<text x="%.1f" y="%.1f" font-family=%q font-size="8" fill="%s">NSG: %s</text>`+"\n",
			iconX+nsgIconSz+3, barY+nsgBarH-6, fontFamily, colorLabelLight, escapeXML(label))
	}

	// ── Attached Route Table bar ──────────────────────────────────────────
	if sc.AttachedRT != nil {
		rt := sc.AttachedRT
		barY := absY + sc.Height - nsgBarH
		// Divider line.
		fmt.Fprintf(sb,
			`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="0.5" stroke-dasharray="3,2"/>`+"\n",
			absX+4, barY, absX+sc.Width-4, barY, colorSubnetStroke)
		// Small RT icon.
		const rtIconSz = 14.0
		iconX := absX + 6
		iconY := barY + (nsgBarH-rtIconSz)/2
		if content, vb, ok := globalIconRegistry.Lookup(rt); ok {
			fmt.Fprintf(sb,
				`<svg x="%.1f" y="%.1f" width="%.0f" height="%.0f" viewBox="%s" preserveAspectRatio="xMidYMid meet">%s</svg>`+"\n",
				iconX, iconY, rtIconSz, rtIconSz, vb, content)
		}
		// RT display name.
		label := truncate(rt.DisplayName, 22)
		fmt.Fprintf(sb,
			`<text x="%.1f" y="%.1f" font-family=%q font-size="8" fill="%s">UDR: %s</text>`+"\n",
			iconX+rtIconSz+3, barY+nsgBarH-6, fontFamily, colorLabelLight, escapeXML(label))
	}
}

// ── Resource card ─────────────────────────────────────────────────────────────

const (
	nsgBarH          = 22.0 // height of the NSG bar at the bottom of a subnet
	iconSize         = 19.0 // fallback badge size
	officialIconSize = 24.0 // official icon display size
	iconRadius       = 8.0
	cardRadius       = 8.0
)

func renderCard(sb *strings.Builder, r *model.Resource) {
	x, y, w, h := r.X, r.Y, r.Width, r.Height

	// Card shadow (offset rect).
	fmt.Fprintf(sb,
		`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.0f" fill="%s"/>`,
		x+2, y+3, w, h, cardRadius, colorCardShadow)
	sb.WriteString("\n")

	// Card background.
	fmt.Fprintf(sb,
		`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.0f" fill="%s" stroke="%s" stroke-width="1"/>`,
		x, y, w, h, cardRadius, colorCardFill, colorCardStroke)
	sb.WriteString("\n")

	centerX := x + w/2

	// Try official icon first; fall back to built-in badge icon.
	var nameY float64
	if content, vb, ok := globalIconRegistry.Lookup(r); ok {
		// ── Official Microsoft icon ──────────────────────────────────────
		// Embed as a nested <svg> so its viewBox scales it correctly.
		iconX := centerX - officialIconSize/2
		iconY := y + 6
		fmt.Fprintf(sb,
			`<svg x="%.1f" y="%.1f" width="%.0f" height="%.0f" viewBox="%s" preserveAspectRatio="xMidYMid meet">%s</svg>`+"\n",
			iconX, iconY, officialIconSize, officialIconSize, vb, content)
		nameY = iconY + officialIconSize + 10
	} else {
		// ── Fallback: coloured badge + white built-in shape ───────────────
		iconBgColor := r.IconColor
		if iconBgColor == "" {
			iconBgColor = "#78909C"
		}
		badgeX := centerX - iconSize/2
		badgeY := y + 8
		fmt.Fprintf(sb,
			`<rect x="%.1f" y="%.1f" width="%.0f" height="%.0f" rx="%.0f" fill="%s"/>`,
			badgeX, badgeY, iconSize, iconSize, iconRadius, iconBgColor)
		sb.WriteString("\n")

		scale := iconSize / 32.0
		fmt.Fprintf(sb,
			`<g transform="translate(%.2f,%.2f) scale(%.4f)">`,
			badgeX, badgeY, scale)
		sb.WriteString(IconSVG(r))
		sb.WriteString("</g>\n")

		nameY = badgeY + iconSize + 12
	}

	// Resource display name — compress only if wider than the card.
	{
		label := r.DisplayName
		const nameFontSize = 10.0
		const charW = 6.0           // approximate px per character at font-size 10
		const compressBuffer = 12.0 // only compress when clearly wider than card (avoids stretching near-threshold text)
		maxW := w - 16              // card width minus side padding
		naturalW := float64(len(label)) * charW
		if naturalW > maxW+compressBuffer {
			fmt.Fprintf(sb,
				`<text x="%.1f" y="%.1f" font-family=%q font-size="%.0f" fill="%s" text-anchor="middle" font-weight="500" textLength="%.1f" lengthAdjust="spacingAndGlyphs">%s</text>`+"\n",
				centerX, nameY, fontFamily, nameFontSize, colorLabel, maxW, escapeXML(label))
		} else {
			fmt.Fprintf(sb,
				`<text x="%.1f" y="%.1f" font-family=%q font-size="%.0f" fill="%s" text-anchor="middle" font-weight="500">%s</text>`+"\n",
				centerX, nameY, fontFamily, nameFontSize, colorLabel, escapeXML(label))
		}
	}

	// Resource type label (segment after provider prefix).
	typeLabel := r.TypeLabel
	if typeLabel == "" {
		typeLabel = r.ShortName
	}
	if r.Type == "microsoft.cognitiveservices/accounts" && r.Kind != "" {
		typeLabel = typeLabel + "/" + r.Kind
	}
	fmt.Fprintf(sb,
		`<text x="%.1f" y="%.1f" font-family=%q font-size="9" fill="%s" text-anchor="middle">%s</text>`+"\n",
		centerX, nameY+12, fontFamily, colorLabelLight, escapeXML(typeLabel))

	// ── Attached NIC row ───────────────────────────────────────────────────
	if len(r.AttachedNICs) > 0 {
		const inlineRowH = 22.0
		nsgTotalH := float64(len(r.AttachedNicNSGs)) * nsgBarH
		divY := y + r.Height - nsgTotalH - inlineRowH
		if len(r.AttachedPIPs) > 0 {
			divY = y + r.Height - nsgTotalH - inlineRowH*2
		}
		fmt.Fprintf(sb,
			`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="0.5" stroke-dasharray="3,2"/>`,
			x+8, divY, x+w-8, divY, colorCardStroke)
		sb.WriteString("\n")

		const nicIconSz = 14.0
		nicRowY := divY + 3
		curX := x + 6.0

		for i, nic := range r.AttachedNICs {
			if i > 0 {
				curX += 6
			}
			// Small NIC icon.
			if content, vb, ok := globalIconRegistry.Lookup(nic); ok {
				fmt.Fprintf(sb,
					`<svg x="%.1f" y="%.1f" width="%.0f" height="%.0f" viewBox="%s" preserveAspectRatio="xMidYMid meet">%s</svg>`+"\n",
					curX, nicRowY, nicIconSz, nicIconSz, vb, content)
			}
			curX += nicIconSz + 3
			// NIC display name — compress only if it exceeds the remaining card width.
			label := nic.DisplayName
			maxW := (x + w - 8) - curX // remaining width to right edge
			if maxW < 1 {
				maxW = 1
			}
			const charW = 4.5 // approximate px per character at font-size 8
			naturalW := float64(len(label)) * charW
			if naturalW > maxW {
				fmt.Fprintf(sb,
					`<text x="%.1f" y="%.1f" font-family=%q font-size="8" fill="%s" textLength="%.1f" lengthAdjust="spacingAndGlyphs">%s</text>`+"\n",
					curX, nicRowY+nicIconSz-2, fontFamily, colorLabelLight, maxW, escapeXML(label))
				curX += maxW + 2
			} else {
				fmt.Fprintf(sb,
					`<text x="%.1f" y="%.1f" font-family=%q font-size="8" fill="%s">%s</text>`+"\n",
					curX, nicRowY+nicIconSz-2, fontFamily, colorLabelLight, escapeXML(label))
				curX += naturalW + 2
			}
		}
	}

	// ── Attached PIP row ───────────────────────────────────────────────────
	// Firewall / VPN GW / Bastion: render a PIP row at the card bottom.
	if len(r.AttachedPIPs) > 0 {
		const inlineRowH = 22.0
		nsgTotalH := float64(len(r.AttachedNicNSGs)) * nsgBarH
		divY := y + r.Height - nsgTotalH - inlineRowH
		fmt.Fprintf(sb,
			`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="0.5" stroke-dasharray="3,2"/>`,
			x+8, divY, x+w-8, divY, colorCardStroke)
		sb.WriteString("\n")

		const pipIconSz = 14.0
		pipRowY := divY + 3
		curX := x + 6.0

		for i, pip := range r.AttachedPIPs {
			if i > 0 {
				curX += 6
			}
			if content, vb, ok := globalIconRegistry.Lookup(pip); ok {
				fmt.Fprintf(sb,
					`<svg x="%.1f" y="%.1f" width="%.0f" height="%.0f" viewBox="%s" preserveAspectRatio="xMidYMid meet">%s</svg>`+"\n",
					curX, pipRowY, pipIconSz, pipIconSz, vb, content)
			}
			curX += pipIconSz + 3
			label := pip.DisplayName
			maxW := (x + w - 8) - curX
			if maxW < 1 {
				maxW = 1
			}
			const charW = 4.5
			naturalW := float64(len(label)) * charW
			if naturalW > maxW {
				fmt.Fprintf(sb,
					`<text x="%.1f" y="%.1f" font-family=%q font-size="8" fill="%s" textLength="%.1f" lengthAdjust="spacingAndGlyphs">%s</text>`+"\n",
					curX, pipRowY+pipIconSz-2, fontFamily, colorLabelLight, maxW, escapeXML(label))
				curX += maxW + 2
			} else {
				fmt.Fprintf(sb,
					`<text x="%.1f" y="%.1f" font-family=%q font-size="8" fill="%s">%s</text>`+"\n",
					curX, pipRowY+pipIconSz-2, fontFamily, colorLabelLight, escapeXML(label))
				curX += naturalW + 2
			}
		}
	}

	// ── NIC NSG bars ──────────────────────────────────────────────────────────
	// Rendered at the very bottom of the VM card, one bar per distinct NIC NSG.
	if len(r.AttachedNicNSGs) > 0 {
		nNSGs := len(r.AttachedNicNSGs)
		const nsgIconSz = 14.0
		for i, nsg := range r.AttachedNicNSGs {
			barY := y + r.Height - float64(nNSGs-i)*nsgBarH
			// Divider line.
			fmt.Fprintf(sb,
				`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="%s" stroke-width="0.5" stroke-dasharray="3,2"/>`+"\n",
				x+4, barY, x+w-4, barY, colorCardStroke)
			// Small NSG icon.
			iconX := x + 6
			iconY := barY + (nsgBarH-nsgIconSz)/2
			if content, vb, ok := globalIconRegistry.Lookup(nsg); ok {
				fmt.Fprintf(sb,
					`<svg x="%.1f" y="%.1f" width="%.0f" height="%.0f" viewBox="%s" preserveAspectRatio="xMidYMid meet">%s</svg>`+"\n",
					iconX, iconY, nsgIconSz, nsgIconSz, vb, content)
			}
			label := truncate(nsg.DisplayName, 22)
			fmt.Fprintf(sb,
				`<text x="%.1f" y="%.1f" font-family=%q font-size="8" fill="%s">NSG: %s</text>`+"\n",
				iconX+nsgIconSz+3, barY+nsgBarH-6, fontFamily, colorLabelLight, escapeXML(label))
		}
	}
}

// ── Edge renderer ─────────────────────────────────────────────────────────────

func renderEdge(sb *strings.Builder, from, to *model.Resource) {
	// Use the centre-bottom of the source and centre-top of the target.
	// If the target is above, flip to centre-top → centre-bottom.
	x1 := from.X + from.Width/2
	y1 := from.Y + from.Height
	x2 := to.X + to.Width/2
	y2 := to.Y

	if to.Y+to.Height < from.Y {
		// Target is above source.
		y1 = from.Y
		y2 = to.Y + to.Height
	} else if math.Abs(from.Y-to.Y) < 20 {
		// Side-by-side: connect right edge of from to left edge of to (or vice versa).
		if from.X < to.X {
			x1 = from.X + from.Width
			y1 = from.Y + from.Height/2
			x2 = to.X
			y2 = to.Y + to.Height/2
		} else {
			x1 = from.X
			y1 = from.Y + from.Height/2
			x2 = to.X + to.Width
			y2 = to.Y + to.Height/2
		}
	}

	// Cubic bezier control points.
	dy := y2 - y1
	dx := x2 - x1
	cpY := math.Abs(dy) * 0.5
	if cpY < 30 {
		cpY = 30
	}
	cpX := math.Abs(dx) * 0.3

	var path string
	if math.Abs(dy) >= math.Abs(dx) {
		// Predominantly vertical.
		path = fmt.Sprintf("M %.1f,%.1f C %.1f,%.1f %.1f,%.1f %.1f,%.1f",
			x1, y1,
			x1, y1+cpY,
			x2, y2-cpY,
			x2, y2)
	} else {
		// Predominantly horizontal.
		path = fmt.Sprintf("M %.1f,%.1f C %.1f,%.1f %.1f,%.1f %.1f,%.1f",
			x1, y1,
			x1+cpX, y1,
			x2-cpX, y2,
			x2, y2)
	}

	fmt.Fprintf(sb,
		`<path d="%s" fill="none" stroke="%s" stroke-width="1.5" marker-end="url(#arrow)" opacity="0.7"/>`+"\n",
		path, colorEdge)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// escapeXML replaces special characters that are invalid inside SVG text / attribute values.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// truncate shortens a string to n runes, appending "…" if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}
