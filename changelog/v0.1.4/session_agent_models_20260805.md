# 会话级多 Agent 模型快照

日期：2026-08-05

## 变更

- 新增 schema v12 和 `session_agent_models`，按 `session_id + agent_id` 固化端点、推理档位和配置来源。
- 模型选择优先级统一为“系统 Default → Agent Profile → 会话快照 → 用户会话覆盖”。
- Agent Profile 的 `model` 允许留空；默认 Leader、Query 和 Monitor 均继承系统 Default，不再各自保存重复的 Mock 配置。
- 设置页或 Agent Profile 的后续修改只影响新会话，不会静默热切已有会话。
- 新增会话模型控制接口：
  - `GET /api/v1/chat/sessions/{id}/agents`
  - `PUT /api/v1/chat/sessions/{id}/agents/{agent_id}/model`
- 活动模型或工具运行期间拒绝模型切换并返回 `409 SESSION_BUSY`；空闲后显式切换会淘汰旧 Runner，下一轮使用新端点。
- Leader 与 SubAgent 均从自己的会话快照解析模型；Query、Monitor 等 Agent 不再继承 Leader 模型或隐式回退 Mock。
- 助手消息和运行元数据记录实际 Provider、端点、model ID、配置来源和 Default 继承状态，这些审计信息不注入模型提示。
- 模型切换与会话主循环使用同一会话锁，并在加锁后复核 Runner，消除切换成功后仍运行旧模型的竞态窗口。
- 会话删除时同步清理模型快照；无效 effort 会被拒绝，不支持固定 effort 的端点恢复为 `auto`。

## 验证

- 会话快照只创建一次，Profile 变化不改写已有会话，用户覆盖可更新且随会话删除。
- Agent Profile 留空继承 Default，清除固定模型后可恢复继承。
- Query 与 Leader 在同一会话中分别构建自己的模型端点。
- 活动 Run 切换返回 `SESSION_BUSY`，取消后切换并继续对话使用新模型。
- 会话模型列表、单 Agent 覆盖、归属校验和 REST 创建时快照初始化测试通过。
- `go test ./...`
