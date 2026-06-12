// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	inventoryv1alpha2 "go.miloapis.com/inventory/api/v1alpha2"
)

func listErr(what string, err error) error {
	return fmt.Errorf("could not list %s: %w", what, err)
}

func newGetCmd() *cobra.Command {
	var edgeType, from, to string
	cmd := &cobra.Command{
		Use:   "get (TYPE | edges)",
		Short: "List inventory nodes of a type, or edges",
		Long: `List inventory objects from the property graph.

  datumctl inventory get <TYPE>   list nodes of a node type (e.g. Site, Region,
                                  Cluster) — columns are derived from the
                                  matching NodeType's attribute schema.
  datumctl inventory get edges    list edges (relationships), optionally
                                  filtered by --type/--from/--to.

Run 'datumctl inventory types' to see the available node and edge types.`,
		Example: `  # Nodes of a type, with attribute columns
  datumctl inventory get Site
  datumctl inventory get Region

  # Edges, filtered
  datumctl inventory get edges
  datumctl inventory get edges --type=located-in
  datumctl inventory get edges --from=site-dfw1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if args[0] == "edges" {
				return getEdges(cmd.Context(), cmd, c, edgeType, from, to)
			}
			return getNodes(cmd.Context(), cmd, c, args[0])
		},
	}
	cmd.Flags().StringVar(&edgeType, "type", "", "Filter edges by type (use with 'get edges')")
	cmd.Flags().StringVar(&from, "from", "", "Filter edges by source node name (use with 'get edges')")
	cmd.Flags().StringVar(&to, "to", "", "Filter edges by target node name (use with 'get edges')")
	return cmd
}

func getNodes(ctx context.Context, cmd *cobra.Command, c client.Client, nodeType string) error {
	var nodes inventoryv1alpha2.NodeList
	if err := c.List(ctx, &nodes); err != nil {
		return listErr("nodes", err)
	}
	kept := nodes.Items[:0]
	for _, n := range nodes.Items {
		if n.Spec.Type == nodeType {
			kept = append(kept, n)
		}
	}
	nodes.Items = kept
	sort.Slice(nodes.Items, func(i, j int) bool { return nodes.Items[i].Name < nodes.Items[j].Name })

	keys := attributeColumns(ctx, c, nodeType, nodes.Items)
	headers := append([]string{"NAME"}, upperAll(keys)...)
	headers = append(headers, "READY")

	rows := make([][]string, 0, len(nodes.Items))
	for _, n := range nodes.Items {
		row := make([]string, 0, len(headers))
		row = append(row, n.Name)
		for _, k := range keys {
			row = append(row, orNone(n.Spec.Attributes[k]))
		}
		row = append(row, ready(n.Status.Conditions))
		rows = append(rows, row)
	}
	return emit(cmd, &nodes, headers, rows)
}

// attributeColumns returns the attribute keys to render as columns for a node
// type: the NodeType's declared attribute schema (authoritative order) when it
// exists, otherwise the union of keys actually present on the listed nodes.
func attributeColumns(ctx context.Context, c client.Client, nodeType string, nodes []inventoryv1alpha2.Node) []string {
	var nt inventoryv1alpha2.NodeType
	if err := c.Get(ctx, types.NamespacedName{Name: nodeType}, &nt); err == nil {
		keys := make([]string, 0, len(nt.Spec.Attributes))
		for _, a := range nt.Spec.Attributes {
			keys = append(keys, a.Key)
		}
		if len(keys) > 0 {
			return keys
		}
	} else if !errors.IsNotFound(err) {
		// A real error (not just "no such NodeType") — fall through to union;
		// the node list already succeeded, so don't fail the command on it.
		_ = err
	}

	set := map[string]struct{}{}
	for _, n := range nodes {
		for k := range n.Spec.Attributes {
			set[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func getEdges(ctx context.Context, cmd *cobra.Command, c client.Client, edgeType, from, to string) error {
	var edges inventoryv1alpha2.EdgeList
	if err := c.List(ctx, &edges); err != nil {
		return listErr("edges", err)
	}
	kept := edges.Items[:0]
	for _, e := range edges.Items {
		if edgeType != "" && e.Spec.Type != edgeType {
			continue
		}
		if from != "" && e.Spec.From.Name != from {
			continue
		}
		if to != "" && e.Spec.To.Name != to {
			continue
		}
		kept = append(kept, e)
	}
	edges.Items = kept
	sort.Slice(edges.Items, func(i, j int) bool { return edges.Items[i].Name < edges.Items[j].Name })

	headers := []string{"NAME", "TYPE", "FROM", "TO", "READY"}
	rows := make([][]string, 0, len(edges.Items))
	for _, e := range edges.Items {
		rows = append(rows, []string{e.Name, e.Spec.Type, e.Spec.From.Name, e.Spec.To.Name, ready(e.Status.Conditions)})
	}
	return emit(cmd, &edges, headers, rows)
}

func upperAll(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.ToUpper(s)
	}
	return out
}
