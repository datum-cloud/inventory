// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	inventoryv1alpha1 "go.miloapis.com/inventory/api/v1alpha1"
)

// resourceView bundles a list subcommand's identity with a binder that
// registers its filter flags and returns the closure that lists, filters,
// sorts, and renders one inventory kind.
type resourceView struct {
	use   string
	short string
	bind  func(cmd *cobra.Command) runFunc
}

type runFunc func(ctx context.Context, c client.Client) (list runtime.Object, headers []string, rows [][]string, err error)

func newListCmd(v resourceView) *cobra.Command {
	cmd := &cobra.Command{Use: v.use, Short: v.short, Args: cobra.NoArgs, SilenceUsage: true}
	run := v.bind(cmd)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		list, headers, rows, err := run(cmd.Context(), c)
		if err != nil {
			return err
		}
		return emit(cmd, list, headers, rows)
	}
	return cmd
}

func listErr(resource string, err error) error {
	return fmt.Errorf("could not list %s: %w", resource, err)
}

func regionLabelOpt(region string) []client.ListOption {
	if region == "" {
		return nil
	}
	return []client.ListOption{client.MatchingLabels{inventoryv1alpha1.TopologyRegionLabel: region}}
}

var providersView = resourceView{
	use:   "providers",
	short: "List inventory providers",
	bind: func(_ *cobra.Command) runFunc {
		return func(ctx context.Context, c client.Client) (runtime.Object, []string, [][]string, error) {
			var list inventoryv1alpha1.ProviderList
			if err := c.List(ctx, &list); err != nil {
				return nil, nil, nil, listErr("providers", err)
			}
			sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })
			rows := make([][]string, 0, len(list.Items))
			for _, p := range list.Items {
				rows = append(rows, []string{p.Name, orNone(p.Spec.DisplayName), string(p.Spec.Type), ready(p.Status.Conditions)})
			}
			return &list, []string{"NAME", "DISPLAY", "TYPE", "READY"}, rows, nil
		}
	},
}

var regionsView = resourceView{
	use:   "regions",
	short: "List inventory regions",
	bind: func(_ *cobra.Command) runFunc {
		return func(ctx context.Context, c client.Client) (runtime.Object, []string, [][]string, error) {
			var list inventoryv1alpha1.RegionList
			if err := c.List(ctx, &list); err != nil {
				return nil, nil, nil, listErr("regions", err)
			}
			sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })
			rows := make([][]string, 0, len(list.Items))
			for _, r := range list.Items {
				rows = append(rows, []string{r.Name, orNone(r.Spec.DisplayName), ready(r.Status.Conditions)})
			}
			return &list, []string{"NAME", "DISPLAY", "READY"}, rows, nil
		}
	},
}

var sitesView = resourceView{
	use:   "sites",
	short: "List inventory sites",
	bind: func(cmd *cobra.Command) runFunc {
		region := cmd.Flags().String("region", "", "Filter by region name")
		provider := cmd.Flags().String("provider", "", "Filter by provider name")
		return func(ctx context.Context, c client.Client) (runtime.Object, []string, [][]string, error) {
			var list inventoryv1alpha1.SiteList
			if err := c.List(ctx, &list, regionLabelOpt(*region)...); err != nil {
				return nil, nil, nil, listErr("sites", err)
			}
			if *provider != "" {
				kept := list.Items[:0]
				for _, s := range list.Items {
					if s.Spec.ProviderRef != nil && s.Spec.ProviderRef.Name == *provider {
						kept = append(kept, s)
					}
				}
				list.Items = kept
			}
			sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })
			rows := make([][]string, 0, len(list.Items))
			for _, s := range list.Items {
				prov := none
				if s.Spec.ProviderRef != nil {
					prov = orNone(s.Spec.ProviderRef.Name)
				}
				rows = append(rows, []string{s.Name, orNone(s.Spec.RegionRef.Name), prov, string(s.Spec.Type), ready(s.Status.Conditions)})
			}
			return &list, []string{"NAME", "REGION", "PROVIDER", "TYPE", "READY"}, rows, nil
		}
	},
}

var clustersView = resourceView{
	use:   "clusters",
	short: "List inventory clusters",
	bind: func(cmd *cobra.Command) runFunc {
		region := cmd.Flags().String("region", "", "Filter by region name")
		site := cmd.Flags().String("site", "", "Filter by control-plane site name")
		return func(ctx context.Context, c client.Client) (runtime.Object, []string, [][]string, error) {
			var list inventoryv1alpha1.ClusterList
			if err := c.List(ctx, &list, regionLabelOpt(*region)...); err != nil {
				return nil, nil, nil, listErr("clusters", err)
			}
			if *site != "" {
				kept := list.Items[:0]
				for _, cl := range list.Items {
					if cl.Spec.ControlPlaneSiteRef.Name == *site {
						kept = append(kept, cl)
					}
				}
				list.Items = kept
			}
			sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })
			rows := make([][]string, 0, len(list.Items))
			for _, cl := range list.Items {
				rows = append(rows, []string{
					cl.Name,
					orNone(cl.Labels[inventoryv1alpha1.TopologyRegionLabel]),
					orNone(cl.Spec.ControlPlaneSiteRef.Name),
					string(cl.Spec.Role),
					orNone(cl.Spec.Provider),
					ready(cl.Status.Conditions),
				})
			}
			return &list, []string{"NAME", "REGION", "CP-SITE", "ROLE", "PROVIDER", "READY"}, rows, nil
		}
	},
}

var nodesView = resourceView{
	use:   "nodes",
	short: "List inventory nodes",
	bind: func(cmd *cobra.Command) runFunc {
		region := cmd.Flags().String("region", "", "Filter by region name")
		site := cmd.Flags().String("site", "", "Filter by site name")
		cluster := cmd.Flags().String("cluster", "", "Filter by cluster name")
		return func(ctx context.Context, c client.Client) (runtime.Object, []string, [][]string, error) {
			sel := client.MatchingLabels{}
			if *region != "" {
				sel[inventoryv1alpha1.TopologyRegionLabel] = *region
			}
			if *site != "" {
				sel[inventoryv1alpha1.TopologySiteLabel] = *site
			}
			if *cluster != "" {
				sel[inventoryv1alpha1.TopologyClusterLabel] = *cluster
			}
			var list inventoryv1alpha1.NodeList
			if err := c.List(ctx, &list, client.MatchingLabels(sel)); err != nil {
				return nil, nil, nil, listErr("nodes", err)
			}
			sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })
			rows := make([][]string, 0, len(list.Items))
			for _, n := range list.Items {
				clusterName, role := none, none
				if n.Spec.Assignment != nil {
					clusterName = orNone(n.Spec.Assignment.ClusterRef.Name)
					role = string(n.Spec.Assignment.Role)
				}
				rows = append(rows, []string{
					n.Name,
					orNone(n.Spec.SiteRef.Name),
					clusterName,
					role,
					string(n.Spec.Hardware.CPUArchitecture),
					strconv.Itoa(int(n.Spec.Hardware.CPUCores)),
					orNone(string(n.Status.Phase)),
					ready(n.Status.Conditions),
				})
			}
			return &list, []string{"NAME", "SITE", "CLUSTER", "ROLE", "ARCH", "CPU", "PHASE", "READY"}, rows, nil
		}
	},
}
