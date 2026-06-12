// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	inventoryv1alpha1 "go.miloapis.com/inventory/api/v1alpha1"
)

const fieldManager = "datumctl-inventory"

// applyOrder lists the inventory kinds apply handles, in dependency order:
// parents are applied before the children that reference them.
var applyOrder = []string{"Provider", "Region", "Site", "Cluster", "Node"}

func kindOrder(kind string) (int, bool) {
	for i, k := range applyOrder {
		if k == kind {
			return i, true
		}
	}
	return 0, false
}

func newApplyCmd() *cobra.Command {
	var files []string
	var dryRun string
	cmd := &cobra.Command{
		Use:   "apply -f FILE",
		Short: "Create or update inventory objects from a manifest",
		Long: `Create or update inventory objects (providers, regions, sites, clusters,
nodes) from a YAML or JSON manifest.

apply is an idempotent, declarative upsert: re-applying the same manifest makes
no changes. Objects are applied in dependency order (providers, then regions,
then sites, then clusters, then nodes) so a single mixed manifest lands cleanly.
It uses server-side apply with field manager "datumctl-inventory".

This is for populating the inventory from declared configuration — not fleet
management. Inventory lives on the Datum Cloud platform root, so apply takes no
organization or project scope.`,
		Example: `  # Apply a manifest file
  datumctl inventory apply -f fleet.yaml

  # Apply from stdin (e.g. piped from a renderer)
  render-fleet | datumctl inventory apply -f -

  # Validate against the server without persisting
  datumctl inventory apply -f fleet.yaml --dry-run=server`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(files) == 0 {
				return fmt.Errorf("at least one -f/--filename is required")
			}
			var server bool
			switch dryRun {
			case "", "none":
				server = false
			case "server":
				server = true
			default:
				return fmt.Errorf("invalid value %q for --dry-run; allowed: none, server", dryRun)
			}

			objs, err := readManifests(cmd.InOrStdin(), files)
			if err != nil {
				return err
			}
			if len(objs) == 0 {
				return fmt.Errorf("no inventory objects found in input")
			}
			sort.SliceStable(objs, func(i, j int) bool { return objs[i].order < objs[j].order })

			c, err := newClient()
			if err != nil {
				return err
			}
			return applyObjects(cmd.Context(), cmd.OutOrStdout(), c, objs, server)
		},
	}
	cmd.Flags().StringArrayVarP(&files, "filename", "f", nil, "Manifest file (YAML or JSON), or - for stdin. Repeatable.")
	cmd.Flags().StringVar(&dryRun, "dry-run", "none", `Must be "none" or "server". "server" validates against the API without persisting.`)
	return cmd
}

type applyObj struct {
	obj   client.Object
	kind  string
	order int
}

// readManifests parses every document from the given files (and stdin for "-")
// into typed inventory objects, giving client-side validation and rejecting
// kinds apply does not handle.
func readManifests(stdin io.Reader, files []string) ([]applyObj, error) {
	scheme := runtime.NewScheme()
	if err := inventoryv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("build scheme: %w", err)
	}
	decoder := serializer.NewCodecFactory(scheme).UniversalDeserializer()

	var out []applyObj
	for _, f := range files {
		r, closeFn, err := openInput(stdin, f)
		if err != nil {
			return nil, err
		}
		docs := utilyaml.NewYAMLOrJSONDecoder(r, 4096)
		for {
			var raw runtime.RawExtension
			if derr := docs.Decode(&raw); derr != nil {
				if derr == io.EOF {
					break
				}
				closeFn()
				return nil, fmt.Errorf("parse %s: %w", f, derr)
			}
			if len(raw.Raw) == 0 {
				continue
			}
			obj, gvk, derr := decoder.Decode(raw.Raw, nil, nil)
			if derr != nil {
				closeFn()
				return nil, fmt.Errorf("decode %s: %w", f, derr)
			}
			order, ok := kindOrder(gvk.Kind)
			if !ok {
				closeFn()
				return nil, fmt.Errorf("unsupported kind %q in %s (apply handles: %s)", gvk.Kind, f, strings.Join(applyOrder, ", "))
			}
			co, ok := obj.(client.Object)
			if !ok {
				closeFn()
				return nil, fmt.Errorf("%s in %s is not an applyable object", gvk.Kind, f)
			}
			co.GetObjectKind().SetGroupVersionKind(*gvk)
			out = append(out, applyObj{obj: co, kind: gvk.Kind, order: order})
		}
		closeFn()
	}
	return out, nil
}

func openInput(stdin io.Reader, f string) (io.Reader, func(), error) {
	if f == "-" {
		return stdin, func() {}, nil
	}
	fh, err := os.Open(f)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open %s: %w", f, err)
	}
	return fh, func() { _ = fh.Close() }, nil
}

func applyObjects(ctx context.Context, w io.Writer, c client.Client, objs []applyObj, server bool) error {
	opts := []client.PatchOption{client.FieldOwner(fieldManager), client.ForceOwnership}
	suffix := ""
	if server {
		opts = append(opts, client.DryRunAll)
		suffix = " (server dry-run)"
	}
	for _, o := range objs {
		if err := c.Patch(ctx, o.obj, client.Apply, opts...); err != nil {
			return fmt.Errorf("apply %s/%s: %w", strings.ToLower(o.kind), o.obj.GetName(), err)
		}
		fmt.Fprintf(w, "applied %s/%s%s\n", strings.ToLower(o.kind), o.obj.GetName(), suffix)
	}
	return nil
}
