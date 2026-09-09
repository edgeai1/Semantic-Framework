# SubAgent 默认模型继承验收

日期：2026-08-05

## 变更

- 增加 Query Agent 未指定模型时的运行时验收，明确其会话快照继承系统 Default。
- 验证 Leader 的固定 Profile 模型与 Query 的系统 Default 模型相互独立。
- 验证 Query 快照来源记录为 `system_default`，不会隐式回退到名为 Mock 的端点。

## 验证

- `GOTOOLCHAIN=local go test ./internal/agent/runtime -run 'TestSubAgentUsesOwnSessionModel|TestSubAgentInheritsSystemDefault' -count=1`
