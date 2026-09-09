# Runtime 工具 Schema 与模型重试判断修正

## 变更

- 额外 Eino 工具通过 `ParamsOneOf.ToJSONSchema()` 生成参数 Schema，不再直接序列化内部字段未导出的结构。
- Schema 转换失败时返回带工具名称的明确错误，避免 Studio 收到空对象后继续显示错误参数。
- 通用网络重试只识别网络超时；移除对 Go 已废弃 `net.Error.Temporary()` 判断的依赖。

## 原因

直接序列化 `ParamsOneOf` 会得到空对象，导致有效工具目录与模型实际工具参数不一致。`Temporary()` 自 Go 1.18 起已废弃，其含义不稳定；HTTP 限流、超时和可恢复服务端错误仍由现有明确规则处理。

## 验证

- 新增参数 Schema 转换回归测试。
- Agent Kernel 与 Runtime 单测通过。
- `make lint` 通过。
