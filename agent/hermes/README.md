# AgentOrgs Hermes Member runtime

Built on [hermes-agent](https://github.com/NousResearch/hermes-agent).

- Entrypoint: MinIO `mc mirror` ↔ `/workspace`
- `hermes-worker`: bridge `openclaw.json` → `.hermes/`, start Matrix gateway

```bash
make build-agent-hermes
```

See `docs/hermes-runtime-plan.zh-CN.md`.
