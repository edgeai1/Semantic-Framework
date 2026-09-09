# 对话消息 Trace 精确关联

- `message.done` 新增 `trace_id`，成功、失败和用户中断均返回当前主 Agent Runner 的精确链路 ID。
- 助手历史消息原有的 `trace_id` 持久化保持不变，实时事件与 REST 历史现在使用同一关联值。
- 增加运行时回归断言，禁止前端再通过完成时间猜测最近 Trace。
