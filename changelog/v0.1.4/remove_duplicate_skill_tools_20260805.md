# 删除重复 Skill 目录工具

日期：2026-08-05

## 变更

- 删除自研 `skill.list` 和 `skill.search` 模型工具及其专用读取接口。
- Skill 清单、按需选择和正文加载统一交给 Eino Skill Middleware 的 `skill` 工具，避免同一能力存在两套模型调用入口。
- 前端 Skill 库继续使用只读 REST API，不受模型工具删除影响。
- 全局内置工具注册不再依赖 Skill Store，Skill Store 仅在 Agent 运行器装配时按 Agent/Project 有效集接入。
- 内置工具数由 12 个收敛为 10 个；这不会改变现有系统、Artifact、执行和 SubAgent 工具的名称或调用契约。

## 验证

- 验证内置工具注册表只包含 10 个仍有独立职责的工具。
- 验证 Eino Skill Middleware 的渐进加载和完整运行集成用例继续通过。
- 搜索确认运行代码不再引用 `skill.list`、`skill.search` 或 `SkillSource`。
- `GOTOOLCHAIN=local go test ./...`
