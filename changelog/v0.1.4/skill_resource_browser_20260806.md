# Skill 资源浏览接口

- 技能详情新增 `resources` 清单，列出标准 `scripts/`、`references/` 和 `assets/` 下的文件路径、类型、媒体类型与大小。
- 新增 `GET /api/v1/skills/{name}/resources/*`，按需读取不超过 1 MiB 的 UTF-8 文本资源。
- 资源路径只允许位于当前 Skill 标准目录，拒绝路径穿越、包外符号链接、目录和非文本内容。
- 服务端绝对路径不会进入 API 响应；资源正文不内联到技能清单和详情首包。
