# AgentOrgs Hermes agent image
#
# Member.spec.runtime.provider=hermes
# Syncs MinIO workspace ({namespace}/members/{name}/), bridges openclaw.json
# into HERMES_HOME, then starts hermes-agent Matrix gateway.
#
# Build:
#   make build-agent-hermes
#
# Optional mirrors:
#   --build-arg APT_MIRROR=mirrors.aliyun.com
#   --build-arg PIP_INDEX_URL=https://mirrors.aliyun.com/pypi/simple/

ARG MC_IMAGE=minio/mc:latest
ARG PYTHON_IMAGE=python:3.11-slim
ARG APT_MIRROR=
ARG PIP_INDEX_URL=https://pypi.org/simple/
ARG HERMES_GIT_REF=v2026.4.16
ARG HERMES_REPO_URL=https://github.com/NousResearch/hermes-agent.git

FROM ${MC_IMAGE} AS mc

FROM ${PYTHON_IMAGE}

ARG APT_MIRROR=
ARG PIP_INDEX_URL=https://pypi.org/simple/
ARG HERMES_GIT_REF=v2026.4.16
ARG HERMES_REPO_URL=https://github.com/NousResearch/hermes-agent.git

USER root

RUN if [ -n "${APT_MIRROR}" ]; then \
        sed -i "s|deb.debian.org|${APT_MIRROR}|g" \
            /etc/apt/sources.list.d/debian.sources 2>/dev/null || true; \
    fi && \
    apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        cmake \
        libolm-dev \
        libffi-dev \
        libjemalloc2 \
        curl \
        git \
        jq \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN JEMALLOC=$(find /usr/lib -name 'libjemalloc.so.2' -print -quit) \
    && echo "LD_PRELOAD=${JEMALLOC}" >> /etc/environment \
    && echo "${JEMALLOC}" > /etc/ld.so.preload

COPY --from=mc /usr/bin/mc /usr/local/bin/mc

COPY pyproject.toml README.md /tmp/hermes-worker/
RUN mkdir -p /tmp/hermes-worker/src/hermes_worker \
    && mkdir -p /tmp/hermes-worker/src/hermes_matrix \
    && echo '__version__ = "0.0.0"' > /tmp/hermes-worker/src/hermes_worker/__init__.py \
    && echo '__version__ = "0.0.0"' > /tmp/hermes-worker/src/hermes_matrix/__init__.py

RUN git clone --depth 1 --branch "${HERMES_GIT_REF}" "${HERMES_REPO_URL}" /opt/hermes-src

RUN python -m venv /opt/venv/hermes \
    && /opt/venv/hermes/bin/pip install --upgrade pip setuptools wheel \
        --index-url "${PIP_INDEX_URL}" \
        --trusted-host "$(echo ${PIP_INDEX_URL} | sed 's|https\?://||;s|/.*||')" \
    && /opt/venv/hermes/bin/pip install --no-cache-dir \
        --index-url "${PIP_INDEX_URL}" \
        --trusted-host "$(echo ${PIP_INDEX_URL} | sed 's|https\?://||;s|/.*||')" \
        /opt/hermes-src \
    && /opt/venv/hermes/bin/pip install --no-cache-dir \
        --index-url "${PIP_INDEX_URL}" \
        --trusted-host "$(echo ${PIP_INDEX_URL} | sed 's|https\?://||;s|/.*||')" \
        "mautrix[encryption]" \
        "matrix-nio[e2e]>=0.24.0" \
        "markdown-it-py>=3.0" \
        "linkify-it-py>=2.0" \
        "Markdown>=3.6" \
        "typer>=0.9" \
        "rich>=13.0" \
        "httpx>=0.25" \
        "pyyaml>=6.0" \
    && /opt/venv/hermes/bin/pip install --no-cache-dir \
        --index-url "${PIP_INDEX_URL}" \
        --trusted-host "$(echo ${PIP_INDEX_URL} | sed 's|https\?://||;s|/.*||')" \
        /tmp/hermes-worker/

COPY src/ /tmp/hermes-worker/src/
RUN SITE=/opt/venv/hermes/lib/python3.11/site-packages \
    && cp -a /tmp/hermes-worker/src/hermes_worker/* "$SITE/hermes_worker/" \
    && cp -a /tmp/hermes-worker/src/hermes_matrix/* "$SITE/hermes_matrix/" \
    && rm -rf /tmp/hermes-worker/

RUN SITE=/opt/venv/hermes/lib/python3.11/site-packages \
    && mv "$SITE/gateway/platforms/matrix.py" "$SITE/gateway/platforms/_matrix_native.py" \
    && cp "$SITE/hermes_matrix/_shim.py" "$SITE/gateway/platforms/matrix.py"

RUN if [ -f /opt/venv/hermes/bin/hermes ]; then \
        ln -sf /opt/venv/hermes/bin/hermes /usr/local/bin/hermes; \
    fi \
    && if [ -f /opt/venv/hermes/bin/hermes-agent ]; then \
        ln -sf /opt/venv/hermes/bin/hermes-agent /usr/local/bin/hermes-agent; \
    fi \
    && ln -sf /opt/venv/hermes/bin/hermes-worker /usr/local/bin/hermes-worker

RUN mkdir -p /workspace /opt/agentorgs/hermes
COPY scripts/entrypoint.sh /opt/agentorgs/hermes/entrypoint.sh
RUN chmod +x /opt/agentorgs/hermes/entrypoint.sh /usr/local/bin/mc

ENV AGENTORGS_WORKSPACE_DIR=/workspace \
    HOME=/workspace \
    AGENTORGS_SYNC_INTERVAL_SECONDS=30 \
    AGENTORGS_RUNTIME=hermes \
    HERMES_YOLO_MODE=1 \
    MATRIX_HOME_CHANNEL=disabled

WORKDIR /workspace
ENTRYPOINT ["/opt/agentorgs/hermes/entrypoint.sh"]
