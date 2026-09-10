# Semantic Framework

[English](README.md) | [简体中文](README.zh-CN.md)

🧭 Semantic 的控制中心。Server 管理项目、任务、注册表与 Runtime 调度；Pilot 将 Robot 执行环境接入 Server；`semantic` CLI 提供配置与 Runtime 管理入口。

## 工程结构

- `cmd/`：Server、Pilot 与 CLI 入口。
- `internal/` · `pkg/`：调度、服务、契约及公共库。
- `configs/`：配置模板；`scripts/`：构建与 Bundle 工作流。
- `tests/` · `ci/`：单元测试、集成门禁与构建自动化。

## 🛠 编译与启动

需要 Go **1.23+**、Make 和 Linux 开发环境。

```bash
make build
make init
go test ./...
make run
```

编译生成 `.output/bin/semantic-server`、`semantic-pilot`、`semantic`；初始化将配置写入 `.output/configs/`。初始化前通过本地环境变量或未跟踪的 `.env` 设置 `SEMANTIC_ADMIN_PASSWORD`，不要公开密码。

实例配置应修改 `.output/configs/semantic-server.yaml`，不要直接改共享模板。默认开发端点为 HTTP `127.0.0.1:8080` 和 WebSocket `127.0.0.1:8081`；Studio 是独立项目。

## 产物使用

仅编译 Server 不会配置好 Robot。完整部署应遵循 quick-start 的目录布局与版本清单：登记原生 MuJoCo、构建 AbilityFramework / SDK Wheel，再组装并激活 Robot Bundle。

准备好输入产物后，在本仓库查看构建入口：

```bash
python3 scripts/refresh_v050_mujoco.py --help
```

使用 Robot 的 Python **3.13** 解释器执行 `build` 工作流；Server 启动后再执行独立的 `publish` 工作流发布所需 Skill。替换正在使用的 Bundle 前，先停止相关场景与 Robot。

## 常见问题

- Robot 离线：检查 Pilot 连接、激活目录、Runtime 登记和 Skill 发布版本。
- 配置不生效：确认进程实际使用的生成配置路径。
- 配置热重载钩子失败时，已应用的配置段会恢复为旧值。如果回滚失败，系统会明确报错，设置快照保留该段最后成功应用的值。请修正无效配置后重试；配置文件本身不会被回滚。
- 单元测试不等于整栈验证：仿真门禁需要资产与关联仓库，模型服务门禁可能产生调用费用。
- 开发端口只向可信网络开放，令牌不要提交。

[详细技术参考](README.reference.md) · [配置模板](configs/) · [构建工作流](scripts/)

## 许可证

Copyright 2026 InsightOS。自有代码采用 [Apache-2.0](LICENSE)；第三方组件与资产请查看 [NOTICE](NOTICE) 和[许可范围](LICENSE_SCOPE.md)。
