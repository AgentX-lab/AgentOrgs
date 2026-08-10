# AgentOrgs Core Concepts

## Purpose

AgentOrgs provides a small set of composable Kubernetes resources for describing how autonomous participants are organized and allowed to collaborate.

The model does not prescribe a specific hierarchy or coordination structure. Organizational forms are compositions of four core resources: `Member`, `Group`, `Collaboration`, and `Policy`.

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
