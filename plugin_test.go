// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"bytes"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	inventoryv1alpha1 "go.miloapis.com/inventory/api/v1alpha1"
)

func readyConds(status metav1.ConditionStatus) []metav1.Condition {
	return []metav1.Condition{{Type: "Ready", Status: status}}
}

func TestReady(t *testing.T) {
	if got := ready(readyConds(metav1.ConditionTrue)); got != "True" {
		t.Errorf("ready(True) = %q", got)
	}
	if got := ready(nil); got != none {
		t.Errorf("ready(nil) = %q, want %s", got, none)
	}
	if got := ready([]metav1.Condition{{Type: "Accepted", Status: metav1.ConditionTrue}}); got != none {
		t.Errorf("ready(no Ready) = %q, want %s", got, none)
	}
}

func TestOrNone(t *testing.T) {
	if orNone("") != none {
		t.Error("orNone empty should be <none>")
	}
	if orNone("x") != "x" {
		t.Error("orNone non-empty should pass through")
	}
}

func TestPrintTable(t *testing.T) {
	var buf bytes.Buffer
	if err := printTable(&buf, []string{"A", "B"}, [][]string{{"1", "2"}}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "A") || !strings.Contains(out, "1") {
		t.Errorf("table missing content:\n%s", out)
	}

	buf.Reset()
	_ = printTable(&buf, []string{"A"}, nil)
	if !strings.Contains(buf.String(), "No matching inventory found.") {
		t.Errorf("empty table should print no-match message, got:\n%s", buf.String())
	}
}

func site(name, region, provider string) inventoryv1alpha1.Site {
	s := inventoryv1alpha1.Site{}
	s.Name = name
	s.Spec.RegionRef = inventoryv1alpha1.LocalObjectReference{Name: region}
	if provider != "" {
		s.Spec.ProviderRef = &inventoryv1alpha1.LocalObjectReference{Name: provider}
	}
	return s
}

func node(name, siteName string) inventoryv1alpha1.Node {
	n := inventoryv1alpha1.Node{}
	n.Name = name
	n.Spec.SiteRef = inventoryv1alpha1.LocalObjectReference{Name: siteName}
	return n
}

func TestPrintTree(t *testing.T) {
	regions := inventoryv1alpha1.RegionList{Items: []inventoryv1alpha1.Region{
		{ObjectMeta: metav1.ObjectMeta{Name: "us-central-2"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "eu-west-1"}},
	}}
	sites := inventoryv1alpha1.SiteList{Items: []inventoryv1alpha1.Site{site("us-central-2a", "us-central-2", "")}}
	cl := inventoryv1alpha1.Cluster{}
	cl.Name = "edge-1"
	cl.Labels = map[string]string{inventoryv1alpha1.TopologyRegionLabel: "us-central-2"}
	clusters := inventoryv1alpha1.ClusterList{Items: []inventoryv1alpha1.Cluster{cl}}
	nodes := inventoryv1alpha1.NodeList{Items: []inventoryv1alpha1.Node{node("node-1", "us-central-2a")}}

	var buf bytes.Buffer
	printTree(&buf, "", regions, sites, clusters, nodes)
	out := buf.String()
	for _, want := range []string{"us-central-2", "eu-west-1", "clusters: edge-1", "  us-central-2a", "    node-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("tree missing %q:\n%s", want, out)
		}
	}

	buf.Reset()
	printTree(&buf, "us-central-2", regions, sites, clusters, nodes)
	if strings.Contains(buf.String(), "eu-west-1") {
		t.Errorf("--region filter leaked:\n%s", buf.String())
	}
}

func TestPrintSummary(t *testing.T) {
	sites := inventoryv1alpha1.SiteList{Items: []inventoryv1alpha1.Site{
		site("a", "r1", "netactuate"),
		site("b", "r1", "netactuate"),
		site("c", "r2", "vultr"),
	}}
	var buf bytes.Buffer
	printSummary(&buf, inventoryv1alpha1.ProviderList{}, inventoryv1alpha1.RegionList{}, sites, inventoryv1alpha1.ClusterList{}, inventoryv1alpha1.NodeList{})
	out := buf.String()
	for _, want := range []string{"Totals", "Per region", "Sites per provider", "netactuate", "r1"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}
