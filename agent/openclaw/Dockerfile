# AgentOrgs OpenClaw agent image
#
# Runtime-specific box for Member.spec.runtime.provider=openclaw.
# Syncs MinIO workspace ({namespace}/members/{name}/) then starts OpenClaw.
# Hermes and other runtimes live under agent/<runtime>/ separately.
#
# Build:
#   docker build -t agentorgs/agent-openclaw:local \
#     -f agent/openclaw/Dockerfile \
#     agent/openclaw/
#
# Override base (optional):
#   --build-arg BASE_IMAGE=your-registry/openclaw:tag

ARG MC_IMAGE=minio/mc:latest
# Official OpenClaw gateway image (see openclaw docs/install/docker.md).
ARG BASE_IMAGE=ghcr.io/openclaw/openclaw:latest

FROM ${MC_IMAGE} AS mc

FROM ${BASE_IMAGE}

USER root
COPY --from=mc /usr/bin/mc /usr/local/bin/mc

RUN mkdir -p /workspace /opt/agentorgs/openclaw
COPY scripts/entrypoint.sh /opt/agentorgs/openclaw/entrypoint.sh
RUN chmod +x /opt/agentorgs/openclaw/entrypoint.sh /usr/local/bin/mc

ENV AGENTORGS_WORKSPACE_DIR=/workspace \
    HOME=/workspace \
    AGENTORGS_SYNC_INTERVAL_SECONDS=30 \
    AGENTORGS_RUNTIME=openclaw

WORKDIR /workspace
ENTRYPOINT ["/opt/agentorgs/openclaw/entrypoint.sh"]
