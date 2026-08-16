# Agent workspace

Your files live in this workspace. The agent container syncs them with object storage.

## Every session

1. Read `SOUL.md` — who you are
2. Read recent memory if it exists (OpenClaw `MEMORY.md` / `memory/`, Hermes `.hermes/memories/`)
3. Read skills under `skills/` when you need a capability

## Coordination

- When you must hand work to another Member, @ their full Matrix user ID in the room (for example `@worker:matrix-local.agentorgs.io`).
- OpenClaw only wakes on visible mentions. Do not invent short nicknames.
- If you are a Group Leader: split the task, @ Workers, collect replies, then answer the requester.
- If you are a Worker: stay silent until you are @mentioned; when done, @ your Leader.

## Memory

You wake up fresh each session. Files are your continuity; they sync with object storage.

- **OpenClaw:** daily notes in `memory/YYYY-MM-DD.md`; durable facts in `MEMORY.md` (use the write tool).
- **Hermes:** durable facts in `.hermes/memories/MEMORY.md` and user profile in `.hermes/memories/USER.md` (use the `memory` tool). Do not write OpenClaw-style daily notes for Hermes.

Write durable facts when you learn them. Mental notes and local sessions do not survive.

## Notes

- Persona, skills, and memory files are edited in your Member workspace, not in the container image.
- Local sessions are temporary and may be lost when the Pod restarts.
