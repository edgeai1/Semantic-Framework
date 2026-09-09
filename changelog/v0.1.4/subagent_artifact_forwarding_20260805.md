# SubAgent 图片 Artifact 安全转发

日期：2026-08-05

## 变更

- Eino AgentTool 的委派契约扩展为 `task + artifact_refs`，仍由 Eino 负责子 Agent 执行、事件冒泡和中断传播。
- Runtime 把当前用户消息中已完成用户归属校验的 ArtifactRef 放入本轮运行上下文，不向子 Agent 共享 Leader 的完整私有历史。
- 未显式指定引用时，委派自动携带当前用户消息中的全部图片，解决 Query 等子 Agent 看不到对话图片的问题。
- 显式指定引用时，只允许使用当前用户消息白名单内的 ArtifactRef，拒绝模型猜测或越权读取其他用户产物。
- 子 Agent 输入中间件按需从 Artifact Store 读取图片并转换为 Eino 多模态 `schema.Message`；非图片引用不会被错误地当作视觉输入。
- 子 Agent 的模型选择、消息、工具调用和 Trace 归属保持独立，图片转发不会把内部调用计入 Leader。

## 验证

- 验证当前用户图片可自动传入 SubAgent 模型，模型实际收到任务文本和 Base64 图片内容。
- 验证白名单之外的 ArtifactRef 返回结构化拒绝，不会读取 Store 内容。
- 验证既有 AgentTool 调用、嵌套限制、事件冒泡和委派结果归一行为不变。
- `go test ./...`
