// Package bicep provides a best-effort regex-based parser for Bicep source files.
// It handles the most common resource declaration patterns without requiring
// the full Bicep compiler toolchain.
package bicep

import (
	"regexp"
	"strings"

	"github.com/kongou-ae/AzDiagram/internal/model"
)

// ---- compiled regexes ----

var (
	// Matches: resource <symName> '<Type>@<apiVersion>' =
	reDeclare = regexp.MustCompile(
		`(?m)^[ \t]*resource\s+(\w+)\s+'([^@']+)@([^']+)'\s*=`)

	// Matches the first  name: 'literal'  line
	reNameLit = regexp.MustCompile(`(?m)^\s*name:\s*'([^']+)'`)

	// Matches the first  name: identifier  line (without quotes)
	reNameVar = regexp.MustCompile(`(?m)^\s*name:\s*([a-zA-Z_]\w*)`)

	// Matches  kind: 'value'
	reKind = regexp.MustCompile(`(?m)^\s*kind:\s*'([^']+)'`)

	// Matches dependsOn: [ ... ]  (non-greedy, handles line breaks)
	reDependsOn = regexp.MustCompile(`(?s)dependsOn:\s*\[([^\]]*)\]`)

	// Matches subnet name inside a subnets array:  name: 'subnet-name'
	reSubnetName = regexp.MustCompile(`name:\s*'([^']+)'`)

	// Matches addressPrefix: '...' inside a subnet block
	reSubnetAddr = regexp.MustCompile(`addressPrefix(?:es)?:\s*'([^']+)'`)

	// Matches ipConfigurations subnet reference: '${vnetSym.id}/subnets/SubnetName'
	// Captures: [1] VNet symbolic name, [2] subnet name (allows hyphens, e.g. "app-subnet")
	reIPConfigSubnet = regexp.MustCompile(`\$\{(\w+)\.id\}/subnets/([\w-]+)`)

	// Matches parent: symbolName  (child resource declaration)
	// Captures: [1] parent symbolic name
	reParent = regexp.MustCompile(`(?m)^\s*parent:\s*(\w+)`)

	// Matches remoteVirtualNetwork reference inside a peering resource body:
	//   remoteVirtualNetwork: { id: remoteVnetSym.id }
	// Captures: [1] remote VNet symbolic name
	reRemoteVNet = regexp.MustCompile(`remoteVirtualNetwork:\s*\{[^}]*id:\s*(\w+)\.id`)

	// Matches subnet-level NSG association inside a VNet subnets block:
	//   networkSecurityGroup: { id: nsgSym.id }
	// Captures: [1] NSG symbolic name
	reSubnetNSG = regexp.MustCompile(`networkSecurityGroup:\s*\{[^}]*id:\s*(\w+)\.id`)

	// Matches subnet-level Route Table association inside a VNet subnets block:
	//   routeTable: { id: rtSym.id }
	// Captures: [1] Route Table symbolic name
	reSubnetRT = regexp.MustCompile(`routeTable:\s*\{[^}]*id:\s*(\w+)\.id`)

	// Matches privateLinkServiceId inside a privateEndpoints resource body:
	//   privateLinkServiceId: sqlServer.id
	// Captures: [1] linked service symbolic name
	rePELinkedService = regexp.MustCompile(`privateLinkServiceId:\s*(\w+)\.id`)

	// Matches loadBalancerBackendAddressPools resourceId references in NIC ipConfigurations:
	//   resourceId('Microsoft.Network/loadBalancers/backendAddressPools', lbSym.name, 'pool')
	// Captures: [1] LB symbolic name
	reLBBackendPool = regexp.MustCompile(`resourceId\s*\(\s*'Microsoft\.Network/loadBalancers/backendAddressPools'\s*,\s*(\w+)\.name`)

	// Matches serverFarmId property inside microsoft.web/sites body:
	//   serverFarmId: planSym.id
	// Captures: [1] App Service Plan symbolic name
	reServerFarmID = regexp.MustCompile(`serverFarmId:\s*(\w+)\.id`)

	// Matches virtualNetwork: { id: vnetSym.id } inside a privateDnsZones/virtualNetworkLinks body.
	// (?m)^\s*virtualNetwork: anchors at line start to avoid matching the
	// substring inside `remoteVirtualNetwork:` used by VNet peerings.
	// Captures: [1] target VNet symbolic name
	reDNSLinkVNet = regexp.MustCompile(`(?m)^\s*virtualNetwork:\s*\{[^}]*id:\s*(\w+)\.id`)
)

// ParseFile parses a Bicep source file and returns a slice of model.Resource.
func ParseFile(content string) ([]*model.Resource, error) {
	// First pass: collect all symbolic names so we can detect implicit deps.
	symbolNames := collectSymbolNames(content)

	// Second pass: extract each resource declaration.
	matches := reDeclare.FindAllStringSubmatchIndex(content, -1)

	resources := make([]*model.Resource, 0, len(matches))

	for _, idx := range matches {
		symName := content[idx[2]:idx[3]]
		resType := content[idx[4]:idx[5]]
		apiVer := content[idx[6]:idx[7]]

		// Extract the resource body (brace-delimited block after '=').
		body := extractBody(content, idx[1])

		// Build the resource.
		res := &model.Resource{
			SymbolicName: symName,
			Type:         strings.ToLower(strings.TrimSpace(resType)),
			APIVersion:   strings.TrimSpace(apiVer),
		}
		// TypeLabel: the segment after the last '/' in the original type string, preserving casing.
		if idx := strings.LastIndex(strings.TrimSpace(resType), "/"); idx >= 0 {
			res.TypeLabel = strings.TrimSpace(resType)[idx+1:]
		} else {
			res.TypeLabel = strings.TrimSpace(resType)
		}

		// Display name: look for name: before properties: to avoid nested matches.
		res.DisplayName = extractResourceName(body, symName)

		// Kind property.
		if m := reKind.FindStringSubmatch(body); m != nil {
			res.Kind = strings.ToLower(m[1])
		}

		// Explicit dependsOn.
		res.Dependencies = extractDependsOn(body)

		// parent: declaration — prepend the parent symbol so it appears first.
		if m := reParent.FindStringSubmatch(body); m != nil {
			parentSym := m[1]
			res.ParentSymbol = parentSym
			if !containsStr(res.Dependencies, parentSym) {
				res.Dependencies = append([]string{parentSym}, res.Dependencies...)
			}
		}

		// Implicit dependencies: references to other symbolic names in the body.
		for _, sym := range symbolNames {
			if sym == symName {
				continue
			}
			if referencesSymbol(body, sym) {
				if !containsStr(res.Dependencies, sym) {
					res.Dependencies = append(res.Dependencies, sym)
				}
			}
		}

		// Subnets for VNet resources.
		if strings.Contains(res.Type, "virtualnetworks") &&
			!strings.Contains(res.Type, "/subnets") {
			res.Subnets = extractSubnets(body)
			res.Subnets = attachSubnetBadges(body, res.Subnets)
		}

		// ipConfigurations subnet reference (Firewall, VPN GW, Bastion, etc.).
		if m := reIPConfigSubnet.FindStringSubmatch(body); m != nil {
			res.SubnetRef = &model.SubnetRefDef{
				VNetSymbol: m[1],
				SubnetName: m[2],
			}
		}

		// VNet peering: store remote VNet symbol in SubnetRef.VNetSymbol (reused field)
		// and mark with a sentinel SubnetName so builder can identify it.
		if strings.Contains(res.Type, "virtualnetworkpeerings") {
			if m := reRemoteVNet.FindStringSubmatch(body); m != nil {
				res.SubnetRef = &model.SubnetRefDef{
					VNetSymbol: m[1],
					SubnetName: "__peering__",
				}
			}
		}

		// Private endpoint: record the linked service symbol.
		if strings.Contains(res.Type, "microsoft.network/privateendpoints") {
			if m := rePELinkedService.FindStringSubmatch(body); m != nil {
				res.LinkedServiceSymbol = m[1]
			}
		}

		// Private DNS Zone VNet Link: record the target VNet symbolic name.
		if res.Type == "microsoft.network/privatednszones/virtualnetworklinks" {
			if m := reDNSLinkVNet.FindStringSubmatch(body); m != nil {
				res.LinkedServiceSymbol = m[1]
			}
		}

		// App Service: treat serverFarmId as a parent reference so the site
		// is placed to the right of its App Service Plan in the diagram.
		// Also extract virtualNetworkSubnetId for VNet integration connector.
		if res.Type == "microsoft.web/sites" && res.ParentSymbol == "" {
			if m := reServerFarmID.FindStringSubmatch(body); m != nil {
				res.ParentSymbol = m[1]
			}
		}
		if res.Type == "microsoft.web/sites" {
			if m := reIPConfigSubnet.FindStringSubmatch(body); m != nil {
				res.VNetIntSubnet = &model.SubnetRefDef{
					VNetSymbol: m[1],
					SubnetName: m[2],
				}
			}
		}

		// NIC: record Load Balancer backend pool associations and NIC-level NSG.
		if strings.Contains(res.Type, "microsoft.network/networkinterfaces") {
			for _, m := range reLBBackendPool.FindAllStringSubmatch(body, -1) {
				if !containsStr(res.LBSymbols, m[1]) {
					res.LBSymbols = append(res.LBSymbols, m[1])
				}
			}
			if m := reSubnetNSG.FindStringSubmatch(body); m != nil {
				res.NICNSGSymbol = m[1]
			}
		}

		resources = append(resources, res)
	}

	return resources, nil
}

// attachSubnetBadges scans each subnet block inside the VNet body for
// networkSecurityGroup and routeTable references, storing the symbolic names.
func attachSubnetBadges(vnetBody string, subnets []model.SubnetDef) []model.SubnetDef {
	// Split the subnets array out of the body by finding the subnets: [ ... ] block.
	subnetsBlockRe := regexp.MustCompile(`(?s)subnets:\s*\[(.*)\]`)
	m := subnetsBlockRe.FindStringSubmatch(vnetBody)
	if m == nil {
		return subnets
	}
	block := m[1]
	// Each subnet is a brace-delimited object; scan sequentially in the same
	// order as subnets[] so we can match by index.
	depth := 0
	start := -1
	idx := 0
	for i := 0; i < len(block) && idx < len(subnets); i++ {
		switch block[i] {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				subBlock := block[start : i+1]
				if nsgM := reSubnetNSG.FindStringSubmatch(subBlock); nsgM != nil {
					subnets[idx].NSGSymbol = nsgM[1]
				}
				if rtM := reSubnetRT.FindStringSubmatch(subBlock); rtM != nil {
					subnets[idx].RTSymbol = rtM[1]
				}
				start = -1
				idx++
			}
		}
	}
	return subnets
}

// collectSymbolNames returns every symbolic name declared with "resource … =".
func collectSymbolNames(content string) []string {
	matches := reDeclare.FindAllStringSubmatch(content, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	return names
}

// extractBody locates the opening '{' at or after startPos and returns the
// complete brace-delimited block (including the outer braces).
func extractBody(content string, startPos int) string {
	i := startPos
	for i < len(content) && content[i] != '{' {
		i++
	}
	if i >= len(content) {
		return ""
	}
	depth := 0
	start := i
	for i < len(content) {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
		i++
	}
	return content[start:] // unterminated – return what we have
}

// extractResourceName finds the top-level name property, ignoring any nested
// name properties that appear inside properties: { ... }.
func extractResourceName(body, fallback string) string {
	// Restrict search to the part of the body before "properties:" to avoid
	// matching nested name fields.
	needle := body
	if idx := strings.Index(body, "properties:"); idx != -1 {
		needle = body[:idx]
	}

	if m := reNameLit.FindStringSubmatch(needle); m != nil {
		return m[1]
	}
	if m := reNameVar.FindStringSubmatch(needle); m != nil {
		v := m[1]
		// Skip keywords that aren't meaningful names.
		if v != "location" && v != "kind" && v != "sku" && v != "tags" {
			return v
		}
	}
	return fallback
}

// extractDependsOn parses the dependsOn: [...] block and returns
// the list of symbolic names referenced.
func extractDependsOn(body string) []string {
	m := reDependsOn.FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	var deps []string
	for _, part := range strings.Split(m[1], "\n") {
		// Each line may look like: "  symName" or "  symName,"
		tok := strings.TrimSpace(part)
		tok = strings.Trim(tok, ",")
		tok = strings.Trim(tok, "[]'\" \t\r")
		if tok != "" && isIdentifier(tok) {
			deps = append(deps, tok)
		}
	}
	return deps
}

// referencesSymbol reports whether body contains a reference to sym
// in the form "sym." or "sym[".
func referencesSymbol(body, sym string) bool {
	return strings.Contains(body, sym+".") ||
		strings.Contains(body, sym+"[")
}

// extractSubnets parses the subnets array from a VNet resource body.
func extractSubnets(body string) []model.SubnetDef {
	// Locate "subnets: [" or "subnets:[".
	subnetsIdx := strings.Index(body, "subnets:")
	if subnetsIdx == -1 {
		return nil
	}

	// Find the opening '[' for the subnets array.
	arrStart := strings.Index(body[subnetsIdx:], "[")
	if arrStart == -1 {
		return nil
	}
	arrStart += subnetsIdx

	// Extract the array content up to the matching ']'.
	arrContent := extractArray(body, arrStart)
	if arrContent == "" {
		return nil
	}

	// Split the array into individual subnet objects by splitting on outer '{'.
	subnetBlocks := splitOuterObjects(arrContent)

	var subnets []model.SubnetDef
	for _, block := range subnetBlocks {
		nameMatches := reSubnetName.FindAllStringSubmatch(block, -1)
		addrMatches := reSubnetAddr.FindAllStringSubmatch(block, -1)

		// Use only the first name: match — it is the subnet name itself.
		// Subsequent matches (e.g. inside delegations[]) are nested object names.
		if len(nameMatches) == 0 {
			continue
		}
		sd := model.SubnetDef{Name: nameMatches[0][1]}
		if len(addrMatches) > 0 {
			sd.AddressPrefix = addrMatches[0][1]
		}
		subnets = append(subnets, sd)
	}
	return subnets
}

// extractArray returns the bracket-delimited content starting at pos.
func extractArray(content string, pos int) string {
	depth := 0
	start := pos
	for i := pos; i < len(content); i++ {
		switch content[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
	}
	return ""
}

// splitOuterObjects splits array content into individual top-level {…} blocks.
func splitOuterObjects(content string) []string {
	var blocks []string
	depth := 0
	start := -1
	for i, ch := range content {
		switch ch {
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start != -1 {
				blocks = append(blocks, content[start:i+1])
				start = -1
			}
		}
	}
	return blocks
}

// isIdentifier reports whether s is a valid Go/Bicep identifier.
func isIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, ch := range s {
		if i == 0 {
			if !isLetter(ch) {
				return false
			}
		} else {
			if !isLetter(ch) && (ch < '0' || ch > '9') && ch != '_' {
				return false
			}
		}
	}
	return true
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
