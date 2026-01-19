#!/bin/bash
# setup-rig.sh - Set up a rig on a remote machine using bare repo + worktrees.
#
# Creates the same structure as local rigs:
#   ~/rigs/<rigname>/.repo.git    - Bare repo (shared by all polecats)
#   ~/rigs/<rigname>/polecats/    - Polecat worktrees go here
#
# Usage:
#   ./scripts/remote/setup-rig.sh <ssh-cmd> <rig-name> <repo-url> <local-beads-path>
#
# Example:
#   ./scripts/remote/setup-rig.sh "ssh ubuntu@remote" "gastown" "git@github.com:user/gastown.git" "/home/ubuntu/town/gastown/.beads"

set -e

SSH_CMD="$1"
RIG_NAME="$2"
REPO_URL="$3"
LOCAL_BEADS="$4"

if [ -z "$SSH_CMD" ] || [ -z "$RIG_NAME" ] || [ -z "$REPO_URL" ] || [ -z "$LOCAL_BEADS" ]; then
    echo "Usage: $0 <ssh-cmd> <rig-name> <repo-url> <local-beads-path>"
    echo ""
    echo "Arguments:"
    echo "  ssh-cmd          SSH command to reach remote"
    echo "  rig-name         Name of the rig (e.g., 'gastown')"
    echo "  repo-url         Git repo URL"
    echo "  local-beads-path Path to .beads on LOCAL machine"
    echo ""
    echo "Example:"
    echo "  $0 'ssh ubuntu@remote' 'gastown' 'git@github.com:user/repo.git' '/home/ubuntu/town/gastown/.beads'"
    exit 1
fi

RIG_PATH="\$HOME/rigs/$RIG_NAME"
BARE_REPO="$RIG_PATH/.repo.git"

echo "=== Setting up rig '$RIG_NAME' on remote ==="
echo "SSH: $SSH_CMD"
echo "Rig path: $RIG_PATH"
echo "Repo: $REPO_URL"
echo "Local beads: $LOCAL_BEADS"
echo ""

# Step 1: Create rig directory structure
echo "1. Creating rig directory structure..."
$SSH_CMD "mkdir -p $RIG_PATH/polecats"
echo "   ✓ Created $RIG_PATH/polecats/"

# Step 2: Clone or update bare repo
echo ""
echo "2. Setting up bare repo..."
if $SSH_CMD "test -d $BARE_REPO" 2>/dev/null; then
    echo "   Bare repo exists, updating..."
else
    echo "   Cloning bare repo..."
    $SSH_CMD "git clone --bare $REPO_URL $BARE_REPO"
    echo "   ✓ Bare repo cloned"
fi

# Ensure fetch refspec is configured (bare clone doesn't set this up)
echo "   Configuring fetch refspec..."
$SSH_CMD "git -C $BARE_REPO config remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'"
echo "   ✓ Fetch refspec configured"

# Fetch all branches
echo "   Fetching branches..."
$SSH_CMD "git -C $BARE_REPO fetch origin --prune"
echo "   ✓ Fetched latest"

# Step 3: Store config for worktree creation
echo ""
echo "3. Storing rig config..."
$SSH_CMD "cat > $RIG_PATH/.rig-config << 'EOF'
RIG_NAME=$RIG_NAME
LOCAL_BEADS=$LOCAL_BEADS
EOF"
echo "   ✓ Config saved"

echo ""
echo "=== Rig setup complete ==="
echo ""
echo "Remote rig ready at: ~/rigs/$RIG_NAME"
echo ""
echo "To create a polecat worktree:"
echo "  ./scripts/remote/add-polecat.sh '$SSH_CMD' '$RIG_NAME' 'polecat-01'"
