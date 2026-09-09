# Agent Runtime 与对话闭环（B4 / M1.4）

> 日期：2026-08-03　模块：agent（profile/runtime/kernel）、server（handlers/ws）、store、configs/agents

## 为什么改

Phase 1 目标要求打通"登录 → 多轮流式对话 → 会话持久化与恢复"的最小闭环。B3 完成了 eino 接入与单轮 agent 冒烟，但缺少：角色驱动的 agent 构建、会话持久化、对话 REST/WS 协议面。本里程碑按架构文档 02/04/10/12 落地 AgentRuntime 最小版。

## 内容清单

- **internal/agent/profile/**：角色 profile 加载器（AGENT.md 正文=系统提示、role.yaml 字段建模、SAFETY.md 可选总则）。
- **configs/agents/leader/**：Leader 角色 profile（AGENT.md 五章节：Role/Behavior/Safety/Tools；role.yaml：coordinator 模式、model=deepseek-chat、approval_required=[artifact.*]）。
- **internal/agent/kernel/ 扩展**：
  - `middleware.go`：middleware 栈最小集统一装配（patchtoolcalls→agentsmd→summarization→context→trace）；
  - `agent.go` v2：`AgentConfig` 扩展（Name/Role/SafetyDoc/AgentsMDFile/ModelName/MaxTurns/ContextTokens/Store/Price/Purpose/Logger），`BuildAgent` 新签名（返回 `*Runner`，Store 非空时内部挂接 TraceHandler）；
  - `run.go`：`Runner`/`EventStream`/`Message`/`Event`——eino 类型不外泄，历史由调用方按 store 重放（无跨 Run 状态）。
- **internal/agent/runtime/**：`Service.HandleMessage`（会话加载/创建 → 消息落库 → 构建或复用运行器 → run → 事件流转 envelope 下行 → 助手消息与 run 终态落库）；会话运行器进程内缓存；**重启恢复**（首条消息按 chat_messages 重建历史）；**无密钥降级**（profile 模型缺 key 时回退全局默认端点并 WARN，与 bootstrap 降级语义一致）。
- **store 迁移 v3**：chat_sessions / chat_messages / run_sessions 三表与 CRUD。
- **对话协议面**：REST（`handlers/chat.go`：sessions CRUD + messages 分页查询，user_id 归属过滤）；WS（`ws/chat.go` ChatGateway：上行 chat.message → runtime，下行 dialogue 事件复用 hub 按 session 投递）。
- **bootstrap 接线**：profile/registry/runtime/handlers/ChatGateway 装配（wire 按域拆文件）。

## 影响面（跨模块）

| 影响 | 涉及模块 | 处理 |
|---|---|---|
| `kernel.BuildAgent` 签名变化（三返回→单返回，Callbacks 字段移除，改由 Store 触发内部 TraceHandler） | kernel、runtime、tests/integration/llm_kernel_test.go | 已全部迁移，测试通过 |
| 新增"无密钥降级"行为 | runtime（ WARN 可观测） | 本文件记录；生产环境 profile 模型 key 必须配置，降级仅面向开发/测试 |
| store schema v3 三表 | store、runtime、handlers | 迁移幂等，老库自动升级 |

## 测试内容与标准

- 单测：profile 加载（正常/缺文件/默认值）；runtime.HandleMessage 全流程（mock 脚本两轮+工具调用，断言落库顺序/envelope 序列/run 状态迁移/恢复后历史延续）；kernel middleware 栈顺序与 context 组装；store 三表 CRUD；chat handlers 归属过滤。
- 集成测试 `TestChatDialogueLoop`：login → 建会话 → /ws/chat 发消息 → delta+done 下行 → REST 消息验证 → **重启 App** 再发一条验证历史延续。
- 收尾门禁：gofmt/go vet/golangci-lint 零告警；`go test ./... -race` 全绿（13 个包 ok）。

## 遗留 TODO

- `docs/api/` 契约文档待补（chat REST 与 /ws/chat 协议已定稿于代码，随 B6 统一落文档）；
- run_sessions 的 turns/usage 汇总字段待 B6 聚合器接入后填充；
- 会话标题自动生成（首条消息摘要）暂为占位（前 20 字截断），后续可经小模型生成；
- DeepSeek 真机联调待有 key 环境执行 `TestDeepSeekSmoke`。
