# 单次任务派发意图计划

目标：每次任务可以有不同的「要谁、不要谁」，且不和永久 Policy 混在一起。

## 小白先懂

三件事分开：

| 概念 | 管什么 | 变不变 |
|---|---|---|
| **Policy** | 有没有权限派活；谁永远不能碰 | 长期 |
| **Collaboration / Group Role** | 这类协作默认怎么选人（如只要 Developer） | 按协作配置 |
| **派发意图** | **这一次**任务要谁、不要谁 | **每次可不同** |

例子：

- Policy：实习生不能 Start（永久）。
- Collaboration：@ Group 时默认选 Role=Developer。
- 派发意图：这次 `@backend-team`，但不要 `be-2`。

Matrix 只负责把消息送来；**最终谁接到活 = 展开 Group → 应用派发意图 → 再过 Policy 底线**。

## 和 Matrix plan 的关系

- [Matrix plan](matrix-plan.zh-CN.md)：进房、@、开 run、回帖。
- **本 plan**：`ResolvedTargets` 怎么从「@ 的目标」变成「真正干活的人」。

第一版 Matrix 可以先做「@ 谁就展开谁」；本 plan 补上单次选人能力。

## 核心场景

```
同一 Group：backend-team = be-1, be-2

任务 A：@backend-team -@be-2 联调
  → ResolvedTargets = [be-1]

任务 B：@backend-team 全员联调
  → ResolvedTargets = [be-1, be-2]

任务 C：@backend-team，但 Policy Deny → be-2
  → 即使没写 -@be-2，be-2 也会被去掉
```

## 处理顺序（固定）

```
1. 解析 targets（Member / Group）
2. 按 Collaboration.whenTargetIsGroup 展开 Group
3. 应用本次派发意图（include / exclude / role 覆盖）
4. 对每个候选成员做 Policy 检查（Deny 则去掉；无人允许则失败）
5. 写入 ResolvedTargets，再下发
```

第 3 步是本 plan；第 4 步是 Engine 小改（展开后过滤），属于本 plan。

## 派发意图长什么样

挂在 **这一次** 请求上，不写进 Policy CR。

| 字段 | 含义 | 例子 |
|---|---|---|
| `exclude` | 展开后去掉这些人 | `["be-2"]` |
| `include` | 若填写，展开后只保留这些人（须本就在展开结果里） | `["be-1"]` |
| `role` | 覆盖本次的 Group Role 筛选（可选） | `"Developer"` |

来源（同一结构，多种入口）：

| 入口 | 怎么表达 |
|---|---|
| HTTP / `ago` | JSON：`exclude` / `include` / `role` |
| Matrix 消息 | `@backend-team -@be-2 ...`（`-@` = exclude） |
| 默认 | 不写 = 不做额外过滤，只走 Collaboration + Policy |

## 要实现什么

### 1. 协议

- `StartCollaboration` / `CollaborationEvent` 增加可选派发意图字段（或放在 payload 约定键下，二选一，实现时定一种）。
- run 记录最终 `ResolvedTargets`；可选记录本次 `exclude`/`include` 便于审计。

### 2. Engine

- 展开 Group 后调用「派发意图过滤器」。
- 再对每个成员 `Policy.Check(from, Member, Start)`：Deny 则剔除。
- 过滤后为空 → 明确报错。

### 3. Matrix 入站（依赖 Matrix plan）

- 解析正文里的 `-@member`（或等价约定）→ `exclude`。
- 普通 `@` 仍是 targets；不要把 `-@` 当成正目标。

### 4. 测试

- `@Group` + `exclude=be-2` → 少一人。
- `@Group` + Policy Deny be-2 → 少一人（不写 exclude）。
- `exclude` 后无人 → 失败。
- `include` 含组外成员 → 忽略或报错（实现时定一种，需测）。

## 不在本计划

- 每次任务动态创建 / 删除 Policy CR。
- 用 LLM 猜「谁该干活」。
- 改 Group 成员名单来实现「这次不要某人」。

## 验收标准

1. 同一 Group，两次任务可得到不同 `ResolvedTargets`，且不改 Policy。
2. Policy Deny 仍能挡住被排除的人。
3. Matrix `-@` 与 HTTP `exclude` 效果一致。
4. 相关 Go 测试通过。
