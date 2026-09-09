# B2（M1.2 接入层骨架）：store/auth/HTTP 网关/WS 网关/事件总线落地

日期：2026-08-03
里程碑：Phase 1 / B2（M1.2 接入层骨架）

## 为什么做这次变更

B1 的 server 只有 healthz/version 两个探活端点，没有持久化、没有鉴权、
没有实时通道，前端与域模块都无从下手。本里程碑按架构文档 02 把接入层
骨架立起来：SQLite 元数据存储（版本化迁移）、本地账号认证（bcrypt +
token）、HTTP 网关（中间件链 + 公开/受保护路由）、WS 事件通道
（envelope 协议 + Hub 投递）与进程内事件总线，并把它们在 bootstrap
按启动序列接线为 HTTP(:8080)/WS(:8081) 双监听服务。接入层只做协议
适配，不含业务逻辑。

## 包含内容

- **go.mod 依赖**：新增 `modernc.org/sqlite`（v1.34.5，纯 Go SQLite，
  无 CGO）、`github.com/coder/websocket`（v1.8.15）、
  `golang.org/x/crypto`（v0.36.0，bcrypt）；版本均固定在 go 1.23
  兼容线内，`go mod tidy` 干净。
- **internal/store**：`Open(cfg)` 仅支持 sqlite 驱动（其他值报错，
  原则 7），连接池 MaxOpenConns(1) 规避 SQLITE_BUSY；`Migrate()`
  版本化迁移（`schema_migrations` 表 + 有序迁移列表，逐迁移事务，
  重复执行幂等），v1 只建 `users`/`tokens` 两表（chat 表留 B4）；
  users 的 `CreateUser/GetByUsername`、tokens 的
  `CreateToken/GetToken/DeleteToken/DeleteExpired`；统一
  `ErrNotFound` 哨兵错误。
- **internal/server/auth**：`Service` 提供 `Login`（bcrypt 校验，
  用户不存在与密码错误统一 `AUTH_INVALID_CREDENTIALS`）、
  `IssueToken`（32 字节随机 hex，TTL 24h）、`ValidateToken`（过期
  惰性剔除并返回 `AUTH_TOKEN_EXPIRED`）、`Refresh`（旧 token 作废
  换新）、`Logout`（幂等）；`SeedAdmin` 启动时若无 admin 则创建，
  密码取 `SEMANTIC_ADMIN_PASSWORD`，未设置用默认 `admin123` 并
  WARN 提示；HTTP 中间件按白名单（login/healthz/version）放行，
  其余校验 Bearer 并注入 user_id；handlers 提供
  login/refresh/logout 三端点，错误统一 `{"error":{"code","message"}}`。
- **internal/server/http**：中间件 requestid（透传/生成
  `X-Request-Id`，复用 pkg/log 的 trace_id 上下文）、recovery
  （panic → 500 统一错误 + ERROR 日志带 trace_id）、logging（请求
  完成 INFO：method/path/status/耗时/trace_id）；`WriteError`/
  `WriteJSON` 统一响应助手；`NewRouter` 装配公开组（login、
  healthz、version）与受保护组（refresh、logout、占位
  `GET /api/v1/system/ping` 回显 pong + user_id 验证鉴权链路）。
- **internal/server/ws**：`Envelope` 结构（id/session_id/ts/agent/
  channel/type/importance/parent/payload，snake_case，channel 与
  importance 枚举与架构 §3.3 一致），`NewEnvelope` 构造（ID =
  evt-毫秒时间戳-随机数，crypto/rand，不引入新依赖）；`Hub` 维护
  session_id → 连接集合，`Publish` 按 session_id 投递、空则广播，
  `Subscribe/Unsubscribe/CloseAll/ConnCount`；`Gateway` 处理
  `GET /ws/agent-events` 升级（token 取 query `token=` 或
  `Sec-WebSocket-Protocol: bearer.<token>`），30s ping / 90s 无 pong
  断开，读泵丢弃未知上行消息不崩，写泵消费每连接 64 缓冲（满时
  trace 丢弃 DEBUG、其余阻塞等位，对齐架构 §3.4 背压分级）。
- **internal/event**：进程内总线 `Bus`，`Publish` 异步非阻塞
  （订阅通道满即丢弃 + DEBUG，慢消费者不反压发布方）、
  `Subscribe/Unsubscribe`，`Event{Topic,Payload,Ts}`，定义
  `TopicAgentEvents` 供 Agent 事件下行。
- **internal/bootstrap 接线**：拆分为 bootstrap.go（App/Run/
  shutdown/事件桥接）与 wire_access.go（装配序列），单文件均
  ≤300 行；`Wire` 改为返回 `(*App, error)`：Open store → 迁移 →
  auth 种子 → bus/hub/gateway → HTTP/WS 两个 server；`App.Run`
  同时监听 :8080/:8081，桥接 goroutine 把 bus 上
  `TopicAgentEvents` 的 envelope 投递到 hub；优雅关闭按序执行：
  双 server Shutdown → hub.CloseAll（hijack 连接不受 Server 管理）
  → store.Close。
- **配置**：`server.ws_addr` 进入 Config 模型（默认 `:8081`，
  env `SEMANTIC_SERVER_WS_ADDR` 覆盖），configs yaml 同步；
  doctor 端口检查改为读配置（http_addr/ws_addr），不再硬编码。
- **cmd/semantic-server**：适配 Wire 新签名，装配失败即 Fatal。

## 影响面

- `bootstrap.Wire` 签名由 `*App` 改为 `(*App, error)`：
  cmd/semantic-server 与 tests/integration/healthz_test.go 已同步适配。
- healthz/version handler 从 bootstrap 移至 internal/server/http
  （路由装配的职责所在），行为不变。
- doctor 端口检查数据源由硬编码改为配置，端口清单与 server 实际
  监听保持一致。
- 其余为新增包，无既有逻辑改动；分层方向 server → store/event/
  log/config，无反向依赖；auth 不引用 http 包（避免与路由装配成环），
  自带同格式错误写出。

## 测试内容与标准

- `internal/store` 单测（-race 通过）：临时文件库，迁移幂等、
  非 sqlite 驱动报错、users CRUD（含 username 唯一冲突、
  ErrNotFound、时间戳往返一致）、tokens CRUD（删除幂等）、
  DeleteExpired 只清过期。
- `internal/event` 单测（-race 通过）：发布/订阅（topic/payload/
  时间戳）、多订阅者扇出、退订后不再投递、通道满时 Publish
  不阻塞且恰好保留缓冲容量个事件、无订阅者安全发布。
- `internal/server/auth` 单测（-race 通过）：种子用户创建与幂等
  （密码不被重置）、登录成功/失败（统一 AUTH_INVALID_CREDENTIALS）、
  token 全生命周期（签发→校验→换发旧值作废→登出幂等）、过期
  token 惰性剔除（AUTH_TOKEN_EXPIRED → AUTH_TOKEN_INVALID）、
  中间件白名单放行/缺 token 401/非法 token 401/合法 token 注入
  user_id、handlers 三类错误（401 统一码/400 非法 JSON/400 缺字段）
  与 refresh/logout 端点流转。
- `internal/server/http` 单测（-race 通过）：WriteError 格式与
  Content-Type、requestid 生成（32 hex）/透传/context 注入、
  recovery panic → 500 INTERNAL_ERROR + 日志含 trace_id 与 panic
  内容、logging 五字段齐全、路由级鉴权链路（公开端点免 token、
  ping 无 token 401 统一格式、带 token 200 + pong + user_id）。
- `internal/server/ws` 单测（-race 通过）：envelope 序列化九个
  协议字段齐全且 snake_case、agent/parent/payload 嵌套正确、
  同毫秒千次构造 ID 不重复；真实 websocket 端到端——按
  session_id 投递（异会话连接收不到）、空 session_id 广播全在线
  连接、断开后 Hub 注销（ConnCount 归零）、无 token 升级前 401
  拒绝、未知上行消息不断连且下行正常。
- `tests/integration`（-race 通过）：startApp 升级为双监听 +
  临时库 + 固定测试口令；TestAccessAuthFlow 走通
  login → 带 token ping 200/无 token 401；TestAccessEventDownlink
  走通 WS 连接 → bootstrap 内 EventBus 发布 envelope → 客户端
  收到并校验全部协议字段；healthz/version 用例适配后保持绿。
- 手工实测：见下"收尾验证"。
- 门禁：`gofmt -l .` 无输出、`go vet ./...` 通过、
  `golangci-lint run` 零告警、`go test ./... -race` 全绿、
  `make build` 成功。

## 遗留 TODO

- healthz 仍是进程存活探活，store ping 等真实依赖探活留 B3。
- WS 断连期间事件不落库、无 last_event_id 续传，随会话聚合器
  （B4+）一并实现。
- 上行消息（对话输入/交互应答/控制指令）骨架阶段只读丢弃，
  待对话通道 /ws/chat 与聚合器落地。
- token TTL（24h）为常量未入配置；auth.provider 配置段待外部
  用户中心实现时按原则 7 再开放。
- 心跳参数（30s/90s）硬编码，如需按部署环境调整再建模配置。
- 集成测试端口仍用"申请-释放"方式，竞争窗口未消除（B1 遗留）。
