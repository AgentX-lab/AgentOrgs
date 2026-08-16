# Hermes Runtime 接入计划

目标：Member 可选用 Hermes 作为 Agent Runtime，与 OpenClaw Member 共用同一套组织与 Matrix 协作，不改 CRD。

## 结论

可以接。组织层（Member / Group / Collaboration / Policy / Engine）不变；只补 Hermes 镜像与 RuntimeAdapter。

## 边界

| 做 | 不做（首版） |
|---|---|
| Worker 级 Hermes Member | 新协作协议 / 新 CRD |
| 与 OpenClaw 同房混跑 | Hermes 与 OpenClaw 共用 entrypoint |
| 被 `@` 才回（requireMention） | 全员免 `@`、投票等 |

## 现状（首版已落地）

| 已有 |
|---|
| `Execution`：`runtime.provider=hermes` 选镜像 |
| `AGENTORGS_HERMES_AGENT_IMAGE` / Helm `hermesAgentImage` |
| `agent/hermes` 镜像与 entrypoint |
| `providers/runtime/hermes`（复用 openclaw.json 写入 + room 门控） |
| 样例 `config/samples/hermes-member.yaml` |

Leader e2e：`worker` 使用 Hermes，`lead` 仍为 OpenClaw（见 `tests/e2e/fixtures/mention_group_leader.yaml`）。

## 数据流

```
Member(runtime=hermes)
  → Hermes RuntimeAdapter.Apply
      写工作区配置（Matrix 凭证 + 精确 roomId + requireMention）
  → Execution 拉起 agentorgs/agent-hermes
  → 容器：拉 MinIO → bridge → Hermes Matrix gateway
  → Engine / Matrix 可见 @ 叫醒（与 OpenClaw 相同）
```

配置必须在 Pod 启动前写好（容器只在启动时拉工作区，不热加载 runtime 配置）。

## 分层

| 层 | 职责 |
|---|---|
| YAML | `Member.spec.runtime.provider: hermes`；可选 `spec.image` |
| RuntimeAdapter | 工作区 runtime 配置；精确 `groups[roomId].requireMention` |
| Execution | 按 provider 选镜像与 env |
| 镜像 entrypoint | sync → bridge → 启 Hermes；ready 上报 |
| Engine / Matrix / Policy | 不变 |

## 实施步骤

1. **镜像 `agent/hermes`**  
   Entrypoint：`AGENTORGS_*` 拉/推 MinIO → bridge → Hermes gateway；对齐 ready 上报。

2. **RuntimeAdapter `hermes`**  
   读 `matrix-<member>` Secret；写 Hermes 可读配置（可桥接现有工作区配置形状）。  
   所属 Collaboration 的真实 `roomId` 未就绪则 Apply 失败并重试（与 OpenClaw 一致）。

3. **注册**  
   `app.go` RegisterRuntime；确认 Helm / env 已指向 `AGENTORGS_HERMES_AGENT_IMAGE`。

4. **样例**  
   至少一个 Hermes Member；可选与 OpenClaw 同 Collaboration。

5. **验收**  
   - Adapter 单测（凭证、roomId、requireMention）  
   - e2e：`@` 有回复；无 `@` 不回  
   - 可选：OpenClaw Leader + Hermes Worker

## 风险

- AgentTeams bridge 的 env / 路径必须改成 AgentOrgs 约定。  
- Hermes Matrix 策略必须等价 requireMention；仅通配 `*` 不够，需精确 roomId。  
- 晚于 Pod 启动的配置变更不生效，除非重建 Pod。

## 顺序

镜像骨架 → Adapter（含 room 门控）→ 注册 → 样例 → e2e 冒烟

## 验收标准

1. 只改 YAML：`runtime.provider=hermes` 可启动并进协作房。  
2. 同房 OpenClaw / Hermes 均可被可见 `@` 叫醒。  
3. 无 `@` 时 Hermes Member 不回复。
