// SPDX-License-Identifier: AGPL-3.0-only

// Command datumctl-inventory is a datumctl plugin for the Datum Cloud inventory
// property graph (typed Nodes and Edges with attribute bags). Invoked as
// `datumctl inventory ...` once installed.
package main

import (
	"os"

	"github.com/spf13/cobra"
	"go.datum.net/datumctl/plugin"
)

// version is set via -ldflags at build time; it feeds the plugin manifest.
var version = "dev"

func main() {
	plugin.ServeManifest(plugin.Manifest{
		Name:          "inventory",
		Version:       version,
		Description:   "Browse and populate the Datum Cloud inventory graph (typed nodes and edges)",
		APIVersion:    1,
		MinAPIVersion: 1,
	})

	root := &cobra.Command{
		Use:   "inventory",
		Short: "Browse the Datum Cloud inventory graph",
		Long: `Browse and populate the Datum Cloud inventory, modeled as a property graph:
typed Nodes (Region, Site, Cluster, Provider, Host, ...) connected by typed
Edges (located-in, member-of, provided-by, ...), each carrying an attribute
bag. The available types and their attributes live in the NodeType/EdgeType
schema registry.

Use 'get <TYPE>' to list nodes of an asset class, 'get edges' for
relationships, 'types' to browse the schema registry, 'tree' and 'neighbors'
to walk the graph, and 'apply' to populate it.

Inventory lives on the Datum Cloud platform root, so these commands talk to the
platform API directly; they do not take an organization or project scope.`,
		Example: `  # Nodes of a type, and the schema registry
  datumctl inventory get Site
  datumctl inventory types

  # Relationships and traversal
  datumctl inventory get edges --type=located-in
  datumctl inventory neighbors site-dfw1 --edge=located-in
  datumctl inventory tree

  # Counts, and populating the graph
  datumctl inventory summary
  datumctl inventory apply -f graph.yaml`,
		SilenceUsage: true,
	}
	root.PersistentFlags().StringP("output", "o", "table", "Output format. One of: table, json, yaml.")

	root.AddCommand(
		newGetCmd(),
		newTypesCmd(),
		newNeighborsCmd(),
		newTreeCmd(),
		newSummaryCmd(),
		newApplyCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
