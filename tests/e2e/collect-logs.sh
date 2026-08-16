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
CHANNEL_CM="${AGENTORGS_E2E_CHANNEL_CM:-mention-group-leader-channel}"
kubectl -n "$NAMESPACE" get configmap "$CHANNEL_CM" -o yaml >"${OUT_DIR}/${CHANNEL_CM}.yaml" 2>&1 || true
kubectl -n "$NAMESPACE" get members.agentorgs.io,groups.agentorgs.io,collaborations.agentorgs.io,policies.agentorgs.io -o yaml \
  >"${OUT_DIR}/agentorgs-crs.yaml" 2>&1 || true

# Room transcript (sender/body/mentions) via Matrix Client-Server API.
MATRIX_URL="${AGENTORGS_MATRIX_URL:-http://127.0.0.1:18080}"
MATRIX_URL="${MATRIX_URL%/}"
ROOM_ID="$(kubectl -n "$NAMESPACE" get configmap "$CHANNEL_CM" -o jsonpath='{.data.roomId}' 2>/dev/null || true)"
TOKEN="$(kubectl -n "$NAMESPACE" get secret matrix-requester -o jsonpath='{.data.accessToken}' 2>/dev/null | base64 -d 2>/dev/null || true)"
{
  echo "matrix_url=${MATRIX_URL}"
  echo "room_id=${ROOM_ID}"
  if [ -z "$ROOM_ID" ] || [ -z "$TOKEN" ]; then
    echo "skip: missing roomId or matrix-requester accessToken"
  elif ! command -v python3 >/dev/null 2>&1; then
    echo "skip: python3 required to fetch/format room messages"
  else
    ROOM_ENC="$(python3 -c 'import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=""))' "$ROOM_ID")"
    RAW="${OUT_DIR}/room-messages.raw.json"
    if curl -fsS -H "Authorization: Bearer ${TOKEN}" \
      "${MATRIX_URL}/_matrix/client/v3/rooms/${ROOM_ENC}/messages?dir=b&limit=50" \
      -o "$RAW"; then
      python3 - "$RAW" <<'PY'
import json, sys
path = sys.argv[1]
with open(path, encoding="utf-8") as f:
    data = json.load(f)
chunk = data.get("chunk") or []
print(f"events_in_chunk={len(chunk)} (m.room.message lines below, newest first)")
n = 0
for ev in chunk:
    if ev.get("type") != "m.room.message":
        continue
    n += 1
    content = ev.get("content") or {}
    body = content.get("body", "")
    mentions = (content.get("m.mentions") or {}).get("user_ids") or []
    extra = f" mentions={','.join(mentions)}" if mentions else ""
    print(f"[{n}] sender={ev.get('sender')} body={body!r}{extra}")
if n == 0:
    print("no m.room.message in chunk")
PY
    else
      echo "curl failed fetching room messages from ${MATRIX_URL}"
    fi
  fi
} >"${OUT_DIR}/room-messages.txt" 2>&1 || true

log "done: $(find "$OUT_DIR" -type f | wc -l | tr -d ' ') files under ${OUT_DIR}"
