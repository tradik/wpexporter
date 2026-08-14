# Installation

wpexporter ships as a single static binary for Linux, macOS, Windows and FreeBSD, plus a Docker image. Every package installs the umbrella `wpexporter` command alongside the three standalone tools — `wpexportjson`, `wpxmlrpc` and `wpmcp` — which behave identically to their subcommand form.

## Homebrew (macOS / Linux)

```bash
brew install tradik/tap/wpexporter
```

This installs the `wpexporter` umbrella command plus the `wpexportjson`, `wpxmlrpc` and
`wpmcp` binaries, and the man pages.

## Snap (Linux)

```bash
sudo snap install wpexporter
```

Provides the `wpexporter` command, plus `wpexporter.wpexportjson`, `wpexporter.wpxmlrpc`
and `wpexporter.wpmcp`.

## From Source

```bash
git clone https://github.com/tradik/wpexporter.git
cd wpexporter
make build

# Optional: Install man pages (requires sudo)
sudo make install-man
man wpexportjson
```

## Using Go Install

```bash
go install github.com/tradik/wpexporter/cmd/wpexporter@latest
go install github.com/tradik/wpexporter/cmd/wpexportjson@latest
go install github.com/tradik/wpexporter/cmd/wpxmlrpc@latest
go install github.com/tradik/wpexporter/cmd/wpmcp@latest
```

## Using Docker

Docker images are available from GitHub Container Registry:

```bash
# Pull the latest image
docker pull ghcr.io/tradik/wpexporter:latest

# Run wpexporter
docker run --rm -v $(pwd)/export:/export ghcr.io/tradik/wpexporter:latest \
  wpexportjson export --url https://example.com --output /export

# Run wpxmlrpc
docker run --rm -v $(pwd)/export:/export ghcr.io/tradik/wpexporter:latest \
  wpxmlrpc export --url https://example.com --username admin --password mypassword --output /export

# Run wpmcp (MCP server over stdio)
docker run --rm -i ghcr.io/tradik/wpexporter:latest wpmcp
```

All four binaries — `wpexporter`, `wpexportjson`, `wpxmlrpc` and `wpmcp` — ship in the image.
