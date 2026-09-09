# v0.1.2 版本汇总（技能 + 协同 + MCP + 验收）

> 2026-08-05 后续增量：安装配置、模型解析、推理与多模态对话链路已在
> [runtime_config_model_multimodal_20260805.md](runtime_config_model_multimodal_20260805.md)
> 记录。本文件下方的“未合入本分支”描述是 `feature/observability` 当时的历史状态，
> 不代表当前 `develop` 的合入状态。

日期：2026-08-05
分支：feature/observability（本汇总与 R18/R20 所在分支）

## 版本总览

v0.1.2 对应开发计划 §3.2（M2.4-M2.7）：通用技能系统、SubAgent 与多 Agent
协同、MCP 工具接入、观测补齐与版本验收。本文件是 R20 的版本收官汇总，
如实记录各里程碑的交付状态与版本关键指标。

## 里程碑交付清单（含合入状态）

| # | 里程碑 | 状态 | 说明 |
|---|--------|------|------|
| M2.4 | 通用技能系统 | ✅ 已完成，⏳ 未合入本分支 | 3 个提交在 `feature/skill-system`（SKILL.md 解析/store/渐进披露、skill.search 与 ToolSearch、skills REST） |
| M2.5 | SubAgent 与多 Agent 协同 | ✅ 已完成，⏳ 未合入本分支 | 2 个提交在 `feature/subagent-collab`（委派注册表/agent-as-tool/事件冒泡/黑板协同；审批穿透 CompositeInterrupt + 三项根因修正），集成测试 subagent_test.go / subagent_approval_test.go 随分支交付 |
| M2.6 | MCP 工具接入 | ✅ 已完成，⏳ 未合入本分支 | 2 个提交在 `feature/mcp-integration`（pkg/mcp 客户端封装；mcpregistry 目录/健康/热重载） |
| M2.7 | 观测补齐与版本验收 | ◐ 部分在本分支 | R18 traces/metering/interactions 只读 REST（已合入）；R20 性能基线 + 协同剧本验收 + 本汇总（本 MR）；前端 Trace 视图与 DeepSeek 真机联调不在本仓库/本 MR 范围 |

### 本分支实际包含的 v0.1.2 变更

- **R18 观测补齐**（`observability_rest_20260805.md`）：traces/metering/
  interactions 三组只读 REST 端点与 store 查询层，审批卡刷新恢复数据源。
- **R20 版本验收**（本 MR）：
  - 性能基线工具 `semantic-perf`（`cmd/semantic-perf`，`make perf-baseline`
    构建到 `.output/bin`）：计量分布（purpose/model 占比）、每轮平均输入
    token、耗时三分解、数据窗口、D1/D3 归因说明；store 层新增
    `SummarizeSpanKinds`/`PerfDataWindow`（`internal/store/perf.go`）；
  - 多 Agent 协同剧本验收（`tests/integration/acceptance_v012_test.go`）：
    ②monitor 告警、③审批穿透、④单窗口多角色、⑤刷新恢复四条端到端
    全绿；①委派查询因 M2.5 未合入以 SKIP 显性标注（合入后启用）；
  - 真实基线报告 `perf_baseline_20260805.md`（make build 产物 + mock
    跑验收剧本后由 semantic-perf 生成，含读数说明与数据缺口）。

## 关键指标

- 测试：23 个包带测试（全仓 27 个包），270 个测试函数，
  `go test ./... -race` 全绿（含 12 个集成测试文件的端到端用例）。
- 端点：REST 21 个（auth 3、system 3、agents 1、chat 4、settings 5、
  观测 5）+ WS `/ws/chat` 1 个。
- 性能基线：见 `perf_baseline_20260805.md`（mock 数据下 observe 70% /
  chat 30% 的计量分布；真实数值待真机联调）。

## 已知限制

- **M2.4/M2.5/M2.6 未合入本分支**：技能/委派/MCP 能力在各自特性分支
  完成开发但尚未 merge 到 feature/observability，验收剧本步骤①
  （委派查询）与步骤③的 query-1 归因、④的第三角色来源因此在本分支
  以 SKIP/旁注形式标注，合入后启用。
- **耗时三分解的工具/其他类别无数据**：工具执行不产生 `Tool` 跨度
  （TraceHandler 只挂模型组件回调），mock 模型跨度耗时亚毫秒落库为 0；
  三分解真实形态待真实模型接入与工具回调补齐。
- **每轮平均输入 token 为 mock 固定值**（10/20/30），仅证明计量链路
  贯通；D1/D3 分流 purpose 在 depth_models 启用后才有归因。
- `GET /api/v1/metrics/summary` 未实现（指标 registry 未建，R18 遗留）。
- 观测端点不做按用户过滤（表无 user 列，R18 遗留）。

## 升级注意

- **VERSION 保持 0.1.2**：Makefile `VERSION ?= 0.1.2` 自 R18 起已是
  0.1.2，本 MR 检查确认无需变更。
- **无 schema 迁移**：R20 只读查询不建表不改表，v0.1.1 的库（schema v7）
  直接可用；`semantic-perf` 对旧库只读、不执行迁移。
- **新构建目标**：`make perf-baseline`（构建 `.output/bin/semantic-perf`），
  `make build` 产物集不变（server/pilot/cli）。
- 观测库默认路径 `.output/semantic.db`，`-db` 可覆盖；库文件缺失时
  工具报错退出而不创建空库。
