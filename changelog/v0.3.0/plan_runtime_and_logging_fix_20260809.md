# Plan Runtime 与运行日志修复

## 结果

- Plan 创建和结构反馈先持久化 `planning` 状态并立即响应，Leader 与 Task Agent
  的 Planning Run 改由 Workflow Service 在后台管理，不再受 HTTP 超时或浏览器断开影响。
- Planning 成功后通过 Project 事件发布完整 Workflow/Task/SubTask；失败或 Server
  重启时保存明确原因，用户可以查看后重新规划，不自动重放模型或工具。
- 结构反馈开始时原子递增 revision，使后台重新规划期间无法批准旧计划。
- Server 关闭时先取消并等待 Planning Run，再关闭 Agent Runtime 与 Store。
- Project Conversation 上行消息支持显式 `collaborate/plan` 意图。Plan Run 使用独立规划提示，
  并在 Runtime 工具策略与工具执行层双重拒绝文件修改、命令、部署等副作用。
- Plan Mode 不再等同于立即创建 Workflow：Leader 可以正常回复和创建结构化 Interaction；
  `plan.suggest` 在协作模式只请求用户进入规划，在规划模式才创建或修订当前 Conversation 的草案。
- 标准开发实例日志固定写入 `.output/logs/semantic-server.jsonl`，并新增
  `make logs` 查看入口；终端仍同步输出结构化日志。

## 验证

- Workflow Service 覆盖立即响应、完整规划、失败、反馈 revision、关闭和恢复。
- 真实 App 集成测试覆盖普通对话、异步 Plan、精确 revision 确认和 Developer Task 闭环。
- WebSocket、Runtime 与工具测试覆盖 Plan 意图传递、只读双层门禁、首个计划创建和同 Conversation
  计划 revision 修订。
- 日志测试覆盖标准实例、自定义数据库路径、结构化字段和文件权限。
