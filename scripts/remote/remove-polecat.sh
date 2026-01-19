#!/bin/bash
# remove-polecat.sh - Remove a polecat worktree from a remote machine.
#
# Usage:
#   ./scripts/remote/remove-polecat.sh <ssh-cmd> <rig-name> <polecat-name>
#
# Example:
#   ./scripts/remote/remove-polecat.sh "ssh ubuntu@remote" "gastown" "polecat-01"

set -e

SSH_CMD="$1"
RIG_NAME="$2"
POLECAT_NAME="$3"

if [ -z "$SSH_CMD" ] || [ -z "$RIG_NAME" ] || [ -z "$POLECAT_NAME" ]; then
    echo "Usage: $0 <ssh-cmd> <rig-name> <polecat-name>"
    exit 1
fi

RIG_PATH="\$HOME/rigs/$RIG_NAME"
BARE_REPO="$RIG_PATH/.repo.git"
POLECAT_DIR="$RIG_PATH/polecats/$POLECAT_NAME"
WORKTREE_PATH="$POLECAT_DIR/$RIG_NAME"

echo "Removing polecat '$POLECAT_NAME' from rig '$RIG_NAME'..."

# Remove worktree via git
$SSH_CMD "git -C $BARE_REPO worktree remove --force $WORKTREE_PATH 2>/dev/null || true"

# Remove polecat directory
$SSH_CMD "rm -rf $POLECAT_DIR"

# Prune
$SSH_CMD "git -C $BARE_REPO worktree prune"

echo "✓ Polecat removed"
