# GitHub Actions Versions Report

Updated: 2026-04-07

## Current Versions in CI/CD Pipeline

| Action | Version | Node.js | Notes |
|--------|---------|---------|-------|
| `actions/checkout` | v6 | 24 | |
| `actions/setup-go` | v6 | 24 | |
| `actions/upload-artifact` | v7 | 24 | |
| `actions/download-artifact` | v8 | 24 | |
| `codecov/codecov-action` | v6 | 24 | |
| `golangci/golangci-lint-action` | v9 | 24 | |
| `securego/gosec` | master | Docker | |
| `softprops/action-gh-release` | v2 | 24 | |
| `docker/build-push-action` | v7 | 24 | |
| `docker/setup-buildx-action` | v4 | 24 | |
| `docker/login-action` | v4 | 24 | |
| `docker/metadata-action` | v6 | 24 | |
| `snapcore/action-build` | v1 | - | |
| `snapcore/action-publish` | v1 | - | |

## Update Commands

```bash
# Upgrade to latest Node 24 compatible versions
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|actions/upload-artifact@v5|actions/upload-artifact@v7|g'
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|actions/download-artifact@v4|actions/download-artifact@v8|g'
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|codecov/codecov-action@v5|codecov/codecov-action@v6|g'
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|docker/build-push-action@v6|docker/build-push-action@v7|g'
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|docker/setup-buildx-action@v3|docker/setup-buildx-action@v4|g'
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|docker/login-action@v3|docker/login-action@v4|g'
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|docker/metadata-action@v5|docker/metadata-action@v6|g'
```

**Note:** All actions updated to Node.js 24 runtime. Node.js 20 will be removed from GitHub Actions runners on September 16th, 2026.
