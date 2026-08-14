# Development

Building, testing and finding your way around the source tree. The toolchain is Go 1.26.6 and GNU Make; every task below has a Make target so CI and a laptop run the same command.

## Setup and build

### Prerequisites

- Go 1.26.6 or later (the version `go.mod` declares; earlier 1.26 patches carry
  the standard-library advisories this tool is exposed to)
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
│   ├── mcp/                 # MCP protocol server
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
