# datumctl-inventory

A [datumctl](https://github.com/datum-cloud/datumctl) plugin that provides a
read view over the Datum Cloud physical inventory — providers, regions, sites,
clusters, and nodes — served by the [milo inventory
service](https://github.com/milo-os/inventory) (`inventory.miloapis.com/v1alpha1`).
Once installed it is invoked as `datumctl inventory ...`.

## Install

```sh
datumctl plugin install datum-cloud/inventory
```

The binary inside the release archive is `datumctl-inventory`, so datumctl
exposes it as `datumctl inventory`.

## Commands

| Command | Description |
|---|---|
| `datumctl inventory providers` | List providers |
| `datumctl inventory regions` | List regions |
| `datumctl inventory sites [--region R] [--provider P]` | List sites |
| `datumctl inventory clusters [--region R] [--site S]` | List clusters |
| `datumctl inventory nodes [--region R] [--site S] [--cluster C]` | List nodes |
| `datumctl inventory tree [--region R]` | region → site → node hierarchy |
| `datumctl inventory summary` | Fleet-wide counts |
| `datumctl inventory apply -f FILE [--dry-run=server]` | Create/update objects from a manifest |

The list subcommands accept `-o table|json|yaml` (default `table`).

`--region`, `--site`, and `--cluster` filter server-side using the
`topology.inventory.miloapis.com/*` labels the inventory controllers propagate
onto objects. `--provider` filters on the site's `providerRef`.

## Populating the inventory

`apply` is an idempotent, declarative upsert for inventory objects — for
loading the inventory from declared configuration, not fleet management:

```sh
# Apply a manifest (objects land in dependency order: provider, region,
# site, cluster, node — regardless of order in the file)
datumctl inventory apply -f fleet.yaml

# Pipe from a renderer
render-fleet | datumctl inventory apply -f -

# Validate against the server without persisting
datumctl inventory apply -f fleet.yaml --dry-run=server
```

It uses server-side apply with field manager `datumctl-inventory`, so
re-applying the same manifest makes no changes. Only `Provider`, `Region`,
`Site`, `Cluster`, and `Node` are accepted.

Inventory objects are cluster-scoped on the Datum Cloud platform root, so the
plugin talks to the platform API directly and takes no organization or project
scope.

## How it works

datumctl injects context via environment variables and execs the plugin. The
plugin reads `DATUM_API_HOST`, fetches a short-lived token through the
credentials helper (`plugin.Token()`), and builds a controller-runtime client
against the platform root using the milo inventory project's published typed
API (`go.miloapis.com/inventory/api/v1alpha1`). See the
[datumctl plugin docs](https://github.com/datum-cloud/datumctl/blob/main/docs/developer/plugins.md).

This split keeps Datum's CLI surface in `datum-cloud/` while depending on the
milo inventory API as an external module — Datum consumes milo's published
types rather than vendoring the CLI into the milo repo.

## Build

```sh
go build -o datumctl-inventory .
```

The version reported by `--plugin-manifest` is set via
`-ldflags "-X main.version=<version>"` at release time.

## Releases

Push a semver tag (`vX.Y.Z`); `.github/workflows/release.yaml` runs goreleaser,
which publishes `datumctl-inventory_{OS}_{Arch}` archives and `checksums.txt`
to a GitHub release. The plugin is versioned independently of both datumctl and
the inventory service.

## Local development

Build the binary onto your `PATH` named `datumctl-inventory`, then:

```sh
datumctl plugin trust inventory   # unmanaged plugins must be trusted once
datumctl inventory summary
```

## License

AGPL-3.0-only. This plugin links the inventory service's API types, which are
AGPL-licensed.
