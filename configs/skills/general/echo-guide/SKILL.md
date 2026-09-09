---
name: echo-guide
description: 使用 system.echo 与 system.time 完成链路探测与时间查询的规范
category: general
when_to_use: 需要回显文本验证工具链路，或需要获取当前时间时
tags: [debug, time, system]
---

# 链路探测与时间查询

## 适用场景

- 验证工具链路是否可用（联调、排障）：用 `system.echo` 回显一段文本；
- 需要知道当前日期/时间：用 `system.time`（返回 UTC，RFC3339 格式）。

## 使用规范

1. `system.echo` 只用于链路探测与演示，不要用它代替回答用户的问题；
2. `system.time` 返回的是 UTC 时间，用户需要本地时间时应说明时区换算；
3. 两个工具都是无副作用的只读工具（risk=low），可直接调用，无需审批。

## 示例

- 用户：「现在几点了？」→ 调用 `system.time`，把返回的 UTC 时间换算为用户所在时区后回答。
- 用户：「测试一下工具通不通」→ 调用 `system.echo`，参数 `{"text": "ping"}`，确认回显一致。
