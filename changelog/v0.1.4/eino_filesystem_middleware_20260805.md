# Eino Project 文件系统中间件

日期：2026-08-05

## 变更

- 接入 Eino v0.9.13 Filesystem Middleware，向会话 Agent 提供 `ls`、`read_file`、`glob` 和 `grep` 四个成熟的只读文件工具。
- 文件操作复用 Eino-ext Local Backend v0.2.5；Semantic 只保留 Project 路径边界和虚拟路径转换的薄适配。
- 模型只看到 `/workspace` 虚拟根，不接收或暴露 Project workspace 的宿主真实路径。
- 路径解析检查最深已有父目录和符号链接，拒绝从 Project 内链接到外部目录；省略 grep/glob 路径时固定使用 `/workspace`，不会落到 Server 当前目录。
- Filesystem Middleware 不注册 `write_file`、`edit_file` 或自带 `execute`。文件修改和命令执行继续通过 Semantic 的 `execute`/`execute_host` 及 ask/auto/full 策略处理。
- Leader 与短时 SubAgent 使用同一会话 Project workspace，但各自的模型、消息、Trace 和 Skill 有效集仍保持独立。
- 四个只读文件工具由 Project 边界 Backend 保证安全，不进入针对注册表业务工具的审批门禁。

## 验证

- 验证 `/workspace` 内文件可分页读取，宿主真实绝对路径和符号链接逃逸均被拒绝。
- 验证 Eino Middleware 只注入四个只读文件工具，不出现写入或重复 execute 工具。
- 验证 `read_file` 经过完整 ChatModelAgent 工具循环成功执行，且不会被误判为未知工具。
- 验证被拒绝的写入不会在 Project workspace 创建文件。
- `GOTOOLCHAIN=local go test ./...`
