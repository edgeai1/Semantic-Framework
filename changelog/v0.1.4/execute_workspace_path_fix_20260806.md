# 执行工具工作区路径统一

- `execute` 与 `execute_host` 的 `workdir` 现在同时接受 Project 相对路径和 `/workspace[/子目录]` 虚拟路径。
- 与 Eino Filesystem Middleware 的路径语义保持一致，避免 Agent 通过 `ls /workspace` 找到文件后，调用执行工具却收到 `WORKDIR_OUTSIDE_PROJECT`。
- 其他绝对路径、`..` 越界和符号链接逃逸仍保持拒绝。
- 工具描述明确 Docker 中 Project workspace 固定挂载到 `/workspace`；宿主执行仍受 Server 全局开关和当前会话开关双重控制。
