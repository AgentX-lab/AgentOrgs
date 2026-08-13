#!/bin/bash
set -euo pipefail
CLUSTER_NAME="${AGENTORGS_CLUSTER_NAME:-agentorgs}"
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  kind delete cluster --name "$CLUSTER_NAME"
fi
