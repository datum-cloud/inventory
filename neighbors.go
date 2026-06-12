// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	inventoryv1alpha2 "go.miloapis.com/inventory/api/v1alpha2"
)

func newNeighborsCmd() *cobra.Command {
	var edgeType, direction string
	cmd := &cobra.Command{
		Use:   "neighbors NODE",
		Short: "List nodes adjacent to NODE via edges",
		Long: `List the nodes directly connected to NODE by an edge.

By default both directions are shown. Use --edge to follow only one
relationship type (e.g. located-in, member-of) and --direction to restrict to
outgoing or incoming edges.`,
		Example: `  # Everything adjacent to a node
  datumctl inventory neighbors site-dfw1

  # Only what site-dfw1 is located in (outgoing located-in edges)
  datumctl inventory neighbors site-dfw1 --edge=located-in --direction=out

  # What is located in region us-central (incoming located-in edges)
  datumctl inventory neighbors region-us-central --edge=located-in --direction=in`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch direction {
			case "out", "in", "both":
			default:
				return fmt.Errorf("invalid value %q for --direction; allowed: out, in, both", direction)
			}
			c, err := newClient()
			if err != nil {
				return err
			}
			return neighbors(cmd.Context(), cmd, c, args[0], edgeType, direction)
		},
	}
	cmd.Flags().StringVar(&edgeType, "edge", "", "Only follow edges of this type")
	cmd.Flags().StringVar(&direction, "direction", "both", "Edge direction: out, in, or both")
	return cmd
}

func neighbors(ctx context.Context, cmd *cobra.Command, c client.Client, node, edgeType, direction string) error {
	var edges inventoryv1alpha2.EdgeList
	if err := c.List(ctx, &edges); err != nil {
		return listErr("edges", err)
	}
	var nodes inventoryv1alpha2.NodeList
	if err := c.List(ctx, &nodes); err != nil {
		return listErr("nodes", err)
	}
	typeOf := map[string]string{}
	for _, n := range nodes.Items {
		typeOf[n.Name] = n.Spec.Type
	}

	type hop struct{ edgeType, dir, neighbor, neighborType string }
	var hops []hop
	for _, e := range edges.Items {
		if edgeType != "" && e.Spec.Type != edgeType {
			continue
		}
		if (direction == "out" || direction == "both") && e.Spec.From.Name == node {
			hops = append(hops, hop{e.Spec.Type, "out", e.Spec.To.Name, orNone(typeOf[e.Spec.To.Name])})
		}
		if (direction == "in" || direction == "both") && e.Spec.To.Name == node {
			hops = append(hops, hop{e.Spec.Type, "in", e.Spec.From.Name, orNone(typeOf[e.Spec.From.Name])})
		}
	}
	sort.Slice(hops, func(i, j int) bool {
		if hops[i].edgeType != hops[j].edgeType {
			return hops[i].edgeType < hops[j].edgeType
		}
		return hops[i].neighbor < hops[j].neighbor
	})

	rows := make([][]string, 0, len(hops))
	for _, h := range hops {
		rows = append(rows, []string{h.edgeType, h.dir, h.neighbor, h.neighborType})
	}
	return printTable(cmd.OutOrStdout(), []string{"EDGE-TYPE", "DIRECTION", "NEIGHBOR", "NEIGHBOR-TYPE"}, rows)
}
