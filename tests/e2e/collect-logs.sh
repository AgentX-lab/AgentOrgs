#!/usr/bin/env bash
# Collect full diagnostics from the e2e kind cluster (all pods/containers).
# Usage: collect-logs.sh [output_dir]
set -euo pipefail

OUT_DIR="${1:-/tmp/e2e-logs}"
NAMESPACE="${AGENTORGS_NAMESPACE:-agentorgs}"
CLUSTER_NAME="${AGENTORGS_CLUSTER_NAME:-agentorgs-e2e}"
CONTEXT="${AGENTORGS_KUBE_CONTEXT:-kind-${CLUSTER_NAME}}"

log() { echo "[collect-logs] $*"; }

mkdir -p "$OUT_DIR"
kubectl config use-context "$CONTEXT" >/dev/null 2>&1 || true

log "writing cluster/namespace overview to ${OUT_DIR}"
{
  echo "=== context ==="
  kubectl config current-context || true
  echo
  echo "=== nodes ==="
  kubectl get nodes -o wide || true
  echo
  echo "=== namespaces ==="
  kubectl get ns || true
} >"${OUT_DIR}/cluster.txt" 2>&1 || true

kubectl -n "$NAMESPACE" get all,cm,secret,pvc -o wide >"${OUT_DIR}/namespace-resources.txt" 2>&1 || true
kubectl -n "$NAMESPACE" get pods -o wide >"${OUT_DIR}/pods.txt" 2>&1 || true
kubectl -n "$NAMESPACE" get events --sort-by=.lastTimestamp >"${OUT_DIR}/events.txt" 2>&1 || true

PODS_DIR="${OUT_DIR}/pods"
mkdir -p "$PODS_DIR"

POD_COUNT=0
while IFS= read -r pod; do
  [ -n "$pod" ] || continue
  POD_COUNT=$((POD_COUNT + 1))
  safe="$(echo "$pod" | tr '/:' '__')"
  pod_dir="${PODS_DIR}/${safe}"
  mkdir -p "$pod_dir"

  kubectl -n "$NAMESPACE" describe pod "$pod" >"${pod_dir}/describe.txt" 2>&1 || true

  containers="$(
    {
      kubectl -n "$NAMESPACE" get pod "$pod" -o jsonpath='{range .spec.initContainers[*]}{.name}{"\n"}{end}' 2>/dev/null || true
      kubectl -n "$NAMESPACE" get pod "$pod" -o jsonpath='{range .spec.containers[*]}{.name}{"\n"}{end}' 2>/dev/null || true
    } | sed '/^$/d'
  )"

  while IFS= read -r c; do
    [ -n "$c" ] || continue
    # Full current logs (no --tail).
    kubectl -n "$NAMESPACE" logs "$pod" -c "$c" >"${pod_dir}/${c}.log" 2>&1 || true
    # Previous container instance if restarted.
    kubectl -n "$NAMESPACE" logs "$pod" -c "$c" --previous >"${pod_dir}/${c}.previous.log" 2>&1 || true
  done <<< "$containers"
done < <(kubectl -n "$NAMESPACE" get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)

if [ "$POD_COUNT" -eq 0 ]; then
  log "no pods found in namespace ${NAMESPACE}"
fi

# Useful config snapshots for Matrix e2e debugging.
kubectl -n "$NAMESPACE" get configmap mention-group-leader-channel -o yaml >"${OUT_DIR}/mention-group-leader-channel.yaml" 2>&1 || true
kubectl -n "$NAMESPACE" get members.agentorgs.io,collaborations.agentorgs.io,policies.agentorgs.io -o yaml \
  >"${OUT_DIR}/agentorgs-crs.yaml" 2>&1 || true

log "done: $(find "$OUT_DIR" -type f | wc -l | tr -d ' ') files under ${OUT_DIR}"
