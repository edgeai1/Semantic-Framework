# Agent 与 Project Skill 有效集

日期：2026-08-05

## 变更

- Agent Profile 新增 `skills.allowlist`，空列表表示该 Agent 不加载 Skill。
- 会话运行时使用 Agent allowlist 与 Project Skill 绑定计算有效集；Project 空绑定表示不增加 Project 级限制，非空绑定只能继续收窄。
- 新增全局 Skill Store 的只读过滤视图，不复制正文和快照；Skill 热更新后，同一授权视图仍读取最新内容。
- Leader 默认允许当前交付的四个标准 Skill，Query 与本版本 Monitor 默认不加载执行类 Skill。
- Leader 工具命名空间不再声明 `skill.*`；标准 Skill 正文由 Eino Skill Middleware 的 `skill` 工具渐进加载。
- Agent 目录中的 Skill 列表改为展示 Profile 允许且当前已安装的 Skill，不再把全局 Skill 清单误报为每个 Agent 均可使用。
- SubAgent 也按自己的 Profile 和当前会话 Project 解析 Skill，不继承 Leader 的有效集。

## 验证

- 验证 Profile Skill 白名单去空、去重并保留人工配置顺序。
- 验证 Agent 白名单、Project 空绑定和 Project 非空绑定三种有效集语义。
- 验证 Project 不能给 Agent 增加 Profile 未授权的 Skill。
- 验证过滤视图跟随全局 Store 热更新，不保留过期正文快照。
- 验证 Leader、SubAgent 和 Observer 的运行器装配均使用有效 Skill 视图。
- `GOTOOLCHAIN=local go test ./...`
