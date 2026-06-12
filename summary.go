// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	inventoryv1alpha2 "go.miloapis.com/inventory/api/v1alpha2"
)

func newSummaryCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "summary",
		Short:   "Show fleet-wide inventory counts",
		Long:    `Print fleet-wide counts: nodes per node type and edges per edge type.`,
		Example: `  datumctl inventory summary`,
		Args:    cobra.NoArgs,
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
			printSummary(cmd.OutOrStdout(), nodes, edges)
			return nil
		},
	}
}

func printSummary(out io.Writer, nodes inventoryv1alpha2.NodeList, edges inventoryv1alpha2.EdgeList) {
	nodeByType := map[string]int{}
	for _, n := range nodes.Items {
		nodeByType[n.Spec.Type]++
	}
	edgeByType := map[string]int{}
	for _, e := range edges.Items {
		edgeByType[e.Spec.Type]++
	}

	fmt.Fprintf(out, "Nodes (%d total)\n", len(nodes.Items))
	_ = printTable(out, []string{"NODE-TYPE", "COUNT"}, countRows(nodeByType))
	fmt.Fprintf(out, "\nEdges (%d total)\n", len(edges.Items))
	_ = printTable(out, []string{"EDGE-TYPE", "COUNT"}, countRows(edgeByType))
}

func countRows(m map[string]int) [][]string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, []string{k, strconv.Itoa(m[k])})
	}
	return rows
}
