# AgentOrgs

AgentOrgs is a Kubernetes-native organization and collaboration control plane for AI agents.

## Run and test

**Local cluster (kind)** — controller, MinIO, Matrix (Tuwunel), CRDs, and the demo organization:

```bash
export AGENTORGS_LLM_API_KEY=sk-your-key
make local-k8s-up
```

Kind e2e covers two Groups and four OpenClaw/Hermes agents in Leader–Worker and Swarm scenarios:

```bash
make e2e
make e2e-swarm
```

```bash
make local-k8s-down
```

## Architecture

<img width="1672" height="941" alt="image" src="https://github.com/user-attachments/assets/17825f30-4c6b-4430-9bc9-70a99d8a3131" />

## Components

**`Member`** — one participant (human or agent). This is who can act.

**`Group`** — a named roster of Members, with optional roles (for example Leader). `@Group` expands to the current roster.

**`Collaboration`** — a working contract among Members and/or Groups: who may take part, how a Group is addressed (Leader vs All), and one Matrix room for that contract.

**`Policy`** — who may start or continue work toward which Member or Group under a Collaboration.
