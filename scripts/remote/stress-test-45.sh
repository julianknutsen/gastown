#!/bin/bash
# Stress test: spawn polecats in parallel to work on simple tasks
#
# Simple mode: sling plain beads directly to polecats (no formula overhead)
# Formula mode: use --formula flag for full molecule workflow
#
# Usage:
#   ./stress-test-45.sh           # Sling plain beads to polecats
#   ./stress-test-45.sh --status  # Check progress
#   ./stress-test-45.sh --create-beads  # Create test beads first

GT_ROOT="${GT_ROOT:-$HOME/gt2}"
GT_BIN="${GT_BIN:-$HOME/.local/bin/gt}"
RIG="gastown"
PARALLELISM=15
CONVOY_ID="hq-cv-3atyo"  # 50-Polecat Stress Test

# 50 plain beads for stress test (no formula overhead)
BEADS=(
  gt-gwfaj gt-kyv9q gt-087cj gt-nhgvi gt-30kdc gt-x8lzw gt-m0rhl gt-02tht gt-ysv2m gt-46ix6
  gt-l5c5o gt-qzgo3 gt-yofzf gt-78h72 gt-8pb0h gt-ax3gx gt-zphhy gt-dfx2k gt-ng13r gt-6f2n7
  gt-pre1u gt-tpqz9 gt-pw7a5 gt-tshal gt-kpf8b gt-seopv gt-uhkz7 gt-52vpn gt-wpckz gt-sdnzp
  gt-e8rpl gt-rcxoh gt-v8e8q gt-fa62u gt-6typn gt-7rhqx gt-wyk4o gt-jqn7f gt-7wtn8 gt-6v1cg
  gt-jb1uq gt-ygcyo gt-xcryj gt-gue5l gt-yo6u2 gt-hx0my gt-niyfq gt-vveh1 gt-33ybu gt-tpfye
)

sling_bead() {
  local bead=$1
  local index=$2
  local total=$3

  echo "[$index/$total] Slinging $bead to $RIG..."
  if GT_ROOT="$GT_ROOT" "$GT_BIN" sling "$bead" --force --no-convoy "$RIG" 2>&1 | sed "s/^/[$index] /"; then
    echo "[$index/$total] ✓ $bead done"
  else
    echo "[$index/$total] ✗ $bead failed"
    return 1
  fi
}

run_stress_test() {
  echo "Running stress test: ${#BEADS[@]} polecats, parallelism=$PARALLELISM"
  echo "Rig: $RIG"
  echo ""

  local running=0
  local i=0
  local total=${#BEADS[@]}

  for bead in "${BEADS[@]}"; do
    i=$((i + 1))
    sling_bead "$bead" "$i" "$total" &
    running=$((running + 1))

    if [ "$running" -ge "$PARALLELISM" ]; then
      echo "--- Waiting for batch..."
      wait
      running=0
    fi
  done

  if [ "$running" -gt 0 ]; then
    echo "--- Waiting for final batch..."
    wait
  fi

  echo ""
  echo "========================================"
  echo "All slings complete."
}

create_test_beads() {
  echo "Creating test beads..."

  FILES=(
    "internal/cmd/status.go"
    "internal/cmd/done.go"
    "internal/cmd/sling.go"
    "internal/cmd/polecat.go"
    "internal/cmd/deacon.go"
    "internal/agent/agent.go"
    "internal/daemon/daemon.go"
    "internal/factory/factory.go"
    "internal/polecat/manager.go"
    "internal/polecat/backend.go"
  )

  cd "$GT_ROOT/gastown"

  for file in "${FILES[@]}"; do
    echo "Creating bead for $file..."
    branch="comment-$(basename $file .go)"
    bd --no-daemon create "Add header comment to $file" \
      --type task \
      --description "Add a brief comment at the top of $file explaining its purpose.

## Instructions
\`\`\`bash
# 1. Create branch
git checkout -b $branch

# 2. Edit the file - add a 1-2 line comment at the top explaining what this file does

# 3. Build to verify
go build ./...

# 4. Commit and push
git add $file
git commit -m \"docs: add header comment to $file\"
git push -u origin $branch

# 5. Create PR
gh pr create --title \"docs: add header comment to $file\" --body \"Simple documentation improvement\"

# 6. Done
gt done --status COMPLETE
\`\`\`" \
      --json 2>&1 | grep '"id"' &
  done

  wait
  echo ""
  echo "Done. Add the bead IDs to the BEADS array in this script."
}

case "${1:-}" in
  --status)
    echo "Convoy: $CONVOY_ID"
    GT_ROOT="$GT_ROOT" "$GT_BIN" convoy status "$CONVOY_ID"
    echo ""
    echo "Polecats:"
    GT_ROOT="$GT_ROOT" "$GT_BIN" polecat list "$RIG"
    ;;
  --create-beads)
    create_test_beads
    ;;
  --dry-run)
    echo "Dry run: would sling ${#BEADS[@]} beads to $RIG"
    for bead in "${BEADS[@]}"; do
      GT_ROOT="$GT_ROOT" "$GT_BIN" sling "$bead" --force "$RIG" --dry-run
      echo "---"
    done
    ;;
  *)
    run_stress_test
    ;;
esac
