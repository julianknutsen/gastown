#!/bin/bash
# add-polecat.sh - Create a polecat worktree on a remote machine.
#
# Creates a worktree from the rig's bare repo:
#   ~/rigs/<rigname>/polecats/<polecat-name>/<rigname>/
#
# The worktree gets a unique branch and .beads/redirect pointing to local.
#
# Usage:
#   ./scripts/remote/add-polecat.sh <ssh-cmd> <rig-name> <polecat-name> [branch-base]
#
# Example:
#   ./scripts/remote/add-polecat.sh "ssh ubuntu@remote" "gastown" "polecat-01"
#   ./scripts/remote/add-polecat.sh "ssh ubuntu@remote" "gastown" "polecat-01" "origin/main"

set -e

SSH_CMD="$1"
RIG_NAME="$2"
POLECAT_NAME="$3"
BRANCH_BASE="${4:-origin/main}"

if [ -z "$SSH_CMD" ] || [ -z "$RIG_NAME" ] || [ -z "$POLECAT_NAME" ]; then
    echo "Usage: $0 <ssh-cmd> <rig-name> <polecat-name> [branch-base]"
    echo ""
    echo "Arguments:"
    echo "  ssh-cmd       SSH command to reach remote"
    echo "  rig-name      Name of the rig"
    echo "  polecat-name  Name for the polecat (e.g., 'polecat-01')"
    echo "  branch-base   Starting point for branch (default: origin/main)"
    echo ""
    echo "Example:"
    echo "  $0 'ssh ubuntu@remote' 'gastown' 'polecat-01'"
    exit 1
fi

RIG_PATH="\$HOME/rigs/$RIG_NAME"
BARE_REPO="$RIG_PATH/.repo.git"
POLECAT_DIR="$RIG_PATH/polecats/$POLECAT_NAME"
WORKTREE_PATH="$POLECAT_DIR/$RIG_NAME"
TIMESTAMP=$(date +%s)
BRANCH_NAME="polecat/$POLECAT_NAME-$TIMESTAMP"

echo "=== Creating polecat '$POLECAT_NAME' on remote ==="
echo "SSH: $SSH_CMD"
echo "Rig: $RIG_NAME"
echo "Worktree: $WORKTREE_PATH"
echo "Branch: $BRANCH_NAME"
echo ""

# Step 1: Check rig exists
echo "1. Checking rig setup..."
if ! $SSH_CMD "test -d $BARE_REPO" 2>/dev/null; then
    echo "   ERROR: Rig not set up. Run setup-rig.sh first."
    exit 1
fi
echo "   ✓ Rig exists"

# Step 2: Fetch latest
echo ""
echo "2. Fetching latest from origin..."
$SSH_CMD "git -C $BARE_REPO fetch origin --prune"
echo "   ✓ Fetched"

# Step 3: Check if polecat already exists
echo ""
echo "3. Checking for existing polecat..."
if $SSH_CMD "test -d $WORKTREE_PATH" 2>/dev/null; then
    echo "   Polecat exists, removing old worktree..."
    $SSH_CMD "git -C $BARE_REPO worktree remove --force $WORKTREE_PATH 2>/dev/null || rm -rf $POLECAT_DIR"
    echo "   ✓ Old worktree removed"
fi

# Step 4: Create polecat directory
echo ""
echo "4. Creating polecat directory..."
$SSH_CMD "mkdir -p $POLECAT_DIR"
echo "   ✓ Created $POLECAT_DIR"

# Step 5: Create worktree
echo ""
echo "5. Creating worktree..."
$SSH_CMD "git -C $BARE_REPO worktree add -b $BRANCH_NAME $WORKTREE_PATH $BRANCH_BASE"
echo "   ✓ Worktree created"

# Step 6: Set up .beads/redirect
echo ""
echo "6. Setting up .beads/redirect..."
# Read local beads path from rig config
LOCAL_BEADS=$($SSH_CMD "grep LOCAL_BEADS $RIG_PATH/.rig-config | cut -d= -f2")
if [ -n "$LOCAL_BEADS" ]; then
    $SSH_CMD "mkdir -p $WORKTREE_PATH/.beads && echo '$LOCAL_BEADS' > $WORKTREE_PATH/.beads/redirect"
    echo "   ✓ Redirect -> $LOCAL_BEADS"
else
    echo "   WARNING: No LOCAL_BEADS in rig config, skipping redirect"
fi

# Step 7: Prune stale worktrees
echo ""
echo "7. Pruning stale worktrees..."
$SSH_CMD "git -C $BARE_REPO worktree prune"
echo "   ✓ Pruned"

echo ""
echo "=== Polecat created ==="
echo ""
echo "Worktree: ~/rigs/$RIG_NAME/polecats/$POLECAT_NAME/$RIG_NAME"
echo "Branch: $BRANCH_NAME"
echo ""
echo "To spawn Claude in this polecat, use RemoteTmuxWithCallback with:"
echo "  workDir: ~/rigs/$RIG_NAME/polecats/$POLECAT_NAME/$RIG_NAME"
