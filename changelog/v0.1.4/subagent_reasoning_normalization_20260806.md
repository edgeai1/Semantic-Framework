# SubAgent 推理内容归一化

## 修复

- 修复 SubAgent 最终结果仍携带 `<think>...</think>`，导致推理内容在委派块中重复展示的问题。
- Eino AgentTool 返回 Leader 前只保留可见正文；推理内容继续通过独立的 `reasoning.delta` 事件展示和归档。
- 防止 SubAgent 内部推理随工具结果进入 Leader 模型上下文。

## 兼容性

- 原生返回 `ReasoningContent` 的模型保持原有处理方式。
- 将推理嵌入 Assistant Content 的 OpenAI 兼容服务复用同一个增量标签解析器，流式事件和最终工具结果采用一致语义。
