# Project 工作区 Artifact 显式登记

日期：2026-08-05

## 变更

- 新增 `POST /api/v1/chat/artifacts/register`，把当前用户 Project workspace 内的普通文件显式复制登记为 ArtifactRef。
- 普通 `execute` / `execute_host` 输出文件继续留在 workspace，不会自动创建 Artifact。
- 登记路径只接受相对路径，并解析符号链接后再次检查 Project 边界；单文件上限 100MB。
- 新增当前用户 Artifact 列表、内容读取和删除接口：
  - `GET /api/v1/chat/artifacts`
  - `GET /api/v1/chat/artifacts/{id}`
  - `DELETE /api/v1/chat/artifacts/{id}`
- Artifact 仍被消息引用时默认返回 `409 ARTIFACT_REFERENCED`；`force=true` 可删除本体，但不改写历史消息。
- 强制删除后的历史引用在模型上下文中显示明确缺失标记，不会阻断会话继续执行。
- 非图片 Artifact 的历史上下文只注入 URI、媒体类型和摘要，文件内容继续通过 `artifact.get` 按需读取，避免大文件挤占模型预算。
- Artifact 列表按 owner 隔离，不暴露其他用户或系统内部产物。

## 验证

- 验证工作区 JSON 文件显式登记、内容复制、用户归属和列表读取。
- 验证工作区符号链接逃逸在读取前被拒绝。
- 验证消息引用阻止普通删除，强制删除后消息 ArtifactRef 仍保留。
- 验证缺失历史引用和非图片摘要引用均可正常重建 Eino `schema.Message`。
- `GOTOOLCHAIN=local go test ./...`
