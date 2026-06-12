// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"sort"
	"strings"
	"testing"
)

const sampleManifest = `
apiVersion: graph.inventory.miloapis.com/v1alpha2
kind: Edge
metadata:
  name: site-dfw1-in-uc
spec:
  type: located-in
  from:
    name: site-dfw1
  to:
    name: region-uc
---
apiVersion: graph.inventory.miloapis.com/v1alpha2
kind: NodeType
metadata:
  name: Site
spec:
  displayName: Site
---
apiVersion: graph.inventory.miloapis.com/v1alpha2
kind: Node
metadata:
  name: site-dfw1
spec:
  type: Site
  attributes:
    displayName: Dallas
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
	got := []string{objs[0].kind, objs[1].kind, objs[2].kind}
	want := []string{"NodeType", "Node", "Edge"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("order[%d] = %s, want %s (full: %v)", i, got[i], want[i], got)
		}
	}
	for _, o := range objs {
		if o.obj.GetObjectKind().GroupVersionKind().Kind == "" {
			t.Errorf("%s/%s missing GVK", o.kind, o.obj.GetName())
		}
	}
}

func TestReadManifestsRejectsUnsupportedKind(t *testing.T) {
	const m = `
apiVersion: graph.inventory.miloapis.com/v1alpha2
kind: Widget
metadata:
  name: w
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
	nt, ok := kindOrder("NodeType")
	if !ok {
		t.Fatal("NodeType should be ordered")
	}
	e, ok := kindOrder("Edge")
	if !ok {
		t.Fatal("Edge should be ordered")
	}
	if !(nt < e) {
		t.Errorf("NodeType (%d) should sort before Edge (%d)", nt, e)
	}
	if _, ok := kindOrder("Widget"); ok {
		t.Error("Widget should not be applyable")
	}
}
