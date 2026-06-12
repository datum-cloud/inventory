// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	inventoryv1alpha2 "go.miloapis.com/inventory/api/v1alpha2"
)

func newTypesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "types",
		Short: "List node and edge types (the schema registry)",
		Long: `List the NodeType and EdgeType definitions that make up the inventory schema
registry. Each type declares the attribute keys its nodes or edges may carry;
'datumctl inventory get <TYPE>' uses these to choose columns.`,
		Example: `  datumctl inventory types
  datumctl inventory types -o yaml`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			var nodeTypes inventoryv1alpha2.NodeTypeList
			if err := c.List(ctx, &nodeTypes); err != nil {
				return listErr("nodetypes", err)
			}
			var edgeTypes inventoryv1alpha2.EdgeTypeList
			if err := c.List(ctx, &edgeTypes); err != nil {
				return listErr("edgetypes", err)
			}
			sort.Slice(nodeTypes.Items, func(i, j int) bool { return nodeTypes.Items[i].Name < nodeTypes.Items[j].Name })
			sort.Slice(edgeTypes.Items, func(i, j int) bool { return edgeTypes.Items[i].Name < edgeTypes.Items[j].Name })

			format, _ := cmd.Flags().GetString("output")
			switch format {
			case "json":
				return writeMarshaled(cmd, json.MarshalIndent, nodeTypes, edgeTypes)
			case "yaml":
				return writeMarshaled(cmd, func(v any, _, _ string) ([]byte, error) { return yaml.Marshal(v) }, nodeTypes, edgeTypes)
			case "", "table":
				rows := make([][]string, 0, len(nodeTypes.Items)+len(edgeTypes.Items))
				for _, nt := range nodeTypes.Items {
					rows = append(rows, []string{"NodeType", nt.Name, orNone(nt.Spec.DisplayName), attrKeys(nodeAttrKeys(nt))})
				}
				for _, et := range edgeTypes.Items {
					rows = append(rows, []string{"EdgeType", et.Name, orNone(et.Spec.DisplayName), attrKeys(edgeAttrKeys(et))})
				}
				return printTable(cmd.OutOrStdout(), []string{"KIND", "NAME", "DISPLAY", "ATTRIBUTES"}, rows)
			default:
				return fmt.Errorf("invalid value %q for --output; allowed: table, json, yaml", format)
			}
		},
	}
}

func nodeAttrKeys(nt inventoryv1alpha2.NodeType) []string {
	keys := make([]string, 0, len(nt.Spec.Attributes))
	for _, a := range nt.Spec.Attributes {
		k := a.Key
		if a.Required {
			k += "*"
		}
		keys = append(keys, k)
	}
	return keys
}

func edgeAttrKeys(et inventoryv1alpha2.EdgeType) []string {
	keys := make([]string, 0, len(et.Spec.Attributes))
	for _, a := range et.Spec.Attributes {
		k := a.Key
		if a.Required {
			k += "*"
		}
		keys = append(keys, k)
	}
	return keys
}

func attrKeys(keys []string) string {
	if len(keys) == 0 {
		return none
	}
	return strings.Join(keys, ", ")
}

func writeMarshaled(cmd *cobra.Command, marshal func(any, string, string) ([]byte, error), nodeTypes inventoryv1alpha2.NodeTypeList, edgeTypes inventoryv1alpha2.EdgeTypeList) error {
	payload := map[string]any{"nodeTypes": nodeTypes.Items, "edgeTypes": edgeTypes.Items}
	b, err := marshal(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), strings.TrimRight(string(b), "\n"))
	return err
}
