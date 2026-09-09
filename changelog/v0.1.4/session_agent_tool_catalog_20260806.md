# 会话 Agent 有效工具目录

- 新增 `GET /api/v1/chat/sessions/{id}/agents/{agent_id}/tools`，返回指定会话中指定 Agent 真正可用的工具，而不是全局安装目录。
- 有效工具计算复用运行器的 Profile 命名空间、MCP、ToolSearch 和会话宿主权限规则。
- 目录现在包含 Eino Middleware 注入的 `skill`、`ls`、`read_file`、`glob`、`grep`、`tool_search`，以及 `ask_query` 等运行时 AgentTool。
- 响应区分直接注入、动态检索、Middleware 和 AgentTool，并同时返回领域工具名与模型侧安全化名称。
- 增加接口回归测试，验证 Leader 可以看到查询委派和 Project 文件工具，Query 不会递归获得自己的委派入口。
