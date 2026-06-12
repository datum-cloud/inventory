// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const none = "<none>"

// emit renders a typed list in the format selected by the --output flag. For
// table output it prints headers + rows; for json/yaml it marshals the (already
// filtered) typed list so scripted callers get full objects.
func emit(cmd *cobra.Command, list runtime.Object, headers []string, rows [][]string) error {
	format, _ := cmd.Flags().GetString("output")
	out := cmd.OutOrStdout()

	switch format {
	case "json":
		b, err := json.MarshalIndent(list, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(b))
		return err
	case "yaml":
		b, err := yaml.Marshal(list)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(out, string(b))
		return err
	case "", "table":
		return printTable(out, headers, rows)
	default:
		return fmt.Errorf("invalid value %q for --output; allowed: table, json, yaml", format)
	}
}

func printTable(out io.Writer, headers []string, rows [][]string) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(out, "No matching inventory found.")
		return err
	}
	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	return w.Flush()
}

// ready returns the status of the "Ready" condition, or "<none>" when absent.
func ready(conds []metav1.Condition) string {
	if c := meta.FindStatusCondition(conds, "Ready"); c != nil && c.Status != "" {
		return string(c.Status)
	}
	return none
}

func orNone(s string) string {
	if s == "" {
		return none
	}
	return s
}
