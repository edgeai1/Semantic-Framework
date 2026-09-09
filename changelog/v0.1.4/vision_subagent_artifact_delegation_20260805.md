# 非视觉 Leader 委派图片 Artifact

日期：2026-08-05

## 变更

- 用户带图时不再强制要求 Leader 自身模型具备视觉能力。
- 视觉 Leader 继续直接接收图片本体；非视觉 Leader 只接收已校验的 ArtifactRef、媒体类型和摘要，可据此委派视觉 SubAgent。
- 切换到非视觉模型后，历史图片自动降为文本引用，避免旧多模态消息导致新端点能力错误。
- 图片本体仍只通过当前 Run 的 ArtifactRef 白名单传给目标 SubAgent，不共享 Leader 私有历史。

## 验证

- 视觉模型仍接收 Eino 多模态消息。
- 文本模型只接收图片 ArtifactRef，不包含 Base64 图片本体。
- 组合验收覆盖文本 Leader 调用 `ask_query`，视觉 Query 使用自己的会话模型读取同一图片 ArtifactRef。
- 跨用户图片引用继续被拒绝。
