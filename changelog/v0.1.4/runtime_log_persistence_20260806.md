# 运行日志持久化与滚动

## 修复

- Semantic Server 的结构化日志不再只输出到启动终端，同时持久化为 JSON Lines 文件。
- 日志路径根据当前配置中的 SQLite 数据目录推导为 `logs/semantic-server.jsonl`，使用 `-c` 或 `SEMANTIC_CONFIG` 切换安装实例时不会继续写入固定路径。
- 日志目录和文件权限分别固定为 `0700` 与 `0600`。
- 复用 lumberjack 完成大小轮转、历史保留与压缩，不在框架内自研日志轮转算法。

## 默认保留策略

- 单文件最大 100 MiB。
- 最多保留 5 个轮转文件。
- 最长保留 14 天。
- 历史文件启用 gzip 压缩。

## 诊断

- Server 启动日志新增 `log_path` 字段，明确当前实例的实际日志位置。
- Trace 继续保存在 SQLite；运行日志通过 `trace_id`、`run_id` 和 `session_id` 与 Trace、对话运行关联。
