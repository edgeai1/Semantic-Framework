---
name: data-profile
description: 使用 Docker 沙箱分析 Project workspace 中的 CSV/JSON，生成结构化画像与 Markdown 报告
category: data
when_to_use: 用户需要了解表格或 JSON 数据的行列、缺失值、字段类型、唯一值和数值统计时
tags: [data, csv, json, profiling, docker]
---

# 数据画像

## 适用范围

- 输入必须已经位于当前 Project workspace，支持 `.csv` 和 `.json`；
- 本技能只做只读分析，不修改输入文件；
- 使用 Docker `execute`，不要改用 `execute_host`；
- 不要把完整数据集粘贴进模型上下文。

## 执行步骤

1. 确认输入文件在 Project workspace 中的相对路径；路径不明确时先询问用户。
2. 调用 `execute`，在 Project workspace 根目录运行：

   ```text
   python /skills/data/data-profile/scripts/profile.py \
     --input <输入文件相对路径> \
     --json-output .semantic-output/data-profile.json \
     --markdown-output .semantic-output/data-profile.md
   ```

3. 检查 `exit_code`：非零时读取 `stderr`，修正路径或输入格式后最多重试一次。
4. 读取命令输出中的简要 JSON，总结行数、字段、缺失值和明显异常。
5. 只有报告需要进入对话、交给其他 Agent 或长期保存时，分别调用 `artifact.register` 登记：
   - `.semantic-output/data-profile.json`，媒体类型 `application/json`；
   - `.semantic-output/data-profile.md`，媒体类型 `text/markdown`。
6. 普通实验不需要登记 Artifact，报告文件保留在 Project workspace 即可。

## 输出说明

- JSON 是后续程序和 Agent 使用的结构化结果；
- Markdown 是给用户阅读的摘要；
- 数值统计包含计数、最小值、最大值和平均值；
- 类型为基于样本值的保守推断，不替代正式数据契约。
