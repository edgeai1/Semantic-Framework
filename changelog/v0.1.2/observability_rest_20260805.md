# R18 观测补齐：traces/metering/interactions 只读 REST 端点

日期：2026-08-05
里程碑：v0.1.2 / R18（观测补齐）
分支：feature/observability

## 为什么做这次变更

Phase 1.1 把链路跨度、计量记录、交互请求全部落进了 SQLite
（trace_spans/metering/interactions 表），但数据只进不出：前端 Trace 视图、
成本归因视图没有读取通道（验收③债），审批卡在页面刷新后也因缺少
interactions 查询端点无法恢复。本 MR 为三张表补齐只读 REST 面，严格对齐
架构文档 13 §8 的端点设计，无写入能力、无预留字段。

## 包含内容

### 1. store 查询方法（`internal/store/`）

无新迁移：`trace_id`（idx_trace_spans_trace_id）与
`(session_id, status)`（idx_interactions_session_status）索引在 v2/v4 已建，
查询路径全部落在既有索引上。

- `trace.go`：`ListTraces(TraceFilter, limit, offset)` 返回分页聚合视图
  `TraceSummary{trace_id, name, kind, started_at, duration_ms, span_count}`
  与总链路数。实现分两步：SQL 侧 `GROUP BY trace_id` 聚合分页
  （`MIN(started_at)` 只参与 `ORDER BY` 不落结果集——实测 modernc.org/sqlite
  对聚合表达式返回 string，无法扫进 `time.Time`），再按页内（≤100）
  trace_id 回查最早开始的跨度补 name/kind/started_at；`duration_ms` 口径为
  全部跨度耗时合计（嵌套不去重，文档已注明）。新增 `GetSpans(traceID)`
  按开始时间升序（同刻按写入序），与既有 `QuerySpans`（写入序）共用
  扫描助手 `scanSpans`。
- `metering.go`：`MeteringFilter` 增加 `TraceID`（任务明细归属维度）与
  `Offset`（分页）；WHERE 构建抽出 `meteringWhere` 供查询与新增的
  `CountMetering` 共用。新增 `SummarizeMetering(since)`：
  `GROUP BY model, agent, purpose` 聚合 calls/tokens/cost，按总 token 降序。
- `interaction.go`：新增 `ListInteractions(InteractionFilter{SessionID, Status},
  limit, offset)`，按创建时间倒序（同刻 ID 倒序）返回本页与总数。
- 单测：聚合分组/倒序分页/精确过滤/时间窗/冲突空集，全部经
  `openTestStore` 真实库验证。

### 2. REST 端点（`internal/server/http/handlers/`）

全部端点在受保护路由组（Bearer 鉴权），统一错误格式，分页约定
`page`（默认 1）/`page_size`（默认 50，上限 100——新增 `parsePageLimit`
 参数化上限，消息分页 200 上限不受影响）：

- `traces.go`：`GET /api/v1/traces`（task_id 与 trace_id 均精确匹配链路
  ID——v1 数据模型中任务的追踪单元即链路；两者同时给出且不一致时短路
  返回空列表）与 `GET /api/v1/traces/{trace_id}/spans`（attrs 反序列化为
  JSON 对象下发，脏数据回落 `{}`）。
- `metering.go`：`GET /api/v1/metering/summary?window=24h`（Go duration，
  非法或 ≤0 返回 400；响应含 `window`/`since`/`rows`）与
  `GET /api/v1/metering/tasks/{id}`（{id} 即链路 ID，明细分页）。
- `interactions.go`：`GET /api/v1/interactions?session_id=&status=`，
  status 校验状态机取值（非法 400）；payload/reply 以 JSON 对象下发
  （未应答 reply/answered_at 为 null）；`checkpoint_id` 属内核断点衔接
  字段不下发。
- 单测（httptest + 真实 store + chi 路由）：三文件各端点的分页/过滤/
  聚合/400/空集与 JSON 形态（含 page_size 超限收敛到 100）。

### 3. 接线与文档

- `NewRouter` 增加三个 handler 参数（唯一调用方 bootstrap 已同步，
  路由装配测试同步）；handlers 包 doc.go 登记观测域。
- `docs/api/rest.md`：新增 traces/metering/interactions 三节契约
  （含聚合口径与 task_id/trace_id 同义约定）。
- `docs/architecture/13-observability.md` §8：端点表标注"已实现（R18）"，
  `metrics/summary` 保持未实现（指标 registry 未建）。

### 4. 集成测试（`tests/integration/observability_test.go`）

起 App → login → 建会话 → WS 发消息（mock 调 artifact.put 触发审批中断）
→ 中断期间 `interactions?status=pending&session_id=` 断言 1 条审批卡
（payload 对象/reply null/run_id 非空）→ 批准跑完 → answered 1 条且
pending 归 0 → `traces` 列表断言聚合字段（span_count≥2，轮询等待流式
回调落库）→ `task_id` 过滤与分页冒烟 → `spans` 断言升序、ChatModel 跨度、
attrs 对象 → `metering/summary` 断言 mock/leader/chat 分组 calls≥2 →
`metering/tasks/{trace_id}` 明细归属断言 → 五端点无 token 均 401。

## 影响面

- `internal/store`：新增 ListTraces/GetSpans/CountMetering/SummarizeMetering/
  ListInteractions 与 TraceSummary/TraceFilter/MeteringSummary/InteractionFilter；
  MeteringFilter 增两字段（零值行为不变）；QuerySpans 重构出 scanSpans
  （排序语义不变，既有测试钉住）。
- `internal/server/http.NewRouter` 签名 +3 参数（唯一调用方已同步）。
- `handlers` 包新增公共助手 `parsePageLimit`/`rawJSON` 与常量
  `maxListPageSize`（chat 端点行为不变）。
- 无新增第三方依赖，无 schema 迁移。

## 测试内容与标准

- 单测（-race）：store 三组查询（聚合/分页/过滤/时间窗）+ handlers 三个
  端点文件（httptest 全路径）。
- 集成测试：`TestObservabilityREST` 全链路（见上节），-race 通过。
- 门禁：gofmt 无输出、go vet 通过、golangci-lint 零告警、
  `go test ./... -race` 全绿、`make build` 成功；手工 curl 实测三组端点
  （login → traces 列表/spans/summary/tasks 明细/interactions pending）。

## 遗留 TODO

- `GET /api/v1/metrics/summary`（架构 13 §8 仪表盘聚合）未实现：指标
  registry（§6）尚未建立，随指标里程碑补齐。
- trace 列表 `duration_ms` 为跨度耗时合计（嵌套不去重）；若前端需要墙钟
  时长（max(end)-min(start)），需在 store 层另算，当前数据量下收益不抵
  复杂度。
- 观测端点不做按用户过滤（trace_spans/metering 表无 user 列）：定位是
  运维/调试视图，与 settings 域同级；若后续开放给普通用户视图，需要
  先补归属列。
