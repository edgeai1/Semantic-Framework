# R1 设置系统：.env 加载 + 配置 schema 校验 + 热重载

日期：2026-08-04
里程碑：v0.1.1 / R1（设置系统）
分支：feature/settings-system

## 为什么做这次变更

v0.1.0 的配置链（yaml + SEMANTIC_ env 覆盖，启动一次性 Load）有三个工程缺口：

1. **密钥/本地覆盖无落点**：API key、admin 密码只能手工 export，无标准 .env 机制；
2. **配置笔误静默生效**：yaml 写错键名（如 `http-addr`）会被 yaml.v3 静默忽略，
   服务带着"以为已生效"的错误预期启动；
3. **任何配置变更都要重启进程**：log.level、llm.providers 这类本可热调的项
   也必须中断服务。

本 MR 按最小必要实现补齐三块，严格不设预留能力（原则 3/7）。

## 包含内容

### 1. .env 加载（`pkg/config/dotenv.go`，自写 parser ~100 行，零新增依赖）

- `LoadDotEnv()` 启动早期依次读 `./.env`、`~/.semantic/.env`（不存在则跳过）；
  解析 `KEY=VALUE`，支持 `export ` 前缀、单双引号、整行 `#` 注释、非引用值的
  行尾注释（`" #"` 起）与空行；非法行（缺少 `=`）带文件/行号报错。
- **优先级：进程 env > ./.env > ~/.semantic/.env**——已存在的键一律不覆盖，
  先加载的文件自然压住后加载的同名键。
- `DotEnvResult` 只记录文件路径与键名（绝不记值），供启动日志与 doctor 输出。
- 双引号内不展开转义序列：密钥值原样保留，避免静默改写。
- `.env` 此前已在 .gitignore（含 `!.env.example` 豁免），本轮新增 `.env.example`
  占位（SEMANTIC_LLM_API_KEY_DEEPSEEK_CHAT / SEMANTIC_ADMIN_PASSWORD）。
- 接线：`cmd/semantic-server` main 早期 fail-closed 调用（解析失败即退出）；
  `cmd/semantic` main 早期调用，失败仅告警（doctor 会列为 ✗）。

### 2. 配置 schema 校验 fail-closed（`pkg/config/validate.go`）

- `Load` 内置 `validateYAML`：用反射从 Config 结构树提取 yaml tag 白名单，
  递归比对节点树——**未知键**与**类型错误**聚合为 `ValidationError`
  （每条含完整 yaml 路径，如 `server.http-addr: 未知配置键`）。
- `llm.providers` 条目名是动态 key 不参与比对，但条目内部字段仍按
  `LLMProviderConfig` 校验（`llm.providers.mock.componet: 未知配置键`）。
- 类型判定对字符串类字段显式拦截 yaml 推断标量（yaml.v3 会把 `!!int 8080`
  宽松解码成 `"8080"`，不拦则类型笔误被吞）；Duration 复用其自定义
  UnmarshalYAML 报错。
- 未用 `yaml.Decoder.KnownFields`：它只在首个未知字段处报错且无完整路径，
  不满足"聚合全部问题 + yaml 路径"的要求。
- 未知 component（如 `opean`）由 `kernel.KnownComponent` 在 bootstrap Wire
  启动与热重载两处 fail-closed 拦截（`pkg/config` 按分层纪律不能 import
  internal，component 白名单的唯一事实源在 kernel）：
  `llm.providers.foo.component: unknown component "opean"`。
- `configs/semantic-server.yaml` 原样通过校验（单测锁定），无历史未消费键需要清理；
  `configs/semantic-pilot.yaml` 属 pilot 空壳占位，不经 `pkg/config.Load`，不在本 schema 范围。

### 3. 热重载（`pkg/config/reload.go` + bootstrap 接线，新增 fsnotify v1.10.1）

- `Reloader`：fsnotify 监听配置文件与 `./.env` 所在**目录**（兼容编辑器
  替换式保存），按文件名过滤，500ms 去抖合并连续事件。
- 白名单热应用表（同步写在 `pkg/config/doc.go` 与配置参考文档）：
  - `llm.*` → `pkg/llm.Registry.Reload`（新增）：校验通过后**原子替换**端点快照、
    清空 API key 缓存重读；bootstrap 钩子先做 component 白名单校验、
    替换后按启动同一策略巡检缺 key 端点（WARN 降级）。
  - `log.level` → `Logger.SetLevel` 即时生效。
  - `agents.profiles_dir` → bootstrap 钩子重建 profile 加载器并预加载 leader
    校验，经新增的 `runtime.Service.SetProfiles` 原子替换；**本轮只到 loader
    重建，运行中的会话持旧 profile 快照，不做运行中 agent 热替换**。
  - 其余（`server.*`/`store.*`）→ WARN"配置项 X 已变更，需重启生效"。
- 热应用成功打 INFO 审计日志（变更段/结果）；schema 校验失败或钩子拒绝时
  保持旧配置运行并记 ERROR（坏配置不进运行时）。
- `Reloader.Current()` 快照只记录"实际生效"的配置：非白名单段保留旧值，
  保证下次 diff 基线是运行中的真实状态。
- `.env` 热同步只更新启动时由 `./.env` 写入的"自有键"（值变覆盖、键消失移除），
  外部设置的进程 env 一律不碰。
- watcher 随 `App.Run` 启动、随优雅关闭最先停止；初始化失败降级 WARN
  （"配置变更需重启生效"），不阻塞启动。
- 新增依赖 `github.com/fsnotify/fsnotify` v1.10.1：跨平台文件监听事实标准
  （BSD-3，维护活跃），自写 inotify 绑定 Linux 且易错，无标准库替代。

### 4. doctor 增强（`cmd/semantic/doctor.go`）

- 新增 ".env 加载状态" 项：✓ 已加载（文件+键数）/! 未找到/✗ 解析失败+修复建议；
  复用 main 早期的加载结果，避免二次加载把已设置键误报为 0。
- 配置加载失败区分 schema 校验失败：逐条列出全部问题并给修复建议
  （对照 `docs/developer/06-configuration-reference.md`）。
- LLM 检查新增未知 component ✗ 项（此前会被误判为"无需密钥 ✓"）。

### 5. 文档

- 新建 `docs/developer/06-configuration-reference.md`：server/log/store/llm/agents
  全量真实配置项，逐键说明 + env 覆盖名 + 是否热重载（spec §4 要求的登记表）。
- `pkg/config/doc.go` 重写：优先级链、fail-closed 校验、热重载白名单表。

## 影响面

- `pkg/llm.Registry`：互斥锁升级为 RWMutex，Get/Default/Names/APIKey 全部
  在锁内读（此前 APIKey 在锁外读 providers map，Reload 引入后必须修正）；
  对外签名不变，新增 Reload。
- `internal/agent/runtime.Service`：仅新增 `SetProfiles`（spec §11.1 允许的新增函数），
  既有行为不变。
- `internal/agent/kernel`：仅新增 `KnownComponent`。
- `bootstrap.Wire` 签名不变（热重载经 `App.EnableConfigReload` 装配，
  集成测试不受影响）；App 新增 reloader 字段，Run/shutdown 各增一处启停。
- go.mod 新增直接依赖 fsnotify v1.10.1（理由见上）。

## 测试内容与标准

- `pkg/config` 单测（-race 通过）：
  - dotenv：基本解析（export/引号/整行与行尾注释/空行）、优先级
    （进程 env > 先加载文件）、不存在文件忽略、非法行带文件/行号报错；
  - validate：未知键拒绝（含 provider 条目内）、类型错误拒绝（时长非法/
    整数赋字符串/标量赋结构体段）、聚合多错误 + errors.As 还原、
    仓内真实 yaml 原样通过、新增动态 provider 合法通过；
  - reload：去抖合并（窗口内两次写入只触发一次且取末值）、白名单命中
    （llm/log.level）、非白名单 WARN 且快照不替换、坏配置保持旧快照不触发钩子、
    .env 自有键同步（改值/消失移除/外部键不碰）。
- `pkg/llm` 单测（-race）：Reload 快照替换、校验失败保留旧快照、
  密钥缓存清空重读、并发读 + 连续 Reload 无竞争。
- 门禁：`gofmt -l .` 无输出、`go vet ./...` 通过、`golangci-lint run` 零告警、
  `go test ./... -race` 19 包全绿、`make build` 成功。
- 手工实测（2026-08-04，本机）：
  - 启动拒绝：未知键 → `server.http-addr: 未知配置键` 退出码 1；
    未知 component → `llm.providers.foo.component: unknown component "opean"` 退出码 1；
  - 热应用：改 `log.level: debug` → INFO `配置热应用成功 section=log.level`；
    改 `llm.default: mock` → INFO `LLM 注册表已热更新` + `配置热应用成功 section=llm.*`；
    改 `server.http_addr` → WARN `配置项已变更，需重启生效 key=server.http_addr`；
  - .env：`./.env` 写 `SEMANTIC_LOG_LEVEL=debug` → 启动日志 `log_level=debug`
    且 `.env 已加载 path=.env keys=1`（不含值）；
  - doctor：有 .env 时 `✓ .env 已加载 .env（2 个键）`+`✓ schema 校验通过`，
    无 .env 时 `!` 提示，退出码 0。

## 遗留 TODO

- settings REST API 与密钥托管（web 控制台写配置/轮换 key）在 R2 落地。
- 热重载只监听 `./.env`，`~/.semantic/.env` 不监听（用户级文件变更需重启）——
  R2 视需求评估。
- `agents.profiles_dir` 热应用只到 loader 重建；运行中会话的 profile 热替换
  （含会话运行器失效重建）留待后续里程碑评估。
- `.env` 中被删除的"自有键"会从进程 env 移除，但已被其他模块缓存的派生值
  （如本进程早前读出的 key）不追溯——密钥轮换建议配合 llm.* 热重载触发重读。
- semantic-pilot 配置体系（`configs/semantic-pilot.yaml`）未纳入本 schema，
  随 pilot 实现一并建立。
