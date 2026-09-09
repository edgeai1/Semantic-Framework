# R15 pkg/mcp：MCP 客户端封装（client + 连接池 + list-changed 桥接）

日期：2026-08-05
里程碑：v0.1.2 / R15（MCP 集成）
分支：feature/mcp-integration

## 为什么做这次变更

按架构文档 16 §3，semantic-server 是系统集成总线的 MCP client 消费方
（自身不暴露 MCP Server，避免第二个对外协议面）；R16 的 mcpregistry
（工具目录发现/调用/热更新）需要一个薄、可控的客户端底座。本 MR 落地
pkg/mcp：官方 Go SDK 引入与锁定、单 server 会话封装、连接池（memoize +
并发上限 + 短路熔断）、tools/list-changed 通知桥接，以及配置段
`mcp_servers` 的加载/校验/热重载白名单登记。设计对齐 Claude Code MCP
实现的关键取舍（调研：claude-code-analysis 04d）：连接 memoize、描述
截断 2048、失败短路 15 分钟、建连并发本地 3/远程 20。

## 包含内容

### 1. 官方 SDK 引入与锁定（go.mod）

- `github.com/modelcontextprotocol/go-sdk` **v1.3.1**——最新 release 为
  v1.7.0，但 v1.4.0 起 go 指令要求 go 1.24、v1.5.0 起要求 go 1.25；
  本仓 go 指令保持 1.23，v1.3.1 是兼容 1.23 的最高版本（逐版本核对
  proxy.golang.org 的 .mod 文件选定）。`go mod tidy` 干净；
  间接联动：`golang.org/x/sys` v0.31.0 → v0.35.0。

### 2. `pkg/mcp`（新包，薄封装）

- `client.go`：
  - `ServerConfig{Name, Transport(http|stdio), Endpoint, Command, Args, Env, Timeout}`；
    校验 fail-closed（缺 name、未知传输、http 缺 endpoint、stdio 缺 command）。
  - `Connect(ctx, cfg)`：构建传输并完成 initialize 握手。
    http 用 `StreamableClientTransport`（不设 `http.Client.Timeout`——会
    切断 SDK 默认建立的独立 SSE 推送长连，操作超时一律走 context）；
    stdio 用 `CommandTransport` + `exec.CommandContext`（Close 后子进程
    必然退出，Env 以 "K=V" 叠加在进程环境之上）。
  - `ListTools(ctx)`：SDK 迭代器自动翻页；`ToolInfo{Name, Description,
    InputSchemaJSON}`；**描述按 rune 截断 2048**（OpenAPI 衍生 server 会塞
    15-60KB 文档进 description，与 Claude Code 上限同值）；schema 序列化
    为 JSON 文本。
  - `CallTool(ctx, name, argsJSON)`：argsJSON 本地预检（非法 JSON 不发往
    server）；返回拼接的 TextContent（无文本而有 StructuredContent 时返回
    其 JSON）；工具 IsError 以 error 返回（含 server 文本）；连接类错误置
    `broken` 标记（`Broken()`），协议/工具错误不影响连接复用。
  - `Close()`：http 发 DELETE 终止会话；stdio 关 stdin 等退出 + cancel 兜底。
- `errors.go`：`ErrServerUnavailable`（错误文本即稳定错误码
  `MCP_SERVER_UNAVAILABLE`，errors.Is 判定）；`isConnError` 区分连接类
  错误（EOF/EPIPE/ECONNRESET/net.ErrClosed + "session not found" 等消息
  兜底——SDK v1.3.1 未导出会话过期错误类型）与协议/工具错误，只有前者
  触发剔除重建。
- `pool.go`：`Pool`——
  - **memoize**：键 = name/transport/endpoint/command/args/env/timeout
    规范化（env 排序消歧、args 保序）JSON 的 SHA-256；同配置全进程复用
    同一连接实例（对齐 Claude Code connectToServer memoize）。
  - **并发上限**：建连信号量本地(stdio) 3 / 远程(http) 20（与 Claude Code
    batch size 同源：本地 fork/exec 是资源尖峰，远程是网络 IO）；信号量
    只限建连尖峰，已建连接不占额度。
  - **健康语义**：Get 返回缓存连接前 ping 一次（Get 是低频路径，一次
    ping 换"返回即可用"的确定性）；调用侧报连接类错误置 broken 后，
    下一次 Get 不再发旧连接。
  - **剔除重建与短路**：失效（ping 失败/broken）剔除重建一次；重建失败
    （二次失败）短路 `CircuitOpenDuration`（默认 15min，与 Claude Code
    认证短路 TTL 同源），期内 Get 直接 `ErrServerUnavailable`；到点半开
    放行一次试连，成功即恢复，失败重新短路整周期。首次连接失败不短路
    （没有"一次失败"在先，每次 Get 允许重试）。
  - `PoolOptions{LocalConcurrency, RemoteConcurrency, CircuitOpenDuration,
    Now}`：时钟可注入（测试快进退短路窗口）。
- `notify.go`：`OnToolsChanged(fn)`——**SDK v1.3.1 原生支持
  `notifications/tools/list_changed`**（`ClientOptions.ToolListChangedHandler`），
  直接桥接，不提供轮询对账入口（交付清单二选一按 SDK 实际能力落定）。
  处理器在 SDK client 构造时挂载、经 dispatch 转发，连接前/后注册均可；
  http 传输依赖 SDK 默认开启的独立 SSE 流（DisableStandaloneSSE=false）。

### 3. config 增补 `mcp_servers` 段（pkg/config）

- `Config.MCPServers []MCPServerConfig`（`{name, transport, endpoint,
  command, args, env, enabled, namespace}`，yaml + json 双 tag）；
  Default 为显式空列表（与 yaml `[]`/env `[]` 解析结果 DeepEqual 一致，
  热重载 diff 不受 nil/空切片差异干扰）。
- env 覆盖 `SEMANTIC_MCP_SERVERS`（JSON 数组整体替换；非法 JSON 加载失败）。
- schema 校验：validate 新增"结构体切片"递归——`mcp_servers[0].endpiont`
  这类条目内未知键 fail-closed（路径带元素下标），此前无 []struct 字段，
  属新增能力。
- 热重载白名单 +`mcp_servers`：`Hooks.OnMCPServersChanged`；
  bootstrap 登记空实现（只记审计日志，标 TODO(phase-r16)，R16 mcpregistry
  消费：连接池重建 + 目录对账广播）；pkg/config doc.go 白名单表同步。
- `configs/semantic-server.yaml` 加空列表 + 注释示例（http/stdio 各一）；
  配置参考（docs/developer/06）登记整段键表。
- settings GET 掩码跟进（`internal/server/http/handlers/settings.go`）：
  `mcp_servers[].env` 的 "K=V" 元素在配置树中没有键名可比对，
  原掩码逻辑会原样透出 token——`maskEntry` 列表分支新增
  `maskEnvAssignment`（K 命中 key/password/token 敏感片段时掩码 V），
  堵住本段引入的密钥泄漏路径。

### 4. 测试基础设施

- 真实 streamable HTTP test server（SDK server 端 `NewStreamableHTTPHandler`
  + httptest）：echo/fail/verbose（3000 汉字描述）三工具。
- 可原地重启的裸 `http.Server`（`restartableServer`）：模拟进程猝死与原址
  拉起。注意：**不能用 `httptest.Server.Close` 模拟猝死**——它阻塞等待
  在途请求，会被 SDK 服务端 SSE 挂起响应（hangResponse）卡死；
  `http.Server.Close` 不等待，直接断连。
- `pkg/mcp/testdata/stdio_server`（Go 实现的最小 stdio MCP server，含
  ping/getenv 两工具）：用真实 MCP 握手而非 shell 脚本模拟 JSON-RPC
  （不依赖 jq）；TestMain 编译一次全测试共享（t.TempDir 随单测试删除，
  不能承载共享二进制——这是本轮实测修掉的坑）。

## 测试内容与标准

- `pkg/mcp` 单测（-race 全绿）：
  - client：ListTools 描述 2048 rune 截断（多字节）/短描述原样/schema 合法
    JSON 且含推断属性；CallTool 正常回显、IsError 透出 server 文本、未知
    工具协议错误、非法 argsJSON 本地报错——且这三类都不置 broken；
    server 猝死调用失败并置 broken；Connect 参数校验四例。
  - notify：注册后 server 端 AddTool 触发 list-changed，10s 内收到回调
    （循环加热加工具消除 SSE 通道建立时序抖动）。
  - pool：同配置同实例、异配置异实例、env 序无关键；猝死后原址拉起
    → ping 失败剔除重建成功（不短路）；调用侧 broken → Get 剔除重建；
    二次失败短路 → 窗口内（含 server 已恢复）直接 ErrServerUnavailable →
    到点半开试连成功并恢复复用；半开失败重新短路整周期、下个周期恢复；
    首次连接失败不短路、server 起后即连。
  - stdio：真实子进程 connect/list/call/close；Env 透传经子进程 getenv
    工具读回验证。
- `pkg/config` 单测（-race 全绿）：yaml 解析（http/stdio 条目全字段）、
  env JSON 整体替换、非法 env JSON 报错、默认显式空列表、条目内未知键
  拒绝（`mcp_servers[0].endpiont`）、非列表拒绝、热重载白名单命中
  （钩子触发 + 快照替换）。
- 门禁：`gofmt -l .` 无输出、`go vet ./...` 通过、`golangci-lint run` 零告警、
  `go test ./... -race` 全绿、`make build` 成功。

## 遗留 TODO

- TODO(phase-r16)：`OnMCPServersChanged` 当前为空实现（只记审计），
  R16 mcpregistry 消费 mcp_servers（连接池重建 + 工具目录对账 + 热更新广播）。
- Get 的 ping-per-Get 换取确定性健康判定；若 R16 调用路径需要高频 Get，
  可评估健康检查间隔缓存（当前 Get 语义满足交付清单）。
- `isConnError` 对 SDK 未导出的会话过期错误按消息子串匹配，SDK 后续版本
  导出判定接口后应切换。
- 架构 16 §8 的 `mcp.discovery_interval/call_timeout/circuit_breaker` 段
  属 R16 mcpregistry 消费面，本 MR 不建模（交付清单只含 mcp_servers）。
- OAuth/resources/prompts/ws 按架构取舍本版不做。
