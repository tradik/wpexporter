#!/bin/bash

# Simple script to check GitHub Actions versions
# Usage: ./scripts/check-actions.sh

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REPORT_FILE="$PROJECT_ROOT/docs/github-actions-versions.md"

echo -e "${BLUE}🔍 Checking GitHub Actions versions...${NC}"

# Find workflow files
workflow_files=$(find "$PROJECT_ROOT/.github/workflows" -name "*.yml" -o -name "*.yaml" 2>/dev/null)

if [[ -z "$workflow_files" ]]; then
    echo -e "${RED}❌ No workflow files found${NC}"
    exit 1
fi

# Extract actions
actions=$(echo "$workflow_files" | xargs grep -h "uses:" | sed 's/.*uses:[[:space:]]*//' | sed 's/[[:space:]]*$//' | sort -u)

echo -e "${GREEN}Found actions:${NC}"
echo "$actions"

# Known latest versions (updated Dec 2025 - from GitHub API)
declare -A latest_versions=(
    ["actions/checkout"]="v6"
    ["actions/setup-go"]="v6"
    ["actions/setup-node"]="v6"
    ["actions/upload-artifact"]="v5"
    ["golangci/golangci-lint-action"]="v9"
    ["softprops/action-gh-release"]="v2"
    ["docker/build-push-action"]="v6"
    ["docker/login-action"]="v3"
    ["docker/metadata-action"]="v5"
    ["docker/setup-buildx-action"]="v3"
    ["docker/setup-qemu-action"]="v3"
    ["anchore/sbom-action"]="v0"
)

# Create report
cat > "$REPORT_FILE" << EOF
# GitHub Actions Versions Report

Generated on $(date)

## Summary

| Action | Current | Latest | Status |
|--------|---------|--------|--------|
EOF

echo -e "\n${BLUE}🔄 Checking versions...${NC}"

# Track updates needed
updates_needed=""

# Process actions
while IFS= read -r action; do
    [[ -z "$action" ]] && continue
    
    if [[ "$action" =~ ^([^@]+)@(.+)$ ]]; then
        repo="${BASH_REMATCH[1]}"
        current="${BASH_REMATCH[2]}"
        latest="${latest_versions[$repo]:-unknown}"
        
        if [[ "$latest" == "unknown" ]]; then
            status="❓ Unknown"
        elif [[ "$current" == "$latest" ]]; then
            status="✅ Up to date"
        else
            status="⚠️ Update available"
            updates_needed="$updates_needed$repo|$current|$latest\n"
        fi
        
        echo "| $repo | $current | $latest | $status |" >> "$REPORT_FILE"
        echo -e "${YELLOW}$action${NC} → $status"
    fi
done <<< "$actions"

# Add recommendations if needed
if [[ -n "$updates_needed" ]]; then
    cat >> "$REPORT_FILE" << EOF

## Recommendations

### Actions to Update

EOF
    echo -e "$updates_needed" | while IFS='|' read -r repo current latest; do
        [[ -z "$repo" ]] && continue
        echo "#### $repo" >> "$REPORT_FILE"
        echo "- Current: \`$current\`" >> "$REPORT_FILE"
        echo "- Latest: \`$latest\`" >> "$REPORT_FILE"
        echo "- Update to: \`$repo@$latest\`" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
    done
fi

# Add footer
cat >> "$REPORT_FILE" << EOF

## How to Update

Use these commands to update:

\`\`\`bash
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
\`\`\`

**Note:** Version numbers are based on major releases. Always review release notes before updating for breaking changes.
EOF

echo -e "\n${GREEN}✅ Report saved to: $REPORT_FILE${NC}"

if [[ -n "$updates_needed" ]]; then
    echo -e "\n${YELLOW}⚠️  Actions that need attention:${NC}"
    echo -e "$updates_needed" | while IFS='|' read -r repo current latest; do
        [[ -z "$repo" ]] && continue
        echo "  - $repo: $current → $latest"
    done
fi
