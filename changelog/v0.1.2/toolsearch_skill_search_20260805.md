# R10 技能目录工具（skill.search/list）+ ToolSearch 动态检索挂接

日期：2026-08-05
里程碑：v0.1.2 / R10（技能系统 × 工具体系）
分支：feature/skill-system

## 为什么做这次变更

R9 落地了技能系统的被动披露面（清单摘要常驻 skill 工具描述、命中加载
正文），但模型无法编程式枚举/检索技能目录——清单是工具描述里的文本，
不是结构化数据。同时架构文档 05 §6 的工具三级选择策略里，"动态检索 +
常用置顶"两级一直是解析透传未消费（profile 的 tools.pinned/tool_search
字段空转）。本 MR 一次补齐两块：技能目录的主动查询工具（skill.list/
skill.search，06 §5 的兜底路径），与 ToolSearch 动态检索的真实挂接
（eino v0.9.13 `adk/middlewares/dynamictool/toolsearch`），为大工具库
（MCP 来源接入前夜）把检索链路打通。严格最小必要：检索就是 eino 的
关键词匹配，不写自研评分器；阈值做成常量不做配置项（无消费方不预留）。

## 包含内容

### 1. 技能目录工具 `internal/tool/builtin/skill.go`（新）

- `skill.list`（无参数）：按 category 分组返回全部技能的
  name/category/description（分组与组内均名升序，渲染稳定），结果
  `{total, categories:[{category, skills:[...]}]}`。
- `skill.search`（`query` 必填）：name/description/when_to_use/body 四
  字段大小写不敏感子串匹配，返回 `{query, total, matches}`——matches
  前 5 条（名升序截取，与 eino toolsearch 默认 max_results 取齐），每条
  含 name/category/description/matched_fields；total 是全量命中数
  （模型据此知道截断外还有命中，可换关键词再查）。**when_to_use 经本
  工具首次消费**（R9 只透传）——适用时机描述正是检索的高信号字段。
- 注解 risk=low + 幂等；命名空间 `skill.*`（profile 授权维度）；注册进
  builtin 注册表（`RegisterAll` 第三参 `SkillSource` 接口，实现方
  internal/skill.Store）。
- 降级契约：nil 源（启动时技能目录不可用）工具仍在册，调用返回结构化
  `SKILL_UNAVAILABLE`——装配形态不随技能目录可用性变化，降级面显式
  可见而不是静默缺席；空技能集是正常答案（list 空分组/search 零命中）
  不是错误。参数缺失/空白返回 `EMPTY_QUERY`，非法 JSON 返回
  `BAD_ARGUMENTS`。
- 单测：list 分组（跨类排序/字段完整/空集）、search 命中（大小写不敏感/
  四字段 matched_fields/多命中名升序）、无命中、query 缺失与非法、
  nil 源降级、契约（命名空间/风险/幂等）；存量契约测试 5→7 工具。

### 2. kernel ToolSearch middleware 挂接（`internal/agent/kernel/`）

- `toolsearch.go`（新）：`buildToolSearchMiddleware`——`cfg.ToolSearch`
  为 false 或 `DynamicTools` 为空不挂接（返回 nil, nil）；挂接即用 eino
  toolsearch 默认 client 侧检索模式（UseModelToolSearch 未启用：模型
  端点无原生检索能力，且该模式把工具清单交给服务端检索，行为不可在
  框架侧观测）。
- **机制**（读 eino 源码确认）：middleware BeforeAgent 把 tool_search
  元工具 + 动态集追加进 tools 节点（可执行面）；首轮模型调用前把动态集
  从 ToolInfos 剥离（可见面）；模型调 tool_search（关键词或
  `select:<名>` 直选）后，命中清单以工具结果消息留在历史里，
  middleware 每轮重放扫描、把命中工具追加回后续轮次的 ToolInfos——
  **同一 run 内多次检索累积**，新 run 重新隐藏。
- **检索累积对前缀缓存的影响**（阈值的 why）：client 侧检索模式下
  ToolInfos 在 run 内会变（首轮剥离 + 检索后追加），工具清单是多数厂商
  请求前缀的一部分，每次检索命中都可能使后续轮次前缀缓存失效。累积
  语义把代价收敛到最小（命中只追加到清单尾部、不重排，已命中部分的
  前缀稳定），但代价真实存在——所以动态工具很少时不值得检索：
  runtime 设阈值 8（见下），kernel 只认"开关 + 非空动态集"。
- **AgentConfig 扩展**：`ToolSearch bool` + `DynamicTools []tool.BaseTool`
  （与 `Tools` 互斥切分——同一工具不能同时在两处，否则 tools 节点
  重复；pinned 在 Tools 直通，动态集在 DynamicTools 经 middleware 注入）。
- **栈位**：skill 之后、safety 之前（栈注释重排为 10 位）。tool_search
  元工具进 safety 豁免集（只读工具目录、无注册表契约，同 skill 工具
  先例）；safety 的 净化名→原始名 映射覆盖 Tools + DynamicTools——
  检索后才出现的动态工具调用照常过门禁且按契约原名判定。
- 单测：挂接条件（关闭/空动态集跳过）、BeforeAgent 注入（tool_search +
  动态集进 tools 节点）、全链路（mock 脚本：pinned 直通调用 →
  tool_search 检索 → 命中工具执行；断言两工具各执行 1 次、门禁只记录
  两个业务工具的原名、tool_search 经豁免、检索事件进运行事件流
  EventToolResult、首轮输入含只列动态集的
  `<available-deferred-tools>` 提醒）。

### 3. profile 接线（runtime.buildToolchain 消费 pinned/tool_search）

- `buildToolchain` 返回扩展为（直通工具, 动态集, safety, 错误）：
  namespaces 过滤（第一级）→ splitToolDefs 按 pinned 切分（第三级，
  两份清单均保持注册表名升序）→ tool_search 开启且动态集**严格超过
  阈值 8** 时动态集经 ToolSearch 检索（第二级），否则并入直通全量注入。
- **阈值 8 的选择**：单个工具 schema 约百 token 量级，8 个以内全量注入
  的固定开销可忽略，抵不过一次额外检索轮 + ToolInfos 变动的前缀缓存
  失效风险；取值是经验起点（常量 `toolSearchDynamicThreshold`，不做
  配置项——无消费方不预留），待 MCP 真实工具库上线后按计量数据校准。
- pinned 未命中命名空间边界：记 WARN 忽略（配置笔误不阻断装配——
  工具可能尚未注册或未授权，与 namespaces 空匹配同策略）。
- safety 门禁始终以**全量命中清单**构建：L2 校验与 L4 审批按工具契约
  工作，与工具对模型的可见形态（直通/检索）无关。
- 对话（runtimeFor）与 observer 长驻（startObserver）两条 AgentConfig
  装配路径同步接入——栈形态一致。
- leader role.yaml 示例：`tools.tool_search: true`、`tools.pinned:
  [system.time]`、namespaces +`skill.*`（目录工具对 leader 可见）；
  当前 7 个内置工具减 1 pinned = 6 动态 ≤ 8，共享 configs 仍是全量直通
  形态（行为不变），超阈值形态由集成测试覆盖。
- profile 注释同步：Pinned/ToolSearch 字段与 doc.go 的"解析透传"声明
  改为"buildToolchain 消费"。

### 4. bootstrap 接线

- Wire 顺序调整：技能存储（⑤）先于工具体系（⑤.5）——技能目录工具
  以技能快照为数据源；装配失败路径补 `stopSkillStore` 清理（nil 安全），
  不留半启动状态。newToolchain 做 typed-nil 修正（nil 存储不赋接口）。
- `App.ToolRegistry()`：集成测试通道（补注册工具把动态集推过阈值）——
  注册表并发安全，buildToolchain 每次按 profile 重新过滤，运行期注册
  对后续构建的会话生效（也是 MCP 来源接入的预演形态）。

### 5. 集成测试 `tests/integration/toolsearch_test.go`

自建 leader profile（tool_search: true，pinned: [system.time]，命名空间
覆盖 system.*/artifact.*/skill.*/demo.*）→ Wire 真实装配（内置 5 + 技能
目录 2 + 种子技能库）→ 经 `App.ToolRegistry()` 补注册 9 个 demo.* 工具
（动态集 15 > 8）→ mock 脚本：第一轮 tool_search("probe") → 第二轮调
demo_probe → 第三轮总结。断言：run 成功 3 轮、探测工具执行计数为 1
（区分检索形态与"工具缺席"形态的外部可观测信号——未知工具只产生
错误结果，run 照样走完）、三轮模型调用跨度全部落库 trace_spans。
**断言路径说明**：ToolInfos 可见性（初始隐藏/检索后累积）的内容级
断言在 eino 侧已有覆盖，挂接条件与 pinned 直通在 kernel/runtime 单测；
集成层验证装配链与数据流（与 skill_test 同一取舍）。

## 影响面

- `internal/tool/builtin`：+skill.go（目录工具）；RegisterAll 签名加
  SkillSource（5→7 工具）；单测 +skill_test.go。
- `internal/agent/kernel`：AgentConfig +ToolSearch/DynamicTools；
  middleware 栈 +toolsearch 位（10 位）；safety 豁免集 +tool_search、
  nameMap 覆盖动态集；doc.go 栈清单同步（补记 R9 遗漏的 skill.go）。
- `internal/agent/runtime`：buildToolchain 返回扩展（+动态集），阈值
  常量 + splitToolDefs；runtimeFor/startObserver 装配接入。
- `internal/agent/profile`：仅注释（字段消费方落地声明）。
- `internal/bootstrap`：Wire 顺序（技能存储先于工具体系）+ 失败清理；
  App +ToolRegistry()。
- `configs/agents/leader/role.yaml`：tool_search: true、pinned:
  [system.time]、namespaces +skill.*（monitor/query 不动）。
- `docs/architecture/05-tool-system.md` §6：三级选择策略表 +"当前实现
  状态"列（仿 10 §4）。
- 无新增第三方依赖；无新配置项（阈值为常量）。

## 测试内容与标准

- 单测（-race）：builtin（list 分组/search 命中/无命中/参数缺失/nil 降级/
  契约）；kernel（挂接条件/tools 节点注入/全链路 pinned 直通 + 检索调用
  + safety 共存 + 事件流 + 提醒消息）；runtime（关闭直通/等于阈值边界
  直通/超阈值切分/pinned 未命中/safety 覆盖动态集）。
- 集成测试：上节⑤；存量 chat/skill/approval/team/depth 等不受影响
  （共享 configs 动态集 6 ≤ 8 仍全量直通，行为不变——全量套件原样通过）。
- 门禁：gofmt 无输出、go vet 通过、golangci-lint 零告警、
  go test ./... -race 全绿、make build 成功；手动实测见下节。

## 手动实测

启动 server（DEBUG 日志）→ 日志 "内置工具已注册 tools=7"（含
skill.list/skill.search）→ 发送对话消息，leader 构建运行器时 DEBUG 出现
"动态工具数未超阈值，ToolSearch 不启用，全量注入 dynamic=6 threshold=8"
（阈值以下直通形态）；把若干测试工具注册进注册表（或临时调低阈值）
使动态集 >8 → 新会话构建时 INFO 出现 "ToolSearch 动态检索已启用
pinned=1 dynamic=N threshold=8"，DEBUG 出现 "toolsearch middleware 已启用"
（阈值以上检索形态）。两种形态的日志分界线即阈值判定。

## 遗留 TODO

- 阈值 8 是经验起点：MCP 来源接入、真实工具库上线后按计量数据
  （检索轮次占比/前缀缓存命中率）校准。
- skills.dir 热重载的**补建路径**（启动时无技能形态、运行中补建存储）
  下目录工具仍报 SKILL_UNAVAILABLE：注册表启动期封闭，补建只恢复
  middleware 注入路径；如需一致，随 MCP 动态注册能力一并解决。
- `UseModelToolSearch`（模型原生检索）未启用：依赖端点原生能力，
  框架侧不可观测，待支持的端点出现再评估。
- skill.search 是子串匹配（无架构文档 06 §5 的多信号评分）：语义匹配/
  成功率信号随匹配器落地再增强，目录工具届时可复用同一评分。
- 目录工具对 monitor/query 角色未开放（namespaces 未加 skill.*）：
  按需开放，不预授权。
