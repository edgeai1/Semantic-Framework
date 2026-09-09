# 标准 Skill 与真实执行样例

日期：2026-08-05

## 变更

- 新增标准 `data-profile` Skill，通过 Docker `execute` 分析 Project workspace 中的 CSV/JSON，并生成 JSON 与 Markdown 数据画像。
- 新增标准 `semantic-diagnostics` Skill，通过受控宿主 `execute_host` 采集操作系统、Go、Docker 和 Semantic Server 健康状态。
- 两个 Skill 仅使用标准 `SKILL.md/scripts` 结构，不携带 Semantic 自定义 `runtime`、`permissions` 或模型配置。
- Skill Middleware 只负责渐进加载指导正文，不隐式执行脚本；Agent 必须按照正文显式调用 `execute` 或 `execute_host`。
- Docker 执行环境把安装的 Skill 根目录挂载到 `/skills`，Project workspace 继续挂载到 `/workspace`，脚本输出默认留在 workspace。
- 报告只有在需要进入对话、跨 Agent/会话共享或长期保存时，才通过 `artifact.register` 显式登记。
- 示例脚本只依赖 Python 标准库，新增逻辑、异常边界和测试均提供中文注释。

## 验证

- 验证两个 Skill 可被现有 Store 和 Eino Skill Middleware 使用的元数据契约正常加载。
- 验证 `data-profile` 对 CSV 的行数、字段缺失和 Markdown 报告生成结果。
- 验证 `semantic-diagnostics` 在 Server 不可达时仍生成包含局部失败信息的完整报告。
- 使用真实 Eino-ext DockerSandbox 运行 `data-profile` 并把报告写回 Project workspace。
- 使用真实 Eino-ext Local Backend 运行 `semantic-diagnostics`，并继续覆盖超时取消和进程清理。
- `SEMANTIC_DOCKER_INTEGRATION=1 SEMANTIC_DOCKER_IMAGE=python:3.12-alpine GOTOOLCHAIN=local go test ./internal/tool/builtin -run TestExecuteToolDockerIntegration -count=1 -v`
- `GOTOOLCHAIN=local go test ./...`
