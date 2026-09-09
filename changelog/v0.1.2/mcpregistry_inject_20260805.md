# R16 mcpregistry：MCP 工具目录 + 静态声明接入 + 工具注入 Agent

日期：2026-08-05
里程碑：v0.1.2 / R16（MCP 集成）
分支：feature/mcp-integration

## 为什么做这次变更

R15 落地了 MCP 客户端底座（pkg/mcp：连接池/list-changed/config mcp_servers
段）但没有任何消费方——`OnMCPServersChanged` 还是空钩子，MCP 工具到不了
Agent。本 MR 按架构 16 §4 落地 **mcpregistry**（MCP 工具目录）并打通整条
链路：静态声明（mcp_servers）→ 目录发现（对账三态 + 双频 + list-changed）
→ 命名空间过滤 → kernel 适配注入 Agent（与内置工具同权：同一净化契约、
同一 safety 门禁、同一审批名单）→ 热重载对账。设计沿用调研结论（agent_ori
reconcile 三态 + warning 不清仓 + 双频对账）与强类型契约教训（旧框架
map[string]any 猜谜，本轮 Entry 全显式字段，编译期门禁）。

## 包含内容

### 1. `internal/mcpregistry`（新包）

- **强类型契约 v1**（`entry.go`）：`Entry{FullName(<server>.<tool>), Server,
  Tool, Description, SchemaJSON, Risk, SourceKind(mcp), Health(healthy|
  unavailable), UpdatedAt}`。doc.go 逐条写清"为什么"：与 agent_ori
  map[string]any 猜谜的教训；**内存目录不落库**的理由（目录是 server 侧
  tools/list 的投影，落库只会制造第二事实源；重启后首轮同步秒级重建）；
  **命名空间 = server 名**（不另设别名，双事实源必然发散——`mcp_servers
  [].namespace` 字段本轮保留不消费）。
- **Registry**（`registry.go`）：
  - `SyncServer(ctx, spec)`：pool 取连 → 桥接 list-changed 钩子 →
    ListTools → **reconcile 三态**（added/updated/removed，按 description/
    schema/risk 比对）→ 快照替换该 server 条目集；返回 `SyncReport`
    （排序清单，日志审计与测试断言用）。内容未变的条目保留原 UpdatedAt
    （对账幂等：无变化即无写）。
  - **warning 不清仓**：取连/ListTools 失败只标 unavailable 保留旧条目
    （网络瞬断/进程重启是常态，失败即清空会让 Agent 工具视图随抖动反复
    增删）；条目只在成功对账确认下线（reconcile removed）或配置删除
    （`RemoveServer`）时消失。调用方 ctx 取消不改健康状态（关停不误标）。
  - `List()`（含 unavailable，REST 目录视图）/ `Get(fullName)` /
    `MatchNamespaces(patterns)`（只含 healthy，Agent 注入视图——失联
    来源的工具不进模型视图，架构 16 §4.3）。
  - `MarkUnavailable/MarkHealthy`（外部健康标记入口，provider 心跳语义
    预留的正规通道）；健康按 server 维护（全部条目同生共死），迁移才记
    日志（WARN 失联 / INFO 恢复）。
  - `CallTool(ctx, server, tool, argsJSON)`：kernel 适配层的执行后端——
    spec 定位 → pool 取连 → 调用 → **结果归一**（OK 信封；连接类失败
    `MCP_SERVER_UNAVAILABLE` retryable、协议/工具错误 `TOOL_ERROR`
    不重试、未知 server `MCP_SERVER_UNKNOWN`；Go error 仅父 ctx 取消，
    与内置执行器同一出口约定）。连接失败顺带标 unavailable、调用成功
    顺带标 healthy（自愈，不等对账轮）。
- **Syncer**（`syncer.go`）：随 App 生命周期启停（Start/Stop）；
  每个 enabled server 一个同步项（独立 goroutine）：
  - 立即首轮对账 → **双频**：快频 2s×3 轮（启动期 server 未就绪/首轮
    失败时数秒内收敛）→ 稳态 30s（list-changed 已覆盖即时变化，周期
    只是未声明 listChanged 能力的 server 的兜底；远低于 15min 短路窗口，
    不放大故障）。
  - **list-changed 即时同步**：`OnToolsChanged` 桥接到 Registry 钩子，
    非阻塞投递（通知合并，SDK 接收 goroutine 不阻塞）→ 只对该 server
    单独对账。
  - `Reconcile(specs)`（热重载入口）：DeepEqual diff——新增启动同步项、
    删除停同步并 `RemoveServer`、变更重建同步项（pool 按配置 hash
    memoize，连接参数变化自动新建连接）。
  - `SyncerOptions`：节奏可注入（集成测试关周期对账，显式 SyncServer
    驱动上下线形态，消除后台时序干扰）。

### 2. 工具适配与注入（kernel + runtime）

- **kernel `AdaptMCPTools(entries, caller)`**（`tools_mcp.go`）：Entry 经
  `Entry.Definition()` 投影为工具契约（与安全门禁共用同一份投影），
  适配为 eino BaseTool——**复用 AdaptTools 的净化与安全契约**：
  SafeToolName 净化（`test.echo` → `test_echo`）、净化后重名适配期报错、
  描述带原始全名前缀、schema 预解析（抽出共用的 parseToolSchema/
  toolInfo 助手，两个适配器同源）。执行经 `MCPCaller` 接口路由到目录
  （不经内置执行器——MCP 工具不走本地 registry）。
- **toolNamed 契约**（`tools.go`/`middleware.go`）：safety middleware 的
  净化名→原名映射原本类型断言 `*toolAdapter`，MCP 适配器会被漏掉导致
  门禁误拒（按 UNKNOWN_TOOL 拒绝）。抽 `toolNamed` 接口（`fullName()`），
  两个适配器同源实现，映射对两类工具一致生效。
- **runtime.buildToolchain**：内置注册表与 MCP 目录（仅 healthy）经同一
  `tools.namespaces` 过滤后合并注入，同名清单构建 safety middleware——
  MCP 工具与内置工具同权：ToolSearch 动态集（预留开关，本轮工具直注）、
  L2 参数校验、L4 审批名单（`risk: high` 的 MCP server 同样触发人工
  审批中断）。内置/MCP 同名冲突构建期报错（门禁清单按名索引，静默
  覆盖会绕过审批名单）。

### 3. 热重载消费（bootstrap）

- `wire_mcp.go`：`applyMCPServers`（OnMCPServersChanged 实现）——结构
  校验（重名/非法 risk，fail-closed 拒绝整段；settings PATCH 同源预校验
  返回 400）→ `Syncer.Reconcile` 对账。**目录变化不额外广播**：Agent
  工具链在会话运行器构建时重新查询目录（与 profile 热重载同一语义——
  已缓存会话持旧工具快照，新会话/重建即见新目录，"下一轮 PrepareAgent
  生效"）。
- 生命周期：`WireWithOptions` 装配 pool/registry/syncer（`Wire` 委托零值
  Options；Options 供集成测试注入短路窗口与对账节奏）；`App.Run` 启动
  syncer（先于热重载 watcher）；优雅关闭在 runtime 之后停 syncer 并关
  连接池。启动期对 `mcp_servers` 结构校验 fail-closed；连接不在装配期
  建立（server 不可达只降级不阻塞启动）。
- config：`MCPServerConfig` 加 `risk` 字段（per-server 风险覆盖，默认
  medium；写操作类 server 显式提级触发 L4 审批）；pkg/config doc.go
  白名单表与 reload.go 钩子注释同步为已实现。

### 4. REST 目录端点

- `GET /api/v1/tools`（鉴权）：按来源分组的只读目录（builtin 组 +
  mcp 组），统一形状（name/namespace/description/risk/schema/health，
  mcp 组另有 server/updated_at）——为 R17 前端工具目录页供数据；
  MCP 组含 unavailable 条目（目录页需要展示失联工具）。
- `docs/api/rest.md` 补 tools 节；`docs/developer/06` 配置参考更新
  mcp_servers 段（risk 行、namespace 预留说明、热重载语义）；
  `configs/semantic-server.yaml` 注释示例同步。

## 测试内容与标准

- `internal/mcpregistry` 单测（-race 全绿）：
  - reconcile 三态（首同步全 added/幂等无变化保留 UpdatedAt/描述变化
    updated/工具下线 removed）；risk per-server 覆盖；
  - warning 不清仓（失联后条目保留标 unavailable、注入视图排除、恢复
    回 healthy）；外部健康标记；MatchNamespaces 语义（前缀/精确/空模式）；
  - CallTool 归一（OK 信封/真实到达 server/TOOL_ERROR 不重试且不影响
    健康/MCP_SERVER_UNKNOWN/失联 MCP_SERVER_UNAVAILABLE retryable/
    恢复调用自愈）；RemoveServer（条目+spec 移除，调用归 UNKNOWN）。
  - syncer：启动首轮对账；**list-changed 只触发该 server 对账**（另一
    server 条目 UpdatedAt 不变）；Reconcile 新增/变更（risk 刷新）/删除/
    清空；周期对账兜底收敛；崩溃-恢复自愈；未启动 Reconcile 忽略。
- kernel 单测：AdaptMCPTools 净化名/描述前缀/schema/路由透传/净化冲突/
  非法 schema/toolNamed 契约。
- runtime 单测：命名空间过滤（命中/未命中/unavailable 不注入）；
  **MCP 工具经门禁 L2 校验**（非法参数 PARAM_VIOLATION 不执行、合法
  参数放行真实到达 server）；内置/MCP 同名冲突构建期报错。
- handlers 单测：builtin 组契约/空目录空数组（非 null）。
- 集成测试 `tests/integration/mcp_test.go`（-race 全绿）：内存 MCP
  server（echo/weather）→ mcp_servers 声明 → App 启动 → REST 目录含
  `test.echo`（healthy，分组正确）→ mock leader 调 `test_echo`（净化名）
  → 经 pool 真实执行 1 次 → 停 server → 条目 unavailable 未清除
  （REST 同步展示，调用归 MCP_SERVER_UNAVAILABLE）→ 恢复 → healthy
  且可再次调用。
- 门禁：`gofmt -l .` 无输出、`go vet ./...` 通过、`golangci-lint run`
  零告警、`go test ./... -race` 全绿、`make build` 成功。
- 手动实测：真实 MCP server 上下线两形态的目录（GET /api/v1/tools）
  与调用日志（对账/降级/恢复全链路 WARN-INFO 序列正确）。

## 遗留 TODO

- `mcp_servers[].namespace` 字段保留未消费（R16 契约：命名空间 =
  server 名）；R17 评审移除或启用为别名映射。
- 架构 16 §8 的 `mcp.discovery_interval/call_timeout/circuit_breaker`
  配置段未建模：本轮对账节奏（2s×3/30s）与短路（15min）走代码默认值
  （SyncerOptions/PoolOptions 可注入），待真实多 server 运维数据再开
  配置面。
- list-changed 桥接在成功对账时注册：连接重建（剔除/半开）到下一次
  对账之间的通知窗口由稳态对账兜底（30s 内收敛）。
- ToolSearch（profile `tool_search` 开关）本轮工具直注；工具规模大后
  落地检索时，MCP 目录（MatchNamespaces/List）是其数据源。
- provider（卫星服务生命周期编排，架构 16 §5-6）未在本 MR 范围；
  MarkUnavailable/MarkHealthy 是其心跳状态的预留入口。
