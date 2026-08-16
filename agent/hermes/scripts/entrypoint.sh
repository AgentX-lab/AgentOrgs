#!/bin/bash
# AgentOrgs Hermes Member entrypoint: pull MinIO workspace, bridge, start Hermes.
set -euo pipefail

log() {
  echo "[agentorgs-hermes $(date '+%Y-%m-%d %H:%M:%S')] $*"
}

MEMBER_NAME="${AGENTORGS_MEMBER_NAME:?AGENTORGS_MEMBER_NAME is required}"
NAMESPACE="${AGENTORGS_NAMESPACE:-agentorgs}"
WORKSPACE_DIR="${AGENTORGS_WORKSPACE_DIR:-/workspace}"
BUCKET="${AGENTORGS_MINIO_BUCKET:-agentorgs}"
ENDPOINT="${AGENTORGS_MINIO_ENDPOINT:-minio:9000}"
ACCESS_KEY="${AGENTORGS_MINIO_ACCESS_KEY:-minioadmin}"
SECRET_KEY="${AGENTORGS_MINIO_SECRET_KEY:-minioadmin}"
USE_SSL="${AGENTORGS_MINIO_USE_SSL:-false}"
SYNC_INTERVAL="${AGENTORGS_SYNC_INTERVAL_SECONDS:-30}"
ALIAS="agentorgs"
VENV="/opt/venv/hermes"

if [ ! -x "${VENV}/bin/hermes-worker" ]; then
  log "ERROR: hermes-worker not found in ${VENV}"
  exit 1
fi

if [ "${USE_SSL}" = "true" ] || [ "${USE_SSL}" = "1" ]; then
  SCHEME="https"
else
  SCHEME="http"
fi

REMOTE_PREFIX="${ALIAS}/${BUCKET}/${NAMESPACE}/members/${MEMBER_NAME}"

mkdir -p "${WORKSPACE_DIR}"
cd "${WORKSPACE_DIR}"
export HOME="${WORKSPACE_DIR}"
export HERMES_HOME="${WORKSPACE_DIR}/.hermes"
: "${HERMES_YOLO_MODE:=1}"
: "${MATRIX_HOME_CHANNEL:=disabled}"
export HERMES_YOLO_MODE MATRIX_HOME_CHANNEL

log "configuring MinIO alias ${ALIAS} -> ${SCHEME}://${ENDPOINT}"
mc alias set "${ALIAS}" "${SCHEME}://${ENDPOINT}" "${ACCESS_KEY}" "${SECRET_KEY}" >/dev/null

log "pulling workspace from ${REMOTE_PREFIX}/"
RETRY=0
until mc mirror "${REMOTE_PREFIX}/" "${WORKSPACE_DIR}/" --overwrite; do
  RETRY=$((RETRY + 1))
  if [ "${RETRY}" -gt 12 ]; then
    log "ERROR: failed to pull workspace after retries"
    exit 1
  fi
  log "waiting for workspace objects (attempt ${RETRY}/12)..."
  sleep 5
done

if [ ! -f "${WORKSPACE_DIR}/SOUL.md" ] || [ ! -f "${WORKSPACE_DIR}/AGENTS.md" ]; then
  log "ERROR: workspace missing SOUL.md or AGENTS.md"
  exit 1
fi

if [ ! -f "${WORKSPACE_DIR}/openclaw.json" ]; then
  log "ERROR: workspace missing openclaw.json (Hermes bridges from it)"
  exit 1
fi

mkdir -p "${WORKSPACE_DIR}/skills" "${HOME}/.agents"
ln -sfn "${WORKSPACE_DIR}/skills" "${HOME}/.agents/skills"
mkdir -p "${HERMES_HOME}"

log "starting background push every ${SYNC_INTERVAL}s"
(
  while true; do
    sleep "${SYNC_INTERVAL}"
    if ! mc mirror "${WORKSPACE_DIR}/" "${REMOTE_PREFIX}/" --overwrite \
      --exclude ".hermes/**" \
      --exclude ".agents/**" \
      --exclude ".npm/**" \
      --exclude ".mc/**" \
      --exclude "*.lock"; then
      log "WARNING: push to MinIO failed; will retry"
    fi
    # Persist Hermes native memory files only; leave sessions, .env, and config.yaml local.
    if [ -d "${HERMES_HOME}/memories" ]; then
      mc mirror "${HERMES_HOME}/memories/" "${REMOTE_PREFIX}/.hermes/memories/" --overwrite || \
        log "WARNING: push Hermes memories/ failed"
    fi
  done
) &
PUSH_PID=$!

shutdown() {
  log "stopping push loop"
  kill "${PUSH_PID}" 2>/dev/null || true
}
trap shutdown EXIT INT TERM

if [ -n "${AGENTORGS_CONTROLLER_URL:-}" ]; then
  (
    url="${AGENTORGS_CONTROLLER_URL%/}/api/v1/members/${NAMESPACE}/${MEMBER_NAME}/ready"
    n=0
    CONFIG_FILE="${HERMES_HOME}/config.yaml"
    until [ -f "${CONFIG_FILE}" ] && grep -q '^matrix:' "${CONFIG_FILE}" 2>/dev/null; do
      n=$((n + 1))
      if [ "${n}" -ge 24 ]; then
        log "WARNING: timed out waiting for hermes config.yaml"
        break
      fi
      sleep 5
    done
    n=0
    until curl -sf -X POST "${url}"; do
      n=$((n + 1))
      if [ "${n}" -ge 24 ]; then
        log "WARNING: report-ready failed"
        exit 0
      fi
      sleep 5
    done
    log "reported ready"
  ) &
fi

log "starting hermes-worker member=${MEMBER_NAME}"
exec "${VENV}/bin/hermes-worker" \
  --name "${MEMBER_NAME}" \
  --workspace "${WORKSPACE_DIR}"
