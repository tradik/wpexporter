# GitHub Actions Versions Report

Generated on Fri Dec  5 13:39:20 GMT 2025

## Summary

| Action | Current | Latest | Status |
|--------|---------|--------|--------|

## How to Update

Use these commands to update:

```bash
# Update actions/checkout from v4 to v6
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|actions/checkout@v4|actions/checkout@v6|g'

# Update actions/setup-go from v5 to v6
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|actions/setup-go@v5|actions/setup-go@v6|g'

# Update actions/setup-node from v4 to v6
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|actions/setup-node@v4|actions/setup-node@v6|g'

# Update actions/upload-artifact from v4 to v5
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|actions/upload-artifact@v4|actions/upload-artifact@v5|g'

# Update docker/build-push-action from v5 to v6
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|docker/build-push-action@v5|docker/build-push-action@v6|g'

# Update docker/login-action from v2 to v3
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|docker/login-action@v2|docker/login-action@v3|g'

# Update docker/metadata-action from v4 to v5
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|docker/metadata-action@v4|docker/metadata-action@v5|g'

# Update docker/setup-buildx-action from v2 to v3
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|docker/setup-buildx-action@v2|docker/setup-buildx-action@v3|g'

# Update golangci/golangci-lint-action from v6 to v9
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|golangci/golangci-lint-action@v6|golangci/golangci-lint-action@v9|g'

# Update softprops/action-gh-release from v1 to v2
find .github/workflows -name "*.yml" -o -name "*.yaml" | xargs sed -i 's|softprops/action-gh-release@v1|softprops/action-gh-release@v2|g'
```

**Note:** Version numbers are based on major releases. Always review release notes before updating for breaking changes.
