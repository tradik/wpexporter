#!/bin/bash

# Dynamic GitHub Actions version checker - working version
# Usage: ./scripts/check-actions-dynamic-v2.sh

set -eo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔍 Checking GitHub Actions versions (Dynamic)...${NC}"

# Function to get latest version from GitHub API
get_latest_version() {
    local repo="$1"
    local version
    version=$(curl -s "https://api.github.com/repos/$repo/releases/latest" | jq -r '.tag_name' 2>/dev/null)
    if [[ -z "$version" || "$version" == "null" ]]; then
        echo "unknown"
    else
        echo "$version"
    fi
}

# Function to get current version
get_current_version() {
    local repo="$1"
    local version
    version=$(grep -h "uses.*$repo@" .github/workflows/*.yml | head -1 | sed 's/^[[:space:]]*//' | cut -d'@' -f2)
    if [[ -z "$version" ]]; then
        echo "unknown"
    else
        echo "$version"
    fi
}

# Actions to check
actions=(
    "actions/checkout"
    "actions/setup-go"
    "actions/setup-node"
    "actions/upload-artifact"
    "anchore/sbom-action"
    "docker/build-push-action"
    "docker/login-action"
    "docker/metadata-action"
    "docker/setup-buildx-action"
    "docker/setup-qemu-action"
    "golangci/golangci-lint-action"
    "softprops/action-gh-release"
)

echo "Found actions:"
printf '%s\n' "${actions[@]}"
echo ""

echo -e "${BLUE}🔄 Checking versions...${NC}"

up_to_date=0
updates=0
unknown=0

# Create report
cat > docs/github-actions-versions-dynamic-v2.md << EOF
# GitHub Actions Versions Report (Dynamic)

Generated on $(date)

## Summary

| Action | Current | Latest | Status |
|--------|---------|--------|--------|
EOF

for action in "${actions[@]}"; do
    echo "Checking $action..."
    current=$(get_current_version "$action")
    latest=$(get_latest_version "$action")
    
    if [[ "$latest" == "unknown" ]]; then
        status="❓ Unknown"
        ((unknown++))
    elif [[ "$current" == "unknown" ]]; then
        status="❓ Unknown"
        ((unknown++))
    else
        # Extract major versions for comparison
        current_major=$(echo "$current" | sed 's/v//' | cut -d'.' -f1)
        latest_major=$(echo "$latest" | sed 's/v//' | cut -d'.' -f1)
        
        if [[ "$current_major" -eq "$latest_major" ]]; then
            status="✅ Up to date"
            ((up_to_date++))
        elif [[ "$current_major" -lt "$latest_major" ]]; then
            status="⚠️ Update available"
            ((updates++))
        else
            status="🔥 Newer than latest"
            ((unknown++))
        fi
    fi
    
    echo "$action → $status"
    echo "| $action | $current | $latest | $status |" >> docs/github-actions-versions-dynamic-v2.md
done

echo ""
echo -e "${GREEN}✅ Report saved to: docs/github-actions-versions-dynamic-v2.md${NC}"
echo ""
echo -e "${BLUE}📊 Summary:${NC}"
echo "- Up to date: $up_to_date"
echo "- Updates available: $updates"
echo "- Unknown: $unknown"

if [[ $updates -gt 0 ]]; then
    echo ""
    echo -e "${YELLOW}⚠️  Actions that need attention:"
    for action in "${actions[@]}"; do
        current=$(get_current_version "$action")
        latest=$(get_latest_version "$action")
        
        if [[ "$current" != "unknown" && "$latest" != "unknown" ]]; then
            current_major=$(echo "$current" | sed 's/v//' | cut -d'.' -f1)
            latest_major=$(echo "$latest" | sed 's/v//' | cut -d'.' -f1)
            
            if [[ "$current_major" -lt "$latest_major" ]]; then
                echo "  - $action: $current → $latest"
            fi
        fi
    done
fi
