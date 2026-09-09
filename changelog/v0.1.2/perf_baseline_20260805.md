# R20 性能基线报告（v0.1.2 首版）

日期：2026-08-05
里程碑：v0.1.2 / R20（版本验收）
分支：feature/observability

## 工具用法

```bash
make perf-baseline                          # 构建到 .output/bin/semantic-perf
.output/bin/semantic-perf                   # 默认读 .output/semantic.db，markdown 到 stdout
.output/bin/semantic-perf -db path/to.db    # -db 覆盖库路径
```

实现：`cmd/semantic-perf`（聚合逻辑 `buildReport` 可测），store 层新增
`SummarizeSpanKinds`/`PerfDataWindow`（`internal/store/perf.go`）。
时间边界不用 MIN/MAX 聚合（实测 modernc.org/sqlite 对聚合表达式返回
字符串，见 `ListTraces` 注记），排序后 LIMIT 1 取边界行。

## 数据产生方式（可复现）

`make build` 产物 + mock 模型跑验收剧本（与
`tests/integration/acceptance_v012_test.go` 同剧本）：

1. 清空 `.output/semantic.db` 与 `.output/artifacts`（R18 手工实测遗留）；
2. `SEMANTIC_ADMIN_PASSWORD=*** SEMANTIC_LLM_DEFAULT=mock SEMANTIC_MOCK_SCRIPT=<审批剧本> .output/bin/semantic-server -c configs/semantic-server.yaml`；
3. 一次性 WS 驱动（用后已清理）：login → 建会话 → 发消息触发
   artifact.put 审批 → 批准 → message.done → 再发一条普通消息；
4. 待机 ~40s 让 monitor 心跳产生 observe 计量后停服；
5. `.output/bin/semantic-perf -db .output/semantic.db`。

## 报告原文

---

# 性能基线报告

- 生成时间：2026-08-04T09:29:59Z
- 数据库：`.output/semantic.db`
- 工具：`semantic-perf`（`cmd/semantic-perf`，`make perf-baseline` 构建）

## 数据窗口

| 指标 | 值 |
|---|---|
| 链路数 | 2 |
| 跨度总数 | 10 |
| 计量记录数 | 10 |
| run 数 | 2 |
| 链路最早开始 | 2026-08-04T09:27:53Z |
| 链路最近开始 | 2026-08-04T09:28:37Z |
| 计量最早记录 | 2026-08-04T09:27:53Z |
| 计量最近记录 | 2026-08-04T09:28:37Z |

## 计量分布

按用途（purpose）分组，占比按 calls / total tokens 计算：

| purpose | calls | calls 占比 | prompt | completion | total tokens | tokens 占比 | cost 估算 |
|---|---|---|---|---|---|---|---|
| observe | 7 | 70.0% | 70 | 140 | 210 | 70.0% | 0.0000 |
| chat | 3 | 30.0% | 30 | 60 | 90 | 30.0% | 0.0000 |
| **合计** | 10 | 100.0% | - | - | 300 | 100.0% | - |

按模型（model）分组：

| model | calls | calls 占比 | total tokens | tokens 占比 | cost 估算 |
|---|---|---|---|---|---|
| mock | 10 | 100.0% | 300 | 100.0% | 0.0000 |

## 每轮平均输入 token

口径：每次模型调用的平均 prompt tokens（来源 `metering.prompt_tokens`）。

| purpose | 调用次数 | prompt tokens 合计 | 平均每轮输入 token |
|---|---|---|---|
| observe | 7 | 70 | 10.0 |
| chat | 3 | 30 | 10.0 |
| **全量** | 10 | 100 | 10.0 |

> 数据缺口：trace_spans 模型跨度的 attrs 当前只记录 content_len / finish_reason / stream_chunks，不含输入 token 与上下文体积——每轮输入 token 以 metering 为准；上下文体积与缓存表现（§3.4 验收证据）待 span attrs 补齐后才有数据源。

## 耗时三分解（按跨度类型）

归类口径：`ChatModel` → 模型，`Tool` → 工具，其余（Chain/Agent 等编排跨度）→ 其他；耗时为跨度合计，嵌套跨度不去重。

| 类别 | 跨度数 | 耗时合计 (ms) | 占比 |
|---|---|---|---|
| 模型 | 10 | 0 | 0.0% |
| 工具 | 0 | 0 | 0.0% |
| 其他 | 0 | 0 | 0.0% |
| **合计** | - | 0 | 100.0% |

## D1/D3 分流归因说明

当前库中 purpose 实际取值：`chat`、`observe`。D1/D3 轮次分流（depth_models）的 purpose 归因在启用后才会出现，本报告如实呈现现状、不做推算。

---

## 读数说明与数据缺口（如实记录）

- **observe 占 70%**：monitor 心跳（30s 周期）驱动长驻循环评估产生，
  每轮评估两次模型调用（mock 脚本首条为工具调用，observer 无工具 →
  工具错误回报 → 第二次调用出文本）。这是演示观测源的频率特性，
  不是生产负载模型。
- **耗时三分解全部 0ms 且工具/其他类别无跨度**：mock 调用亚毫秒落库为
  0；工具执行当前不产生 `Tool` 跨度（TraceHandler 只覆盖模型组件回调，
  工具节点未接入）——三分解的工具/其他类别在真实模型接入前无数据，
  列入遗留 TODO。
- **每轮平均输入 token 恒为 10**：mock 的固定计量（freshUsage 10/20/30），
  仅证明计量链路端到端贯通；真实数值待 DeepSeek 真机联调（M2.7 联调项）
  产生。
- **D1/D3 无归因**：purpose 当前只有 chat/observe，depth_models 轮次路由
  的分流 purpose 在启用后才会出现（见报告末节）。
