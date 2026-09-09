# Eino Local Backend 宿主执行工具

日期：2026-08-05

## 变更

- 固定接入 `eino-ext/adk/backend/local v0.2.5`，Go 版本继续保持 1.23，Eino Core 继续保持 0.9.13。
- 新增 `execute_host(command, workdir, timeout_seconds)`，实际命令创建、输出收集和退出码处理复用 Eino-ext Local Backend。
- `execute_host` 与 Docker `execute` 是两个平行工具，不互相 fallback。
- 宿主工具只有在 `execution.allow_host=true` 且当前会话显式开启后才进入 Agent 工具集；Observer 等无对话会话的运行器看不到该工具。
- `ask` 模式下两个执行工具均审批；`auto` 自动批准 Docker、宿主仍审批；`full` 跳过两个执行工具的普通审批，但不会绕过其他高风险工具规则。
- 宿主 `workdir` 只接受 Project workspace 内相对路径，并额外解析符号链接，拒绝通过 workspace 内链接逃逸到外部目录。
- Local Backend 没有原生 workdir 和进程组句柄，薄适配器使用 `cd` 前缀与 `setsid` 建立本次命令进程组；context 取消或超时时补充终止该组，避免只结束外层 Shell 后遗留同组子进程。
- 服务端宿主硬开关热关闭时取消活动 Run，阻止已按旧配置构建的 Runner 继续发起宿主命令。
- 普通命令文件仍留在 Project workspace，不自动创建 Artifact。

## 边界

- `full` 表示用户明确授予当前会话宿主命令能力，命令主动创建新 Session/进程组属于宿主完全访问能力本身；框架只负责清理本次默认进程组，不伪装成容器隔离。
- Local Backend 模块包含其多模态文件读取依赖，但 Semantic 本次只使用 `Execute`，没有启用 PDF 或其他额外能力。

## 验证

- 验证 Server/会话双开关决定工具可见性，关闭任一开关均不能执行。
- 验证 ask/auto/full 审批矩阵，且 full 不会绕过普通高风险工具审批。
- 验证 workdir Shell 转义、符号链接逃逸拒绝和结构化退出结果。
- 使用真实 Eino Local Backend 完成工作区文件回写、超时取消和后台子进程组无残留测试。
- `GOTOOLCHAIN=local go test ./...`
