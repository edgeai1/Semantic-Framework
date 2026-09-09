# Claude 原生模型驱动

日期：2026-08-05

## 变更

- 接入兼容 Go 1.23 的 `eino-ext/components/model/claude v0.1.21`，Anthropic 服务改用原生 Messages API。
- Framework 模型组件白名单扩展为 `openai / claude / mock`；未知组件继续 fail-closed。
- Claude Token 使用现有服务级 SecretStore 解析，不写入安装 YAML、日志或 Trace。
- Claude 支持 `temperature` 和 `max_tokens`；未配置时默认最大输出 4096 tokens。
- 不把 OpenAI `reasoning_effort` 静默映射为 Claude thinking，错误配置会在构建阶段明确拒绝。
- Claude 的 408、429 和可恢复 5xx 纳入同端点单次安全重试，400、401 等仍直接返回。
- MiniMax、DeepSeek 和其他兼容服务继续使用统一的 OpenAI 薄适配，不受本次改动影响。

## 验证

- 验证 Claude 配置构建为 Eino 原生 `claude.ChatModel`。
- 验证 Claude 拒绝不受支持的 `reasoning_effort`，缺少 Token 时给出明确错误。
- 验证 Anthropic 429 只在同端点重试一次。
- `go test ./...`
