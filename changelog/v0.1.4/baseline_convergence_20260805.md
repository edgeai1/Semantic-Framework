# v0.1.4 稳定基线收敛（Framework）

日期：2026-08-05

## 变更目标

从稳定提交 `15df5b52d0edaa0d7686f7d6eb9299471ef906fe` 重新建立主线，停止继续叠加 v0.1.3 实验实现。本阶段只收敛 Agent 基座所需的版本、模型、Project、数据初始化和协议边界，不引入任务系统、Pilot 或高级协同能力。

## 主要变更

- 版本调整为 `v0.1.4-dev`。
- 删除 Depth Model、D1/D3 路由、中间件、配置字段和测试；`reasoning_effort=auto` 不再向模型发送固定档位。
- 删除模型身份提示和跨模型自动回退；端点缺少 Token 时直接返回明确错误。
- 新增最小 Project 数据模型、Default Project、会话归属以及 Agent/Skill 绑定表。
- 新增 v0.1.4 schema 历史校验；检测到 v0.1.3 实验迁移时返回 `STORE_SCHEMA_INCOMPATIBLE`，不自动改写旧数据。
- 新增 `semantic init --reset-data`：把数据库、WAL/SHM、Artifact 与 Project 工作区所在的整个 data 目录移动到时间戳备份目录，再创建干净数据目录。
- SQLite 文件权限收敛为 `0600`，Project 工作区和运行数据目录使用 `0700`。
- 删除任务语义协议残留：计量端点改为 `/api/v1/metering/traces/{id}`，Trace 查询删除 `task_id` 别名，WS 删除 `progress` 通道，事件关联改用 `run_id/trace_id`。
- 修复服务关闭竞态：关闭 SQLite 前显式取消并等待事件聚合器退出，避免关闭期间重新创建 journal/WAL。
- 将被 Depth 集成测试错误承载的 ToolSearch 公共等待逻辑迁回 ToolSearch 测试。

## 数据兼容说明

- v0.1.3 是未发布实验版本，其数据库不自动迁移到 v0.1.4。
- 需要重建时执行 `semantic init --reset-data`；旧 data 目录会保存在安装根目录的 `backups/` 下。
- `semantic init --force` 仍只重建安装配置，不删除运行数据。

## 验证

- `go test ./...`
- `TestTeamAssemblyAndAlert` 连续运行 20 次，验证关闭竞态已消除。
- Project 默认归属、绑定、目录权限与旧 schema 拒绝启动测试通过。
- `semantic init --reset-data` 备份与重建测试通过。

