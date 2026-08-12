# AgentOrgs 组件与流程

AgentOrgs 使用四种 Kubernetes 资源：`Member`、`Group`、`Collaboration` 和 `Policy`。

## 组件

- `Controller`：监听四种资源、配置 Provider、更新状态。
- `Collaboration Engine`：发送协作请求、校验结果、记录进度。
- `RuntimeAdapter`：让 Agent Runtime 接入 AgentOrgs。
- `ExecutionBackend`：在 Kubernetes、Sandbox 或外部环境运行 Agent。
- `CollaborationProvider`：传递协作事件和普通消息。
- `StorageProvider`：保存 Member 工作区，以及协作状态（run、事件、产物）。
- `MemoryProvider`：可选的长期记忆读写（不是工作区）。

## 协作

AgentOrgs 首先支持：

- `Delegation`：委派任务，结果包含 `status`、`summary` 和 `deliverables`。
- `Review`：审核产物，结果包含 `decision`、`summary` 和 `findings`。
- `Discussion`：收集多个参与者的意见，结果包含 `outcome`、`summary` 和 `contributions`。

每个协作请求和结果都是一个 `CollaborationEvent`，记录所属 Collaboration、运行编号、事件类型、发送者、接收者、状态、数据和产物引用。

协作范式定义具体数据格式。普通自然语言消息和产物内容不限制格式。

## 流程

1. 用户创建四种 Kubernetes 资源。
2. Controller 配置需要的运行时、运行环境、通信和存储 Provider。
3. Agent 或人类发起协作。
4. Collaboration Engine 检查 Collaboration 和 Policy。
5. 目标 Agent 通过 Runtime Adapter 收到请求。
6. Agent 提交结构化结果。
7. Collaboration Engine 校验结果、保存进度并发送下一个事件。

## 数据

工作区、记忆、协作状态分开，避免耦合成一个大网盘：

- Kubernetes 保存四种资源及其状态。
- `StorageProvider` 保存：
  - Member 工作区：人格、技能、运行时配置、工作文件（`members/<name>/`）
  - 协作账本：run、事件、产物（另一前缀）
- `MemoryProvider` 保存可检索的长期记忆；不管人格和技能。
- Secret 保存凭证。
- Agent Runtime 只管本地会话和推理过程；本地会话不是组织记忆。
- Agent 镜像通用；人格和技能在工作区里，不打进镜像。

详见 `docs/design.md` 的 **Workspace, Memory, and Collaboration State**。

## 目录

```text
AgentOrgs/
├── cmd/controller/          # 程序入口
├── api/v1alpha1/            # Kubernetes 资源定义
├── internal/
│   ├── controller/          # 资源调谐
│   ├── collaboration/       # 执行协作并校验结果
│   ├── policy/              # 权限检查
│   └── status/              # 资源状态
├── pkg/
│   ├── protocol/            # 事件和结果规范
│   └── provider/            # Provider 接口
├── providers/
│   ├── runtime/             # Agent Runtime 适配
│   ├── execution/           # Agent 运行环境
│   ├── collaboration/       # 事件和消息传输
│   └── storage/             # 持久化存储
├── config/                  # CRD、RBAC、部署文件和示例
├── charts/agentorgs/        # Helm Chart
├── docs/                    # 文档
└── test/                    # 测试
```

AgentOrgs 只运行一个进程，核心代码只依赖 Provider 接口。
