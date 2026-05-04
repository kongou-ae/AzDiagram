package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/kongou-ae/AzDiagram/internal/generator"

	"github.com/spf13/cobra"
)

// defaultIncludeTypes is the set of resource types rendered by default.
// Pass --all to bypass this filter and render every resource in the template.
const defaultIncludeTypes = "Microsoft.Network/virtualNetworks," +
	"Microsoft.Network/virtualNetworks/virtualNetworkPeerings," +
	"Microsoft.Network/publicIPAddresses," +
	"Microsoft.Network/azureFirewalls," +
	"Microsoft.Network/virtualNetworkGateways," +
	"Microsoft.Network/bastionHosts," +
	"Microsoft.Network/routeTables," +
	"Microsoft.Network/networkSecurityGroups," +
	"Microsoft.Network/loadBalancers," +
	"Microsoft.Network/networkInterfaces," +
	"Microsoft.Network/privateEndpoints," +
	"Microsoft.Network/privateDnsZones," +
	"Microsoft.Network/privateDnsZones/virtualNetworkLinks," +
	"Microsoft.Compute/virtualMachines," +
	"Microsoft.KeyVault/vaults," +
	"Microsoft.Sql/servers," +
	"Microsoft.Sql/servers/databases," +
	"Microsoft.Storage/storageAccounts," +
	"Microsoft.OperationalInsights/workspaces," +
	"Microsoft.CognitiveServices/accounts," +
	"Microsoft.Web/serverfarms," +
	"Microsoft.Web/sites"

var (
	outputFile   string
	iconsDir     string
	includeTypes string
	allTypes     bool
)

var rootCmd = &cobra.Command{
	Use:   "azdiagram [flags] <bicep-or-json-file>",
	Short: "Generate Azure architecture diagrams from Bicep templates",
	Long: `AzDiagram converts Azure Bicep templates (or ARM JSON templates) into
SVG architecture diagrams similar to those in the Azure official documentation.

Examples:
  azdiagram main.bicep
	azdiagram -o arch.svg main.bicep
	azdiagram -i ./icons main.bicep
  azdiagram azuredeploy.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := outputFile
		if out == "" {
			out = "diagram-" + time.Now().Format("20060102150405") + ".svg"
		}
		types := includeTypes
		if allTypes {
			types = ""
		} else if types == "" {
			types = defaultIncludeTypes
		}
		return generator.Generate(generator.Options{
			InputFile:    args[0],
			OutputFile:   out,
			IconsDir:     iconsDir,
			IncludeTypes: types,
		})
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output SVG file path (default: diagram-YYYYMMDDHHMMSS.svg in current directory)")
	rootCmd.Flags().StringVarP(&iconsDir, "icons-dir", "i", "",
		"Directory containing official Microsoft Azure Architecture SVG icons.\n"+
			"Download the icon pack from: https://learn.microsoft.com/azure/architecture/icons/\n"+
			"Extract the ZIP and pass the root folder, e.g. -i ./azure-icons")
	rootCmd.Flags().BoolVar(&allTypes, "all", false, "Render all resource types found in the template (overrides --include-types and the default filter)")
	rootCmd.Flags().StringVar(&includeTypes, "include-types", "",
		"Comma-separated list of resource types to include (case-insensitive).\n"+
			"Defaults to the standard set of network/compute/shared-service types.\n"+
			"Use --all to render every resource in the template.\n"+
			"Example: --include-types Microsoft.Compute/virtualMachines,Microsoft.Network/virtualNetworks")
}
