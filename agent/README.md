# Runtime agent images

Storage is shared (MinIO workspaces). Each agent runtime has its own image:

- `agent/openclaw/` -> `agentorgs/agent-openclaw` (Member `runtime=openclaw`)
- `agent/hermes/` -> `agentorgs/agent-hermes` (future)

OpenClaw base defaults to official `ghcr.io/openclaw/openclaw:latest`.
Override with `OPENCLAW_BASE_IMAGE` when building.

Persona and skills stay in MinIO under `{namespace}/members/{name}/`.
Do not bake Member-specific content into these images.
