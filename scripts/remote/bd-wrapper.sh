#!/bin/bash
# bd-wrapper.sh - Proxies bd commands back to the local machine via SSH.
#
# This script is installed on remote machines where polecats run.
# When a polecat runs bd commands (for beads operations), this wrapper
# intercepts them and executes them on the local machine via SSH.
#
# This ensures all state (beads database) stays on the local machine,
# providing a single source of truth with no sync issues.
#
# Usage:
#   1. Install this as /usr/local/bin/bd on the remote machine
#   2. Rename the real bd binary to /usr/local/bin/bd-real
#   3. Set GT_LOCAL_SSH environment variable when starting the polecat session
#      Example: GT_LOCAL_SSH="ssh -o ControlPath=/tmp/gt-ssh-%r@%h:%p user@local-machine"
#
# The wrapper passes ALL arguments through, including --dir flags.
# The --dir paths are local machine paths (from .beads/redirect), and
# they are valid on the local machine when the command is SSHed back.

set -e

# If GT_LOCAL_SSH is not set, run locally (fallback for non-remote contexts)
if [ -z "$GT_LOCAL_SSH" ]; then
    # Try to run the real bd binary if it exists
    if [ -x /usr/local/bin/bd-real ]; then
        exec /usr/local/bin/bd-real "$@"
    else
        # Otherwise just run bd from PATH (probably not a remote polecat)
        exec bd "$@"
    fi
fi

# Proxy the command to the local machine via SSH
# cd to rig's polecats directory so beads detection works
if [ -n "$GT_ROOT" ] && [ -n "$GT_RIG" ]; then
    POLECATS_DIR="$GT_ROOT/$GT_RIG/polecats"
    exec $GT_LOCAL_SSH "cd '$POLECATS_DIR' && BD_ACTOR='$BD_ACTOR' bd $*"
else
    exec $GT_LOCAL_SSH bd "$@"
fi
