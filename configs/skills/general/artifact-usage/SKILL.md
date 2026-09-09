---
name: artifact-usage
description: 产物（artifact）的存取规范：artifact.put 保存内容与审批语义，artifact.get 读取产物
category: general
when_to_use: 需要保存报告/导出结果/留存文件，或读取已保存产物的详细内容时
tags: [artifact, storage, approval]
---

# 产物存取规范

## 何时使用

- 需要把较长的结果（报告、导出内容、大段文本）保存下来交付用户时：用 `artifact.put`；
- 需要查看已保存产物的完整内容（如上下文中被截断外置的产物引用）时：用 `artifact.get`。

## artifact.put（写产物，risk=high）

1. **必须经人工审批**：该工具会触发审批中断，先向用户说明要保存什么，再发起调用；
2. 参数：`content`（内容本体，必填）、`media_type`（媒体类型，默认 text/plain）、
   `summary`（一句话摘要，缺省自动截取内容开头）、`metadata`（来源、标签等附加信息）；
3. 审批被拒绝时不要以同一内容反复重试，先与用户确认意图；
4. 返回的 `artifact_id` 与 `uri` 是后续引用的凭证，回答中应带上摘要与引用。

## artifact.get（读产物，risk=medium）

1. 按 `artifact_id`（art- 前缀）读取产物的元数据与内容本体；
2. 上下文中出现产物引用（如大段工具结果被截断外置）时，用它取回全文再继续处理；
3. 产物不存在会返回 ARTIFACT_NOT_FOUND，不要反复重试，向用户确认产物 ID。
