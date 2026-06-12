# datumctl-inventory

A [datumctl](https://github.com/datum-cloud/datumctl) plugin for the Datum Cloud
inventory, modeled as a **property graph**: typed `Node`s (Region, Site,
Cluster, Provider, Host, …) connected by typed `Edge`s (located-in, member-of,
provided-by, …), each carrying an attribute bag. The available types and their
attributes live in the `NodeType`/`EdgeType` schema registry. Served by the
[milo inventory service](https://github.com/milo-os/inventory)
(`graph.inventory.miloapis.com/v1alpha2`). Invoked as `datumctl inventory ...`.

## Install

```sh
datumctl plugin install datum-cloud/inventory
```

The binary inside the release archive is `datumctl-inventory`, so datumctl
exposes it as `datumctl inventory`.

## Commands

| Command | Description |
|---|---|
| `datumctl inventory get <TYPE>` | List nodes of an asset class; columns derived from the NodeType schema |
| `datumctl inventory get edges [--type T] [--from N] [--to N]` | List edges (relationships) |
| `datumctl inventory types` | List the NodeType/EdgeType schema registry |
| `datumctl inventory neighbors NODE [--edge T] [--direction out\|in\|both]` | Nodes adjacent to NODE |
| `datumctl inventory tree [--edge T] [--root-type T]` | Containment hierarchy from edges (default: `located-in`, rooted at `Region`) |
| `datumctl inventory summary` | Counts per node type and edge type |
| `datumctl inventory apply -f FILE [--dry-run=server]` | Create/update graph objects from a manifest |

The `get`, `types`, `summary` commands accept `-o table|json|yaml` (default `table`).

Relationships that were typed fields in the old model (a site's region, a
node's cluster) are now **edges** — query them with `get edges`, `neighbors`,
or `tree` rather than as columns.

## Populating the inventory

`apply` is an idempotent, declarative upsert of graph objects — for loading the
inventory from declared configuration, not fleet management:

```sh
# Apply a manifest (objects land in dependency order: node/edge types first,
# then nodes, then edges — regardless of order in the file)
datumctl inventory apply -f graph.yaml

# Pipe from a renderer
render-fleet | datumctl inventory apply -f -

# Validate against the server without persisting
datumctl inventory apply -f graph.yaml --dry-run=server
```

It uses server-side apply with field manager `datumctl-inventory`, so
re-applying the same manifest makes no changes. Only `NodeType`, `EdgeType`,
`Node`, and `Edge` are accepted.

Inventory objects are cluster-scoped on the Datum Cloud platform root, so the
plugin talks to the platform API directly and takes no organization or project
scope.

## How it works

datumctl injects context via environment variables and execs the plugin. The
plugin reads `DATUM_API_HOST`, fetches a short-lived token through the
credentials helper (`plugin.Token()`), and builds a controller-runtime client
against the platform root using the milo inventory project's published typed
API (`go.miloapis.com/inventory/api/v1alpha2`). See the
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
