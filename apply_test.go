// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"sort"
	"strings"
	"testing"
)

const sampleManifest = `
apiVersion: inventory.miloapis.com/v1alpha1
kind: Node
metadata:
  name: node-a
spec:
  siteRef:
    name: us-central-1a
  hardware:
    cpuCores: 8
    cpuArchitecture: amd64
    memoryBytes: 1073741824
---
apiVersion: inventory.miloapis.com/v1alpha1
kind: Provider
metadata:
  name: netactuate
spec:
  displayName: NetActuate
  type: Hosting
---
apiVersion: inventory.miloapis.com/v1alpha1
kind: Site
metadata:
  name: us-central-1a
spec:
  displayName: Dallas
  type: AvailabilityZone
  regionRef:
    name: us-central-1
`

func TestReadManifestsParsesAndOrders(t *testing.T) {
	objs, err := readManifests(strings.NewReader(sampleManifest), []string{"-"})
	if err != nil {
		t.Fatalf("readManifests: %v", err)
	}
	if len(objs) != 3 {
		t.Fatalf("got %d objects, want 3", len(objs))
	}
	sort.SliceStable(objs, func(i, j int) bool { return objs[i].order < objs[j].order })
	gotKinds := []string{objs[0].kind, objs[1].kind, objs[2].kind}
	want := []string{"Provider", "Site", "Node"}
	for i := range want {
		if gotKinds[i] != want[i] {
			t.Errorf("order[%d] = %s, want %s (full: %v)", i, gotKinds[i], want[i], gotKinds)
		}
	}
	// GVK must be set on each object so server-side apply has apiVersion/kind.
	for _, o := range objs {
		if o.obj.GetObjectKind().GroupVersionKind().Kind == "" {
			t.Errorf("%s/%s missing GVK", o.kind, o.obj.GetName())
		}
	}
}

func TestReadManifestsRejectsUnsupportedKind(t *testing.T) {
	const m = `
apiVersion: inventory.miloapis.com/v1alpha1
kind: Rack
metadata:
  name: rack-a
`
	_, err := readManifests(strings.NewReader(m), []string{"-"})
	if err == nil || !strings.Contains(err.Error(), "unsupported kind") {
		t.Fatalf("want unsupported-kind error, got %v", err)
	}
}

func TestReadManifestsEmpty(t *testing.T) {
	objs, err := readManifests(strings.NewReader("\n---\n"), []string{"-"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 0 {
		t.Fatalf("got %d objects, want 0", len(objs))
	}
}

func TestKindOrder(t *testing.T) {
	if _, ok := kindOrder("Provider"); !ok {
		t.Error("Provider should be ordered")
	}
	p, _ := kindOrder("Provider")
	n, _ := kindOrder("Node")
	if !(p < n) {
		t.Errorf("Provider (%d) should sort before Node (%d)", p, n)
	}
	if _, ok := kindOrder("Rack"); ok {
		t.Error("Rack should not be applyable")
	}
}
