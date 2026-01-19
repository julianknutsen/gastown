#!/bin/bash
# setup-remote.sh - Prepare a remote machine for Gas Town polecats.
#
# This script SSHes to a remote machine and sets up everything needed
# to run polecats there. Run this once per remote machine.
#
# Usage:
#   ./scripts/remote/setup-remote.sh <ssh-cmd>
#
# Example:
#   ./scripts/remote/setup-remote.sh "ssh ubuntu@remote-host"
#   ./scripts/remote/setup-remote.sh "ssh -i ~/.ssh/key user@10.0.0.5"

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <ssh-command>"
    echo "Example: $0 'ssh ubuntu@remote-host'"
    exit 1
fi

SSH_CMD="$1"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== Setting up remote machine for Gas Town polecats ==="
echo "SSH: $SSH_CMD"
echo ""

# Check SSH connectivity
echo "1. Testing SSH connection..."
if ! $SSH_CMD "echo 'SSH OK'" 2>/dev/null; then
    echo "ERROR: Cannot connect via SSH"
    exit 1
fi
echo "   ✓ SSH connected"

# Check/install Node.js
echo ""
echo "2. Checking Node.js..."
if $SSH_CMD "which node" >/dev/null 2>&1; then
    NODE_VERSION=$($SSH_CMD "node --version" 2>/dev/null)
    echo "   ✓ Node.js installed: $NODE_VERSION"
else
    echo "   Installing Node.js..."
    $SSH_CMD "curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash - && sudo apt-get install -y nodejs"
    echo "   ✓ Node.js installed"
fi

# Check/install Claude Code
echo ""
echo "3. Checking Claude Code..."
if $SSH_CMD "which claude" >/dev/null 2>&1; then
    CLAUDE_VERSION=$($SSH_CMD "claude --version 2>/dev/null | head -1" 2>/dev/null || echo "installed")
    echo "   ✓ Claude Code installed: $CLAUDE_VERSION"
else
    echo "   Installing Claude Code..."
    $SSH_CMD "sudo npm install -g @anthropic-ai/claude-code"
    echo "   ✓ Claude Code installed"
fi

# Install bd-wrapper
echo ""
echo "4. Installing bd-wrapper..."
scp "$SCRIPT_DIR/bd-wrapper.sh" "$($SSH_CMD 'echo $USER')@${SSH_CMD#* }:/tmp/bd-wrapper.sh" 2>/dev/null || \
    cat "$SCRIPT_DIR/bd-wrapper.sh" | $SSH_CMD "cat > /tmp/bd-wrapper.sh"
$SSH_CMD "sudo mv /tmp/bd-wrapper.sh /usr/local/bin/bd && sudo chmod +x /usr/local/bin/bd"
echo "   ✓ bd-wrapper installed at /usr/local/bin/bd"

# Install gt-wrapper
echo ""
echo "5. Installing gt-wrapper..."
cat "$SCRIPT_DIR/gt-wrapper.sh" | $SSH_CMD "cat > /tmp/gt-wrapper.sh"
$SSH_CMD "sudo mv /tmp/gt-wrapper.sh /usr/local/bin/gt && sudo chmod +x /usr/local/bin/gt"
echo "   ✓ gt-wrapper installed at /usr/local/bin/gt"

# Check GitHub CLI
echo ""
echo "6. Checking GitHub CLI..."
if $SSH_CMD "which gh" >/dev/null 2>&1; then
    echo "   ✓ GitHub CLI installed"
    if $SSH_CMD "gh auth status" >/dev/null 2>&1; then
        echo "   ✓ GitHub CLI authenticated"
    else
        echo "   ⚠ GitHub CLI not authenticated (run 'gh auth login' on remote)"
    fi
else
    echo "   Installing GitHub CLI..."
    $SSH_CMD "curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg && sudo chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg && echo 'deb [arch=\$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main' | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null && sudo apt update && sudo apt install gh -y"
    echo "   ✓ GitHub CLI installed"
    echo "   ⚠ Run 'gh auth login' on remote to authenticate"
fi

# Check git config
echo ""
echo "7. Checking git config..."
GIT_NAME=$($SSH_CMD "git config --global user.name" 2>/dev/null || echo "")
GIT_EMAIL=$($SSH_CMD "git config --global user.email" 2>/dev/null || echo "")
if [ -n "$GIT_NAME" ] && [ -n "$GIT_EMAIL" ]; then
    echo "   ✓ Git configured: $GIT_NAME <$GIT_EMAIL>"
else
    echo "   ⚠ Git user not configured. Setting up AI bot identity..."
    $SSH_CMD "git config --global user.name 'AI Polecat Bot'"
    $SSH_CMD "git config --global user.email 'polecat-bot@gastown.ai'"
    echo "   ✓ Git configured as: AI Polecat Bot <polecat-bot@gastown.ai>"
fi

# Create rigs directory
echo ""
echo "8. Creating rigs directory..."
$SSH_CMD "mkdir -p ~/rigs"
echo "   ✓ ~/rigs directory created"

echo ""
echo "=== Remote machine setup complete ==="
echo ""
echo "Next steps:"
echo "  1. Set up git credentials on remote:"
echo "     $SSH_CMD"
echo "     gh auth login  # Choose SSH, authenticate"
echo ""
echo "  2. Set up bidirectional SSH trust (for beads callback):"
echo "     ./scripts/remote/setup-ssh-trust.sh '$SSH_CMD'"
echo ""
echo "  3. Set up a rig on remote:"
echo "     ./scripts/remote/setup-rig.sh '$SSH_CMD' '<rig-name>' '<repo-url>' '<local-beads-path>'"
echo ""
echo "  4. Configure your local rig's settings/config.json with remote section"
