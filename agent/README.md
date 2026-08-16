# Runtime agent images

Storage is shared (MinIO workspaces). Each agent runtime has its own image:

- `agent/openclaw/` → `agentorgs/agent-openclaw` (`Member.runtime=openclaw`)
- `agent/hermes/` → `agentorgs/agent-hermes` (`Member.runtime=hermes`)

```bash
make build-agent-openclaw
make build-agent-hermes
```

OpenClaw base defaults to `ghcr.io/openclaw/openclaw:latest` (`OPENCLAW_BASE_IMAGE`).

Persona and skills stay in MinIO under `{namespace}/members/{name}/`.
Do not bake Member-specific content into these images.

Hermes bridges workspace `openclaw.json` into `${HERMES_HOME}` at start.
See `docs/hermes-runtime-plan.zh-CN.md`.
