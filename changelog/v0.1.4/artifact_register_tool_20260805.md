# Agent 工作区 Artifact 登记工具

日期：2026-08-05

## 变更

- 新增 `artifact.register(path, media_type, summary)`，让 Agent 能把 `execute` 或 `execute_host` 已生成的工作区文件显式登记为 ArtifactRef。
- 工具通过单次 Run context 取得当前用户、Project 和 workspace，不接受模型提交任意宿主根目录。
- REST 登记接口与 Agent 工具复用同一套工作区路径解析：相对路径、存在性和符号链接边界行为一致。
- 工具只登记普通文件，单文件上限 100MB，并写入用户归属、Project ID 和 workspace 相对路径元数据。
- `artifact.register` 沿用 `artifact.*` 权限范围和高风险审批，不因 `full` 执行模式绕过 Artifact 独立审批。
- Docker `execute` 在 Project workspace 之外额外挂载当前 Skill 根目录到 `/skills`，让标准 Skill 的 `scripts/` 可被实际调用；没有 Skill Store 时不增加该挂载。

## 验证

- 验证 Agent 登记结果包含稳定 Artifact ID/URI，内容、用户归属和 Project 路径正确。
- 验证共享工作区解析拒绝目录越界与符号链接逃逸。
- 验证 DockerSandbox 收到 `/workspace` 与 `/skills` 两个明确挂载。
- `GOTOOLCHAIN=local go test ./...`
