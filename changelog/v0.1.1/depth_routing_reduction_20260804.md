# R8 depth_models 轮次路由 + 上下文瘦身（reduction）

日期：2026-08-04
里程碑：v0.1.1 / R8（推理深度路由与上下文压缩）
分支：feature/settings-system

## 为什么做这次变更

profile.depth_models 自 M2.1 起已解析透传但无人消费——所有轮次都用主
模型推理，复杂规划轮与工具失败恢复轮得不到深度模型，简单轮次又无法
省掉深度成本（架构文档 07 §6：D3 轮次应是少数，这是速度与成本的主要
杠杆）。同时上下文管线的 S8 超限处置（10 §4：大结果外置、引用替代）
未落地，单条超大工具结果会原样挤占全部后续轮次的上下文。本 MR 落地
两件事：kernel WrapModel 轮次深度路由（D1↔D3），与 eino reduction
middleware 的挂接（截断外置 store）。

## 包含内容

### 1. kernel 深度路由（`internal/agent/kernel/depth.go`，新）

- `Depth`（D1/D3，`String()` 即 depth_models 的键）与 `DepthEvent`
  （None/ToolFailed）。
- **判定规则 v1**（纯函数 `ClassifyDepth(turnIndex, lastEvent, input)`，
  显式可单测）：
  1. 上一轮工具调用失败 → D3（表外异常升 D3，优先级最高）；
  2. run 首轮且输入含规划意图 → D3（长消息 >200 runes，或含
     "计划/规划/步骤/怎么/如何" 关键词）；
  3. 其余 → D1。
  为什么用显式规则：路由行为必须可单测、可复盘——每轮的深度选择都能
  从日志与计量直接读出原因；规则只收拢在本函数，演进时单点修改。
- `classifyModelInput` 从模型输入消息推导三个判定输入：turn = 最后一条
  user 之后的 assistant 数（run 内已完成轮次）；event = 输入尾部连续
  tool 消息组任一为结构化错误结果（`{"ok":false...` 信封，`internal/tool`
  新增 `IsErrorResult` 前缀判定）；userText = 最后一条 user 消息文本。
  **为什么从输入推导而不在 middleware 记状态**：middleware 实例被该
  Agent 的全部并发 Run 共享，按消息形态推导天然无状态、并发安全，
  断点恢复重建输入后语义一致；且只认"尾部"工具组——更早轮次的失败
  模型可能已自行恢复，不影响本轮。
- `depthMiddleware`（栈位 4：safety 后、summarization 前）的 WrapModel
  按 ClassifyDepth 在 D1 主模型链路与 D3 模型间切换；切到 D3 打 INFO
  （turn/depth/from→to），D1 默认路径不打。
  **为什么在 WrapModel 路由**：深度选择是"用哪个模型"的决策，WrapModel
  是唯一能替换模型实例的钩子；位置在压缩/组装之前（外层），两个深度
  的模型共享同一份上下文管线，差异只在模型实例。
- 构建期语义（`buildDepthMiddleware`）：只消费 `depth_models["D3"]`——
  未知深度档 key 报错（配置必有消费方）；缺 resolver / 端点解析失败
  报错（配置错误构建期暴露）；**D3 解析结果与主模型同名则不挂接**
  （无 key 环境 resolver 回退默认端点、与主模型同源，切换无意义，
  保证无 key 开发/测试形态行为与未配置一致）。

### 2. 计量归因（`internal/agent/kernel/agent.go`）

- trace middleware 在栈内位于 depth 之后（内层），只包裹主模型链路；
  D3 模型由 depth 构建时用**同一组 trace 处理器**另行包裹
  （`callbackModel.modelName` 新增字段：非空时在 CallbackOutput.Config.Model
  标注实际模型名）。TraceHandler 优先读 Config.Model——D3 轮计量记
  D3 模型名，主模型轮仍走 TraceOptions.Model 兜底（行为不变）。
- AgentConfig 新增 `DepthModels map[string]string` 与
  `ModelResolver`（按端点名解析模型实例+计量归因名，runtime 注入基于
  registry 的实现）。
- 已知限制：计量成本估算沿用主模型单价（TraceHandler 单价随 runner
  固定），D3 轮的 cost_estimate 按主模型价格计——价格分档在后续 MR
  随计量体系演进，TODO 见下。

### 3. reduction middleware 挂接（`internal/agent/kernel/reduction.go`，新）

- 栈位 6（summarization 后）：eino `adk/middlewares/reduction`。
  `MaxLengthForTrunc=20000`（单条工具结果字符阈值）、
  `MaxTokensForClear=limits.context_tokens 的 60%`（与 summarization 同一
  预算比例）、`ReadFileToolName=artifact_get`。无 Store 不挂接（外置
  依赖存储，无观测形态保留全量结果）。
- **外置形态（store 适配）**：`reductionBackend` 实现 eino 的单方法
  Backend 接口（`Write`），全文经 store 的 artifact 机制持久化
  （`PutArtifactWithID`，store 新方法；元数据 `source=reduction` 标记
  来源、摘要取内容前 80 runes）。外置路径直接用产物 ID
  （`art-<uuid>`，`genArtifactOffloadPath`）——截断提示把路径原样展示
  给模型，**路径即引用**，模型用存量 artifact.get 按 ID 取回全文，
  无需另建路径映射（05 §9 "tool result 只返回引用"的产物语义）。
- 已知限制：eino 截断提示模板按 read_file 风格引导 offset/limit 参数，
  artifact.get 只收 artifact_id——多传的参数会被 L2 校验结构化拒绝，
  模型可自修正（成本一轮往返）；定制提示模板留给后续。

### 4. runtime 接线（`internal/agent/runtime/`）

- `resolveNamedEntry(name)` 抽出端点解析+无密钥回退（resolveModelEntry
  委托之）；`modelResolver` 作为 kernel ModelResolver 注入——与主模型
  共享回退语义，返回模型实例与计量归因名（entry.Model）。
- runtimeFor（chat BuildAgent）与 startObserver（BuildLoop）统一传入
  `DepthModels: prof.DepthModels` + `ModelResolver`——monitor/query 的
  role.yaml 不配 depth_models，空映射自然走主模型（无需特判）。
- `configs/agents/leader/role.yaml` 增加示例
  `depth_models: {D3: deepseek-reasoner}`（注释写明回退语义；无 key
  环境回退默认端点 → 与主模型同源 → 不路由，存量测试/开发形态不变）。

### 5. 文档与配置

- `docs/architecture/10-context-knowledge-memory.md` §4 预算表新增
  "当前实现状态"列（已落地/部分/未落地），结构不变；段占比注明为
  规划值（当前仅总预算可配）。

## 影响面

- `internal/agent/kernel`：depth.go/reduction.go 新文件；AgentConfig
  +DepthModels/+ModelResolver；callbackModel +modelName 标注；middleware
  栈 6 → 8 位（depth 位 4、reduction 位 6），栈注释更新。
- `internal/store`：`PutArtifactWithID`（PutArtifact 委托之，行为不变）。
- `internal/tool`：`IsErrorResult`（信封前缀判定，供深度路由识别工具失败）。
- `internal/agent/runtime`：resolveNamedEntry 抽取 + modelResolver 注入，
  两处 AgentConfig 装配 +2 字段。
- `configs/agents/leader/role.yaml`：+depth_models 示例。
- 无新增第三方依赖；无 store schema 变更（复用 artifacts 表）；无配置
  项变更（depth_models 为 profile 层既有字段的消费）。

## 测试内容与标准

- 单测（-race）：
  - 规则全分支（ClassifyDepth：长消息/五个关键词/阈值边界/工具失败
    优先/后续轮不升级）与 classifyModelInput 推导（首轮/工具组失败/
    更早轮次失败不波及/system 头不影响）；
  - WrapModel 路由（长→D3、短→主、失败轮→D3、未配置原样返回）与
    构建期回退（同名不路由/未知档报错/缺 resolver 报错/解析失败报错）；
  - 计量归因端到端（TestDepthRoutingMetering：长消息轮记 mock-d3-model、
    短消息轮记 mock-d1-model、两模型各服务一轮）；
  - reduction（TestReductionTruncatesAndOffloads：30002 字符结果被截断、
    模型输入不含全文、提示含产物 ID 与 artifact_get、按 ID 从 store
    取回全文与元数据；小结果原样通过且无外置产物）。
- 集成测试 `tests/integration/depth_routing_test.go`：自建临时 leader
  profile（mock-d1 主、D3→mock-d3，双 mock 端点以模型 ID 区分）→
  首轮 201 runes 长消息 → 计量 mock-d3-model（归因 leader）→ 短消息
  → 计量 mock-d1-model → 全程恰 2 条计量。为什么用计量做切换断言：
  mock 实例在 runtime 内部构建拿不到调用计数，计量是切换后模型名的
  唯一持久化证据。存量集成测试不受影响（共享 leader 的 D3 在无 key
  环境回退同源 → 不路由）。
- 门禁：gofmt 无输出、go vet 通过、golangci-lint 零告警、
  go test ./... -race 全绿、make build 成功；手动实测见下节。

## 手动实测

- 深度切换日志：临时 Go 程序经 kernel.BuildAgent 装配 mock-d1/mock-d3
  双模型 → INFO 日志输出 "深度路由：本轮切换到 D3 模型"（turn=0、
  depth=D3、from=mock-d1-model、to=mock-d3-model）；短消息轮无日志、
  主模型应答。
- reduction 截断：同一程序脚本化 mock 调用返回 30002 字符的工具 →
  第二轮模型输入为截断提示（含 art- 产物 ID 与 artifact_get 引导），
  store.GetArtifact 按 ID 取回全文一致。
- 集成测试 `go test ./tests/integration -run TestDepthRouting -v` 通过。

## 遗留 TODO

- 计量成本估算按主模型单价（D3 轮 cost_estimate 未按 D3 端点价格计）——
  随计量体系的价格分档演进处理。
- eino 截断提示的 read_file 风格文案（offset/limit）与 artifact.get 的
  artifact_id 参数形态不匹配——定制 TruncHandler 提示模板留给后续。
- D0/D2 档与 profile 深度档占比（07 §6 效果目标：D3 轮次 ≤ 3/20）
  的运行数据统计未落地——规则 v1 先用显式启发式，数据回收后演进。
