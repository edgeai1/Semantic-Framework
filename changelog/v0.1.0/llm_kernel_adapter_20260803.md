# B3（M1.3 LLM 与 kernel 适配）：eino 引入、注册表、内核适配层与观测落库

日期：2026-08-03
里程碑：Phase 1 / B3（M1.3 LLM 与 kernel 适配）——eino 首次引入

## 为什么做这次变更

框架不自建大模型内核（架构文档 01 §3）：LLM 接入与 Agent 执行复用 eino
生态，自研价值集中在领域层。本里程碑把 eino 首次引入仓库并锁定分层：
`pkg/llm` 只做注册表与计量类型（零内核依赖），`internal/agent/kernel`
作为**唯一 import eino/eino-ext 的包**完成模型构建、mock、最小 agent
编排与 trace/metering 落库，证明 agent loop（模型 ↔ 工具 ReAct 循环）
在本仓端到端可用。角色 profile、委派与完整 middleware 栈是 B4 的事。

## eino 版本选择与接口确认

- **`github.com/cloudwego/eino v0.9.13`**（2026-08-03 查询的最新 stable；
  v0.10.0 系列当时只有 alpha，不选）。`go 1.18` 指令，与本仓
  `go 1.23` 兼容，go 指令保持 1.23 不变。
- **`github.com/cloudwego/eino-ext/components/model/openai v0.1.13`**
  （最新 release；其 go.mod 声明 eino v0.7.13 为最低版本，MVS 解析到
  v0.9.13 编译验证通过）。传递引入 `eino-ext/libs/acl/openai v0.1.17`。
- 接口确认（读 v0.9.13 源码核实）：
  - `model.BaseChatModel = BaseModel[*schema.Message]`：`Generate` + `Stream`；
  - 工具经 `model.WithTools` 调用期 option 传入（adk react 流 line 1482），
    模型实现自行从 options 读取，mock 无需实现 `WithTools`；
  - `adk.NewChatModelAgent` + `adk.NewRunner`（`RunnerConfig{Agent,
    EnableStreaming}`），`Runner.Query/Run` 返回 `AsyncIterator[*AgentEvent]`，
    事件取 `Output.MessageOutput.GetMessage()`；
  - `callbacks.Handler` 五时机（OnStart/OnEnd/OnError/两个流式），
    同一处理器的 ctx 按时机链式传递（OnStart 存值、OnEnd 取值）；
  - 计量提取：非流式 `model.CallbackOutput.TokenUsage`；流式回调帧
    **usage 只在末帧**（Message 可为 nil，OpenAI 兼容端点约定，
    eino-ext acl 源码注释确认）；
  - 回调触发是**模型实现的职责**（eino-ext acl 自行调用
    `callbacks.On*`，框架只负责 ctx 中的管理器）——因此 kernel 的
    mock 与 callbackModel 装饰器都显式触发回调。

## 包含内容

- **go.mod 依赖**：新增 `eino v0.9.13`、`eino-ext/components/model/openai
  v0.1.13`（直接依赖），`google/uuid v1.6.0` 转直接依赖（trace_id 生成）；
  `go mod tidy` 干净。
- **pkg/config**：`Config` 新增 `llm` 段（`default` + `providers` map，
  字段按 02 §5 schema：component/base_url/model/capabilities/options/
  price）；`Default()` 与 `configs/semantic-server.yaml` 同步增加
  deepseek-chat、deepseek-reasoner、mock 三端点。注意 yaml 叠加是
  **map 合并语义**（出现的 key 覆盖、未出现保留默认），与标量字段
  "未出现保留默认值"一致，测试已锁定该行为。
- **pkg/llm**（无 eino import）：`Provider`/`Registry`（`Load` 校验
  default 在清单内、component/model 必填；`Get` 缺名报错；`Default`；
  `Names` 升序；`APIKey` 读 `SEMANTIC_LLM_API_KEY_<名称大写,'-'→'_'>`，
  同 base_url 条目共享并按 base_url 缓存；`APIKeyEnv` 导出命名规则供
  doctor/factory 复用）；`Usage`/`MeteringRecord`/`EstimateCost`
  （按每 1K tokens 单价）。
- **internal/store**：迁移 v2 建 `trace_spans`（trace_id 索引）与
  `metering`（model/agent + created_at 索引）；`InsertSpan`/
  `QuerySpans(traceID)`、`InsertMetering`/`QueryMetering(filter)`——
  filter 支持 model/agent/时间范围/Limit（默认 500），供后续 API。
- **internal/agent/kernel**（eino 唯一入口）：
  - `factory.go`：`NewChatModel(ctx, entry, apiKey)`——openai 驱动
    构建 eino-ext 客户端（BaseURL/APIKey/Model + options 严格映射
    temperature/max_tokens，未识别键报错不静默忽略）；mock 驱动返回
    本包 mock；未知驱动报错。`RequiresAPIKey` 供 bootstrap/doctor 做
    key 检查。签名相对任务书增加 ctx 与 apiKey：eino 客户端构建需要
    ctx，密钥只经环境变量注入不进配置结构（Registry.APIKey 取得）。
  - `mock.go`：`MockChatModel`（实现 `model.BaseChatModel`）——预设
    文本/工具调用/流式分块/错误/调用计数 + 脚本化多轮（`SetScript`，
    耗尽重复最后一条，建模 agent 多轮调用）；固定 usage 10/20/30
    （每次调用独立副本）；**与真实模型对齐主动触发 eino 回调**，
    否则 TraceHandler 对 mock 不可见，冒烟就验证不到回调→落库链路。
  - `agent.go`：`BuildAgent(ctx, AgentConfig{Name, Instruction, Model,
    Tools, Callbacks})`——`adk.NewChatModelAgent` + `adk.NewRunner`
    最小编排。Callbacks 经 `WrapModel` middleware（kernel 挂接 trace
    的位置，01 §3）固化为 `callbackModel` 装饰器：**直接调用处理器**
    而非 ctx 注入——eino 的 ctx 回调合并 API（`AppendHandlers`）不导出，
    `InitCallbacks` 会覆盖运行期管理器；装饰器与"模型实现自行触发
    回调"的职责对齐，不受运行期管理器有无影响，也不与
    `adk.WithCallbacks` 互相覆盖。流式时机按框架惯例在 goroutine 中
    消费流拷贝（`Copy` 扇出，每处理器独立拷贝）。
  - `trace.go`：`TraceHandler` 实现 `callbacks.Handler`（含
    `TimingChecker`，跳过流式输入时机）：OnStart 存开始时间（ctx），
    OnEnd/OnEndWithStreamOutput 写 `trace_spans`（名称/类型/起止/耗时/
    属性 JSON）+ 模型调用的 `metering`（usage + 按端点 price 估算成本），
    OnError 写带错误属性的跨度；落库失败只 WARN 不打断业务调用。
- **bootstrap 接线**：`Wire` 在 auth 种子后加载 `llm.Registry`——配置
  错误（如 default 不在清单）直接失败；逐端点 key 检查，缺 key 只
  WARN 降级（该模型暂不可用、仅 mock 可用），不阻塞启动；注册表加载
  结果 INFO 落日志。`App` 新增 `LLMRegistry()`/`Store()` 访问器。
- **doctor**：删除硬编码单 key 检查，改为按 providers 逐项输出
  ✓/!（mock 无需密钥记 ✓；缺 key 记 ! 不阻塞；注册表非法记 ✗）。

## 影响面

- 新增直接依赖 eino/eino-ext（生产代码仅 kernel 包引用，ACL 不破）；
  cmd/semantic 与 bootstrap 经 kernel 的 `RequiresAPIKey` 传递依赖 eino
  （仅类型引用，无二进制行为变化）。
- `configs/semantic-server.yaml` 与 `pkg/config.Default()` 增加 llm 段
  （唯一事实源的两个视图，同步修改）。
- store schema 升级到 v2：旧库启动自动迁移（逐迁移事务，幂等）。
- 集成测试作为独立验证入口直接引用内核类型（文件头已注明：不参与
  生产依赖图，生产代码 eino 仍收敛 kernel）。

## 测试清单与通过情况

- `pkg/config`：默认值含 llm 三端点、yaml 覆盖（map 合并语义）——通过。
- `pkg/llm`：注册表加载/default/四类非法配置/缺名报错/key 命名规则/
  本端点读取/同 base_url 共享/未设置返回空/Names 升序/成本估算
  （含零值）——通过。
- `internal/store`：跨度写入与按链路查询、计量多条件过滤（model/
  agent/时间范围/Limit）、v2 迁移幂等——通过。
- `internal/agent/kernel`：factory（openai 只构造不请求/缺 key 提示
  环境变量/缺 base_url/mock/未知驱动/未知 option/类型错误）；mock
  （文本/工具调用/脚本顺序与耗尽重复/错误预设与清除/流式分块 usage
  末帧/单帧整段）；TraceHandler（模型调用跨度+计量、错误跨度、流式
  末帧 usage、Needed 时机声明）；**agent 冒烟**（mock 脚本一轮工具
  调用+一轮文本，断言工具执行 1 次、最终文本、模型调用 2 次、
  trace_spans 2 条、metering 2 条共 60 tokens）——全部通过（-race）。
- `tests/integration`：`TestLLMKernelAgentRound`（bootstrap 装配 →
  注册表 → factory → BuildAgent → 一轮含工具调用对话 → 断言响应文本/
  工具执行/trace_spans/metering 落库）——通过（-race）；
  `TestDeepSeekSmoke`（真模型流式联调）——**本环境未设置
  `SEMANTIC_LLM_API_KEY_DEEPSEEK_CHAT`，按设计 t.Skip 未执行**；
  有 key 时运行：流式 generate "用一句话介绍你自己"，断言响应非空 +
  metering 1 条 usage>0。
- 收尾验证：`gofmt -l .` 无输出、`go vet ./...` 干净、
  `golangci-lint run` 干净、`go test ./... -race` 全绿、
  `make build` 成功（见下方实测记录）。
- `semantic doctor` 实测输出：配置 ✓、两端口 ✓、deepseek-chat/
  deepseek-reasoner 密钥 !（未设置，不阻塞）、mock ✓，退出码 0。

## TODO（后续里程碑）

- B4：角色 profile 驱动的 agent 构建（role.yaml model 字段消费
  capabilities 匹配）、完整 middleware 栈（safety/context）、
  interrupt/resume、流式 chat 下行（EnableStreaming + WS）。
- B4：metering 的 role/purpose 归因接入角色与任务上下文（当前
  TraceOptions 手工传入）；trace_spans 的 parent_id 嵌套（当前扁平）。
- 文档：`docs/developer/06-configuration-reference.md`（02 §5 引用的
  配置参考）随配置面扩大后补齐。
- 真模型联调：`TestDeepSeekSmoke` 待有 key 环境执行一次并记录结果。
