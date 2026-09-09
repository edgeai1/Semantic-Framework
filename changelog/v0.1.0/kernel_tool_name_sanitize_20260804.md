# 工具名端点净化（修复真实模型 400 错误）

> 日期：2026-08-04　模块：agent/kernel、security、tests

## 为什么改

用 deepseek 实测对话时模型调用失败：`400 Bad Request: Invalid 'tools[0].function.name': string does not match pattern '^[a-zA-Z0-9_-]+$'`。我们的工具全名（`artifact.put`、`system.time` 等）含 `.`，OpenAI 兼容端点（deepseek 等）对 function.name 有严格正则约束；mock 端点宽松所以此前未暴露（B5 changelog 已记 TODO）。

## 内容清单

- **kernel/tools.go**：`Info()` 面向模型返回净化名（`SafeToolName`：非 `[a-zA-Z0-9_-]` 字符替换为 `_`，如 `artifact.put → artifact_put`）；描述前缀保留原始全名便于模型理解命名空间；`AdaptTools` 增加净化后重名冲突检测（适配期报错，启动期暴露）。
- **kernel/middleware.go**：safety middleware 增加 `nameMap`（净化名→原始名，构建栈时从适配器收集），门禁收到的 `ToolCallMeta.Name` 还原为契约原名——`approval_required` 命名空间与 risk 注解仍按原名匹配，安全语义不变。
- **测试同步**：mock 脚本中模型侧工具名全部改为净化名（kernel/interrupt_test、security/middleware_test、runtime/approval_test、tests/integration/approval_test）；新增 kernel/tools_test.go（净化规则/冲突检测/Info 净化/门禁名还原 4 组用例）。

## 影响面

- 模型侧可见函数名变化（`artifact_put` 等），领域契约名（`artifact.put`）不变：执行器、注册表、安全门禁、审批信息、AGENT.md 命名空间描述均不受影响；
- 无跨模块接口变更；docs/architecture/05 §3 已补净化说明。

## 测试内容与标准

- 新增：TestSafeToolName（7 组映射）、TestAdaptToolsSanitizesAndDetectsCollision、TestSafetyMiddlewareRestoresOriginalName（还原+透传两路径）；
- 回归：`go test ./... -race` 18 包全 ok（含审批批准/拒绝全链路）；gofmt/vet/golangci-lint 零告警。

## 遗留 TODO

- DeepSeek 实测复验（需有 key 环境重跑对话链路，预期 400 消失）；
- 工具全名如未来引入更多特殊字符（`/`、`:`），净化规则保持统一替换，冲突检测已兜底。
