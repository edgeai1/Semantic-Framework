# 修复 Agent 继承 Default 时无法配置模型

日期：2026-08-06

## 问题

`semantic init` 安装的 Agent Profile 不写 `model`，表示继承系统 Default。设置 API 在 LLM 配置热更新前错误地把空值当成名为 `""` 的端点，导致前端首次配置模型服务失败。

## 修复

- Agent `model` 为空时按正式继承语义处理，不再执行 `registry.Get("")`。
- 固定填写了模型端点的 Agent 继续执行悬空引用校验，删除其正在使用的端点仍会被拒绝。
- 会话运行时继续在创建快照时把当时的系统 Default 固化为实际端点。

## 验证

- 新增完整 Settings REST 回归：使用 init 同形态的空模型 Leader，切换系统 Default 后成功写回并热应用。
- 既有“固定模型端点不可被删除”验收继续通过。
