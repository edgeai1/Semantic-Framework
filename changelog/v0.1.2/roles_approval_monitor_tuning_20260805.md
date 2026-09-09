# 真实联调修正：query 工具面 / 审批范围收窄 / monitor 降噪

> 日期：2026-08-05　模块：agent（profile/monitor）、tool/builtin

## 为什么改

真实模型联调（委派查询产物）暴露三个配置/规则问题：
1. **query-1 无法列举产物**：namespaces 缺 `artifact.list`，SubAgent 如实回答"没有枚举工具"（与 artifact.list 补全配套）；
2. **低风险工具也弹审批**：leader `approval_required: [artifact.*]` 过宽，risk=low 的 artifact.list、medium 的 artifact.get 都被要求审批——与设计语义（risk≥high 才审批）冲突；
3. **monitor 审批噪音**：规则"关键事件告警"匹配全部 critical 事件，而审批请求（channel=interaction，聚合器恒 critical）每次都会触发 level-4 告警——审批是正常流程，不是异常。

## 内容清单

- `configs/agents/query/role.yaml`：namespaces 增加 `artifact.list`，pinned 改为 `[artifact.get, artifact.list]`；
- `configs/agents/leader/role.yaml`：`approval_required` 收窄为 `[artifact.put]`（risk 注解已覆盖 put=high；list/get 只读不审批）；
- `internal/agent/monitor/rules.go`：MatchCondition 增加 `not_field`/`not_equals` 排除条件（成对校验）——高频正常事件可显式排除，防正常流程误判；
- `configs/agents/monitor/rules.yaml`："关键事件告警"排除 `channel=interaction`（审批请求），真实异常（alert/dialogue 的 critical）仍命中；
- 测试：rules_test 新增排除条件用例（interaction 排除/alert、dialogue 命中/字段缺失不排除/不成对报错）。

## 影响面

- 行为变化（有意为之）：artifact.list/artifact.get 不再弹审批卡；审批请求不再触发 monitor 告警；
- 无接口变更；`not_field/not_equals` 为规则表可选扩展字段，存量规则不受影响。

## 测试内容与标准

- monitor 包与 TestTeamAssemblyAndAlert（critical 演示事件仍命中告警）全过；全仓 26 包全绿；gofmt/golangci-lint 零告警。

## 遗留 TODO

- 前端：卡片按时间线排序、已应答审批卡在历史中恢复展示（另列 MR）。
