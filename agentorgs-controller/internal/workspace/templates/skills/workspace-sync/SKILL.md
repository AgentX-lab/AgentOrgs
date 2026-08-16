---
name: workspace-sync
description: Explain that your workspace files sync with object storage automatically. Use when asked where persona, skills, or memory are stored, or how file changes persist.
---

# Workspace sync

Your container pulls `{namespace}/members/{your-name}/` from object storage on start and periodically pushes local changes back.

You do not need to run a sync command for normal edits under `/workspace`.

- Persona: `SOUL.md`, `AGENTS.md`
- Skills: `skills/`
- Runtime config: `openclaw.json`
- Memory: OpenClaw `MEMORY.md` and `memory/`; Hermes `.hermes/memories/`

Sessions and other runtime state are not synced.
