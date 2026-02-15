#!/usr/bin/env bash
set -euo pipefail
litellm --config /app/litellm-config.yaml --port 4000 &
LITELLM_PID=$!
echo "$LITELLM_PID" > /tmp/litellm.pid
# Poll /health endpoint
for i in $(seq 1 30); do
  if curl -sf http://localhost:4000/health > /dev/null 2>&1; then
    echo "LiteLLM ready (PID $LITELLM_PID)"
    exit 0
  fi
  sleep 1
done
echo "LiteLLM failed to start within 30s" >&2
exit 1
