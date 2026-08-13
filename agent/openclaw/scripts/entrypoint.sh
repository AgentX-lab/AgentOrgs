#!/bin/bash
# OpenClaw Member entrypoint: pull MinIO workspace, push changes, start OpenClaw.
# This script is OpenClaw-only. Other runtimes (e.g. Hermes) get their own image.
set -euo pipefail

log() {
  echo "[agentorgs-openclaw $(date '+%Y-%m-%d %H:%M:%S')] $*"
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

if ! command -v openclaw >/dev/null 2>&1; then
  log "ERROR: openclaw binary not found in base image"
  log "default base is ghcr.io/openclaw/openclaw:latest; override with OPENCLAW_BASE_IMAGE if needed"
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
  log "ERROR: workspace missing SOUL.md or AGENTS.md; Controller should initialize members/${MEMBER_NAME}/"
  exit 1
fi

if [ ! -f "${WORKSPACE_DIR}/openclaw.json" ]; then
  log "ERROR: workspace missing openclaw.json"
  exit 1
fi

mkdir -p "${WORKSPACE_DIR}/.openclaw"
ln -sfn "${WORKSPACE_DIR}/openclaw.json" "${WORKSPACE_DIR}/.openclaw/openclaw.json"

log "starting background push every ${SYNC_INTERVAL}s"
(
  while true; do
    sleep "${SYNC_INTERVAL}"
    if ! mc mirror "${WORKSPACE_DIR}/" "${REMOTE_PREFIX}/" --overwrite \
      --exclude ".openclaw/matrix/**" \
      --exclude ".openclaw/agents/**" \
      --exclude "*.lock"; then
      log "WARNING: push to MinIO failed; will retry"
    fi
  done
) &
PUSH_PID=$!

shutdown() {
  log "stopping push loop"
  kill "${PUSH_PID}" 2>/dev/null || true
}
trap shutdown EXIT INT TERM

export OPENCLAW_CONFIG_PATH="${WORKSPACE_DIR}/openclaw.json"
log "starting OpenClaw"
exec openclaw gateway || exec openclaw
