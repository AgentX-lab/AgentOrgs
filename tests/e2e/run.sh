#!/usr/bin/env bash
# Bootstrap kind + mock-llm + helm, then run Go e2e assertions.
# mock-llm manifests live only under tests/e2e/; production must use a real LLM key.
#
# Env:
#   AGENTORGS_E2E_FIXTURE  relative to tests/e2e/ (default fixtures/mention_group_leader.yaml)
#   AGENTORGS_E2E_RUN      go test -run regexp (default TestMentionGroupLeader)
#   AGENTORGS_CLUSTER_NAME kind cluster name (default agentorgs-e2e)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

CLUSTER_NAME="${AGENTORGS_CLUSTER_NAME:-agentorgs-e2e}"
NAMESPACE="${AGENTORGS_NAMESPACE:-agentorgs}"
SKIP_KIND="${AGENTORGS_SKIP_KIND:-0}"
SKIP_BUILD="${AGENTORGS_SKIP_BUILD:-0}"
E2E_FIXTURE="${AGENTORGS_E2E_FIXTURE:-fixtures/mention_group_leader.yaml}"
E2E_RUN="${AGENTORGS_E2E_RUN:-TestMentionGroupLeader}"

log() { echo "[e2e] $*"; }
die() { echo "[e2e ERROR] $*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || die "$1 is required"; }
need kind
need helm
need kubectl
need docker
need go

if [ "$SKIP_KIND" = "0" ]; then
  if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    log "kind cluster ${CLUSTER_NAME} exists"
  else
    log "creating kind cluster ${CLUSTER_NAME}"
    kind create cluster --name "$CLUSTER_NAME" --config "${PROJECT_ROOT}/hack/kind-config.yaml"
  fi
  kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null
fi

CONTROLLER_IMAGE="agentorgs/controller:local"
OPENCLAW_AGENT_IMAGE="${AGENTORGS_OPENCLAW_AGENT_IMAGE:-agentorgs/agent-openclaw:local}"
OPENCLAW_BASE_IMAGE="${OPENCLAW_BASE_IMAGE:-ghcr.io/openclaw/openclaw:latest}"
HERMES_AGENT_IMAGE="${AGENTORGS_HERMES_AGENT_IMAGE:-agentorgs/agent-hermes:local}"

if [ "$SKIP_BUILD" = "0" ]; then
  log "building controller"
  docker build -t "$CONTROLLER_IMAGE" -f "${PROJECT_ROOT}/Dockerfile" "${PROJECT_ROOT}"
  kind load docker-image "$CONTROLLER_IMAGE" --name "$CLUSTER_NAME"

  log "building openclaw agent"
  docker build -t "$OPENCLAW_AGENT_IMAGE" \
    -f "${PROJECT_ROOT}/agent/openclaw/Dockerfile" \
    --build-arg BASE_IMAGE="$OPENCLAW_BASE_IMAGE" \
    "${PROJECT_ROOT}/agent/openclaw"
  kind load docker-image "$OPENCLAW_AGENT_IMAGE" --name "$CLUSTER_NAME"

  log "building hermes agent"
  docker build -t "$HERMES_AGENT_IMAGE" \
    -f "${PROJECT_ROOT}/agent/hermes/Dockerfile" \
    "${PROJECT_ROOT}/agent/hermes"
  kind load docker-image "$HERMES_AGENT_IMAGE" --name "$CLUSTER_NAME"
fi

MOCK_LLM_IMAGE="agentorgs/mock-llm:local"
log "building mock-llm"
docker build -t "$MOCK_LLM_IMAGE" -f "${SCRIPT_DIR}/mock-llm/Dockerfile" "${SCRIPT_DIR}/mock-llm"
kind load docker-image "$MOCK_LLM_IMAGE" --name "$CLUSTER_NAME"

log "ensuring namespace"
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

log "applying e2e-only mock-llm"
kubectl apply -f "${SCRIPT_DIR}/mock-llm/mock-llm.yaml"
kubectl -n "$NAMESPACE" rollout status deployment/mock-llm --timeout=180s

log "installing helm chart (LLM base URL -> in-cluster mock-llm)"
helm upgrade --install agentorgs "${PROJECT_ROOT}/charts/agentorgs" \
  --namespace "$NAMESPACE" --create-namespace \
  --set credentials.llmApiKey="sk-e2e-mock-not-for-production" \
  --set controller.llmBaseURL="http://mock-llm.${NAMESPACE}.svc.cluster.local:6556/v1" \
  --set controller.image.repository=agentorgs/controller \
  --set controller.image.tag=local \
  --set controller.image.pullPolicy=Never \
  --set controller.openclawAgentImage="$OPENCLAW_AGENT_IMAGE" \
  --set controller.hermesAgentImage="$HERMES_AGENT_IMAGE" \
  --wait --timeout 10m

log "applying CRDs + e2e fixture ${E2E_FIXTURE}"
kubectl apply -f "${PROJECT_ROOT}/config/crd/"
kubectl wait --for=condition=Established --timeout=60s \
  crd/collaborations.agentorgs.io \
  crd/groups.agentorgs.io \
  crd/members.agentorgs.io \
  crd/policies.agentorgs.io
kubectl apply -f "${SCRIPT_DIR}/${E2E_FIXTURE}"

log "waiting for controller"
kubectl -n "$NAMESPACE" rollout status deployment/agentorgs-controller --timeout=180s

log "running Go e2e tests (-run ${E2E_RUN})"
cd "${SCRIPT_DIR}"
go test -tags=e2e -count=1 -timeout 0 -failfast -v -run "${E2E_RUN}" ./...
