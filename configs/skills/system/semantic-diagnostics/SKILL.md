---
name: semantic-diagnostics
description: 使用受控宿主执行采集 Semantic Server、Go、Docker 与操作系统的只读诊断信息
category: system
when_to_use: Semantic 服务启动、模型工具执行或 Docker 运行异常，需要收集本机诊断报告时
tags: [semantic, diagnostics, host, docker, health]
---

# Semantic 只读诊断

## 前置条件

- 本技能必须使用 `execute_host`；不要在 Docker `execute` 中运行；
- Server 必须允许宿主执行，并由用户在当前会话显式开启；
- `ask` 和 `auto` 模式下，每次运行仍需用户批准；
- 诊断只读取系统与服务状态，不修改配置、不重启服务、不清理文件。

## 执行步骤

1. 调用 Skill 时会返回本技能的 base directory，取其中 `scripts/diagnose.py` 的绝对路径。
2. 调用 `execute_host`，工作目录使用 Project workspace 根目录：

   ```text
   python3 <base-directory>/scripts/diagnose.py \
     --server-url http://127.0.0.1:8080 \
     --json-output .semantic-output/diagnostics.json \
     --markdown-output .semantic-output/diagnostics.md
   ```

3. 根据实际 Server 地址调整 `--server-url`；未知时保留默认地址并如实报告不可达。
4. 检查 `exit_code` 和输出摘要。单个检查失败不会让脚本整体失败，报告中会记录错误。
5. 只有需要交给用户、其他 Agent 或长期保存时，调用 `artifact.register` 登记 JSON/Markdown 报告。

## 禁止事项

- 不运行 `docker system prune`、`kill`、`systemctl restart` 等修改性命令；
- 不读取或输出模型 Token、环境变量明文、配置中的密钥；
- 发现异常后先汇报，不自行修复。
