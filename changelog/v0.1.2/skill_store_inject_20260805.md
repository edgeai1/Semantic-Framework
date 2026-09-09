# R9 通用技能系统：SKILL.md 解析 + skill store + 渐进披露注入

日期：2026-08-05
里程碑：v0.1.2 / R9（通用技能系统）
分支：feature/skill-system

## 为什么做这次变更

技能（Skill）是系统的程序记忆（架构文档 06）：以行业标准 SKILL.md 格式编码
"如何完成一类任务"，新技能上线不改框架代码、不重启进程。本 MR 落地技能
系统的最小骨架：SKILL.md 加载解析（软错误）、skill store（原子替换 +
fsnotify 热更）、内核经 eino skill middleware 的渐进披露注入（清单摘要常驻、
命中加载全文 inline）。严格最小必要：frontmatter 只消费四标准字段
（name/description/category/when_to_use），具身扩展字段
（goal/safety_rules/component/steps/on_failure）只透传不消费（Phase 3 落地），
不写多信号评分匹配器、不建多档注入（agent_ori 死代码教训）。

## 包含内容

### 1. `internal/skill`（新包）

- `skill.go`：`Skill`（Name/Description/Category/WhenToUse/Body/Dir/
  Extensions）。Extensions 是 frontmatter 标准键以外字段的透传集
  （map[string]any，无扩展为 nil，只读约定）——只解析不消费，不建空结构。
- `loader.go`：`LoadDir(dir) (skills, errs)`——递归扫描，每技能一目录含
  SKILL.md（发现即停钻，scripts/references 是资源不是技能），隐藏目录跳过；
  frontmatter 切分（`---` 包围）+ yaml.v3 解析；**软错误**（单文件失败收集
  继续），目录不可读是硬错误；必填校验（name/description 缺失记 err 跳过），
  category 空回填 `general`，description 空白压缩（清单单行渲染约定）。
- `store.go`：`Store`——`Reload(dir)`（全量重建后**原子替换**快照，RWMutex
  保护；目录不可读保留旧快照；同名后加载覆盖先加载并 WARN）、`List()`
  （名升序）、`Get(name)`、`Summary()`（按 category 分组，每技能一行
  `- name: description (category)`，组间组内排序稳定）；`Start()/Stop()`
  fsnotify 监听全部子目录（新技能 = 新目录，父目录事件可见），500ms 去抖
  自动 Reload（与 pkg/config 热重载同策略），Reload 后同步监听目录集。
  已知限制：技能目录整体删除后需经 Reload（配置热重载钩子）或重启恢复。
- 单测：递归加载/扩展透传/软错误（缺 name、坏 YAML、无 frontmatter）/
  硬错误/必填校验与默认值/原子替换（增删坏）/硬错误保留旧快照/Summary
  分组渲染/监听热更（新增 + 修改，写临时文件触发）。

### 2. kernel skill middleware 挂接（`internal/agent/kernel/`）

- `skill.go`（新）：`SkillStore` 读取端口（kernel 面向接口，实现方
  internal/skill.Store）；`skillBackend` 适配 eino v0.9.13
  `adk/middlewares/skill` 的 `Backend`——清单 = `List()` 标准字段，
  正文加载 = `Get(name).Body`，`BaseDirectory` = 技能目录；context 一律
  为空 = **inline 模式**（无 fork 子 agent）；具身扩展字段不进模型侧
  （面向框架模块，执行指导已在 Body）。
- **backend 适配形态（为什么这样接）**：eino skill middleware 的渐进披露
  = skill 工具描述常驻清单 + 模型按名调用后正文作为工具结果注入。我们用
  `CustomToolDescription` 把清单渲染为 `store.Summary()`（含 category
  分组，eino 默认 XML 清单没有分类信息）+ 中文使用要点；
  `CustomSystemPrompt` 用中文指引（eino 默认英文模板含本框架不支持的
  "技能脚本执行"段落）。工具名固定 `skill`（safety 豁免集与本名同源）。
- **注入位置与缓存关系**：栈位 agentsmd 之后、safety 之前（栈注释更新为
  9 位）。skill 使用指引经 adk Instruction 通道占据头部 system 位——
  这撞上了 context middleware 旧的幂等判定（首条 Role==System 即跳过），
  S1 会被永久挡住。修正为**按内容识别**（`alreadyInjected`：首条 system
  内容 == 本 middleware 组装的 S1，或 S1 为空时按 S4 段标题前缀）：
  外来 system 消息不再挡住 S1，头部顺序稳定为 [S1, S4?, skill 指引, ...]，
  S1 前缀缓存纪律不受影响；summarization 保留全部头部连续 system 消息
  （已核实 eino splitSystemAndContextMsgs），skill 指引不被压缩。
- **safety 共存（关键冲突的解法）**：skill 加载工具由 middleware 注入
  runCtx.Tools，会被 safety middleware 包装——门禁清单只覆盖注册表工具，
  旧行为按 UNKNOWN_TOOL 拒绝。`safetyMiddleware` 增加 `exempt` 豁免集
  （装配时显式登记，当前仅 `skill`）：豁免工具直接放行（只读技能正文，
  无 L2 schema/L4 risk 可判定）；未登记的未知工具仍按 UNKNOWN_TOOL 拒绝
  （装配缺漏兜底不削弱，internal/security 零改动）。
- `AgentConfig.SkillStore`（nil 或空快照跳过不挂——无技能形态与存量一致；
  挂接判定在 agent 构建期，构建后新增技能对已缓存会话不生效，新会话生效）。
- 热更生效粒度（已核实 eino 源码）：`genToolInfos` 每次 run 执行 →
  skill 工具 `Info()` → `Backend.List()` 每次 run 读快照——**技能热更
  下一轮 run 生效**，缓存会话无需重建。
- 单测：backend 映射/清单进工具描述（分组格式）/nil 与空快照跳过/
  inline 正文注入端到端（mock 脚本，断言 [S1, skill 指引, user] 与工具
  结果含正文标记 + base directory）/safety 共存（recordingGuard：skill
  不经门禁、业务工具照常过门禁）/外来 system 消息幂等（S1 与 S4-only
  两形态）。

### 3. 种子技能 `configs/skills/`

- `general/echo-guide/SKILL.md`：system.echo/system.time 的链路探测与
  时间查询规范（frontmatter 四字段 + 正文：适用场景/使用规范/示例）。
- `general/artifact-usage/SKILL.md`：artifact.put/get 存取规范与审批语义
  （put 必须经人工审批、拒绝不重试、引用凭证带回；get 取回截断外置全文）。

### 4. config 增补 `skills.dir`

- `pkg/config`：`SkillsConfig{Dir}`，默认 `configs/skills`，env
  `SEMANTIC_SKILLS_DIR` 覆盖；schema 严格校验经结构体标签自动覆盖。
- **热重载白名单**：`skills.dir` 变更走 `store.Reload` 路径（Hooks 新增
  `OnSkillsDirChanged`；pkg/config doc.go 白名单表同步）。
- `configs/semantic-server.yaml` 与 `docs/developer/06-configuration-reference.md`
  （新增 skills 段表）同步。与架构文档 06 §8 的目标配置（`skill.dirs` 清单 +
  store/match/publish 段）的关系：本 MR 只落地单目录最小项，其余字段随
  Phase 3 消费方落地再增。

### 5. bootstrap/runtime 接线

- bootstrap：Wire 新增 ⑤.5 步 `newSkillStore`（加载快照 + 启动监听；
  目录不可用 WARN 降级无技能形态——技能是上下文增强项，与 teams_dir
  缺失同策略；监听启动失败降级静态快照）；App 持有 store，
  `shutdown` 先停监听（快照全程可读，observer 收尾不受影响）；
  `App.SkillStore()` 供集成测试断言。
- 热重载钩子 `applySkillsDir`：常态走 `store.Reload(dir)`；启动时无技能
  形态的补建路径（NewStore + Start + `runtime.SetSkillStore`，与
  SetProfiles 同语义——已缓存会话不受影响）。
- runtime：`Deps.SkillStore` + `skillReader()`（typed-nil 修正，同
  blackboardReader 先例）；对话（runtimeFor）与 observer 长驻
  （startObserver）两条 AgentConfig 装配路径都注入——栈形态一致。

### 6. 集成测试 `tests/integration/skill_test.go`

种子技能复制到临时目录（不污染真实 configs）起 App → 断言 store 装配
（两种子技能可读、Summary 分组渲染）→ 写入新技能文件断言 fsnotify 热更
进快照 → 脚本化 mock（先调 skill 工具加载 echo-guide 再总结）经 WS 对话
全链路跑通（2 轮无错误）。
**断言路径说明**：mock 模型在 runtime 内部构建，集成层拿不到 CallInputs，
清单/正文 inline 的内容级断言在 kernel 单测；集成层验证装配链与数据流
（与 blackboard_test 同一取舍）。

## 影响面

- `internal/skill`：新包（doc/skill/loader/store + 单测）。
- `internal/agent/kernel`：AgentConfig +SkillStore；middleware 栈 +skill 位
  （9 位）；safetyMiddleware +exempt；contextMiddleware 幂等改按内容识别
  （无技能形态行为不变——存量单测原样通过）。
- `internal/agent/runtime`：Deps +SkillStore、SetSkillStore、双装配路径注入。
- `internal/bootstrap`：Wire +技能存储步、App +skills、shutdown 停监听、
  reload 钩子 +applySkillsDir。
- `pkg/config`：+SkillsConfig（默认/env/白名单热应用）；yaml 与配置参考同步。
- `configs/skills/`：两个种子技能（general 类）。
- 无新增第三方依赖（fsnotify/yaml.v3 已在 go.mod）；`skills.dir` 一个配置项。

## 测试内容与标准

- 单测（-race）：internal/skill 全量（加载/软错误/原子替换/Summary/热更）；
  kernel（backend/清单/跳过/inline 注入/safety 共存/外来 system 幂等）；
  pkg/config 存量 + 新段自动覆盖。
- 集成测试：上节⑥；存量 chat/approval/team/depth 等不受影响（无技能
  形态行为不变——集成测试 CWD 下相对路径 configs/skills 不存在即走
  WARN 降级路径）。
- 门禁：gofmt 无输出、go vet 通过、golangci-lint 零告警、
  go test ./... -race 全绿、make build 成功；手动实测见下节。

## 手动实测

启动 server（DEBUG 日志）→ 日志出现 "技能存储已加载 skills=2" 与
"技能目录热更监听已启动" → 发送对话消息，leader 构建运行器时 DEBUG 出现
"skill middleware 已启用" → 修改 configs/skills 下一个技能的 description
（或新增技能目录）→ 约 500ms 后日志出现 "技能快照已替换" → 下一轮 run
即读新快照（清单经 skill 工具描述逐 run 重渲染）。

## 遗留 TODO

- 具身扩展字段（goal/safety_rules/component/steps/on_failure）消费：
  Phase 3 任务/安全模块（本轮只透传）。
- `when_to_use` 目前只透传，匹配信号落地后消费。
- 技能目录整体删除后的自动恢复（现需配置热重载钩子或重启；监听父目录
  可解，超出本轮最小范围）。
- 架构文档 06 §8 完整配置段（skill.dirs/store/match/publish）随消费方落地。
