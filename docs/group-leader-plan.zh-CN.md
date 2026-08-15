# Group 工作模式（配置驱动）

目标：用 YAML 配出组内协作方式。Leader/Worker 是其中一种配法，不是新 CRD。不同 Group 可以有不同模式。

## 小白先懂

工作模式 = 三份配置叠在一起，不靠专用「组长引擎」：

| 配什么 | 管什么 |
|---|---|
| **Group** | 组里有谁；谁标了 `role: Leader` |
| **Collaboration `whenTargetIsGroup`** | 人 `@Group` 时展开成谁 |
| **Policy** | 谁能找这个组、谁能点名组员 |

已有展开策略（不用新字段）：

| `whenTargetIsGroup.strategy` | `@Group` 叫醒谁 |
|---|---|
| `Leader` | 组里 `role: Leader` 的那个人 → Leader/Worker |
| `All` | 全员 |
| `Any` | 任一成员 |
| `Role` | 指定 `role` 的人 |

OpenClaw 默认仍 `requireMention: true`：没被 `@` 不干活。

## 两个 Group、两种模式：可以

`whenTargetIsGroup` 挂在 **Collaboration** 上，一场协作共用一种展开方式。

所以：

- **不同模式 → 用不同 Collaboration（通常也是不同房间）**
- **同一 Collaboration 里的多个 Group → 同一种展开方式**

例：后端走组长制，评审组走全员：

```
Collaboration team-work:     whenTargetIsGroup=Leader  → 只含 backend-team
Collaboration review-board:  whenTargetIsGroup=All     → 只含 review-group
```

人在对应房间 `@` 对应 Group。两套 Policy 分别写清谁能 Start / 谁能派活。

第一版不做「同一 Collaboration 里，A 组 Leader、B 组 All」。若以后要，再加按目标覆盖，不改 Group CR。

## Leader/Worker 怎么配

```yaml
# 1) Group：标出组长（role 只管协作；技艺在 Member.skillPack）
apiVersion: agentorgs.io/v1alpha1
kind: Group
metadata: { name: backend-team }
spec:
  members:
    - { who: { kind: Member, name: lead }, role: Leader }
    - { who: { kind: Member, name: be-1 } }   # Member.skillPack: backend
    - { who: { kind: Member, name: be-2 } }
  channels: [ ... ]   # Group 自己的 Matrix 账号，才能被 @

---
# Member 技艺示例（与 Group.role 无关）
# lead:    skillPack: coordination
# be-1/2:  skillPack: backend
```

```yaml
# 2) Collaboration：@Group 只找 Leader
apiVersion: agentorgs.io/v1alpha1
kind: Collaboration
metadata: { name: backend-work }
spec:
  participants:
    - { who: { kind: Member, name: requester } }
    - { who: { kind: Group, name: backend-team } }
  whenTargetIsGroup:
    strategy: Leader
  channel: { ... }

---
# 3) Policy：人能找组；组长能派组员
apiVersion: agentorgs.io/v1alpha1
kind: Policy
metadata: { name: allow-requester-start-backend }
spec:
  effect: Allow
  actions: [Start]
  from: [{ kind: Member, name: requester }]
  to:   [{ kind: Group, name: backend-team }]
---
apiVersion: agentorgs.io/v1alpha1
kind: Policy
metadata: { name: allow-lead-dispatch-workers }
spec:
  effect: Allow
  actions: [Start, Continue]
  from: [{ kind: Member, name: lead }]
  to:
    - { kind: Member, name: be-1 }
    - { kind: Member, name: be-2 }
```

跑起来：

```
人：@backend-team 登录接口怎么改？
  → 展开 [lead]，只叫醒 lead

lead：@be-1 改鉴权   @be-2 补测试
  → Policy 允许则叫醒对应 Worker（be-* 的名字只是 Member 名，技艺来自 skillPack）

be-1 / be-2：干完 @lead
lead：汇总回人
```

换全员模式：同一套 Group 名单，把 Collaboration 改成 `strategy: All`，Policy 改成允许人 Start 到该组即可。不必改引擎逻辑。

## 和已有 plan 的关系

- [Matrix plan](matrix-plan.zh-CN.md)：进房、@、开 run。
- [派发意图](dispatch-intent-plan.zh-CN.md)：单次要谁、不要谁。
- **本 plan**：用 Group + Collaboration + Policy 表达组内工作模式；补齐「展开后真叫醒」这一缺口。

`Resolver` 已支持 `Leader` / `All` / `Any` / `Role`。

## 分层（低耦合）

| 层 | 做什么 | 不做什么 |
|---|---|---|
| YAML（Group / Collaboration / Policy） | 定义模式 | 不写死在代码里 |
| Resolver | 按 `whenTargetIsGroup` 展开 | 不管房间、不管提示词 |
| Engine | 过 Policy；叫醒 ResolvedTargets | 不实现「组长专用」分支 |
| Matrix | 传带可见 `@` 的消息 | 不管 Role |
| 提示词 / skillPack（可选） | 提醒协作方式；按 Member 技艺 seed skills | 不改叫醒逻辑；不等同于 Group.role |

## 规则

1. **模式由配置决定**，不为 Leader/Worker 加 Group.kind 或新 CR。
2. **一场 Collaboration 一种 `whenTargetIsGroup`。** 两 Group 要两种模式 → 两场 Collaboration。
3. **`Leader` 策略要求组里能解析出一名 Leader。** 找不到则这次派发失败（可见），不要静默用第一名。
4. **没被 `@` 的成员不干活**（默认 requireMention）。
5. **派活仍过 Policy。** 配错 Policy，模式配了也跑不通。
6. **无 mention 不新开 run。**

## 要实现什么（少代码）

模式本身靠 YAML。代码只补通用能力：

### 1. 真叫醒 ResolvedTargets

人 `@` 的是 Group 账号，展开后的 Member 听不见。  
Engine / Matrix 必须在房间里对每个 ResolvedTarget 发**可见 `@`**（HTML + `m.mentions`）。  
不要依赖空的 `SendRequest` HTTP。对 `All` / `Leader` / `Role` 都一样。

### 2. Leader 解析失败要明确

`strategy: Leader` 且组内无 `role: Leader` → 报错并在房间可见，禁止回退到「名单第一人」。

### 3. 示例 YAML

在 `config/samples/` 放一份 Leader/Worker 与一份 All 的对照样例，证明两种模式只改配置。

### 4. 提示词与技艺（配置侧）

- **Group.members.role**：只表示协作角色（如 `Leader`），决定 `@组` 叫醒谁。
- **Member.spec.skillPack**（+ 可选 `skills`）：表示技艺预设，写入该 Member 的 workspace `skills/`，与 role 解耦。
- SOUL 用 `displayName` 区分人设。

## 不在本计划

- Group.kind / 新 CRD。
- 同一 Collaboration 内按 Group 覆盖不同 strategy（以后再说）。
- 开会回合灯、全员免 `@`。
- 投票、系统宣布共识。
- 强制「组内 @ 必须 Continue」才能跑通（Policy 允许组长 Start 组员即可；Continue 是后补优化）。

## 验收标准

1. 只改 YAML：`whenTargetIsGroup=Leader` + Policy → `@Group` 只有 Leader 被叫醒。
2. 只改 YAML：另一 Collaboration `=All` → `@` 另一 Group 时全员被叫醒。
3. 两套 Collaboration 可同时存在，互不影响。
4. `Leader` 策略但组内无 Leader → 房间里能看到失败，不静默乱派。
5. Worker 未被 `@` 时不回复。
