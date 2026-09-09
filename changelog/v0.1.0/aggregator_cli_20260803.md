# B6（M1.6 聚合器 v1 + CLI + 联调验收）

日期：2026-08-03
里程碑：Phase 1 / B6（M1.6 聚合器 v1 + CLI + 联调验收）

## 为什么做这次变更

B1-B5 的事件流是 bootstrap 里的一段朴素桥接（bus → hub.Publish 透传）：
事件 id 由发布方各自生成（毫秒+随机），同毫秒内无字典序保证，事件不落库——
断连即丢，架构文档 02 §3.4 的会话聚合器与 14 §7 的断连续传契约都无从谈起。
本里程碑把事件面收拢到正式聚合器（归一化 → 分级 → 落库 → 转发），
事件全部持久化并开放 WS `sync` 上行做断连续传；同时交付用户侧入口
`semantic login/chat/sessions` CLI 与两份线上契约文档（docs/api/），
完成 Phase 1 的联调验收闭环。

## 包含内容

- **internal/server/aggregate（聚合器 v1，架构 02 §3.4）**：
  - `aggregator.go`：`Aggregator` 订阅 event bus `agent.events` topic，
    单 goroutine 消费循环串行处理：归一化（**id 由聚合器单点重发**、
    ts/empty agent 兜底）→ 分级打标 → **先落库再转发 hub**；
    构造时即订阅（Run 的 goroutine 调度窗口内发布会静默丢事件，
    装配完成即订阅消除盲区）；`ReplayEvents` 实现 `ws.EventReplayer`
    供断连续传补发。启动/停止、落库失败丢弃、补发计数全部落日志。
  - `rules.go`：分级规则表 v1 纯函数 `Classify(channel, payload)`——
    interaction 一律 critical（审批不可被折叠）；dialogue/progress/artifact
    normal；trace low；alert 按 payload.level 映射（数值 ≥3→critical、
    1-2→low（02 §3.4），字符串 critical/high→critical、warning/medium→
    normal、info/low→low，缺失/不可解析→normal——告警宁可见不可漏）；
    未知频道兜底 normal。
- **internal/store（迁移 v5）**：
  - `events(id PK, session_id, channel, type, importance, agent_id,
    agent_role, parent, payload, ts)` + `(session_id, id)` 索引——
    断连续传与历史事件视图的唯一事实源；
  - `InsertEvent`（重复 id 显性报错：发放单调性被破坏的信号）、
    `ListEventsAfter(sessionID, lastEventID, limit)`——字典序游标
    分页，**查询侧排除 trace 频道**（"可丢不补"的协议语义在 store
    层固化，防未来调用方忘记）。
- **internal/server/ws（sync 上行）**：
  - 上行新增 `sync`（`{type:"sync", last_event_id}`），`/ws/chat` 与
    `/ws/agent-events` 均支持：按游标补发订阅会话的缺失事件（id 升序，
    trace 不补）后回 `{type:"sync.done", count}`；空游标只要实时流
    （直接 sync.done(0)）；新错误码 `SYNC_FAILED`；
  - `EventReplayer` 接口定义在 ws（接口倒置，aggregate 实现，依赖单向）；
  - `/ws/agent-events` 上行从"静默丢弃"改为显式协议：sync 受理，
    其余类型回 `WS_UNKNOWN_TYPE`（调用方立刻发现协议用错）；
  - 两个 Gateway 构造函数新增 syncer 参数。
- **internal/bootstrap**：朴素桥接 `bridgeEvents` 删除，改装配聚合器
  （App 持有 `aggregator`，Run 启动/停止与 server 同生命周期）；
  两个网关注入聚合器作为补发器。
- **cmd/semantic（CLI）**：
  - `semantic login --username --password [--server]`：REST 登录，
    凭据写 `~/.semantic/credentials.json`（目录 0700 / 文件 0600，
    权限语义与 SSH 私钥一致）；
  - `semantic chat [--session <id>] [--server] [--ws]`：无 session 则
    REST 建新会话；REPL 输入消息 → /ws/chat 发送 → 流式打印 delta；
    收到 interaction.request 打印 `[审批] <question>` 并进入待答状态，
    y/n 回 interaction.reply；`/quit` 退出；token 过期给出重新登录指引；
  - `semantic sessions`：列出会话（id/标题/最近活跃）；
  - REST 走标准库、WS 走 coder/websocket；WS 地址由 `--server` 推导
    （scheme http→ws/https→wss、显式端口 +1，默认部署 8080/8081 的约定，
    非标准端口用 `--ws` 显式指定）；地址解析优先级 flag > 凭据 > 缺省；
  - 协议结构为客户端镜像（`protocol.go`）：CLI 只解码关心的字段，
    编解码可脱离服务端依赖独立单测，契约以 docs/api/ws.md 为准；
  - main.go 重构为按文件分子命令（main/doctor/login/chat/sessions/
    client/protocol），doctor 逻辑原样搬迁。
- **tests/integration/aggregate_sync_test.go**：断连续传全链路——
  在线收第一条回复（记 done 游标）→ 断开 → 离线期间直接调
  `runtime.HandleMessage` 再产生两条回复（不经 WS，事件照常落库）→
  重连 sync 补发 4 条（delta+done×2，id 全部晚于游标且严格递增）→
  sync.done count=4 → 游标推进后重复 sync 幂等（count=0）→ 空游标
  不补发 → `/ws/agent-events` 同一游标语义。
- **docs/api/**：新增契约文档 `rest.md`（auth/chat/system 端点的方法/
  路径/请求/响应/错误码）与 `ws.md`（通道、envelope schema、上行类型
  清单、下行 type 清单、错误码、断连续传语义、背压），与代码现状严格
  一致；docs/README.md 增补 api/ 索引。

## 关键设计

### 事件 id 方案：聚合器单点发放"时间戳+序列"，弃 ULID

格式 `evt-<19位纳秒时间戳>-<9位零填充序列>`。断连续传契约要求
**同会话内 id 字典序 = 事件到达序**严格成立（`ListEventsAfter` 按
`id > ?` 补发）：ULID 同毫秒靠随机段区分，字典序不保证单调；发布方
各自生成的"毫秒+随机"id 在流式 delta 同毫秒迸发时同样乱序——乱序
不只是展示问题，`id > 游标` 查询会**漏补**字典序倒插的事件。因此
id 改由聚合器消费循环单点重发（发布方预设值仅作占位被覆盖）：单
goroutine 发放 ⇒ 发放序 = 落库序 = 转发序，原子序列段兜底同纳秒
与时钟回拨，纳秒时间戳段保证跨进程重启仍递增，且零新依赖（与 store
消息 id 的选型一致）。已知边界：序列段 9 位（单进程 10 亿事件内严格
单调）、时钟大幅回拨会破坏跨重启单调性（见 TODO）。

### 先落库再转发

落库是断连续传与历史的事实源，转发是易失通知。反向顺序在"转发后
落库前崩溃"的窗口会产生"客户端见过但补发不到"的事件——sync 契约
就此撒谎。因此**落库失败的事件不转发**（ERROR 日志显性化）：宁可
实时侧缺席（重连可补），不可下发不可持久化的事件。

### sync 语义

游标即最后收到的事件 id；补发 = 订阅会话内 `id > 游标` 的事件按 id
升序下发（trace 可丢不补），sync.done(count) 标记缺口闭合；空游标 =
"只要实时流"不补发。幂等性自然落在游标推进上：以补发末尾 id 再 sync
必得 count=0。协议无状态，服务端不记客户端进度。已知边界按
at-least-once 处理并写入契约文档：重连窗口（订阅生效→sync 处理）内
的实时事件可能与补发重叠，客户端按 id 去重；广播事件（session_id
为空）不参与按会话补发；v1 一次性返回全部缺口不分页（见 TODO）。

### CLI 协议镜像

CLI 不 import 服务端 ws/runtime 包：它是协议客户端，只解码关心的
字段（delta/done/interaction.request/sync.done/error），本地镜像类型
（protocol.go）让编解码可独立单测、CLI 依赖保持轻量（标准库 +
coder/websocket）；漂移风险由 docs/api/ws.md 契约与集成测试兜底。
REPL 审批状态机：下行 goroutine 登记待答审批，主循环读stdin——
待答期间输入解析为 y/n（y/yes/n/no），其余输入拒绝并提示；stdout
写锁防止流式输出与提示语交错。

## 影响面

- **bootstrap 朴素桥接被聚合器替代**：`App.bridgeEvents` 删除，
  `App` 新增 `aggregator` 字段；事件路径由"bus→hub 透传"变为
  "bus→聚合器（重发 id/重打 importance/落库）→hub"。下行事件的
  id 全部变为聚合器格式（`evt-<19ns>-<9seq>`），importance 以规则表
  为准（发布方取值被覆盖）——现有发布方（runtime dialogue、
  interaction）的规则结果与其原占位一致，行为无回归。
- **WS 协议新增 sync**：上行类型 `sync`、下行协议应答 `sync.done`、
  错误码 `SYNC_FAILED`；`/ws/agent-events` 上行从静默丢弃改为显式
  错误应答（原为骨架行为，无线上依赖）。
- **store schema v5**：老库自动迁移（新建 events 表），无需手工干预。
- **网关构造函数签名**：`NewGateway`/`NewChatGateway` 新增 syncer
  参数（调用点仅 bootstrap 与 ws 包内测试，已全部跟进）。
- **CLI 新增三个子命令**（login/chat/sessions），doctor 不变。

## 测试清单与结果

- internal/store：事件落库/重复 id 报错、listAfter 全量回放/游标续传/
  分页/末尾空页、trace 排除、会话隔离 ✅
- internal/server/aggregate：分级规则表 19 用例（interaction 恒 critical、
  alert 数值/字符串/缺省/非标量、静态表、未知频道兜底）、序列器 1000
  次严格递增、归一化兜底（零 ts/空 agent/重发 id）、协议违例丢弃、
  端到端（bus 发布→sink 收到+store 有记录+ReplayEvents 回放）✅
- internal/server/ws：sync 补发保序+sync.done、空游标不补发、补发失败
  与未装配回 SYNC_FAILED、agent-events 未知上行回 WS_UNKNOWN_TYPE ✅
- cmd/semantic：上行编码 golden（含 approved:false 显式拒绝）、下行
  四类解析（delta/interaction.request/sync.done/error/非法 JSON）、
  WS 地址推导、凭据往返+0600/0700 权限+过期拒绝、server 地址优先级、
  审批输入解析 ✅（REPL 交互用例按约定手动实测覆盖，见下）
- tests/integration：既有全部用例（对话闭环/审批批准+拒绝/接入层）
  在聚合器路径下无回归 + 新增断连续传全链路 ✅
- `gofmt -l .` 无输出、`go vet ./...` 零告警、`golangci-lint run` 零告警、
  `go test ./... -race` 全绿、`make build` 成功 ✅
- 联调实测（make build 产物 + mock 模型 + SEMANTIC_MOCK_SCRIPT）：

```bash
# 服务启动（mock 驱动，事件面带聚合器）
SEMANTIC_ADMIN_PASSWORD=e2e-pass SEMANTIC_LLM_DEFAULT=mock \
SEMANTIC_STORE_SQLITE_PATH=/tmp/sem-e2e/semantic.db \
SEMANTIC_MOCK_SCRIPT='[<问候>,<artifact.put 调用>,<批准总结>,<artifact.put 调用>,<拒绝总结>]' \
  .output/bin/semantic-server -c configs/semantic-server.yaml
# 日志：聚合器已启动 topic=agent.events / HTTP :8080 / WS :8081

$ semantic login --username admin --password e2e-pass
登录成功，凭据已保存到 ~/.semantic/credentials.json（0600）

$ semantic chat          # 完整对话：问候 → 审批 y → 审批 n
已创建会话 cs-c7726dec-...
you> 你好
你好，我是你的助手。
[done] turns=1 total_tokens=30
you> 帮我把报告存起来
[审批] 是否批准执行 artifact.put（风险等级：high）？（risk=high，超时未答按拒绝处理）
[审批] 已批准，等待执行…
报告已为您存好。
[done] turns=2 total_tokens=60
you> 再存一次
[审批] 是否批准执行 artifact.put（风险等级：high）？（…）
[审批] 已拒绝，等待继续…
好的，存储操作未执行。
[done] turns=2 total_tokens=60
# 产物侧验证：artifacts/ 仅 1 个文件（批准路径），内容与脚本一致；
# events 表 8 行，id 严格递增，interaction→critical / dialogue→normal

# 重启 server（换脚本）后历史延续
$ semantic chat --session cs-c7726dec-...
you> 重启后继续聊
重启后仍在，历史已加载。
[done] turns=1 total_tokens=30

$ semantic sessions
cs-c7726dec-a83b-406f-b88a-50a0a8725a6c	新会话	2026-08-04 00:58:02
# REST 复核：8 条消息（4 user + 4 assistant），顺序与内容正确
```

## TODO（后续里程碑）

- **补发分页协议**：v1 一次性返回全部缺口（事件量级上来前够用）；
  需要时加 `limit` + 客户端以页尾 id 循环 sync（store 已支持 limit）。
- **事件 id 的跨重启时钟回拨**：序列段只在进程内单调，时钟大幅回拨
  会破坏跨重启字典序（需要时引入启动时 max(id) 水位校验）。
- **广播事件的断连续传**：当前只按会话补发（session_id 为空的事件
  不进任何会话的游标区间），广播事件补发待真实生产方出现再定义。
- **CLI 体验**：输入行重绘（流式输出与提示语分行）、审批倒计时显示、
  断线自动重连 + sync（协议已就绪，CLI 尚未跟踪游标）、`semantic chat`
  读取历史消息回放（REST messages 已具备）。
- **事件清理策略**：events 表随会话删除的级联清理与归档策略
  （当前会话删除不清 events，与 run_sessions 归档语义一致，待容量
  治理时统一设计）。
- **多 Agent 事件的归一化扩展**：progress/alert/artifact 频道的真实
  生产方落地时，按需扩充归一化字段映射（分级规则表已就位）。
