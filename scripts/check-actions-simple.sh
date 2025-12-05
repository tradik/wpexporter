#!/bin/bash

# Simple dynamic GitHub Actions version checker
# Usage: ./scripts/check-actions-simple.sh

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPORT_FILE="$PROJECT_ROOT/docs/github-actions-versions-simple.md"

echo -e "${BLUE}🔍 Checking GitHub Actions versions (Dynamic)...${NC}"

# Define actions to check
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

# Function to get latest version from GitHub API
get_latest_version() {
    local repo="$1"
    local latest_version
    
    latest_version=$(curl -s "https://api.github.com/repos/$repo/releases/latest" | \
        jq -r '.tag_name' 2>/dev/null || echo "unknown")
    
    echo "$latest_version"
}

# Function to get current version
get_current_version() {
    local repo="$1"
    local version
    
    # Use different delimiter to avoid issues with slashes
    version=$(find .github/workflows -name "*.yml" | \
        xargs grep -h "uses.*$repo@" | \
        head -1 | \
        sed -E "s/.*uses.*$repo@([a-zA-Z0-9._-]+).*/\1/" || echo "unknown")
    
    echo "$version"
}

# Function to compare versions
compare_versions() {
    local current="$1"
    local latest="$2"
    
    if [[ "$current" == "unknown" || "$latest" == "unknown" ]]; then
        echo "unknown"
        return
    fi
    
    # Extract major version
    local current_major=$(echo "$current" | sed -E 's/v?([0-9]+).*/\1/')
    local latest_major=$(echo "$latest" | sed -E 's/v?([0-9]+).*/\1/')
    
    if [[ "$current_major" -eq "$latest_major" ]]; then
        echo "up_to_date"
    elif [[ "$current_major" -lt "$latest_major" ]]; then
        echo "update_available"
    else
        echo "newer"
    fi
}

# Create report
cat > "$REPORT_FILE" << EOF
# GitHub Actions Versions Report (Dynamic)

Generated on $(date)

## Summary

| Action | Current | Latest | Status |
|--------|---------|--------|--------|
EOF

echo -e "${BLUE}🔄 Checking versions...${NC}"
up_to_date=0
updates=0
unknown=0

for action in "${actions[@]}"; do
    current=$(get_current_version "$action")
    latest=$(get_latest_version "$action")
    
    if [[ "$latest" == "unknown" ]]; then
        status="❓ Unknown"
        ((unknown++))
    else
        comparison=$(compare_versions "$current" "$latest")
        case "$comparison" in
            "up_to_date")
                status="✅ Up to date"
                ((up_to_date++))
                ;;
            "update_available")
                status="⚠️ Update available"
                ((updates++))
                ;;
            "newer")
                status="🔥 Newer than latest"
                ((unknown++))
                ;;
            *)
                status="❓ Unknown"
                ((unknown++))
                ;;
        esac
    fi
    
    echo "$action → $status"
    echo "| $action | $current | $latest | $status |" >> "$REPORT_FILE"
done

echo ""
echo -e "${GREEN}✅ Report saved to: $REPORT_FILE${NC}"
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
        
        if [[ "$latest" != "unknown" && "$current" != "unknown" ]]; then
            comparison=$(compare_versions "$current" "$latest")
            if [[ "$comparison" == "update_available" ]]; then
                echo "  - $action: $current → $latest"
            fi
        fi
    done
fi
