#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

CLUSTER_NAME="${AGENTORGS_CLUSTER_NAME:-agentorgs}"
NAMESPACE="${AGENTORGS_NAMESPACE:-agentorgs}"
SKIP_KIND="${AGENTORGS_SKIP_KIND:-0}"
SKIP_BUILD="${AGENTORGS_SKIP_BUILD:-0}"
LLM_API_KEY="${AGENTORGS_LLM_API_KEY:-}"

log() { echo "[AgentOrgs] $1"; }
error() { echo "[AgentOrgs ERROR] $1" >&2; exit 1; }

for cmd in kind helm kubectl docker; do
  command -v "$cmd" >/dev/null 2>&1 || error "$cmd is required"
done

if [ -z "$LLM_API_KEY" ]; then
  error "AGENTORGS_LLM_API_KEY is required"
fi

if [ "$SKIP_KIND" = "0" ]; then
  if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    log "kind cluster ${CLUSTER_NAME} already exists"
  else
    log "creating kind cluster ${CLUSTER_NAME}"
    kind create cluster --name "$CLUSTER_NAME" --config "${PROJECT_ROOT}/hack/kind-config.yaml"
  fi
fi

CONTROLLER_IMAGE="agentorgs/controller:local"
OPENCLAW_AGENT_IMAGE="${AGENTORGS_OPENCLAW_AGENT_IMAGE:-agentorgs/agent-openclaw:local}"
OPENCLAW_BASE_IMAGE="${OPENCLAW_BASE_IMAGE:-ghcr.io/openclaw/openclaw:latest}"
if [ "$SKIP_BUILD" = "0" ]; then
  log "building controller image"
  docker build -t "$CONTROLLER_IMAGE" -f "${PROJECT_ROOT}/Dockerfile" "${PROJECT_ROOT}"
  kind load docker-image "$CONTROLLER_IMAGE" --name "$CLUSTER_NAME"

  log "building OpenClaw agent image from ${OPENCLAW_BASE_IMAGE}"
  docker build -t "$OPENCLAW_AGENT_IMAGE" \
    -f "${PROJECT_ROOT}/agent/openclaw/Dockerfile" \
    --build-arg BASE_IMAGE="$OPENCLAW_BASE_IMAGE" \
    "${PROJECT_ROOT}/agent/openclaw"
  kind load docker-image "$OPENCLAW_AGENT_IMAGE" --name "$CLUSTER_NAME"
fi

log "installing helm chart"
helm upgrade --install agentorgs "${PROJECT_ROOT}/charts/agentorgs" \
  --namespace "$NAMESPACE" --create-namespace \
  --set credentials.llmApiKey="$LLM_API_KEY" \
  --set controller.image.repository=agentorgs/controller \
  --set controller.image.tag=local \
  --set controller.image.pullPolicy=Never \
  --set controller.openclawAgentImage="$OPENCLAW_AGENT_IMAGE" \
  --wait --timeout 10m

log "applying CRDs"
kubectl apply -f "${PROJECT_ROOT}/config/crd/"

log "applying demo resources"
kubectl apply -f "${PROJECT_ROOT}/config/samples/demo.yaml"

cat <<EOF

AgentOrgs is ready.

API:      http://127.0.0.1:8090
Matrix:   http://127.0.0.1:18080
Demo run: ago collaborate --api http://127.0.0.1:8090 --from product-owner --to developer --text "implement login API"

EOF
