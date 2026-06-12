// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"go.datum.net/datumctl/plugin"
	inventoryv1alpha1 "go.miloapis.com/inventory/api/v1alpha1"
)

// newClient builds a controller-runtime client against the Datum Cloud platform
// root, where inventory objects live. It reads the API host from the context
// datumctl injects and fetches a fresh token via the credentials helper.
func newClient() (client.Client, error) {
	ctx := plugin.Context()
	if ctx.APIHost == "" {
		return nil, fmt.Errorf("DATUM_API_HOST is not set; run this via 'datumctl inventory ...' (not the bare binary)")
	}
	token, err := plugin.Token()
	if err != nil {
		return nil, fmt.Errorf("get credentials: %w", err)
	}

	host := ctx.APIHost
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}

	scheme := runtime.NewScheme()
	if err := inventoryv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("build scheme: %w", err)
	}

	c, err := client.New(&rest.Config{Host: host, BearerToken: token}, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("build API client: %w", err)
	}
	return c, nil
}
