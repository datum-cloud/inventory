// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	inventoryv1alpha2 "go.miloapis.com/inventory/api/v1alpha2"
)

func readyConds(s metav1.ConditionStatus) []metav1.Condition {
	return []metav1.Condition{{Type: "Ready", Status: s}}
}

func node(name, typ string, attrs map[string]string) inventoryv1alpha2.Node {
	n := inventoryv1alpha2.Node{}
	n.Name = name
	n.Spec.Type = typ
	n.Spec.Attributes = attrs
	n.Status.Conditions = readyConds(metav1.ConditionTrue)
	return n
}

func edge(name, typ, from, to string) inventoryv1alpha2.Edge {
	e := inventoryv1alpha2.Edge{}
	e.Name = name
	e.Spec.Type = typ
	e.Spec.From = inventoryv1alpha2.NodeReference{Name: from}
	e.Spec.To = inventoryv1alpha2.NodeReference{Name: to}
	e.Status.Conditions = readyConds(metav1.ConditionTrue)
	return e
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := inventoryv1alpha2.AddToScheme(s); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	return s
}

func tableCmd() (*cobra.Command, *bytes.Buffer) {
	var buf bytes.Buffer
	c := &cobra.Command{}
	c.Flags().StringP("output", "o", "table", "")
	c.SetOut(&buf)
	return c, &buf
}

func TestReady(t *testing.T) {
	if got := ready(readyConds(metav1.ConditionTrue)); got != "True" {
		t.Errorf("ready(True) = %q", got)
	}
	if got := ready(nil); got != none {
		t.Errorf("ready(nil) = %q, want %s", got, none)
	}
}

func TestPrintTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	_ = printTable(&buf, []string{"A"}, nil)
	if !strings.Contains(buf.String(), "No matching inventory found.") {
		t.Errorf("empty table message missing: %q", buf.String())
	}
}

func TestPrintTree(t *testing.T) {
	nodes := inventoryv1alpha2.NodeList{Items: []inventoryv1alpha2.Node{
		node("region-uc", "Region", nil),
		node("site-dfw1", "Site", nil),
		node("host-1", "Host", nil),
	}}
	edges := inventoryv1alpha2.EdgeList{Items: []inventoryv1alpha2.Edge{
		edge("e1", "located-in", "site-dfw1", "region-uc"),
		edge("e2", "located-in", "host-1", "site-dfw1"),
	}}
	var buf bytes.Buffer
	printTree(&buf, nodes, edges, "located-in", "Region")
	out := buf.String()
	for _, want := range []string{"region-uc (Region)", "  site-dfw1 (Site)", "    host-1 (Host)"} {
		if !strings.Contains(out, want) {
			t.Errorf("tree missing %q:\n%s", want, out)
		}
	}
}

func TestPrintSummary(t *testing.T) {
	nodes := inventoryv1alpha2.NodeList{Items: []inventoryv1alpha2.Node{
		node("a", "Site", nil), node("b", "Site", nil), node("c", "Region", nil),
	}}
	edges := inventoryv1alpha2.EdgeList{Items: []inventoryv1alpha2.Edge{
		edge("e1", "located-in", "a", "c"),
	}}
	var buf bytes.Buffer
	printSummary(&buf, nodes, edges)
	out := buf.String()
	for _, want := range []string{"Nodes (3 total)", "Site", "Region", "Edges (1 total)", "located-in"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestGetEdgesFilters(t *testing.T) {
	c := fakeclient.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(
		ptr(edge("e1", "located-in", "site-dfw1", "region-uc")),
		ptr(edge("e2", "member-of", "host-1", "cluster-a")),
	).Build()
	cmd, buf := tableCmd()
	if err := getEdges(context.Background(), cmd, c, "located-in", "", ""); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "located-in") || strings.Contains(out, "member-of") {
		t.Errorf("--type filter wrong:\n%s", out)
	}
}

func TestNeighbors(t *testing.T) {
	c := fakeclient.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(
		ptr(node("site-dfw1", "Site", nil)),
		ptr(node("region-uc", "Region", nil)),
		ptr(node("host-1", "Host", nil)),
		ptr(edge("e1", "located-in", "site-dfw1", "region-uc")),
		ptr(edge("e2", "located-in", "host-1", "site-dfw1")),
	).Build()
	cmd, buf := tableCmd()
	if err := neighbors(context.Background(), cmd, c, "site-dfw1", "located-in", "both"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "region-uc") || !strings.Contains(out, "host-1") {
		t.Errorf("neighbors missing expected nodes:\n%s", out)
	}
	if !strings.Contains(out, "out") || !strings.Contains(out, "in") {
		t.Errorf("neighbors missing both directions:\n%s", out)
	}
}

func TestAttributeColumnsFallbackUnion(t *testing.T) {
	c := fakeclient.NewClientBuilder().WithScheme(newScheme(t)).Build()
	nodes := []inventoryv1alpha2.Node{
		node("a", "Site", map[string]string{"displayName": "DFW", "siteType": "Edge"}),
		node("b", "Site", map[string]string{"displayName": "ORD"}),
	}
	keys := attributeColumns(context.Background(), c, "Site", nodes)
	if strings.Join(keys, ",") != "displayName,siteType" {
		t.Errorf("fallback union = %v, want [displayName siteType]", keys)
	}
}

func TestAttributeColumnsFromNodeType(t *testing.T) {
	nt := inventoryv1alpha2.NodeType{}
	nt.Name = "Site"
	nt.Spec.Attributes = []inventoryv1alpha2.AttributeSchema{
		{Key: "displayName", Type: inventoryv1alpha2.AttributeString},
		{Key: "siteType", Type: inventoryv1alpha2.AttributeString},
	}
	c := fakeclient.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(&nt).Build()
	keys := attributeColumns(context.Background(), c, "Site", nil)
	if strings.Join(keys, ",") != "displayName,siteType" {
		t.Errorf("NodeType-driven columns = %v, want schema order", keys)
	}
}

func ptr[T any](v T) *T { return &v }
