# Conversation Runtime Foundation

## 为什么改

对话中断后会把空 assistant 消息重放给模型并触发 400；工具与 SubAgent 归因、运行活动持久化、
图片输入、模型服务密钥和 Agent 模型策略也缺少完整契约，前端无法稳定恢复和调试一次运行。

## 影响面

- Agent runtime/kernel：可取消 run，多模态消息，reasoning/tool call 事件，运行活动元数据；
- chat/store/WS/REST：会话安全删除，附件归属与读取，历史恢复；
- LLM/profile：服务级密钥共享，主模型/D3、推理 effort 与可见性；
- Web：单按钮发送/停止、运行中纠偏、图片、工具参数/结果、SubAgent 正确归因、可调 IDE 三栏；
- 文档：API、配置、Skill 优先级、Robot Agent 伴生需求与多 Agent 待确认边界。

## 测试与标准

- `go test -race ./...`
- `go vet ./...`
- `npm test -- --run`（19 files / 165 tests）
- `npm run lint`
- `npm run build`
- 运行时主文件 736 行、前端 chat store 791 行，均低于单文件 800 行门禁。
