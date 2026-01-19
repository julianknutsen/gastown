#!/bin/bash
# gt-wrapper.sh - Proxies gt commands back to the local machine via SSH.
#
# This script is installed on remote machines where polecats run.
# When a polecat runs gt commands, this wrapper intercepts them and
# executes them on the local machine via SSH.
#
# This ensures all state stays on the local machine, providing a
# single source of truth with no sync issues.
#
# Usage:
#   1. Install this as /usr/local/bin/gt on the remote machine
#   2. Rename the real gt binary to /usr/local/bin/gt-real (if exists)
#   3. Set GT_LOCAL_SSH environment variable when starting the polecat session
#      Example: GT_LOCAL_SSH="ssh -o ControlPath=/tmp/gt-ssh-%r@%h:%p user@local-machine"
#
# The wrapper passes ALL arguments through to the local gt command.

set -e

# If GT_LOCAL_SSH is not set, run locally (fallback for non-remote contexts)
if [ -z "$GT_LOCAL_SSH" ]; then
    # Try to run the real gt binary if it exists
    if [ -x /usr/local/bin/gt-real ]; then
        exec /usr/local/bin/gt-real "$@"
    else
        # Otherwise just run gt from PATH (probably not a remote polecat)
        exec gt "$@"
    fi
fi

# Proxy the command to the local machine via SSH
# cd to rig's polecats directory so workspace/beads detection works
# Pass GT_ROLE/GT_RIG/GT_POLECAT for role detection
if [ -n "$GT_ROOT" ] && [ -n "$GT_RIG" ]; then
    POLECATS_DIR="$GT_ROOT/$GT_RIG/polecats"
    exec $GT_LOCAL_SSH "cd '$POLECATS_DIR' && GT_ROLE='$GT_ROLE' GT_RIG='$GT_RIG' GT_POLECAT='$GT_POLECAT' gt $*"
else
    exec $GT_LOCAL_SSH gt "$@"
fi
