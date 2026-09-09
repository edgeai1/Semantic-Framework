# Eino schema.Message 统一消息链路

日期：2026-08-05

## 变更

- 删除 Framework 内部重复的 `kernel.Message` 与 `kernel.Attachment` 类型。
- Agent Runner、Runtime 历史重放和多模态输入统一使用 `*schema.Message`。
- 新增 schema v11，把 `chat_messages.role/content` 重建为薄消息信封：
  - `message_json` 保存序列化后的 Eino 消息；
  - `agent_id/run_id/trace_id/provider/endpoint/model` 保存归属与审计信息；
  - `artifact_refs` 保存稳定产物引用；
  - `metadata` 继续保存前端运行活动投影。
- 迁移时保留稳定基线已有的用户与助手文本、Run ID 和运行活动。
- 用户图片不写入消息 JSON；持久化只保存 ArtifactRef，模型调用时按需读取并组装 Base64 多模态输入。
- REST 继续输出兼容的 `role/content`，但字段由 `schema.Message` 薄投影生成，并增加 Agent、Trace、实际模型和 ArtifactRef 元数据。
- 存储边界拒绝没有正文、工具调用或多模态输出的空 Assistant 消息，避免中断后继续对话触发 provider 400。

## 验证

- `go test ./...`
- 消息 JSON、模型审计字段和 ArtifactRef 往返测试通过。
- 多图片输入、历史图片恢复、审批恢复和中断后继续对话测试通过。

