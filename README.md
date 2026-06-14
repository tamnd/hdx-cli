# hdx

A command line for the Humanitarian Data Exchange (HDX).

`hdx` is a single pure-Go binary. It reads public hdx data
over plain HTTPS, shapes it into clean records, and prints output that pipes
into the rest of your tools. No API key, nothing to run alongside it.

The same package is also a [resource-URI driver](#use-it-as-a-resource-uri-driver),
so a host program like [ant](https://github.com/tamnd/ant) can address
hdx as `hdx://` URIs.

## Install

```bash
go install github.com/tamnd/hdx-cli/cmd/hdx@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/hdx-cli/releases), or run
the container image:

```bash
docker run --rm ghcr.io/tamnd/hdx:latest --help
```

## Usage

```bash
hdx page <path>                      # fetch one page as a record
hdx page <path> -o json              # as JSON, ready for jq
hdx page <path> --template '{{.Body}}'  # just the readable body text
hdx links <path>                     # the pages it links to, one per line
hdx --help                           # the whole command tree
```

Every command shares one output contract:
`-o table|markdown|json|jsonl|csv|tsv|url|raw`, `--fields` to pick columns,
`--template` for a custom line, and `-n` to limit. The default adapts to where
output goes (a color-aware table on a terminal, JSONL in a pipe), so the same
command reads well by hand and parses cleanly downstream.

This is a fresh scaffold. It ships one example resource type, `page`, wired end
to end. Model the real hdx records in `hdx/` and declare their
operations in `hdx/domain.go`; each one becomes a command, an HTTP
route, and an MCP tool at once.

## Serve it

The same operations are available over HTTP and as an MCP tool set for agents,
with no extra code:

```bash
hdx serve --addr :7777    # GET /v1/page/<path>  returns NDJSON
hdx mcp                   # speak MCP over stdio
```

## Use it as a resource-URI driver

`hdx` registers a `hdx` domain the way a program registers a
database driver with `database/sql`. A host enables it with one blank import:

```go
import _ "github.com/tamnd/hdx-cli/hdx"
```

Then [ant](https://github.com/tamnd/ant) (or any program that links the package)
dereferences `hdx://` URIs without knowing anything about hdx:

```bash
ant get hdx://page/<path>   # fetch the record
ant cat hdx://page/<path>   # just the body text
ant ls  hdx://page/<path>   # the pages it links to, each addressable
ant url hdx://page/<path>   # the live https URL
```

## Development

```
cmd/hdx/   thin main: hands cli.NewApp to kit.Run
cli/                 assembles the kit App from the hdx domain
hdx/                the library: HTTP client, data models, and domain.go (the driver)
docs/                tago documentation site
```

```bash
make build      # ./bin/hdx
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the
archives, Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a
cosign signature:

```bash
git tag v0.1.0
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

Apache-2.0. See [LICENSE](LICENSE).
