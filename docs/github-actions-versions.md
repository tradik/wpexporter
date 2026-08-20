# GitHub Actions Versions Report

Updated: 2026-08-14

Every action is pinned by commit SHA, never by tag: a tag is mutable and can be
moved onto different code after review. The trailing comment records which
release that SHA *is*, down to the patch — a bare `# v6` next to a SHA cannot be
checked against anything, and one of them had drifted a whole major behind what
the comment claimed.

## Current Versions in CI/CD Pipeline

### `.github/workflows/ci.yml`

| Action | Version | Node.js | Notes |
|--------|---------|---------|-------|
| `actions/checkout` | v7.0.1 | 24 | |
| `actions/setup-go` | v7.0.0 | 24 | ESM runtime, `@actions/cache` 6.2 |
| `actions/upload-artifact` | v7.0.1 | 24 | |
| `actions/download-artifact` | v8.0.1 | 24 | |
| `codecov/codecov-action` | v7.0.0 | 24 | |
| `golangci/golangci-lint-action` | v9.3.0 | 24 | linter itself resolved as `latest` |
| `softprops/action-gh-release` | v3.0.2 | 24 | v3 = Node 24 runtime; inputs unchanged from v2 |
| `docker/build-push-action` | v7.3.0 | 24 | |
| `docker/setup-buildx-action` | v4.2.0 | 24 | |
| `docker/login-action` | v4.6.0 | 24 | |
| `docker/metadata-action` | v6.2.0 | 24 | |
| `snapcore/action-build` | v1 | 24 (forced) | `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true` |
| `snapcore/action-publish` | v1 | 24 (forced) | `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24=true` |

### `.github/workflows/docs-site.yml`

| Action | Version | Node.js | Notes |
|--------|---------|---------|-------|
| `actions/checkout` | v7.0.1 | 24 | |
| `spagu/ssg` | v1.8.32 | Docker | the site generator, pinned to its newest release |

### Toolchain pinned in the workflow

| Tool | Version | Where |
|------|---------|-------|
| Go | 1.27.0 | `go-version:` in every job, and `go 1.27.0` in `go.mod` |
| gosec | v2.28.0 | `tool` directive in `tools/go.mod`, every transitive version fixed by `tools/go.sum` |
| golangci-lint | latest | `version: latest` in the lint job |

gosec lives in a **separate `tools/` module** rather than in the project's own
`go.mod`: it is built with `go -C tools build`, so the scanner and everything
beneath it come from a lock file, while its dependency tree — gRPC,
OpenTelemetry, a handful of cloud SDKs — stays out of wpexporter's. `go install
pkg@version` would pin only gosec itself and re-resolve the rest on every run.

Upgrading it is two commands:

```bash
go -C tools get -tool github.com/securego/gosec/v2/cmd/gosec@vX.Y.Z
make sec        # builds the pinned scanner and runs it exactly as CI does
```

## Checking for updates

`scripts/check-actions-dynamic-v2.sh` reports what is available. To verify a
pin by hand — resolve the SHA the workflow uses and compare it to the tag the
comment claims:

```bash
# Which release is this SHA?
gh api "repos/actions/checkout/tags?per_page=100" --paginate \
  --jq '.[] | select(.commit.sha=="<SHA>") | .name'

# What is the newest release, and which commit does it point at?
gh api repos/actions/checkout/releases/latest --jq .tag_name
gh api repos/actions/checkout/git/ref/tags/v7.0.1 --jq .object.sha
```

Annotated tags answer with a tag object rather than a commit; dereference it
with `gh api repos/<owner>/<repo>/git/tags/<sha> --jq .object.sha`.

**Note:** All actions run on the Node.js 24 runtime. Node.js 20 leaves the
GitHub Actions runners on September 16th, 2026.
