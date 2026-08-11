# AgentOrgs Architecture

## Core Resources

- `Member`: an agent, human, or external participant.
- `Group`: a collection of Members or Groups.
- `Collaboration`: defines who collaborates and which pattern they use.
- `Policy`: defines permissions and requirements.

## Components

### Controller

Watches the four resources, resolves membership and roles, checks policies, configures providers, and reports status.

### Collaboration Engine

Runs collaborations between participants. It sends requests, validates structured results, stores progress, and advances the collaboration state.

### Providers

- `RuntimeAdapter`: configures an agent runtime and connects it to the collaboration protocol.
- `ExecutionBackend`: runs an agent in Kubernetes, a sandbox, or an external environment.
- `CollaborationProvider`: transports events and messages.
- `StorageProvider`: stores workspaces, collaboration state, and artifacts.

## Collaboration Patterns

AgentOrgs initially supports three patterns:

- `Delegation`: one participant assigns work to another. The result contains `status`, `summary`, and `deliverables`.
- `Review`: one participant reviews another participant's output. The result contains `decision`, `summary`, and `findings`.
- `Discussion`: multiple participants discuss a topic. The result contains `outcome`, `summary`, and `contributions`.

Complex collaboration is built by combining these patterns.

## Collaboration Protocol

Each request and result uses a `CollaborationEvent` with these fields:

```text
version
id
collaborationRef
runId
type
source
targets
status
payload
artifactRefs
```

The selected pattern defines the structure of `payload`. The Collaboration Engine accepts a result only after it passes validation. Natural-language messages and artifact contents remain unrestricted.

## Main Flow

1. The user creates the four Kubernetes resources.
2. The Controller configures the required runtimes, execution environments, communication channels, and storage.
3. An agent or human starts a collaboration.
4. The Collaboration Engine checks the Collaboration and Policy resources.
5. The target agents receive a structured request through their Runtime Adapters.
6. The agents submit structured results.
7. The Collaboration Engine validates the results, stores progress, and sends the next event.

## Data Ownership

- Kubernetes stores the four resources and their status.
- Storage Providers store workspaces, collaboration state, and artifacts.
- Kubernetes Secrets or external secret systems store credentials.
- Agent runtimes own prompts, skills, reasoning, sessions, and memory formats.

## Repository Layout

```text
AgentOrgs/
├── cmd/controller/          # Starts AgentOrgs
├── api/v1alpha1/            # Four Kubernetes resource definitions
├── internal/
│   ├── controller/          # Reconciles Kubernetes resources
│   ├── collaboration/       # Runs collaborations and validates results
│   ├── policy/              # Checks permissions and requirements
│   └── status/              # Reports resource status
├── pkg/
│   ├── protocol/            # CollaborationEvent and result contracts
│   └── provider/            # Provider interfaces
├── providers/
│   ├── runtime/             # OpenClaw, Hermes, and other adapters
│   ├── execution/           # Kubernetes, sandbox, and external backends
│   ├── collaboration/       # Event and message transports
│   └── storage/             # MinIO, object storage, and volume backends
├── config/                  # CRDs, RBAC, deployment, and samples
├── charts/agentorgs/        # Helm chart
├── docs/                    # Documentation
└── test/                    # Unit and end-to-end tests
```

AgentOrgs runs as one process. The core depends only on provider interfaces.
