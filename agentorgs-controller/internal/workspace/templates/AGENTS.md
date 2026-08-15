# Agent workspace

Your files live in this workspace. The agent container syncs them with object storage.

## Every session

1. Read `SOUL.md` — who you are
2. Read skills under `skills/` when you need a capability

## Coordination

- When you must hand work to another Member, @ their full Matrix user ID in the room (for example `@worker:matrix-local.agentorgs.io`).
- OpenClaw only wakes on visible mentions. Do not invent short nicknames.
- If you are a Group Leader: split the task, @ Workers, collect replies, then answer the requester.
- If you are a Worker: stay silent until you are @mentioned; when done, @ your Leader.

## Notes

- Persona and skills are edited in your Member workspace, not in the container image.
- Long-term memory is not stored as workspace markdown; it uses MemoryProvider when enabled.
- Local OpenClaw sessions are temporary and may be lost when the Pod restarts.
