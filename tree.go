// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	inventoryv1alpha1 "go.miloapis.com/inventory/api/v1alpha1"
)

func newTreeCmd() *cobra.Command {
	var region string
	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Show the region -> site -> node hierarchy",
		Long: `Print the inventory as a topology tree: each region, the sites within it,
the nodes at each site, and the clusters anchored in the region.

Use --region to scope the tree to a single region.`,
		Example: `  # Full topology tree
  datumctl inventory tree

  # Just one region
  datumctl inventory tree --region us-central-2`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			var regions inventoryv1alpha1.RegionList
			if err := c.List(ctx, &regions); err != nil {
				return listErr("regions", err)
			}
			var sites inventoryv1alpha1.SiteList
			if err := c.List(ctx, &sites); err != nil {
				return listErr("sites", err)
			}
			var clusters inventoryv1alpha1.ClusterList
			if err := c.List(ctx, &clusters); err != nil {
				return listErr("clusters", err)
			}
			var nodes inventoryv1alpha1.NodeList
			if err := c.List(ctx, &nodes); err != nil {
				return listErr("nodes", err)
			}

			printTree(cmd.OutOrStdout(), region, regions, sites, clusters, nodes)
			return nil
		},
	}
	cmd.Flags().StringVar(&region, "region", "", "Limit the tree to a single region")
	return cmd
}

func printTree(out io.Writer, regionFilter string, regions inventoryv1alpha1.RegionList, sites inventoryv1alpha1.SiteList, clusters inventoryv1alpha1.ClusterList, nodes inventoryv1alpha1.NodeList) {
	sitesByRegion := map[string][]string{}
	for _, s := range sites.Items {
		sitesByRegion[s.Spec.RegionRef.Name] = append(sitesByRegion[s.Spec.RegionRef.Name], s.Name)
	}
	nodesBySite := map[string][]string{}
	for _, n := range nodes.Items {
		nodesBySite[n.Spec.SiteRef.Name] = append(nodesBySite[n.Spec.SiteRef.Name], n.Name)
	}
	clustersByRegion := map[string][]string{}
	for _, cl := range clusters.Items {
		r := cl.Labels[inventoryv1alpha1.TopologyRegionLabel]
		if r == "" {
			r = none
		}
		clustersByRegion[r] = append(clustersByRegion[r], cl.Name)
	}

	names := make([]string, 0, len(regions.Items))
	for _, r := range regions.Items {
		names = append(names, r.Name)
	}
	sort.Strings(names)

	printed := 0
	for _, region := range names {
		if regionFilter != "" && region != regionFilter {
			continue
		}
		printed++
		fmt.Fprintln(out, region)

		if cls := clustersByRegion[region]; len(cls) > 0 {
			sort.Strings(cls)
			fmt.Fprintf(out, "  clusters: %s\n", strings.Join(cls, ", "))
		}

		regionSites := sitesByRegion[region]
		sort.Strings(regionSites)
		for _, site := range regionSites {
			fmt.Fprintf(out, "  %s\n", site)
			siteNodes := nodesBySite[site]
			sort.Strings(siteNodes)
			for _, n := range siteNodes {
				fmt.Fprintf(out, "    %s\n", n)
			}
		}
	}

	if printed == 0 {
		fmt.Fprintln(out, "No matching inventory found.")
	}
}
