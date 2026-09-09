# R4 role.yaml 全字段消费 + Team 组建 + monitor 装配 + Agent 目录 API

日期：2026-08-04
里程碑：v0.1.1 / R4（Team 与角色装配）
分支：feature/settings-system

## 为什么做这次变更

此前只有 leader 一个角色在线：profile 部分字段（pinned）未建模、mode 缺省
静默当协调者；无 Team 概念、无 observer 形态。本 MR 按架构文档 04 §3/§4
落地：role.yaml 全字段消费（含 mode 必填校验）、Team 定义与启动组建、
observer 模式首实例 monitor（长驻循环 + 数据驱动规则表告警）、
Agent 目录（roster）与只读 API。严格最小必要实现：service 角色只注册
待命，黑板只有预留接口的 log 实现，无预留代码。

## 包含内容

### 1. profile 全字段消费（`internal/agent/profile/`）

- `tools.pinned` 常驻工具清单入 schema（解析透传，ToolSearch 落地后消费，
  与 depth_models/tool_search 同策略）；leader/query/monitor 的 role.yaml
  注释同步补全该字段。
- **mode 改为必填**：缺省静默装配成 coordinator 会让 observer/service
  的配置笔误变成错误形态，加载即报错（未知 mode 仍报错）。单测更新：
  全字段（含 pinned）加载、默认值、缺 mode/mode 非法分支。

### 2. kernel 最小 TurnLoop 封装（`internal/agent/kernel/loop.go`）

- `BuildLoop`：adk TurnLoop 封装——WorkItem（kind/text）Push 入队、
  整批消费（每项一条用户消息）、`Stop(grace)` 安全点优雅停止
  （WithGracefulTimeout + Wait）。middleware 栈与对话形态完全一致：
  从 `BuildAgent` 抽出共用的 `buildChatAgent`，两种装配同一构建路径。
- 轮次事件归集为 DEBUG 摘要（轮次/用量/评估文本长度）；CancelError 按
  adk 契约不上抛，其余错误只 WARN 不中断循环（observer 必须长驻）。
- 单测：Push → 模型消费两个工作项 → Stop → 停止后 Push 拒绝。

### 3. Team 定义与组建（`internal/agent/team/` + `internal/agent/runtime/team.go`）

- `team.Def`（name/leader/members[](id/role/device?)/blackboard.retention_days）
  + `LoadTeams(dir)`：目录缺失 = 未配置 Team（单 leader 合法形态）；
  校验 name/leader.role/成员 id/role 必填、id 全局唯一、Team 名跨文件去重，
  leader id 缺省回填 role 名、保留期默认 7 天。
- `configs/agents/teams/default.yaml`：leader + monitor-1（monitor）+
  query-1（query，新建 service profile：model=deepseek-chat、
  namespaces=[system.*,artifact.get]、pinned=[artifact.get]、AGENT.md
  系统/产物查询助手）。
- `Service.AssembleTeam` 组建序列：coordinator 走现有会话装配（登记
  idle）；observer 经 monitor 装配启动（starting→running）；service
  只登记待命（TODO(phase-m2.5) agent-as-tool）；worker 直接报错不降级。
  任一失败清理已启动成员，不留半组建状态。
- roster（`runtime/roster.go`）：id/role/mode/status(starting/idle/
  running/stopped)/model/activity；leader 在 HandleMessage 期间迁移
  running→idle；`Service.Shutdown` 停 observer 并标记 stopped
  （bootstrap 在关 store 前调用——observer 轮次写 trace）。

### 4. monitor 角色装配（`internal/agent/monitor/`）

- observer 首实例：`configs/agents/monitor/`（mode=observer、
  model=deepseek-chat、max_turns=5、AGENT.md 观测与告警职责）+
  `rules.yaml` 规则表。
- 规则表 v1（纯数据驱动）：`match(topic 必中 + field/equals 成对等值)
  → level(1-5) → message 模板（{field} 占位符）`；Evaluate 取最高级别
  命中；加载即全量校验（空表报错）。
- 观测源 v1：事件总线 agent.events 全量 envelope（提取信封元数据为
  字段；**自过滤**——monitor 自己的告警回流跳过，防告警风暴）+
  系统心跳（30s 周期，uptime/goroutines，演示用）。
- 告警输出：level≥3 发 envelope（channel=alert、type=monitor.alert、
  importance=critical、payload 带数值 level——聚合器分级契约）+
  AlertSink 沉淀（预留接口，默认 log 实现，TODO(phase-m2.3) 黑板）；
  level<3 仅 DEBUG。每条观测同时 Push 进长驻循环驱动模型评估
  （本轮评估输出只记 DEBUG，告警唯一权威来源是规则表）。
- 单测：规则匹配分级/模板渲染/加载校验、observer 端到端装配
  （critical 事件→告警 envelope 形态与沉淀）、自过滤、心跳观测源。

### 5. Agent 目录 API（`internal/server/http/handlers/agents.go`）

- `GET /api/v1/agents`（鉴权）：`{"agents":[{id,role,mode,status,model,
  activity}]}`，按 ID 升序；`NewRouter` 增加 agents handler 参数。
- 单测（httptest + 真实 runtime 组建三成员）：排序、字段、snake_case。

### 6. bootstrap 接线与配置

- `agents.teams_dir`（默认 `configs/agents/teams`，env
  `SEMANTIC_AGENTS_TEAMS_DIR`）：config 结构/默认值/yaml/env 四处同步；
  **需重启**（reload restartOnlyDiffs 登记 + 快照保留旧值）。
- Wire 第⑥步后组建常驻 Team：teams_dir 缺失/为空 → 单 leader 模式
  （INFO）；v1 固定组建名为 `default` 的 Team（多 Team 路由落地前不
  引入 default_team 配置项，文档已登记）；缺失 default 定义即启动失败。

### 7. 集成测试 `tests/integration/team_test.go`

真实 configs/agents 起 App（mock 模型）→ GET /api/v1/agents 断言三成员
角色/模式/状态 → 发布 critical 事件 → 断言收到 monitor.alert envelope
（channel=alert、importance=critical、level=4、来源 monitor-1）。

## 影响面

- `internal/server/http.NewRouter` 签名 +1 参数（调用方 bootstrap 与
  router_test 已同步）。
- `pkg/config.AgentsConfig` +TeamsDir（settings 配置树/PATCH 校验自动
  覆盖该键；热重载按"需重启"处理）。
- `internal/agent/runtime`：Service +roster/leaderID/observers，
  新增 AssembleTeam/Roster/Shutdown；HandleMessage 增加 leader 状态迁移。
- `internal/agent/kernel`：BuildAgent 抽出 buildChatAgent（行为不变），
  新增 BuildLoop/WorkItem/Loop。
- profile：mode 必填（原缺省 coordinator 行为移除，仓内 profile 均已
  显式声明）；role.yaml +tools.pinned。
- 无新增第三方依赖。

## 测试内容与标准

- 单测（-race）：profile 全字段/默认值/校验；kernel loop Push/Stop；
  team 加载/校验；monitor 规则表/装配/告警形态/自过滤/心跳；
  runtime 组建三成员+拒绝路径+Shutdown；handlers /agents。
- 集成测试：上节⑦；存量集成测试不受影响（未配 teams_dir 时单 leader
  模式，路径不变）。
- 门禁：gofmt 无输出、go vet 通过、golangci-lint 零告警、
  go test ./... -race 全绿、make build 成功；手动实测 GET /agents 与
  规则命中告警见下节。

## 问题与遗留 TODO

- `TODO(phase-m2.5)`：service 角色（query-1）只注册 roster 待命，
  未接 agent-as-tool，不承接请求。
- `TODO(phase-m2.3)`：AlertSink 只有 log 实现；黑板（WorkSession）
  落地后高级别告警改落黑板。depth_models 同样 M2.3 才消费（本轮只解析）。
- monitor 模型评估输出本轮只记 DEBUG 摘要（告警唯一权威是规则表）；
  "模型评估升级为告警源"留待后续里程碑评估。
- v1 固定组建名为 default 的 Team；多 Team 与 default_team 配置项随
  server 层路由落地。
- observer 的 roster 状态粒度是生命周期级（running=循环存活），
  不含 per-turn 的 idle/running 翻转（04 §5.1 的完整状态机随后续里程碑）。
- 已知存量 flake（非本轮引入）：kernel 中断测试偶发
  `TempDir RemoveAll cleanup: directory not empty`
  （pristine 树可复现，与断点落库文件句柄时序相关），门禁遇发重跑即可。
