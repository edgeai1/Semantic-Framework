# v0.2.0 Agent 基础与版本收敛（Framework）

## 变更

- 开发版本从 0.1.4-dev 调整为 0.2.0-dev。
- v0.1.4 未单独发布，已有 Agent、模型、Skill、Artifact、执行、日志和 Trace 改动并入 v0.2.0。
- 数据库不兼容提示改为 v0.2.0 schema。
- 建立根 CHANGELOG，为后续 Beta、RC 与正式 Release 提供统一说明。

## 兼容性

- v0.1.3 与 v0.1.4 均视为未发布开发版本。
- 旧实验数据库继续通过 STORE_SCHEMA_INCOMPATIBLE 明确拒绝，不自动改写。
- semantic init --reset-data 仍负责备份现有数据库、WAL 和 Artifact 后重新初始化。

## 验证

- Framework 版本输出应为 0.2.0-dev。
- 全量 Go 测试、race、lint 和构建必须通过。

