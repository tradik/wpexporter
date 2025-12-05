#!/bin/bash

# Dynamic GitHub Actions version checker with API fallback
# Usage: ./scripts/check-actions-dynamic-fixed.sh

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
WORKFLOWS_DIR="$PROJECT_ROOT/.github/workflows"
REPORT_FILE="$PROJECT_ROOT/docs/github-actions-versions-dynamic.md"

echo -e "${BLUE}🔍 Checking GitHub Actions versions (Dynamic)...${NC}"

# Find all actions used in workflows
echo -e "${BLUE}📁 Scanning workflows for actions...${NC}"
actions=$(find "$WORKFLOWS_DIR" -name "*.yml" -o -name "*.yaml" | \
    xargs grep -hE "uses:\s*[a-zA-Z0-9/._-]+@" | \
    sed -E 's/.*uses:\s*([a-zA-Z0-9/._-]+)@.*/\1/' | \
    sort | uniq)

if [[ -z "$actions" ]]; then
    echo -e "${RED}❌ No actions found in workflows${NC}"
    exit 1
fi

echo "Found actions:"
echo "$actions"
echo ""

# Function to get latest version from GitHub API
get_latest_version() {
    local repo="$1"
    local latest_version
    
    echo -e "${BLUE}🔄 Fetching latest version for $repo...${NC}" >&2
    
    # Try to get from GitHub API
    latest_version=$(curl -s "https://api.github.com/repos/$repo/releases/latest" | \
        jq -r '.tag_name' 2>/dev/null || echo "")
    
    if [[ -n "$latest_version" && "$latest_version" != "null" ]]; then
        echo "$latest_version"
    else
        echo -e "${YELLOW}⚠️  Failed to fetch from API for $repo${NC}" >&2
        echo "unknown"
    fi
}

# Function to extract current version from workflows
get_current_version() {
    local repo="$1"
    local version
    
    version=$(find "$WORKFLOWS_DIR" -name "*.yml" -o -name "*.yaml" | \
        xargs grep -hE "uses:\s*$repo@" | \
        head -1 | \
        sed -E "s/.*uses:\s*$repo@([a-zA-Z0-9._-]+).*/\1/" || echo "unknown")
    
    echo "$version"
}

# Function to compare versions (simple major version comparison)
compare_versions() {
    local current="$1"
    local latest="$2"
    
    # Handle unknown versions
    if [[ "$current" == "unknown" || "$latest" == "unknown" ]]; then
        echo "unknown"
        return
    fi
    
    # Extract major version number
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

# Check each action
echo -e "${BLUE}🔄 Checking versions...${NC}"
up_to_date_count=0
update_available_count=0
unknown_count=0
recommendations=""

while IFS= read -r action; do
    if [[ -z "$action" ]]; then
        continue
    fi
    
    current_version=$(get_current_version "$action")
    latest_version=$(get_latest_version "$action")
    
    if [[ "$latest_version" == "unknown" ]]; then
        status="❓ Unknown"
        ((unknown_count++))
    else
        comparison=$(compare_versions "$current_version" "$latest_version")
        case "$comparison" in
            "up_to_date")
                status="✅ Up to date"
                ((up_to_date_count++))
                ;;
            "update_available")
                status="⚠️ Update available"
                ((update_available_count++))
                recommendations="$recommendations
#### $action
- Current: \`$current_version\`
- Latest: \`$latest_version\`
- Update to: \`$action@$latest_version\`
"
                ;;
            "newer")
                status="🔥 Newer than latest"
                ((unknown_count++))
                ;;
            *)
                status="❓ Unknown"
                ((unknown_count++))
                ;;
        esac
    fi
    
    echo "$action → $status"
    
    # Add to report
    echo "| $action | $current_version | $latest_version | $status |" >> "$REPORT_FILE"
    
done <<< "$actions"

# Add recommendations to report
cat >> "$REPORT_FILE" << EOF

## Recommendations

### Actions to Update
$recommendations

## How to Update

\`\`\`bash
EOF

# Generate update commands dynamically
while IFS= read -r action; do
    if [[ -z "$action" ]]; then
        continue
    fi
    
    current_version=$(get_current_version "$action")
    latest_version=$(get_latest_version "$action")
    
    if [[ "$latest_version" != "unknown" && "$current_version" != "unknown" ]]; then
        comparison=$(compare_versions "$current_version" "$latest_version")
        if [[ "$comparison" == "update_available" ]]; then
            echo "# Update $action from $current_version to $latest_version" >> "$REPORT_FILE"
            echo "find .github/workflows -name \"*.yml\" -o -name \"*.yaml\" | xargs sed -i 's|$action@$current_version|$action@$latest_version|g'" >> "$REPORT_FILE"
            echo "" >> "$REPORT_FILE"
        fi
    fi
done <<< "$actions"

echo "\`\`\`" >> "$REPORT_FILE"

cat >> "$REPORT_FILE" << EOF

**Note:** Version numbers are based on major releases. Always review release notes before updating for breaking changes.
EOF

echo ""
echo -e "${GREEN}✅ Report saved to: $REPORT_FILE${NC}"

# Show summary
echo ""
echo -e "${BLUE}📊 Summary:${NC}"
echo "- Up to date: $up_to_date_count"
echo "- Updates available: $update_available_count"
echo "- Unknown: $unknown_count"

if [[ $update_available_count -gt 0 ]]; then
    echo ""
    echo -e "${YELLOW}⚠️  Actions that need attention:"
    while IFS= read -r action; do
        if [[ -z "$action" ]]; then
            continue
        fi
        
        current_version=$(get_current_version "$action")
        latest_version=$(get_latest_version "$action")
        
        if [[ "$latest_version" != "unknown" && "$current_version" != "unknown" ]]; then
            comparison=$(compare_versions "$current_version" "$latest_version")
            if [[ "$comparison" == "update_available" ]]; then
                echo "  - $action: $current_version → $latest_version"
            fi
        fi
    done <<< "$actions"
fi
