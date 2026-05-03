// Package generator orchestrates parsing → building → layout → rendering.
package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kongou-ae/AzDiagram/internal/bicep"
	"github.com/kongou-ae/AzDiagram/internal/diagram"
	"github.com/kongou-ae/AzDiagram/internal/model"
	"github.com/kongou-ae/AzDiagram/internal/renderer"
)

// Options controls the generation pipeline.
type Options struct {
	InputFile  string
	OutputFile string
	IconsDir   string
	// IncludeTypes is a comma-separated list of resource types to include.
	// If empty, all types are included.
	IncludeTypes string
}

// GenerateSVG runs the full pipeline and returns the SVG string.
// OutputFile is not used; use Generate to also write to disk.
func GenerateSVG(opts Options) (string, error) {
	// ── 1. Parse resources ──────────────────────────────────────────────────
	resources, err := parseInput(opts)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if len(resources) == 0 {
		return "", fmt.Errorf("no Azure resources found in %q", opts.InputFile)
	}

	// ── 2.5. Filter by included types ──────────────────────────────────────
	if opts.IncludeTypes != "" {
		allowed := make(map[string]bool)
		for _, t := range strings.Split(opts.IncludeTypes, ",") {
			allowed[strings.ToLower(strings.TrimSpace(t))] = true
		}
		filtered := resources[:0]
		for _, r := range resources {
			if allowed[r.Type] {
				// CognitiveServices/accounts is further filtered by kind.
				if r.Type == "microsoft.cognitiveservices/accounts" && r.Kind != "openai" {
					continue
				}
				filtered = append(filtered, r)
			}
		}
		resources = filtered
		fmt.Fprintf(os.Stderr, "AzDiagram: after type filter: %d resource(s)\n", len(resources))
	}

	fmt.Fprintf(os.Stderr, "AzDiagram: found %d resource(s)\n", len(resources))

	// ── 2. Build diagram model ─────────────────────────────────────────
	d := diagram.Build(resources)

	// ── 4. Layout ───────────────────────────────────────────────────────────
	diagram.Layout(d)

	// ── 5. Load official icons (optional) ──────────────────────────────────
	if opts.IconsDir != "" {
		reg, err := renderer.NewIconRegistry(opts.IconsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "AzDiagram: warning: could not load icons dir: %v\n", err)
		} else {
			renderer.SetIconRegistry(reg)
			fmt.Fprintf(os.Stderr, "AzDiagram: loaded %d official icon(s) from %s\n", reg.Count(), opts.IconsDir)
		}
	}

	// ── 6. Render to SVG ────────────────────────────────────────────────────
	return renderer.Render(d), nil
}

// Generate runs the full pipeline and writes the SVG to OutputFile.
func Generate(opts Options) error {
	svg, err := GenerateSVG(opts)
	if err != nil {
		return err
	}

	// ── 7. Write output ─────────────────────────────────────────────────────
	if err := os.WriteFile(opts.OutputFile, []byte(svg), 0644); err != nil {
		return fmt.Errorf("write %s: %w", opts.OutputFile, err)
	}

	fmt.Fprintf(os.Stderr, "AzDiagram: wrote %s\n", opts.OutputFile)
	return nil
}

// parseInput selects the appropriate parser based on file extension and options.
func parseInput(opts Options) ([]*model.Resource, error) {
	ext := strings.ToLower(filepath.Ext(opts.InputFile))

	switch {
	case ext == ".bicep":
		return parseBicepDirect(opts.InputFile)

	case ext == ".json":
		return parseARMJSON(opts.InputFile)

	default:
		return nil, fmt.Errorf("unsupported input file extension %q (supported: .bicep, .json)", ext)
	}
}

// parseBicepDirect reads a .bicep file and uses the regex-based parser.
func parseBicepDirect(path string) ([]*model.Resource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bicep.ParseFile(string(data))
}

// armTemplate is a minimal representation of an ARM template for parsing.
type armTemplate struct {
	Resources []armResource `json:"resources"`
}

type armResource struct {
	Type       string          `json:"type"`
	Name       string          `json:"name"`
	APIVersion string          `json:"apiVersion"`
	Kind       string          `json:"kind"`
	DependsOn  []string        `json:"dependsOn"`
	Properties json.RawMessage `json:"properties"`
}

type armVNetProperties struct {
	Subnets []struct {
		Name       string `json:"name"`
		Properties struct {
			AddressPrefix string `json:"addressPrefix"`
		} `json:"properties"`
	} `json:"subnets"`
}

// parseARMJSON parses an ARM JSON template file.
func parseARMJSON(path string) ([]*model.Resource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tmpl armTemplate
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("invalid ARM JSON: %w", err)
	}

	resources := make([]*model.Resource, 0, len(tmpl.Resources))

	for i, r := range tmpl.Resources {
		symName := fmt.Sprintf("resource%d_%s", i, slugify(r.Type))
		res := &model.Resource{
			SymbolicName: symName,
			Type:         strings.ToLower(r.Type),
			APIVersion:   r.APIVersion,
			DisplayName:  armExtractName(r.Name),
			Kind:         strings.ToLower(r.Kind),
		}

		// Resolve dependsOn resource IDs to symbolic names.
		for _, dep := range r.DependsOn {
			if depSym := resolveARMDep(dep, tmpl.Resources); depSym != "" {
				res.Dependencies = append(res.Dependencies, depSym)
			}
		}

		// Parse subnets for VNet resources.
		if strings.Contains(strings.ToLower(r.Type), "virtualnetworks") &&
			!strings.Contains(strings.ToLower(r.Type), "/subnets") {
			if r.Properties != nil {
				var props armVNetProperties
				if err := json.Unmarshal(r.Properties, &props); err == nil {
					for _, s := range props.Subnets {
						res.Subnets = append(res.Subnets, model.SubnetDef{
							Name:          s.Name,
							AddressPrefix: s.Properties.AddressPrefix,
						})
					}
				}
			}
		}

		resources = append(resources, res)
	}

	return resources, nil
}

// armExtractName strips ARM expression wrappers like [parameters('x')] or [variables('x')]
// and returns a human-readable name.
func armExtractName(name string) string {
	name = strings.TrimSpace(name)
	// Simple heuristic: extract what's inside the first single-quoted string.
	if start := strings.Index(name, "'"); start != -1 {
		if end := strings.LastIndex(name, "'"); end > start {
			return name[start+1 : end]
		}
	}
	return name
}

// resolveARMDep attempts to match an ARM resourceId() dependency expression
// against the list of resources, returning the symbolic name if found.
func resolveARMDep(dep string, resources []armResource) string {
	// Extract the type from [resourceId('Type', 'Name')] patterns.
	lower := strings.ToLower(dep)
	for i, r := range resources {
		t := strings.ToLower(r.Type)
		n := strings.ToLower(armExtractName(r.Name))
		if strings.Contains(lower, t) && (n == "" || strings.Contains(lower, n)) {
			return fmt.Sprintf("resource%d_%s", i, slugify(r.Type))
		}
	}
	return ""
}

// slugify converts a resource type like "Microsoft.Network/virtualNetworks"
// into a safe identifier fragment like "network_virtualnetworks".
func slugify(s string) string {
	parts := strings.Split(strings.ToLower(s), "/")
	if len(parts) >= 2 {
		// Drop the "microsoft." prefix.
		provider := strings.TrimPrefix(parts[0], "microsoft.")
		return provider + "_" + strings.Join(parts[1:], "_")
	}
	return strings.ReplaceAll(strings.ToLower(s), "/", "_")
}
