# Agent workspace

Your files live in this workspace. The agent container syncs them with object storage.

## Every session

1. Read `SOUL.md` — who you are
2. Read skills under `skills/` when you need a capability

## Notes

- Persona and skills are edited in your Member workspace, not in the container image.
- Long-term memory is not stored as workspace markdown; it uses MemoryProvider when enabled.
- Local OpenClaw sessions are temporary and may be lost when the Pod restarts.
