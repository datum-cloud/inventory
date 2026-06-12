// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	inventoryv1alpha2 "go.miloapis.com/inventory/api/v1alpha2"
)

func newTreeCmd() *cobra.Command {
	var edgeType, rootType string
	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Show the containment hierarchy built from edges",
		Long: `Print the inventory as a tree built from containment edges.

By default it follows "located-in" edges (a node is located-in its parent) and
roots the tree at "Region" nodes, so you see region → site → node. Override the
relationship with --edge and the root asset class with --root-type.`,
		Example: `  # region -> site -> node, via located-in
  datumctl inventory tree

  # rack containment
  datumctl inventory tree --edge=mounted-in --root-type=Rack`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			var nodes inventoryv1alpha2.NodeList
			if err := c.List(ctx, &nodes); err != nil {
				return listErr("nodes", err)
			}
			var edges inventoryv1alpha2.EdgeList
			if err := c.List(ctx, &edges); err != nil {
				return listErr("edges", err)
			}
			printTree(cmd.OutOrStdout(), nodes, edges, edgeType, rootType)
			return nil
		},
	}
	cmd.Flags().StringVar(&edgeType, "edge", "located-in", "Containment edge type to follow (child is FROM, parent is TO)")
	cmd.Flags().StringVar(&rootType, "root-type", "Region", "Node type to root the tree at")
	return cmd
}

func printTree(out io.Writer, nodes inventoryv1alpha2.NodeList, edges inventoryv1alpha2.EdgeList, edgeType, rootType string) {
	typeOf := map[string]string{}
	for _, n := range nodes.Items {
		typeOf[n.Name] = n.Spec.Type
	}
	// children[parent] = child names, from `child located-in parent` edges.
	children := map[string][]string{}
	for _, e := range edges.Items {
		if e.Spec.Type != edgeType {
			continue
		}
		children[e.Spec.To.Name] = append(children[e.Spec.To.Name], e.Spec.From.Name)
	}

	roots := make([]string, 0)
	for _, n := range nodes.Items {
		if n.Spec.Type == rootType {
			roots = append(roots, n.Name)
		}
	}
	sort.Strings(roots)

	if len(roots) == 0 {
		fmt.Fprintf(out, "No %s nodes found.\n", rootType)
		return
	}
	for _, r := range roots {
		printNode(out, r, typeOf, children, 0, map[string]bool{})
	}
}

func printNode(out io.Writer, name string, typeOf map[string]string, children map[string][]string, depth int, seen map[string]bool) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	t := typeOf[name]
	if t == "" {
		t = "?"
	}
	fmt.Fprintf(out, "%s%s (%s)\n", indent, name, t)
	if seen[name] {
		fmt.Fprintf(out, "%s  ...cycle\n", indent)
		return
	}
	seen[name] = true
	kids := append([]string(nil), children[name]...)
	sort.Strings(kids)
	for _, k := range kids {
		printNode(out, k, typeOf, children, depth+1, seen)
	}
	delete(seen, name)
}
