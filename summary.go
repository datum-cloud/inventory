// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	inventoryv1alpha1 "go.miloapis.com/inventory/api/v1alpha1"
)

func newSummaryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Show fleet-wide inventory counts",
		Long: `Print fleet-wide counts: totals per kind, sites and nodes per region, and
sites per provider.`,
		Example:      "  datumctl inventory summary",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			var providers inventoryv1alpha1.ProviderList
			var regions inventoryv1alpha1.RegionList
			var sites inventoryv1alpha1.SiteList
			var clusters inventoryv1alpha1.ClusterList
			var nodes inventoryv1alpha1.NodeList
			if err := c.List(ctx, &providers); err != nil {
				return listErr("providers", err)
			}
			if err := c.List(ctx, &regions); err != nil {
				return listErr("regions", err)
			}
			if err := c.List(ctx, &sites); err != nil {
				return listErr("sites", err)
			}
			if err := c.List(ctx, &clusters); err != nil {
				return listErr("clusters", err)
			}
			if err := c.List(ctx, &nodes); err != nil {
				return listErr("nodes", err)
			}

			printSummary(cmd.OutOrStdout(), providers, regions, sites, clusters, nodes)
			return nil
		},
	}
}

func printSummary(out io.Writer, providers inventoryv1alpha1.ProviderList, regions inventoryv1alpha1.RegionList, sites inventoryv1alpha1.SiteList, clusters inventoryv1alpha1.ClusterList, nodes inventoryv1alpha1.NodeList) {
	fmt.Fprintln(out, "Totals")
	_ = printTable(out, []string{"KIND", "COUNT"}, [][]string{
		{"providers", strconv.Itoa(len(providers.Items))},
		{"regions", strconv.Itoa(len(regions.Items))},
		{"sites", strconv.Itoa(len(sites.Items))},
		{"clusters", strconv.Itoa(len(clusters.Items))},
		{"nodes", strconv.Itoa(len(nodes.Items))},
	})

	sitesPerRegion := map[string]int{}
	for _, s := range sites.Items {
		sitesPerRegion[s.Spec.RegionRef.Name]++
	}
	nodesPerRegion := map[string]int{}
	for _, n := range nodes.Items {
		r := n.Labels[inventoryv1alpha1.TopologyRegionLabel]
		if r == "" {
			r = none
		}
		nodesPerRegion[r]++
	}
	fmt.Fprintln(out, "\nPer region")
	regionRows := make([][]string, 0)
	for _, r := range sortedUnion(sitesPerRegion, nodesPerRegion) {
		regionRows = append(regionRows, []string{r, strconv.Itoa(sitesPerRegion[r]), strconv.Itoa(nodesPerRegion[r])})
	}
	_ = printTable(out, []string{"REGION", "SITES", "NODES"}, regionRows)

	sitesPerProvider := map[string]int{}
	for _, s := range sites.Items {
		p := none
		if s.Spec.ProviderRef != nil {
			p = s.Spec.ProviderRef.Name
		}
		sitesPerProvider[p]++
	}
	fmt.Fprintln(out, "\nSites per provider")
	providerRows := make([][]string, 0)
	for _, p := range sortedKeys(sitesPerProvider) {
		providerRows = append(providerRows, []string{p, strconv.Itoa(sitesPerProvider[p])})
	}
	_ = printTable(out, []string{"PROVIDER", "SITES"}, providerRows)
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedUnion(a, b map[string]int) []string {
	seen := map[string]bool{}
	for k := range a {
		seen[k] = true
	}
	for k := range b {
		seen[k] = true
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
