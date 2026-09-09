# B5（M1.5 工具与安全骨架 + interaction 审批链路）

日期：2026-08-03
里程碑：Phase 1 / B5（M1.5 工具与安全骨架 + interaction 审批链路）

## 为什么做这次变更

B4 的对话闭环里 agent 只会"说话"不会"做事"：没有工具可调用，更没有
对危险动作的强制约束。本里程碑落地 Phase 1 最后一条核心链路——
**工具调用经安全管线 → 危险工具触发 confirm 审批 → 用户应答 →
断点恢复执行**，同时立起工具体系骨架（契约/注册表/执行器 + 5 个
内置工具）与 artifact store。安全管线的强制力来自执行路径上的
middleware（架构 09 §3：L0 是引导，L2-L5 是强制），本里程碑落地
L2（参数校验）与 L4（人工审批）两层。

## 包含内容

- **internal/store（迁移 v4）**：
  - `artifacts(id PK, media_type, uri, summary, metadata, size, created_at)`：
    元数据落库，内容本体存 `<db 同级>/artifacts/<id>` 文件（默认库路径
    .output/semantic.db → .output/artifacts，与架构约定一致）；
    `PutArtifact`（先写文件再落库，失败尽力清理）/`GetArtifact`/
    `GetArtifactMeta`/`ListArtifacts`；URI 为 `artifact://<id>`
    （05 §9：tool result 只返回引用与摘要）。
  - `interactions(id PK, session_id, agent, type, status, payload, reply,
    run_id, checkpoint_id, created_at, answered_at, expired_at)`：
    `CreateInteraction/GetInteraction/AnswerInteraction/ExpireInteraction/
    CancelInteraction/CancelPendingInteractions/ListPendingInteractions`；
    终结全部走**条件更新**（`WHERE status='pending'`），防重复应答在
    数据库层原子保证（并发应答只有一个成功，败者得 `ErrNotPending`）。
  - `run_sessions` 增加 `checkpoint BLOB` 列与
    `UpdateRunSessionStatus`（running ↔ awaiting_approval 中途迁移）、
    `SaveRunCheckpoint/GetRunCheckpoint`；新增状态
    `RunStatusAwaitingApproval`。
- **internal/tool（工具体系骨架，05 §3/§5）**：
  - `tool.go`：`Definition`（Name/Namespace/Description/ParametersJSON
    (jsonschema)/Annotations{Risk,Timeout,Streaming,Idempotent}）、
    `Tool` 接口（`Def()`/`Run(ctx, argsJSON)`）、`Registry`
    （Register 查重/List/ByNamespace/MatchNamespaces——`artifact.*`
    前缀模式与精确名）、结构化结果出口 `OKResult`/`ErrorResult`
    （`{ok,data?}` / `{ok,error:{code,message,retryable}}`，线上唯一
    格式）、`*tool.Error`（工具业务错误的 code/retryable 载体）。
  - `executor.go`：并发执行器——`Execute` 同轮多调用并发（结果按
    输入序）、命名空间级串行选项（`SerialNamespaces` 配置驱动，
    命名空间锁互斥）、超时硬上限（annotation.Timeout 优先，默认
    30s，context deadline + select 强约束）、一切失败归一为结构化
    错误（`UNKNOWN_TOOL/TIMEOUT/TOOL_ERROR`，`*tool.Error` 原样
    透传）；父 ctx 取消作为 Go 错误传播（运行级取消不伪装成工具
    结果）。工具调用开始/完成/失败/超时全落日志。
- **internal/tool/builtin（5 个内置工具，全部带 jsonschema）**：
  `system.time`（RFC3339 UTC）、`system.echo`（回显）、`system.calc`
  （四则运算，除零 `DIVIDE_BY_ZERO` 结构化错误）、`artifact.put`
  （写产物返回 `{artifact_id, uri, summary}`，**risk=high**）、
  `artifact.get`（按 id 读元数据+内容，risk=medium，缺失
  `ARTIFACT_NOT_FOUND`）。
- **internal/agent/kernel 适配与断点（ACL 保持：eino 仍只在 kernel）**：
  - `tools.go`：`AdaptTools` 把我们的 `tool.Tool` 适配为 eino
    `tool.BaseTool`（ParametersJSON → eino-contrib/jsonschema.Schema →
    `schema.ToolInfo`，执行委托执行器）；`ToolCallGuard` 契约
    （safety 位的内核自有接口，eino 类型不外泄）与中断原语
    （`InterruptToolCall`/`ToolCallWasInterrupted`/`ToolCallResumed[T]`，
    转发 eino `tool.Interrupt`/`GetInterruptState`/`GetResumeContext`）。
  - `checkpoint.go`：adk `CheckPointStore` 实现（Get/Set 委托
    internal/store，断点随 run_sessions 存——生命周期与 run 一致，
    无孤儿数据）。
  - `run.go`：新增 `EventInterrupted` 事件（携带全部根因中断点
    `{ID, Info}` 与中断前的部分轮次/用量；中断后不出现 EventDone）；
    `Runner.RunWithCheckpoint`（绑定断点 ID，经
    `adk.WithCheckPointID` 开启断点持久化）、`Runner.Resume`
    （`ResumeWithParams` 按中断点 ID 注入恢复数据）。
  - `middleware.go`：middleware 栈插入 safety 位（现栈序
    patchtoolcalls→agentsmd→**safety**→summarization→context→trace）；
    `safetyMiddleware` 只做 adk 端点签名 → `ToolCallGuard` 的转换，
    无策略。
  - `mock.go`：`SEMANTIC_MOCK_SCRIPT` 环境变量脚本（JSON 数组，
    支持 tool_calls）——集成测试与手动联调需要"模型按剧本调用
    工具"，而 mock 实例在运行时内部构建，env 是唯一穿透点；
    脚本非法时全部调用显性报错。
- **internal/security（safety middleware v1，09 §3-§7）**：
  `Middleware` 实现 `kernel.ToolCallGuard`，单次调用序列：
  **L2 参数校验**（jsonschema，引入
  `github.com/santhosh-tekuri/jsonschema/v6`——选型理由：纯 Go、
  覆盖 draft 2020-12、校验专用且维护活跃；eino-contrib/jsonschema
  只做生成不做校验，xeipuuv 系已停更）→ **L4 审批判断**
  （annotations.risk ∈ {high, critical} 或命名空间命中
  profile.interrupt.approval_required）→ 需审批时发起内核中断
  （负载 `ApprovalInfo{Tool,Namespace,Risk,Question,ArgsJSON}`
  自包含，gob 注册随 checkpoint 持久化）；恢复执行时按注入的
  bool 放行/返回结构化拒绝（`APPROVAL_REJECTED`，模型可读并如实
  告知）；未登记工具按拒绝兜底；放行/拦截/审批事件全落 INFO 日志。
  L3 规则引擎/L5 资源锁/L6 安全停止为 Phase 2（同一挂接点扩展）。
- **internal/interaction（交互服务，confirm 单类型，12 §3-§4）**：
  - `Request(ctx, {sessionID, agent, question, risk, runID,
    checkpointID, timeout}) (bool, error)`：先落 Pending 记录 → 经
    event bus 下行 envelope（channel=interaction，
    type=interaction.request，importance=critical，payload 按 12 文档
    confirm schema：`interaction_id/type/question/risk/timeout_ts`）→
    等待应答/超时（默认 300s，**on_timeout=reject**：状态迁移
    Expired 并返回拒绝）/ctx 取消（迁移 Cancelled）；
  - `Reply(ctx, interactionID, approved)`：WS 上行入口——存在性
    校验（`ErrNotFound`）、pending 校验（`ErrNotPending` 防重复
    应答）、**先落应答再唤醒等待方**；
  - `CancelSession`：会话删除/任务取消时批量 Pending→Cancelled
    并唤醒等待方（waiter 通道只发不关，规避 send-on-closed 竞态）；
  - 状态机：Pending→Answered/Expired/Cancelled（全部条件更新）。
- **internal/agent/runtime（run 状态机扩展）**：
  `running → awaiting_approval → running → done/failed`——中断事件
  到达时状态迁移 awaiting_approval（断点已由内核在事件上行前
  持久化），逐中断点经交互服务等待应答，全部收齐后恢复 running 并
  `Resume`（同 run 可多次中断，断点覆盖写、永远从最新暂停点恢复）；
  中断前后轮次/用量跨事件流累计；未知中断负载按拒绝兜底（安全
  默认不放行）。`Deps` 新增 `Registry/Executor/Interaction`
  （`Tools` 保留为测试逃生舱）；运行器按 profile
  `tools.namespaces` 过滤注册表后适配注入，safety middleware 用
  同一份清单 + profile `interrupt.approval_required` 构建；
  `EvictSession` 级联取消会话待应答交互。
- **internal/server/ws**：上行扩展 `interaction.reply`
  （`{type, interaction_id, approved}`，approved 指针类型区分缺省
  与显式拒绝）→ `InteractionReplyHandler`（接口倒置定义在 ws，
  interaction.Service 实现）；非法/过期应答回 errorReply
  （`INTERACTION_REPLY_FAILED`）。**chat.message 改为 goroutine
  异步分发**——run 阻塞等审批期间读泵保持可读，否则同连接的
  interaction.reply 永远到不了（审批死锁）；同会话串行语义仍由
  runtime 会话锁兜底。
- **internal/bootstrap**：`wire_tool.go` 装配注册表（5 内置工具）+
  执行器 + 交互服务，注入 runtime 与 ChatGateway；启动日志输出
  profile 的工具范围与审批名单。
- **go.mod**：新增 `github.com/santhosh-tekuri/jsonschema/v6 v6.0.2`。

## 关键设计

### 审批为什么不阻塞在 middleware 里（interrupt/resume 状态机）

安全 middleware 判定需审批后**不就地等待**，而是返回内核中断错误
（`tool.Interrupt`）：adk 把 run 暂停并把断点交给 CheckPointStore
持久化，随后才上行中断事件；runtime 消费到 EventInterrupted 才创建
交互记录并下行 interaction.request，应答后经 `ResumeWithParams` 携带
审批结果恢复，middleware 在重放的工具调用里读恢复数据决定放行/拒绝。
两条一致性顺序由此成立：

1. **先持久化 checkpoint 再下行请求**：断点落库发生在中断事件上行
   之前（adk runner 的顺序保证），请求下行更在其后（runtime 发起）——
   崩溃后请求与断点永不脱节；
2. **先落应答再恢复执行**：`Reply` 先把应答写入 interactions（条件
   更新）再唤醒等待方，runtime 被唤醒后才 Resume——恢复执行永远
   基于持久事实。

### 审批超时语义

`on_timeout=reject`：默认 300s 未应答则状态迁移 Expired、按拒绝
恢复执行（工具不执行，模型收到结构化拒绝结果并如实告知）。超时与
应答竞速时以数据库条件更新为准（先到者赢），败方读落库状态兜底。
会话删除/任务取消经 `CancelSession` 批量 Cancelled 并唤醒等待方，
run 按拒绝收尾而不是悬挂。

### 工具适配与 ACL

eino 依赖仍收敛在 kernel：`internal/security` 实现的是 kernel 自有
`ToolCallGuard` 契约（中断原语经 kernel 转发），runtime/bootstrap/
interaction 不见 eino 类型。工具名保留 `<命名空间>.<动作>` 原貌
（如 artifact.put）端到端透传——mock 与宽容端点可用；OpenAI 严格
端点的函数名约束（`^[a-zA-Z0-9_-]+$`）需要在适配层做名称映射，
见 TODO。

## 影响面

- **kernel Runner 扩展**：`RunWithCheckpoint`/`Resume`/`EventInterrupted`
  （`Run` 旧签名保留兼容）；BuildAgent 在 Store 非空时挂接
  CheckPointStore；middleware 栈固定插入 safety 位（`AgentConfig.
  Safety` 为 nil 时不挂，纯测试路径）。
- **runtime run 状态机扩展**：running→awaiting_approval→running→
  done/failed（`run_sessions.status` 新枚举值 + 中途迁移方法）；
  `Deps` 新增三依赖；`EvictSession` 级联取消交互。
- **WS 协议扩展**：新增上行 `interaction.reply` 与错误码
  `INTERACTION_REPLY_FAILED`；chat.message 改异步分发（同连接
  消息不再天然排队，同会话串行由 runtime 锁保证）。
- **store schema v4**：老库自动迁移（ALTER + 两张新表），无需手工
  干预；产物目录随库路径自动创建。
- **配置面**：leader role.yaml 的 `tools.namespaces: [system.*,
  artifact.*]` 与 `interrupt.approval_required: [artifact.*]` 即刻
  生效（artifact.put 本身 risk=high，双保险）。

## 测试清单与结果

- internal/store：artifact put/get/list/缺失、interaction 状态机
  （应答/防重复/超时/批量取消）、checkpoint 存取与状态迁移 ✅
- internal/tool：registry 注册/查重/命名空间过滤、executor 并发/
  命名空间串行/超时/错误格式/父 ctx 取消 ✅
- internal/tool/builtin：5 工具契约（jsonschema 合法性、risk 等级）
  与 Run 行为（calc 四则/除零、artifact put/get 闭环/缺省值/缺失）✅
- internal/agent/kernel：完整 interrupt→resume 链路（mock 脚本调
  report.save → 中断（断点落库）→ 批准恢复（工具执行+模型总结）/
  拒绝恢复（工具未执行，模型读到 APPROVAL_REJECTED））✅
- internal/interaction：请求-应答闭环（payload schema/先落库再唤醒）、
  超时按拒绝、重复应答拒绝、会话取消、ctx 取消 ✅
- internal/security：L2 拦截（缺必填/枚举越界/非法 JSON/未知字段/
  类型错误）、放行路径、未登记兜底、needsApproval 判定（risk high/
  critical/approval_required 命中）、高危中断 + 端到端审批门 ✅
- internal/agent/runtime：批准链路（awaiting_approval 状态/断点/
  工具执行/消息落库）、拒绝链路、EvictSession 取消 ✅
- internal/server/ws：interaction.reply 透传/缺参/服务拒绝/未装配、
  run 阻塞期间应答不死锁 ✅
- tests/integration/approval_test.go：真实装配 + mock 脚本——
  批准（interaction.request 下行 → 应答 → 工具执行 → message.done →
  产物写入 + 内容一致 + 交互已应答）、拒绝（工具未执行 + 无产物 +
  重复应答回 errorReply）✅
- `go test ./... -race` 全绿、`go vet ./...` 零告警、`gofmt -l .`
  无输出、`golangci-lint run` 零告警、`make build` 成功 ✅
- 手动实测：semantic-server（mock 驱动 + SEMANTIC_MOCK_SCRIPT）
  WS 链路批准/拒绝各一次 ✅（命令与输出见下）

```bash
SEMANTIC_LLM_DEFAULT=mock SEMANTIC_MOCK_SCRIPT='[{"tool_calls":[{"id":"c1","name":"artifact.put","arguments":"{\"content\":\"# 手动实测\\n批准路径。\",\"summary\":\"实测报告\"}"}]},{"content":"报告已存好（批准）。"}]' \
  .output/bin/semantic-server --config configs/semantic-server.yaml
# WS：login → 建会话 → 发"帮我把报告存起来" → 收 interaction.request
#   → 回 {"type":"interaction.reply","interaction_id":"int-...","approved":true}
#   → 收 message.delta + message.done("报告已存好（批准）。")
#   → .output/artifacts/ 出现产物文件，artifacts 表有记录
# 拒绝用例同上（approved:false）→ message.done 如实告知未存储，无产物
```

## TODO（后续里程碑）

- **真实端点的工具名映射**：OpenAI 严格端点不接受含 `.` 的函数名，
  需要在 kernel 适配层做 `artifact.put ↔ artifact__put` 双向映射
  （mock 与宽容端点不受影响）。
- **L3 规则引擎 / L5 资源锁 / L6 安全停止**：09 文档的剩余层，
  随技能体系与设备体系落地（同一 WrapToolCall 挂接点扩展）。
- **L2 物理约束**：速度/工作空间/负载上限随 device profile 落地
  （当前仅 jsonschema 结构校验）。
- **审批"本会话内记住批准"**：09 §7 的可选优化（profile 控制，
  critical 不适用），待批量审批场景出现。
- **REST 查询面**：artifacts/interactions 的只读 API（前端产物卡片
  与审批历史）随 Studio 里程碑落地。
- **断点跨版本兼容**：checkpoint 为 gob 编码，eino 升级或负载类型
  变更时旧断点可能不可恢复（恢复失败 = run 报错，可重发消息）；
  需要时再引入版本化负载。
