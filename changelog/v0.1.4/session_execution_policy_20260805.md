# 会话级最小执行策略

日期：2026-08-05

## 变更

- 新增会话执行策略 `ask / auto / full` 和会话级宿主执行开关；新会话默认 `ask` 且关闭宿主执行，不跨会话继承。
- 新增 schema v13 `session_execution_policies`，只保存模式、宿主开关和更新时间，不恢复 v0.1.3 的权限指纹或版本授权系统。
- 新增 `GET/PUT /api/v1/chat/sessions/{id}/host-execution`，一次请求原子更新执行模式与宿主开关。
- 服务端配置新增 `execution.allow_host` 硬开关，模板默认关闭；会话不能通过前端请求突破服务端禁用状态。
- `execution.allow_host` 支持配置热应用；关闭时立即收紧能力并淘汰按旧可见性构建的空闲 Runner，运行中 Runner 完成本轮后淘汰。
- 活动 Run 存在时修改执行策略返回 `409 SESSION_BUSY`，避免在一轮工具调用中途改变审批语义。
- 删除会话时同步删除其执行策略。

## 当前边界

- 本提交只建立最小权限状态和控制 API，不包含宿主命令执行器。
- `ask/auto/full` 对 `execute` 与 `execute_host` 的实际门禁行为在执行器提交中接入。

## 验证

- 验证默认策略、持久化、非法模式拒绝和会话删除清理。
- 验证服务端关闭时拒绝开启宿主执行，开启时可保存并查询 `full/true`。
- `GOTOOLCHAIN=local go test ./...`
