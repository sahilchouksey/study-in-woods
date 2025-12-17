#!/bin/bash

# =============================================================================
# Study in Woods - Sync Environment Files to VPS
# =============================================================================
# Transfers .env files (not tracked by git) to the VPS server
#
# Usage:
#   ./scripts/sync-env.sh           # Sync all env files
#   ./scripts/sync-env.sh --dry-run # Preview what would be synced
#   ./scripts/sync-env.sh --pull    # Pull env files FROM VPS to local
# =============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
IP_FILE="$HOME/.ssh/study-in-woods-ip"
REMOTE_DIR="/root/study-in-woods"
LOCAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Read VPS connection string
if [[ ! -f "$IP_FILE" ]]; then
    echo -e "${RED}Error: IP file not found at $IP_FILE${NC}"
    echo "Create it with: echo 'root@YOUR_VPS_IP' > ~/.ssh/study-in-woods-ip"
    exit 1
fi

VPS_HOST=$(cat "$IP_FILE")
echo -e "${BLUE}VPS Host:${NC} $VPS_HOST"
echo -e "${BLUE}Remote Dir:${NC} $REMOTE_DIR"
echo -e "${BLUE}Local Dir:${NC} $LOCAL_DIR"
echo ""

# Environment files to sync (relative to project root)
ENV_FILES=(
    ".env"
    "apps/api/.env"
    "apps/web/.env.local"
    "apps/ocr-service/.env"
)

# Parse arguments
DRY_RUN=false
PULL_MODE=false

for arg in "$@"; do
    case $arg in
        --dry-run)
            DRY_RUN=true
            echo -e "${YELLOW}DRY RUN MODE - No files will be transferred${NC}"
            echo ""
            ;;
        --pull)
            PULL_MODE=true
            echo -e "${YELLOW}PULL MODE - Downloading from VPS to local${NC}"
            echo ""
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --dry-run    Preview what would be synced without transferring"
            echo "  --pull       Pull env files FROM VPS to local machine"
            echo "  --help, -h   Show this help message"
            echo ""
            echo "Environment files synced:"
            for file in "${ENV_FILES[@]}"; do
                echo "  - $file"
            done
            exit 0
            ;;
    esac
done

# Function to sync a single file
sync_file() {
    local file="$1"
    local local_path="$LOCAL_DIR/$file"
    local remote_path="$REMOTE_DIR/$file"
    local remote_dir=$(dirname "$remote_path")

    if $PULL_MODE; then
        # Pull from VPS to local
        if $DRY_RUN; then
            echo -e "${BLUE}[DRY RUN]${NC} Would pull: $VPS_HOST:$remote_path -> $local_path"
        else
            echo -e "${GREEN}Pulling:${NC} $file"
            # Create local directory if needed
            mkdir -p "$(dirname "$local_path")"
            # Pull file (ignore errors if file doesn't exist on remote)
            scp -q "$VPS_HOST:$remote_path" "$local_path" 2>/dev/null && \
                echo -e "  ${GREEN}✓${NC} Downloaded" || \
                echo -e "  ${YELLOW}⚠${NC} Not found on VPS (skipped)"
        fi
    else
        # Push from local to VPS
        if [[ -f "$local_path" ]]; then
            if $DRY_RUN; then
                echo -e "${BLUE}[DRY RUN]${NC} Would push: $local_path -> $VPS_HOST:$remote_path"
            else
                echo -e "${GREEN}Pushing:${NC} $file"
                # Create remote directory if needed
                ssh "$VPS_HOST" "mkdir -p $remote_dir"
                # Copy file
                scp -q "$local_path" "$VPS_HOST:$remote_path"
                echo -e "  ${GREEN}✓${NC} Uploaded"
            fi
        else
            echo -e "${YELLOW}Skipping:${NC} $file (not found locally)"
        fi
    fi
}

# Main execution
echo "=========================================="
if $PULL_MODE; then
    echo "Pulling environment files from VPS..."
else
    echo "Pushing environment files to VPS..."
fi
echo "=========================================="
echo ""

for file in "${ENV_FILES[@]}"; do
    sync_file "$file"
done

echo ""
echo "=========================================="
if $DRY_RUN; then
    echo -e "${YELLOW}Dry run complete. No files were transferred.${NC}"
    echo "Remove --dry-run to actually sync files."
else
    if $PULL_MODE; then
        echo -e "${GREEN}Pull complete!${NC}"
    else
        echo -e "${GREEN}Push complete!${NC}"
    fi
fi
echo "=========================================="

# Show remote directory structure (if pushing)
if ! $DRY_RUN && ! $PULL_MODE; then
    echo ""
    echo "Remote env files:"
    ssh "$VPS_HOST" "find $REMOTE_DIR -name '.env*' -type f 2>/dev/null | head -10" || true
fi
