# Changelog

## v0.3.0（开发中）

当前开发版本为 0.3.0-dev。本版本实现用户显式进入 Plan Mode、Workflow/Task/SubTask、通用结构化 Interaction，以及 Framework 内置的 Semantic Map。

v0.3.0 继续使用现有 Agent Runtime、SQLite 与单 Server 运行方式。Simulation、Pilot、Robot Skill、AbilityFramework 和 Robot 执行不进入本版本；完成范围以代码、自动测试和产品验收结果为准。

## v0.2.0（开发中）

Semantic v0.2.0 将当前 Agent 基座收敛为可恢复的 Project、Conversation、Agent Run、Context、Trace 与 Semantic Studio 状态服务。

当前开发版本为 0.2.0-dev。v0.1.3 与 v0.1.4 均未作为正式版本发布；其中已经完成并继续使用的 Agent、模型、Skill、Artifact、执行和 Trace 基础能力统一纳入 v0.2.0。

### 当前基础

- 使用 Eino Agent、ToolsNode、Skill Middleware 和 schema.Message。
- 支持模型服务、会话级 Agent 模型、图片、Artifact、工具执行与 SubAgent。
- 提供 Project 工作区、安装配置、数据库不兼容检测和 reset-data 备份。
- 提供 Docker 与宿主执行、Skill 示例、日志和 Trace 基础。

### v0.2.0 交付

- 每次 Agent Run 独立保存 Run、Trace 和根 Span，模型、工具、消息与计量使用精确关联。
- 持久保存 Context 摘要及覆盖边界；重启后只装配 Project Memory、摘要和边界后的近期消息。
- 建立单活动 Project、Project Markdown Memory、Conversation、Run 和 Interaction 的归属与状态接口。
- 为 Semantic Studio 提供 Project 快照、可重放增量事件、断线补偿和精确 Run 取消。
- 删除已退出设计的 Blackboard 运行代码与数据表，并移除持续消费全局事件的模型 Monitor。
- 提供发布包校验文件；版本专用 JSON 样例、Fixture Server 和本地组件清单不随源码仓库交付。

Plan、Workflow、Task、Semantic Map、Pilot、Robot Skill 与仿真不进入 v0.2.0 产品验收。

### 数据说明

开发数据库不承诺兼容未发布实验 schema。迁移会删除已经停用的 Blackboard 实验数据；需要保留历史数据时应先备份。遇到 STORE_SCHEMA_INCOMPATIBLE 时，先备份需要保留的数据，再运行 semantic init --reset-data。
