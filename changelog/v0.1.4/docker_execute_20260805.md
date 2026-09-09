# Project 作用域 Docker 执行工具

日期：2026-08-05

## 变更

- 固定接入 `eino-ext/components/tool/commandline v0.0.0-20251114101050-f10cdd30345d`，直接复用其 DockerSandbox，不维护自研容器生命周期。
- 新增 `execute(command, workdir, timeout_seconds)` 工具，通过 `/bin/sh -lc` 在一次性 Docker 容器中执行命令。
- 为单次 Agent Run 增加 Project 执行作用域，工具通过 context 获取会话、Project 和工作区，不在全局实例中保存可变目录。
- Project workspace 固定挂载到容器 `/workspace`；`workdir` 只接受工作区内相对路径，绝对路径和 `..` 越界在创建容器前拒绝。
- 默认关闭容器网络，并沿用 DockerSandbox 的默认镜像、CPU 和内存限制；本阶段不新增镜像管理、虚拟环境或额外容器策略。
- Docker 执行与后续宿主执行互不回退；Docker 不可用时返回明确错误。
- 用户取消或命令结束后使用独立清理 context 删除本次容器，避免已取消 context 阻止资源回收。
- 工具返回结构化 `stdout`、`stderr`、`exit_code` 和相对工作目录；普通文件继续留在 Project workspace，不自动登记 Artifact。
- 为兼容 Docker 28 的传递依赖，固定 OpenTelemetry 1.35 相关模块；项目 Go 版本继续保持 1.23，Eino Core 继续保持 0.9.13。

## 当前边界

- `execute` 暂时沿用现有高风险人工审批入口；`ask/auto/full` 的最小会话执行策略与 `execute_host` 将在独立提交中实现。
- DockerSandbox 不负责自动拉取镜像，部署环境需预先准备默认镜像；镜像管理不在 v0.1.4 范围内。

## 验证

- 验证 Project 挂载、相对工作目录、超时和 Shell 参数正确传递给 DockerSandbox。
- 验证缺少执行作用域、绝对路径和目录越界在容器创建前被拒绝。
- 验证成功、失败和取消路径均执行容器清理，取消返回稳定错误码。
- 使用本机 Docker 与预置 `python:3.12-alpine` 镜像完成真实容器执行、工作区文件回写和清理验收。
- `GOTOOLCHAIN=local go test ./...`
