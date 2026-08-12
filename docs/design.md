# AgentOrgs Core Concepts

## Purpose

AgentOrgs provides a small set of composable Kubernetes resources for describing how autonomous participants are organized and allowed to collaborate.

The model does not prescribe a specific hierarchy or coordination structure. Organizational forms are compositions of four core resources: `Member`, `Group`, `Collaboration`, and `Policy`.

## Product Form

AgentOrgs combines a Kubernetes-native organization control plane with a Matrix-based collaboration interface.

- Users communicate with agents in Matrix rooms.
- Agents can collaborate with other agents through the same interface.
- Members can be organized into dynamic Groups with contextual roles.
- Collaborations define who can work together and how.
- Policies control permissions and limits.
- Groups are resolved to concrete Members when a collaboration starts.

This model supports multiple independent teams and large agent deployments without requiring a fixed hierarchy. Matrix is the initial interaction layer, while AgentOrgs manages organization and collaboration independently of the communication implementation.

The first version supports OpenClaw as the only agent runtime. Additional runtimes may be added later through Runtime Adapters.

## Member

A `Member` is an independently identifiable participant.

It provides a stable organizational identity while allowing the underlying implementation to vary. A Member may be backed by an agent runtime, a human interface, or an external system. Runtime-specific configuration belongs to the Member's runtime reference and does not define its organizational relationships.

A Member answers:

> Who can participate?

## Group

A `Group` is a named collection of Members and other Groups.

Groups provide organizational scope without imposing a fixed meaning or hierarchy. Membership may carry a contextual role, and the same Member may belong to multiple Groups with different roles. Nested Groups form an acyclic structure.

A Group answers:

> Who belongs together, and in what context?

## Collaboration

A `Collaboration` is a coordination contract among Members, Groups, or a combination of both.

It identifies the participants, their roles in the collaboration, and the collaboration pattern to be provided by an execution backend. A Collaboration may involve any number of participants and is not limited to a directional connection between two endpoints.

The resource declares the intended coordination relationship. It does not implement task planning, workflow state, communication transport, or agent reasoning.

A Collaboration answers:

> How are the participants expected to work together?

## Policy

A `Policy` defines constraints that apply to Members, Groups, or Collaborations.

Policies govern what is permitted or required without changing the organizational structure itself. Keeping governance separate allows the same structure to operate under different constraints.

A Policy answers:

> What is allowed or required?

## How the Concepts Fit Together

The four resources have distinct responsibilities:

- `Member` defines participant identity.
- `Group` defines organizational context and membership.
- `Collaboration` defines coordination relationships.
- `Policy` defines governance constraints.

Roles are contextual properties of Group membership or Collaboration participation, not standalone resources. Runtime bindings are execution properties of Members, not organizational relationships.

Higher-level organizational concepts are expressed by composing these resources rather than introducing a separate resource type for every organizational form.

## System Boundary

AgentOrgs owns the declarative organizational model and reconciles it with pluggable backends. Backends remain responsible for realizing collaboration patterns, communication, task execution, and agent behavior.

This boundary keeps the organization model independent of any particular agent runtime or coordination implementation.

## Workspace, Memory, and Collaboration State

These three concerns stay separate. Mixing them creates high coupling and makes providers hard to replace.

| Concern | What it holds | Who owns it | First provider |
|---|---|---|---|
| Workspace | identity and tools for one Member: persona (`SOUL.md` / `AGENTS.md`), skills, runtime config (`openclaw.json`), work artifacts | `StorageProvider` | MinIO prefix `members/<name>/` |
| Long-term memory | durable facts, preferences, and recallable experience | `MemoryProvider` (pluggable) | optional later (e.g. Mem0); not required for MVP |
| Collaboration state | runs, events, and collaboration artifacts | `StorageProvider` (separate keys/prefix) | MinIO prefix for runs/events |

Rules:

1. Object storage is a workspace and collaboration ledger, not a memory engine.
2. A memory engine does not store persona files or skills.
3. Persona and skills belong to the Member workspace. They are not baked into the agent image.
4. The agent image is a generic runtime box (OpenClaw + sync glue). One image serves many Members; each Member has its own workspace prefix.
5. On start, the agent pulls its workspace. While running, it pushes workspace changes back. Memory read/write goes through `MemoryProvider`, not ad-hoc files in MinIO.
6. OpenClaw local sessions are runtime-only. They are not the system of record for organization memory.

This keeps Controller, Storage, Memory, Runtime, and Collaboration independently swappable through provider bindings.
