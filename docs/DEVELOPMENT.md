# Development

Building, testing and finding your way around the source tree. The toolchain is Go 1.27.0 and GNU Make; every task below has a Make target so CI and a laptop run the same command.

## Setup and build

### Prerequisites

- **Go 1.27.0**, which is what `go.mod` declares and what CI, Docker and the
  snap build with. Go 1.27 rebuilt `encoding/json` on the v2 implementation,
  which roughly halves the time this program spends decoding the REST API — its
  single largest cost, measured at 637 µs → 322 µs for one page of a hundred
  posts.

  Worth knowing if you are weighing the same upgrade elsewhere: that speedup
  follows the **toolchain**, not the `go` line. A module declaring 1.26 and
  built with the 1.27 toolchain runs just as fast. This project declared 1.26.6
  for one day for exactly that reason, because golangci-lint 2.12 could not
  analyse a module targeting 1.27 and panicked part-way through; 2.13.1 can, so
  the line moved up to say plainly what the project is built on.
- Make

### Setup

```bash
# Clone the repository
git clone https://github.com/tradik/wpexporter.git
cd wpexporter

# Install dependencies
make deps

# Install development tools
make dev-install

# Run in development mode
make dev
```

### Building

```bash
# Build for current platform
make build

# Build release binaries for all platforms
make release
```

### Testing

```bash
# Run tests
make test

# Run linter
make lint

# Run the security scanner
make sec

# Format code
make format

# Everything CI runs: vet, lint, gosec, tests
make check
```

`make sec` builds gosec from the `tools/` module rather than expecting it on
`PATH`, so it is the same binary — and the same exclusion list — the pipeline
uses. Nothing to install; the first run compiles it into `build/`.

## Project structure

```
wpexporter/
├── cmd/
│   ├── wpexporter/          # umbrella command — export, xmlrpc, mcp
│   ├── wpexportjson/        # REST API exporter
│   ├── wpxmlrpc/            # XML-RPC exporter
│   └── wpmcp/               # MCP server
├── internal/
│   ├── api/                 # WordPress REST client
│   ├── xmlrpc/              # XML-RPC client
│   ├── mcp/                 # MCP protocol server, both eras (see docs/MCP.md)
│   ├── bruteforce/          # ID enumeration for unlisted content
│   ├── cli/                 # command wiring shared by the binaries
│   ├── config/              # configuration, CLI and file
│   ├── export/              # one exporter per output format
│   ├── media/               # media download and URL rewriting
│   ├── seo/                 # assisted crawl and metadata extraction
│   ├── flathtml/            # HTML to Markdown conversion
│   ├── basichtml/           # HTML reduction for store importers
│   ├── filter/              # content filters (paths, types)
│   ├── cache/               # HTTP response cache
│   ├── checkpoint/          # resume state
│   └── version/             # build stamp
├── pkg/
│   └── models/              # data models, the only exported package
├── docs/                    # the guides, published at wpexporter.tradik.com
├── templates/ssgtheme/      # the documentation site's theme
├── man/                     # man pages
├── tools/                   # separate module: gosec, pinned by tools/go.sum
├── Makefile                 # build automation
├── go.mod                   # Go module definition
└── README.md                # project overview
```
