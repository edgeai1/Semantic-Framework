# SubAgent 图片 ArtifactRef 委派修复

- 修复 `ask_query` 的安全门禁 schema 仍只允许 `task`，导致模型生成 `artifact_refs` 后被错误拒绝为 `PARAM_VIOLATION` 的问题。
- 委派工具的 Eino ToolInfo、Semantic 安全契约和执行参数现在统一支持可选的 `artifact_refs` 字符串数组。
- 当前消息附件继续自动转发；用户在后续消息中明确给出既有 Artifact ID 时，按当前 Run 用户归属校验后允许转发给视觉 SubAgent。
- 其他用户、不存在或无有效归属的 ArtifactRef 仍返回结构化拒绝，不向子 Agent 暴露内容。
- 增加 Kernel、SubAgent Registry 和 Runtime 组合测试，覆盖安全 schema、多模态转发、跨轮引用与越权拒绝。
