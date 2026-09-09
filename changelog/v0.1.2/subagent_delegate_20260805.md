# R12 SubAgent 委派机制（注册表 + agent-as-tool + 事件冒泡）

日期：2026-08-05
里程碑：v0.1.2 / R12（SubAgent 委派）
分支：feature/subagent-collab

## 为什么做这次变更

R4 组建的 Team 里 query-1 只登记 roster 待命（TODO(phase-m2.5)），leader
无法把查询类工作分派出去——所有问题都由 leader 单 agent 回答，角色分工
停在纸面。按架构文档 04 §7.3（委派双机制：短调用走 agent-as-tool，长任务
走任务系统），本 MR 落地 agent-as-tool 委派链路：leader 像调工具一样调用
query（同步等结果），委派过程经事件冒泡实时下行，结果摘要沉淀黑板。
严格最小必要实现：单层委派（depth ≤1）、上下文隔离默认、结果摘要回传、
并发不做信号量（leader 串行 ReAct 天然 ≤1）、超时复用工具
annotation.Timeout 语义。

## 包含内容

### 1. profile subagent 段（`internal/agent/profile` + `configs/agents/query/role.yaml`）

- role.yaml 新增 `subagent: {enabled, tool_name, tool_description, max_turns}`：
  角色作为委派目标的对外呈现。加载期校验：**仅 service 模式可启用**
  （委派是请求-响应短调用语义，coordinator/observer/worker 的装配形态与
  "被 leader 同步调用出结果"不兼容）；tool_name 必填且满足端点函数名约束
  `[a-zA-Z0-9_-]+`；tool_description 必填（leader 模型判断委派时机的唯一
  依据，空描述 = 委派能力对模型不可发现）。max_turns ≤0 沿用
  limits.max_turns。
- `configs/agents/query/role.yaml` 启用：`tool_name: ask_query`，描述
  "系统状态与产物查询助手"，max_turns 缺省沿用 limits（6，短调用从紧）。
- 单测：字段透传 + 四类非法（非 service 启用/缺 tool_name/缺描述/非法字符）。

### 2. `internal/agent/subagent`（新包：注册表）

- `registry.go`：`Def{ID, Role, Description, ToolName,
  ToolDescription, Namespaces, MaxTurns}`（可委派成员定义，从 Team 成员
  声明 + role profile 派生）；`Registry`（List 按 ID 升序 / Get）；无启用
  成员返回空目录（非 nil，调用方无需判空）。
- `ToolDefinition(def)`：委派工具在安全管线的契约登记——命名空间按
  tool_name 归属（approval_required 模式按它匹配）、risk=low（SubAgent
  内部工具调用的安全判定由其自身门禁负责）、Timeout=DelegationTimeout
  （2 分钟，annotation.Timeout 语义）。**为什么必须登记**：安全门禁对
  未登记工具 fail-closed（UNKNOWN_TOOL 拒绝），不入册委派会被整体拒执行。
- 单测：派生（启用入册/未启用跳过/max_turns 缺省沿用与显式覆盖）、
  非 service 启用经 profile 加载期报错、ToolDefinition 契约。

### 3. kernel agent-as-tool 适配（`internal/agent/kernel/agent_tool.go`）

- `BuildAgentTool(ctx, cfg, def)`：用 `adk.NewAgentTool` 把按 cfg 构建的
  SubAgent 包装为委派工具，外加内核适配层：
  - **{task} 入参契约**：adk agent-tool 默认入参是 {request}，而自定义
    inputSchema 时 eino 会把原始 JSON 文本直接作为子 agent 的用户消息
    （不解析字段）。适配层对外呈现 `{task: string}`（task 必填）、内部
    转换为 {request}——子 agent 收到的是干净的任务文本（单测断言）。
  - **嵌套防御（depth ≤1）**：cfg.Tools 含委派工具时构建期直接报错
    （标记接口 agentToolMarker，非导出方法外部无法伪造）。
  - **超时强制**：def.Timeout（annotation.Timeout 语义）经
    goroutine+select 强制（与工具执行器同语义，不尊重 ctx 的实现也拖不住
    调用方）；超时归一为结构化 TIMEOUT 结果。
  - **失败归一**：委派执行失败转结构化 TOOL_ERROR 结果（不拖垮 leader
    的 run）；中断错误（含子 agent 审批的 CompositeInterrupt）原样上抛
    （内核靠错误类型识别中断，转换会破坏跨 agent 边界的审批恢复链路）。
- **事件冒泡**：父 agent 工具集含委派工具时 buildChatAgent 自动开启
  `ToolsConfig.EmitInternalEvents`——adk 把父 flow 的事件 generator 作为
  tool option 传给委派工具，子 agent 事件随之转发进父事件流（eino
  agent_tool.go/chatmodel.go 源码确认：流 Copy 扇出，父流收到独立拷贝；
  冒泡事件不进父 run 的 runSession；Exit/Transfer 等 action 不越界）。
  该开关只被 agent-tool 消费，故按内容自动开启而不增配置面。
- 单测：Info schema（task 必填 string）、嵌套防御、{task} 解析与非法
  入参归一、子 agent 输入为任务文本。

### 4. kernel 事件层冒泡转换（`internal/agent/kernel/run.go`）

- 新事件类别：`EventSubAgentDelta`（AgentID + 文本增量）、
  `EventSubAgentResult`（AgentID + Task + 结果全文）。
- pump 转换：冒泡事件按 `AgentName ≠ 父 agent 名` 识别（eino flow 按来源
  agent 标注）——助手文本逐帧转 EventSubAgentDelta；其余（子 agent 内部
  工具调用/结果）排空不下行（下行面只呈现产出文本，执行细节留给 trace）。
  **轮次/用量不归并父 run**：SubAgent 有自己的 trace 链路（TraceHandler
  以成员实例 ID 归因），父 run 只统计本级模型调用，与 eino"冒泡事件不进
  父 runSession"的契约一致。
- 结果归因：委派工具的结果事件（工具名命中 runner 的 工具名→SubAgent
  映射）转 EventSubAgentResult，结果全文 = SubAgent 最终回复（即 leader
  看到的工具结果）；**任务文本配对**：父 run 助手消息的 tool call 经
  schema.ConcatMessages 重组（流式 tool call 分帧到达），call_id → 任务
  文本入表，结果事件按 ToolCallID 回填。重组/解析失败降级为空任务文本
  （摘要素材缺失不阻断 run）；冒泡流读取失败不上报错误事件（旁路拷贝
  断裂不代表委派失败，真实结果仍经工具结果路径到达）。
- 单测（TestDelegationEventFlow）：leader 脚本（轮 1 调 ask_query、轮 2
  总结）+ query 脚本驱动全链路——冒泡 delta 序列拼出 query 回复全文、
  result 事件 task/text/归因正确、父 run 轮次 = 2（子 agent 轮次不归并）。

### 5. mock 路由脚本（`internal/agent/kernel/mock.go`）

- `SEMANTIC_MOCK_SCRIPT` 新增对象形态 `{"scripts":[{match, replies}]}`：
  多 agent 场景（leader/query/monitor 各自持有独立 mock 实例）实例在首次
  调用按输入消息内容子串选定剧本并锁定（角色系统提示的身份标识是稳定
  匹配素材）；按声明顺序首个命中生效，空 match 兜底，无命中回退默认响应。
  数组形态行为不变（存量测试原样通过）。
- 为什么扩展 mock：委派链路涉及多个模型实例且共享同一 env 穿透点，
  单一数组脚本无法表达"leader 调工具、query 回答"两个剧本。

### 6. runtime 接线（`internal/agent/runtime/`）

- `AssembleTeam`：组建时经 `subagent.NewRegistry(def, profiles)` 派生
  SubAgent 注册表存于 Service；service 成员 roster 活动改为"待命：可经
  ask_query 委派"（M2.5 TODO 移除）。
- `buildSubAgentTools`（leader 会话装配时调用）：逐 SubAgent 装配
  AgentConfig（profile 驱动 model/namespaces/max_turns + 共享基础设施
  store/trace/logger/blackboard，Purpose=subagent）→ BuildAgentTool；
  **Name 用成员实例 ID（query-1）**：冒泡事件按 AgentName 归因，roster/
  下行事件都以实例 ID 为身份基准，三者对齐免翻译。返回内核工具 +
  安全契约（并入 leader 门禁清单，buildToolchain 增加 extraSafetyDefs
  通道——契约进门禁但不进 AdaptTools，执行路径是 agent-as-tool 而非
  执行器）。
- 事件转换：run 消费循环把 EventSubAgentDelta/Result 转 envelope
  （type=subagent.delta/subagent.result、channel=dialogue、agent=
  {id: query-1, role: service}）经 bus 下行（复用现有聚合器，与 leader
  的 message.* 同通道，前端单视图呈现委派过程，04 §7.4）。
- **黑板写入**：委派完成（subagent.result）时 `WriteDelegation(author=
  leader 名, summary=委派 query-1：任务前 80 字 → 结果前 80 字)`——R7 的
  delegation 条目类型自此启用（黑板承载结果摘要而非执行细节，10 §8）。
- 单测（TestLeaderDelegation）：ask_query 执行（query 模型收到干净任务
  文本；safety 真实挂接——门禁清单只有委派工具契约，L2 校验过检放行）、
  subagent.* 下行归因、黑板 delegation 条目、message.done 总结。

### 7. 集成测试 `tests/integration/subagent_test.go`

真实 configs/agents 起 App（Team 三成员在线）→ 路由形态 mock 脚本
（leader 命中"团队指挥官"：轮 1 调 ask_query("查 artifact 列表")、轮 2
总结；query 命中"系统与产物查询助手"：回答清单）→ WS 发消息 → 断言：
归因 query-1 的 subagent.delta/subagent.result 下行事件（task/text 正确）、
leader 总结经 message.done 收尾、黑板 1 条 delegation 条目（author=leader、
摘要含任务→结果）。

## 影响面

- `internal/agent/profile`：SubAgentConfig + 加载期校验（存量 role.yaml
  无 subagent 段，零值不启用，行为不变）。
- `internal/agent/subagent`：新包（doc/registry + 单测）。
- `internal/agent/kernel`：新增 agent_tool.go；agent.go 自动开启
  EmitInternalEvents + Runner 携带工具名映射；run.go 新事件类别与冒泡
  转换（无委派工具时 subAgentTools=nil，全部走既有路径零开销——存量
  单测原样通过）；mock.go 路由脚本扩展（数组形态不变）。
- `internal/agent/runtime`：Service +subAgents；team.go 注册表派生 +
  buildSubAgentTools；service.go 事件转换 + writeDelegation + buildToolchain
  增 extraSafetyDefs 通道（observer 调用方不传，行为不变）。
- `configs/agents/query/role.yaml`：启用 subagent。
- 无新增第三方依赖；无新增配置项（DelegationTimeout 为常量 2m）。

## 测试内容与标准

- 单测（-race）：profile 校验、注册表派生、kernel 委派工具（构建/schema/
  嵌套防御/执行/归一）、kernel 冒泡转换全链路、mock 路由、runtime 委派
  链路与 roster 描述。
- 集成测试：上节⑦；存量 team/chat/blackboard/approval 集成测试原样通过
  （startTeamApp 签名增 wsBase 返回值，调用处同步更新）。
- 门禁：gofmt 无输出、go vet 通过、golangci-lint 零告警、
  go test ./... -race 全绿、make build 成功；手动实测见下节。

## 手动实测

启动 server（DEBUG 日志）→ WS 发送"查一下有哪些产物"（mock 路由脚本
同上）→ 日志依次出现"SubAgent 委派工具已装配"、"委派完成
（subagent=query-1）"、"黑板条目已写入 type=delegation"→ 下行事件序列：
query-1 的 subagent.delta/result → leader 的 message.done → 库内
blackboard_entries 可见 delegation 条目；trace_spans 可见 query-1 归因的
委派 run 链路（Purpose=subagent）。
