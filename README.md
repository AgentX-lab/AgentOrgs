# AgentOrgs

AgentOrgs is a Kubernetes-native organization and collaboration control plane for AI agents.

## Quick Start (kind)

```bash
export AGENTORGS_LLM_API_KEY=sk-your-key
make local-k8s-up
```

This creates a kind cluster, installs Controller + MinIO + Matrix (Tuwunel), applies CRDs, and loads the demo organization.

## Demo collaboration

```bash
./bin/ago collaborate \
  --api http://127.0.0.1:8090 \
  --from product-owner \
  --to developer \
  --text "implement login API"
```

## Project layout

```text
agentorgs-controller/   Go controller, engine, providers
config/crd/             Generated CRDs
config/samples/         Demo Member/Group/Collaboration/Policy
charts/agentorgs/       Helm chart
hack/                   kind scripts
docs/                   Design docs
```

## Build locally

```bash
make build
make test
```

## Core resources

- `Member`: human, agent, or external participant
- `Group`: dynamic team membership and roles
- `Collaboration`: collaboration rules and limits
- `Policy`: who may start or continue collaboration

Providers (OpenClaw, Kubernetes, Matrix, MinIO) are pluggable and configured through `ProviderBinding`.

## Agent images

Storage is MinIO for all runtimes. Each runtime has its own image under `agent/<runtime>/`.

```bash
make build-agent-openclaw
# default base: ghcr.io/openclaw/openclaw:latest
```

Override base if needed: `OPENCLAW_BASE_IMAGE=openclaw/openclaw:latest make build-agent-openclaw`

OpenClaw entrypoint requires the `openclaw` binary from the base image. Hermes will live at `agent/hermes/` later.
