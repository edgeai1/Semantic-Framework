# R7 共享黑板 v1（WorkSession）+ 上下文 S4 段

日期：2026-08-04
里程碑：v0.1.1 / R7（黑板与上下文协同段）
分支：feature/settings-system

## 为什么做这次变更

R4 组建的 Team 三成员只有"各自在线"，没有协同状态面：monitor 的告警沉淀
到日志就结束（AlertSink=log 预留），leader 对告警与其他成员的活动无感知。
按架构文档 10 §8（Agent 不读彼此的 run session，协同状态一律经黑板）与
§3（S4 黑板摘要段），本 MR 落地共享黑板 v1 与上下文 S4 段：
monitor 告警落黑板，leader 的每轮模型输入携带黑板摘要。严格最小必要实现：
work_session 只有 default 一个实例，保留期固定 7 天不做 TTL 治理，
条目只有 delegation/alert/run_summary 三类。

## 包含内容

### 1. store 迁移 v7（`internal/store/blackboard.go` + `migrate.go`）

- `work_sessions(id PK, team, status, created_at, updated_at)`：协作态会话，
  v1 仅 default 实例（active 态，无关闭/归档治理）。
- `blackboard_entries(id PK, work_session, author, type, importance, payload, ts)`：
  协作条目，payload 为 JSON（delegation/run_summary 含 summary，alert 含
  level/message，run_summary 另含 run_id）；条目 ID 为 `bb-<纳秒>-<随机>`
  （字典序 ≈ 时间序，与消息 ID 同策略）；索引 `(work_session, ts)`。
- CRUD：`CreateWorkSession` / `GetOrCreateWorkSession`（INSERT OR IGNORE +
  SELECT，并发首建不暴露主键冲突）/ `AppendEntry` / `ListEntries(ws,
  {types?, since?, limit})`——Limit 取最新 N 条（SQL 倒序截取后反转，
  调用方永远看到 (ts,id) 升序）。
- 单测：GetOrCreate 幂等/字段回填；追加与 types/since/limit 过滤、跨
  work_session 隔离。

### 2. `internal/agent/blackboard`（新包）

- `blackboard.go`：`Blackboard`（store + log 门面），`New` 确保默认
  WorkSession 存在（启动路径失败即暴露）。三类写入：
  `WriteDelegation(author, summary)`、`WriteAlert(author, level, message)`
  （level≥3→critical，否则 normal——与 monitor 下行阈值同源）、
  `WriteRunSummary(author, runID, summary)`；payload 统一 JSON。
- `ReadView(role, limit) []Entry`——视角过滤 v1：leader/coordinator 读全量
  类型；其余角色只读 delegation+alert（run_summary 是执行归档沉淀，非协调者
  无需在上下文中消费）。Entry.Summary 从 payload 按类型提取
  （alert→message，其余→summary）。读取失败降级空视图并 WARN：
  S4 是上下文增强项，黑板故障不阻断 run。
- `digest.go`：`Digest(entries, maxTokens) string` 纯函数——按时间倒序
  （最新在前），每条一行 `[time] author: summary`；超预算从旧到新截断，
  最新一条独超预算时硬截断该行而不是放弃整个 S4 段。token 按 rune 保守
  估计（中文 1 rune≈1 token，英文高估——宁紧勿超，不引入厂商分词器）。
- 单测：三类写入的 payload 结构、视角过滤（leader 全量/monitor 部分/limit
  取最新）、Digest 倒序/行格式/截断/空输入。

### 3. monitor AlertSink 黑板落地（`internal/agent/monitor/alert.go`）

- `NewBlackboardSink(bb, author, logger)`：boardSink 实现 AlertSink——
  **log+blackboard 双写**（黑板是 Team 协同面、S4 数据源；日志是排障面，
  职责不互相替代）。author 取成员实例 ID（monitor-1），条目归因到实例。
- AlertSink 接口注释更新（TODO(phase-m2.3) 移除，本 MR 即落地）。
- 单测：WriteAlert 落 type=alert 条目（author/importance/摘要）经
  ReadView 读回。

### 4. kernel context middleware 增加 S4 段（`internal/agent/kernel/`）

- `AgentConfig.Blackboard BlackboardReader`（kernel 定义的读取端口，
  `ReadView(role, limit)`；nil = 无协同面，跳过 S4）。
- context middleware：S4 注入在 S1（+S3）之后、S7 历史之前——**缓存纪律**：
  S1-S3 是跨轮不变的稳定前缀（厂商 prefix 缓存命中段），S4 是缓变段
  （分钟级），放在稳定前缀之后使 S4 的变化不波及 S1-S3 缓存；单次 run 内
  S4 不刷新（与 S1 同一次幂等注入），保住轮间缓存。黑板为空不注入
  （无占位段）。段文本 = `## 黑板摘要（S4）` 标题 + Digest 渲染。
- 预算：条目上限 20 条 + 摘要上限 2000 tokens（常量
  `s4BlackboardEntries` / `s4BlackboardMaxTokens`，注释对应文档 10 §4
  预算表 S4 占比 10% 按默认 20000 级预算折算；固定上限不随 profile
  放大——黑板摘要是协同提示而非知识库）。注入处打 DEBUG（条目数/字符数）。
- 单测：S4 位置（[S1, S4, user] 段落序）、摘要含告警内容、视角角色传递、
  幂等不重复注入、空黑板无占位段；`TestS4InRunInput` 经 BuildAgent 完整栈
  断言 S4 进入 mock 模型输入。

### 5. run 完成摘要写入（`internal/agent/runtime/service.go`）

- run 结束（done/failed，含 RunWithCheckpoint 启动失败路径）写
  `WriteRunSummary`：作者=Agent 名（如 leader），成功摘要=助手输出前 80 字
  （`runSummaryRuneLimit`，黑板承载结果摘要而非执行细节，完整输出在
  chat_messages / run session 归档），失败摘要=错误文本前 80 字。
  写入失败只 WARN（与 finishRun 的归档失败同策略，不反过来影响结果）。
- observer 装配同步接入：monitor 的 Sink 经 `alertSink()` 双写注入；
  monitor 长驻循环的 AgentConfig 也带黑板端口（monitor 视角只读
  delegation+alert，S4 语义对 observer 一致）。
- **typed-nil 修正**：`Blackboard: s.blackboardReader()` 转换——nil
  `*Blackboard` 直接赋接口字段会得到非 nil 接口，kernel 的 nil 检查失效
  （首版直接赋值导致无黑板形态 panic，测试暴露后修正）。
- 单测：成功/失败两条路径的 run_summary 条目（author/摘要/payload.run_id
  对应）、truncateRunes 截断。

### 6. bootstrap 接线（`internal/bootstrap/wire_agent.go`）

- Wire 第④步创建 `blackboard.New(st, logger)`（确保默认 WorkSession）并
  注入 runtime.Deps；初始化失败启动即报错。单 leader 模式同样挂接
  （run 摘要与 S4 不依赖 Team 定义）。

### 7. 集成测试 `tests/integration/blackboard_test.go`

真实 configs/agents 起 App（Team 三成员在线）→ 发布 critical 事件 →
断言黑板 alert 条目（author=monitor-1、critical、payload level=4 且消息
渲染正确）→ HandleMessage（mock 模型）→ 断言 run_summary 条目
（author=leader、摘要=助手输出）→ 断言黑板同时含两类条目。

**S4 断言路径说明**：mock 模型实例在 runtime 内部构建，集成层拿不到
CallInputs，故"S4 注入内容含告警摘要 + 段落位置"在 kernel 单测
（TestContextMiddlewareS4 / TestS4InRunInput，桩驱动黑板）断言；
集成测试负责验证"告警→黑板→run 摘要"的数据流闭环。手动实测的
DEBUG 日志（S4 注入计量）见下节。

## 影响面

- `internal/store`：迁移 v7（两表一索引）；`blackboard.go` CRUD。
- `internal/agent/blackboard`：新包（doc/blackboard/digest + 单测）。
- `internal/agent/kernel`：AgentConfig +Blackboard；context middleware
  增加 S4 段（S1 注入行为不变——存量单测原样通过）。
- `internal/agent/monitor`：AlertSink 增加 boardSink 实现（logSink 保留为
  无黑板形态的默认）。
- `internal/agent/runtime`：Deps +Blackboard；Service +bb/alertSink/
  blackboardReader/writeRunSummary；run 三个终态路径写摘要。
- `internal/bootstrap`：wire_agent 创建并注入黑板。
- 无新增第三方依赖；无配置项变更（黑板目录/保留期不开放配置）。

## 测试内容与标准

- 单测（-race）：store CRUD 与迁移；blackboard 写入/视角/Digest；monitor
  boardSink；kernel S4 注入（位置/内容/幂等/空黑板）与 run 输入端到端；
  runtime run_summary 双路径与截断。
- 集成测试：上节⑦；存量 team/chat/approval 等集成测试不受影响
  （无黑板形态行为不变——typed-nil 修正后 nil 接口语义正确）。
- 门禁：gofmt 无输出、go vet 通过、golangci-lint 零告警、
  go test ./... -race 全绿、make build 成功；手动实测见下节。

## 手动实测

启动 server（DEBUG 日志）→ 发布 critical 事件 → 日志依次出现
"告警沉淀"（log 面）与"黑板条目已写入 type=alert"（黑板面）→ 发送一条
对话消息 → leader run 的 DEBUG 出现 "S4 黑板摘要已注入"（entries/chars
计量）→ 库内 blackboard_entries 可见 alert 与 run_summary 两类条目。
