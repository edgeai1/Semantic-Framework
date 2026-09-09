# 未发布开发成果并入 v0.2.0（Framework）

v0.1.4 没有形成正式 Release。以下已经完成并通过既有测试的内容继续作为 v0.2.0 开发基础；详细开发记录仍保留在 `changelog/v0.1.4/` 供定位历史。

## Agent 与模型

- Agent Runtime 统一使用 Eino `schema.Message`，保留消息、Agent、Run、Trace、模型和 Artifact 引用关联。
- 支持 OpenAI 兼容模型与 Claude 原生模型组件，并提供同端点、无输出时的一次安全重试。
- Agent Profile 与 Conversation 模型快照分离；Leader 与 SubAgent 使用各自实际模型配置。
- SubAgent 保持独立消息、工具和 Trace，并支持把当前图片 Artifact 明确传给视觉 Agent。
- 兼容模型把推理内容写入正文的情况，避免 `<think>` 内容进入最终工具结果。

## Project、工具与 Skill

- Conversation 已归属 Project，并具有独立工作区。
- 接入 Project 范围的只读文件工具、Docker 执行和可选宿主执行。
- Docker 与宿主执行使用一致的 `/workspace` 与相对工作目录语义，二者互不回退。
- 普通 Skill 使用标准 `SKILL.md`、`scripts/`、`references/` 和 `assets/`，由 Skill Middleware 渐进加载。
- 提供 `data-profile` 与 `semantic-diagnostics` 示例，验证 Skill 文档指导 Agent 显式调用执行工具。
- ToolSearch 根据工具 Schema 和上下文预算决定是否启用，小工具集不额外增加一次检索调用。

## Artifact 与多模态

- 用户上传、消息历史和 SubAgent 通过 ArtifactRef 传递图片，不在消息记录中保存 Base64 正文。
- Project 工作区文件可以显式登记为 Artifact，并支持列表、读取、删除和被引用时的缺失提示。
- 非图片 Artifact 默认只向模型提供类型、摘要和引用，需要正文时再按需读取。
- 非视觉 Leader 可以把已校验的图片引用交给视觉 SubAgent，不需要自身模型支持图片。

## 运行与诊断

- Conversation 支持 `ask/auto/full` 宿主命令执行策略；机器人执行确认由后续 Robot 版本单独设计。
- JSONL 运行日志、Trace、模型计量和消息 Trace 关联已经具备基础接口。
- `semantic init --reset-data` 能先备份开发数据目录，再重建不兼容的实验数据库。

## v0.2.0 中继续修正

- Trace 改为每个 Agent Run 独立创建，并补齐父子关联。
- Context 摘要保存覆盖边界，后续 Run 不再读取并重复压缩全部历史。
- Agent 指令只保留一条注入路径。
- 已删除不再使用的 Blackboard 运行代码和数据表；已删除持续消费全局事件的模型 Monitor。后续 Monitor 能力按新设计单独实现，不沿用旧链路。
