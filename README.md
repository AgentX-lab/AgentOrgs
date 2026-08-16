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

## Architecture

<img width="1672" height="941" alt="image" src="https://github.com/user-attachments/assets/17825f30-4c6b-4430-9bc9-70a99d8a3131" />


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


