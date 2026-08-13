# Matrix 完善计划

目标：人在 Matrix 房间里 `@两个 Group + 一个 Member`，系统开一个任务，把 Group 展开成成员并送达。

## 小白先懂

- Matrix 只有 `@用户`，没有 Group。
- 所以每个 Group 也要有一个 Matrix 账号（例如 `@backend-team:...`），才能被 @。
- 一条消息里的多个 @ = 一个任务的多个目标。
- Matrix 只负责传话；谁能干、怎么展开，由 Engine / Policy / Group 决定。

## 核心场景

```
product-owner 在房间发：
  @backend-team @qa-team @architect 请联调登录接口

组织：
  backend-team = be-1, be-2
  qa-team      = qa-1, qa-2
  architect    = 单独 Member

结果：
  1 个 run
  ResolvedTargets = 5 人
  5 个 Agent 收到任务
  房间能看到进度
```

## 分层（低耦合）

| 层 | 做什么 | 不做什么 |
|---|---|---|
| Matrix Provider | 收 AppService、发房间消息、读 roomId | 不管 Policy / Group 展开 |
| MentionIndex | `@mxid` → Member 或 Group | 不管 HTTP / 房间 |
| Engine | 策略检查、展开 Group、写 run、调 Runtime | 不管 Matrix token |
| Resolver | Group → 成员列表 | 不管通道 |
| Storage | run / 事件 / 工作区 | 不管通道 |
| Agent 镜像 | 拉工作区、配 Matrix channel、跑 OpenClaw | 不管组织规则 |

模块之间只通过 `CollaborationEvent` 通信。

## 要实现什么

### 1. 身份与映射

- Group 增加 `spec.channels`（与 Member 对称）。
- Member / Group reconciler 把 Matrix 用户写入 `status.matrixUserId`。
- 新增 `MentionIndex`：用 status 把 MXID 解析成 `{Kind, Name}`。

### 2. 入站（收消息）

- 实现 AppService：`PUT /_matrix/app/v1/transactions/{txnId}`。
- 解析 `m.room.message` + `m.mentions`。
- 同一条消息的多个 @ 合并成一次 `StartCollaboration`。
- 校验：房间属于该 Collaboration；发送者合法；mention 能解析；Policy 允许 Start。

### 3. 出站与 Engine

- Engine 按 `Collaboration.spec.channel.provider` 选 Provider，不再用 Default。
- Deliver 按 Collaboration 的 ConfigMap 取 `roomId`，不用全局默认房间。
- 无 token / 无房间时要报错，不能静默成功。

### 4. Setup（让本地能用）

- 注册 AppService 到 Tuwunel。
- 为 demo 的 Member / Group 建 Matrix 用户。
- 建协作房间，写回 ConfigMap `roomId`。
- 把人与 Agent 拉进房间。
- Helm Secret / env：`as_token`、`hs_token`、homeserver 地址与端口对齐。

### 5. Agent 侧（一并做）

- OpenClaw `Runtime.Apply` 把 Matrix 凭证写入工作区 `openclaw.json`（字段与 AgentTeams 一致）。
- Matrix Setup 只写 Secret + `status.matrixUserId`，不碰 openclaw.json。
- entrypoint 只拉工作区并启动 OpenClaw，不写配置。

### 6. demo 与测试

- 更新 `demo.yaml`：2 Group×2 Member + 1 Member + Human + Policy + Collaboration。
- 简单 e2e（Go test，不测真集群 Matrix smoke）：
  - `StartCollaboration(from, [G1, G2, M1])`
  - 断言 5 个 ResolvedTargets、Runtime 被调 5 次
  - Policy deny 路径
  - MentionIndex：多个 MXID → 2 Group + 1 Member

## 不在本计划

- Hermes
- MemoryProvider
- 一 Collaboration 多房间
- kind 真机 Matrix smoke
- **单次任务选人（include / exclude）** → 见 [派发意图计划](dispatch-intent-plan.zh-CN.md)

## 验收标准

1. 房间里 `@backend-team @qa-team @architect` 能开 1 个 run。
2. run 的目标是 5 个成员，无重复。
3. 进度回帖出现在同一房间。
4. Agent Pod 配好 Matrix，能进房互动。
5. 上述 Go e2e 通过。
