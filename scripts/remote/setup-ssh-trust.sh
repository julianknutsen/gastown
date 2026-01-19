#!/bin/bash
# setup-ssh-trust.sh - Set up bidirectional SSH trust between local and remote.
#
# For remote polecats to work, the remote must be able to SSH back to local
# (for bd-wrapper). This script helps set that up.
#
# Usage:
#   ./scripts/remote/setup-ssh-trust.sh <ssh-cmd> <local-ssh-cmd>
#
# Example:
#   ./scripts/remote/setup-ssh-trust.sh "ssh ubuntu@remote" "ssh ubuntu@local-ip"
#   ./scripts/remote/setup-ssh-trust.sh "ssh ubuntu@remote" "ssh -i ~/.ssh/internal_key ubuntu@10.0.0.1"

set -e

SSH_CMD="$1"
LOCAL_SSH="$2"

if [ -z "$SSH_CMD" ] || [ -z "$LOCAL_SSH" ]; then
    echo "Usage: $0 <ssh-to-remote> <ssh-from-remote-to-local>"
    echo ""
    echo "Arguments:"
    echo "  ssh-to-remote         SSH command to reach remote from here"
    echo "  ssh-from-remote-to-local  SSH command remote will use to reach here"
    echo ""
    echo "Example:"
    echo "  $0 'ssh ubuntu@remote-host' 'ssh ubuntu@192.168.1.100'"
    echo "  $0 'ssh ubuntu@remote-host' 'ssh -i ~/.ssh/internal_key ubuntu@local-ip'"
    exit 1
fi

echo "=== Setting up bidirectional SSH trust ==="
echo "Local → Remote: $SSH_CMD"
echo "Remote → Local: $LOCAL_SSH"
echo ""

# Step 1: Check if remote has an SSH key
echo "1. Checking remote SSH key..."
REMOTE_KEY=$($SSH_CMD "cat ~/.ssh/id_*.pub 2>/dev/null | head -1" 2>/dev/null || echo "")

if [ -z "$REMOTE_KEY" ]; then
    echo "   No SSH key found on remote, generating one..."
    $SSH_CMD "ssh-keygen -t ed25519 -N '' -f ~/.ssh/id_ed25519 -q"
    REMOTE_KEY=$($SSH_CMD "cat ~/.ssh/id_ed25519.pub")
    echo "   ✓ Generated new key"
else
    echo "   ✓ Remote has SSH key"
fi

# Step 2: Add remote's key to local authorized_keys
echo ""
echo "2. Adding remote's key to local ~/.ssh/authorized_keys..."
if grep -q "$REMOTE_KEY" ~/.ssh/authorized_keys 2>/dev/null; then
    echo "   ✓ Key already authorized"
else
    echo "$REMOTE_KEY" >> ~/.ssh/authorized_keys
    echo "   ✓ Key added"
fi

# Step 3: Add local's host key to remote's known_hosts
echo ""
echo "3. Adding local's host key to remote's known_hosts..."
# Extract host from LOCAL_SSH command
LOCAL_HOST=$(echo "$LOCAL_SSH" | grep -oE '[^ ]+$')
$SSH_CMD "ssh-keyscan $LOCAL_HOST >> ~/.ssh/known_hosts 2>/dev/null" || true
echo "   ✓ Host key added (or already present)"

# Step 4: Test the callback
echo ""
echo "4. Testing callback (remote → local)..."
CALLBACK_TEST=$($SSH_CMD "$LOCAL_SSH 'echo callback-ok'" 2>/dev/null || echo "FAILED")

if [ "$CALLBACK_TEST" = "callback-ok" ]; then
    echo "   ✓ Callback works!"
else
    echo "   ✗ Callback failed. You may need to:"
    echo "     - Check that local SSH server is running"
    echo "     - Verify the local IP is reachable from remote"
    echo "     - Check firewall rules"
    echo ""
    echo "   Try manually: $SSH_CMD \"$LOCAL_SSH 'echo test'\""
    exit 1
fi

echo ""
echo "=== SSH trust setup complete ==="
echo ""
echo "GT_LOCAL_SSH for remote polecats: $LOCAL_SSH"
