# R13 SubAgent 审批穿透端到端（CompositeInterrupt 冒泡 + 归因修正）

日期：2026-08-05
里程碑：v0.1.2 / R13（SubAgent 审批穿透端到端）
分支：feature/subagent-collab

## 为什么做这次变更

R12 落地了 SubAgent 委派全链路（leader→ask_query→query-1→冒泡→黑板），
审批穿透"结构性可用但无端到端验证"：SubAgent 内调用 risk=high 工具触发
审批时，中断应经 eino CompositeInterrupt 冒泡到根 run，用户审批卡应答后
SubAgent 恢复执行。本 MR 对该链路做端到端验证并修正验证中发现的缺陷，
严格最小范围：不加新功能，只做穿透验证与修正。

## 穿透链路结论

**内核机制天然可用，端到端验证中发现并修正 3 个链路缺陷（根因见下）**。

eino v0.9.13 的 CompositeInterrupt 语义（`adk/agent_tool.go` action
scoping 注释 + 源码确认）：agent-tool 内的子 agent 运行在内存桥存
（bridge store，固定断点 ID）上；子 agent 中断时，agent-tool 取桥存断点
字节、把子中断链上下文（`internalInterrupted`）重建为子信号树，经
`tool.CompositeInterrupt` 上抛为父工具调用的中断——根因中断点（含审批
负载 ApprovalInfo）原样到达根运行器，父断点（随 run_sessions 持久化）
自包含全部恢复素材（tools-node 状态 + 桥存字节 + 子中断寻址）。
恢复时 resume 数据经 ctx 的全局 resume info 按中断点 ID 寻址，穿透
agent-tool 边界注入子 agent 的工具调用点。**因此 SubAgent 的 AgentConfig
不需要独立断点后端**（R12 装配的 `Store: s.st` 服务于 trace/计量，与断点
无关；父运行器的 CheckPointStore 是唯一持久化面）——kernel 级测试
`agent_tool_interrupt_test.go` 对此直接验证（SubAgent 不带 Store 穿透
批准/拒绝双链路）。

### 修正 1：审批归因——interaction.request 的发起方是 SubAgent 而非 leader

- **现象**：`awaitApproval` 恒以 `rt.profile.Name`（leader）发起交互请求，
  SubAgent 内触发的审批卡显示"leader 请求审批"，前端无法分辨真正发起方。
- **根因**：中断链上下文（eino InterruptCtx：ID/Address/Info/Parent）不
  携带 agent 身份，runtime 无从得知中断发起于哪个 agent。
- **修正**：中断负载自包含发起方身份——`security.ApprovalInfo` 新增
  `Agent` 字段（ApprovalInfo 的设计原则本就是"中断负载必须自包含审批
  所需的全部信息"）；safety middleware 装配时注入归因身份（成员实例 ID：
  同一角色 profile 可装配多个实例，归因以实例为准而非角色名——
  `buildToolchain` 新增 agentID 参数，leader 传 prof.Name、observer 传
  member.ID、SubAgent 传 def.ID）；`awaitApproval` 以 `info.Agent` 发起
  交互请求（空值防御性回退 leader 名）。交互落库（interactions.agent）
  与下行 envelope（agent.id）随之归因 query-1。

### 修正 2：委派任务文本配对在中断-恢复后丢失

- **现象**：委派经审批中断恢复后，`subagent.result` 的 Task 为空——黑板
  delegation 摘要退化为"委派 query-1： → …"。
- **根因**：R12 的任务文本配对表（call_id → 任务文本）挂在 EventStream
  实例上（流级状态）；中断关闭当前事件流，`Resume` 创建新流时配对表
  从零开始，而恢复流里不再有携带 tool call 的助手事件可重建配对。
- **修正**：配对状态提升为 run 级——`Runner.callTrackers`（断点 ID →
  call_id → 调用信息），绑定断点 ID 的事件流共享同 ID 配对表（只有绑定
  断点的流才可恢复、才需跨流延续；无断点的流不注册，零开销）；流正常
  结束（未中断）即注销条目，中断后永不恢复的条目随会话运行器淘汰
  （EvictSession）回收，无泄漏。并发约定保持：不同 run 断点 ID 不同，
  无共享写。

### 修正 3：trace 流式回调 goroutine 与测试清理/停机序列的竞态（既有缺陷）

- **现象**：`go test` 间歇失败 `TempDir RemoveAll cleanup: directory not
  empty`（本次开发中 kernel/security 包失败率高达 50%；基线提交同样
  复现，非本 MR 引入）。
- **根因**：`callbackModel.Stream` 的流式回调处理器（TraceHandler 把
  trace_spans/metering 落库）在独立 goroutine 消费流拷贝，消费流 EOF 与
  处理器收尾之间没有顺序保证；测试以事件流关闭为 run 终结信号随即
  `store.Close()` + 删除临时目录，落后的写事务（sqlite journal 文件
  在 RemoveAll 期间出现）与之竞态。200ms 延迟实验与残留文件分析
  （目录在失败时刻只有 journal、提交后目录为空）证实该机制。
- **修正**：`barrierStream` EOF 屏障——消费流包装一层 pipe，源流耗尽
  （或提前关闭）时先 `WaitGroup` 等处理器 goroutine 收尾再传播结束，
  让"流结束"隐含观测副作用已静止。修正在模型包装层生效，对
  leader/SubAgent（agent-tool 内）/monitor 全部装配形态一体覆盖，
  无 API 变化；非 EOF 错误按流内错误帧约定原样传播。修正后 kernel 包
  连跑 12 次、security 包连跑 10 次零失败（修正前 5/10、基线 4/15）。

## 包含内容

### 1. 演示工具 `query.danger_delete`（`internal/tool/builtin` + `internal/store`）

- 新内置工具：`query.danger_delete`（namespace=query，risk=high）——删除
  指定产物（参数 `{artifact_id}` 必填；缺失返回结构化 ARTIFACT_NOT_FOUND）。
  **为什么存在**：query 工具面全只读，没有高危工具就无法端到端验证
  "SubAgent 内审批中断经 CompositeInterrupt 冒泡"；删除产物是真实且可
  外部断言的副作用，是审批语义的最小演示载体。
- `store.DeleteArtifact(id)`：先删元数据行（引用先消失，读取方永不拿到
  悬空引用）再删内容本体（失败只留孤儿文件，记日志不判失败——与
  PutArtifact 的顺序哲学相反）；不存在返回 ErrNotFound（影响行数判定）。
- `configs/agents/query/role.yaml`：namespaces 增 `query.danger_delete`
  （精确名而非 `query.*`——演示工具按需最小授权）；AGENT.md 的
  Safety/Tools 段同步（只读职责的唯一例外，必须经人工审批）。
- 单测：RegisterAll 契约（6 工具、risk 表）、删除闭环（元数据+本体
  均不可读）、缺失/空参结构化错误；store 删除/重复删除语义。

### 2. 归因修正（`internal/security` + `internal/agent/runtime`）

- `ApprovalInfo.Agent` + `Middleware.agent` + `NewMiddleware` 增 agent
  参数；`gateWithApproval` 中断负载携身份。
- `buildToolchain(prof, agentID, ...)`：三处装配点分别传 leader 角色名、
  member.ID、def.ID；`awaitApproval` 按 `info.Agent` 发起交互请求
  （审批结论日志同步携带 agent）。

### 3. 配对延续与竞态根治（`internal/agent/kernel`）

- `run.go`：Runner.callTrackers + newEventStream(checkpointID) 挂接 +
  pump 正常收尾注销（EventStream.trackerDone 回调）。
- `agent.go`：barrierStream EOF 屏障（callbackModel.Stream 返回路径）。

### 4. 测试

- `internal/agent/kernel/agent_tool_interrupt_test.go`（内核级穿透）：
  leader（委派 ask_query）+ query-1（report.save + 审批门禁，**不带
  Store**）→ 断言：1 个根因中断点且负载为审批请求、复合断点随 run
  持久化、批准恢复后工具真实执行/subagent.result 归因与任务文本配对/
  leader 总结；拒绝路径工具不执行、SubAgent 如实回报。
- `tests/integration/subagent_approval_test.go`（端到端）：真实装配
  （bootstrap.Wire + configs/agents 三成员 Team）+ 路由 mock 脚本
  （leader 轮 1 委派轮 2 总结；query 轮 1 调 query_danger_delete 轮 2
  按结论回报）→ 播种演示产物 → WS 发消息 → 断言：
  - interaction.request 到达（confirm/risk=high/问题含工具名），
    **envelope 归因 query-1**（非 leader）；交互落库归因 query-1；
  - 中断期间 run=awaiting_approval 且产物未删；
  - 批准 → 产物真实删除（GetArtifactMeta → ErrNotFound）→
    subagent.result（task/text 正确）→ leader 总结 → run=done、
    交互已应答批准；
  - 拒绝 → 产物仍在 → SubAgent 如实回报 → 交互已应答拒绝。

### 5. 文档

- `docs/architecture/04-agent-runtime.md` §7.3：新增"审批穿透（v0.1.2
  R13，已落地）"段——冒泡机制、桥存与单断点面、归因来源、恢复寻址、
  验证入口。

## 影响面

- `internal/store`：artifact.go +DeleteArtifact（纯新增，存量语义不变）。
- `internal/tool/builtin`：RegisterAll 注册第 6 个工具（存量工具不变）。
- `internal/security`：ApprovalInfo +Agent 字段（gob 已注册的类型加字段，
  向后兼容）；NewMiddleware 签名变化（唯一调用方 runtime 同步更新）。
- `internal/agent/runtime`：buildToolchain 增 agentID 参数（三处调用点
  同步）；awaitApproval 归因取自中断负载。
- `internal/agent/kernel`：Runner +callTrackers（无委派工具/无断点的流
  零开销）；agent.go Stream 返回路径加 EOF 屏障（无处理器时不包装）。
- `configs/agents/query`：role.yaml namespaces + AGENT.md 文档同步。
- 无新增第三方依赖；无新增配置项。

## 测试内容与标准

- 单测（-race）：store 删除语义、builtin 契约与删除闭环、security 归因
  字段、kernel 穿透双链路（批准/拒绝）、配对延续断言。
- 集成测试：subagent_approval 双链路；存量 subagent/team/approval/chat/
  blackboard 集成测试原样通过。
- 门禁：gofmt 无输出、go vet 通过、golangci-lint 零告警、
  go test ./... -race 全绿、make build 成功；手动实测见下节。
- 竞态回归验证：kernel 12 连跑、security 10 连跑零失败（修正前 50% 失败率）。

## 手动实测

启动 server（DEBUG 日志，mock 路由脚本同集成测试）→ WS 发送"把演示产物
删掉"→ 批准链路日志序列："SubAgent 委派工具已装配" → "危险工具调用，
发起审批中断（tool=query.danger_delete）" → "run 等待人工审批" →
"交互请求已发起（agent=query-1）" → 应答后 "审批结论（agent=query-1,
approved=true）" → "run 恢复执行" → "审批通过，放行工具调用" →
"委派完成（subagent=query-1）" → "run 结束"；拒绝链路："审批结论
（approved=false）" → "审批未通过，取消工具调用" → SubAgent 如实回报，
产物保留。
