---
name: depalletizing-workflow-planning
description: 根据当前场景和资源规划拆码垛 Workflow、搬箱 Task 及其依赖
category: workflow
when_to_use: 用户要求搬运单个周转箱、一层周转箱或完整托盘时
tags: [workflow, depalletizing, planning, robot]
---

# 拆码垛 Workflow 规划

本 Skill 只帮助 Leader 理解业务和生成主要 Task，不执行 Robot Skill，也不生成
Stage、Action、Ability 或关节轨迹。

## 规划原则

- 对“当前最上面一层搬到对应位置”这类短请求，先用 `map.query` 查询当前 Project 的
  托盘、箱体、Region 和 Robot（例如 `entity_types: [pallet, tote, region, robot]`，带有效过滤；
  结果达到 limit 时按类型、区域或实体收窄查询，不能视为全集）。先按当前位置/支撑关系归属托盘，再按几何高度选择各来源栈顶箱体，
  不能选取已搬至目标托盘但仍保留旧层号名称的箱体。以当前槽位身份、行列关系匹配目标。
- 已明确来源和目标且地图足够时直接提交 Proposal，不再次询问其名称。若目标已占用，
  根据当前支撑/容量信息与用户目标判断是否可叠放；缺少容量依据或用户未授权的层位选择
  确实影响方案时才询问。不能为了短 Prompt 的“一次通过”跳过业务歧义或安全校验。
- 地图查询缺少过滤时按返回错误补充过滤；不要退回历史 Artifact/目录扫描猜测当前场景，
  更不要递归读取本轮产生的 reduction 产物作为新证据。

- Semantic Map 是场景记忆和规划参考。对象、槽位和 Robot 数量必须来自当前
  Project 上下文或 `map.query`，不能写死为 1、4 或 12。
- 目标明确且当前 Map 上只有一个匹配实体时直接生成 Plan Proposal；存在多个
  真实候选时先查询，仍无法选择才用一次结构化 `interaction.ask`。
- 一个 Robot Task 表达一台 Robot 搬运一个业务对象到一个目标堆叠列。抓取、
  导航、放置是该 Task 的 SubTask，不是 Leader Task。
- 来源依赖按可访问顺序建立：同一垂直栈中以实时几何中心高度最高者作为当前顶层，
  不能从 `l1/l2/l3` 等名称猜测上下关系；被上层箱体遮挡的箱体必须等待其上方箱体搬走。
- 目标托盘的堆叠列从当前 Map 的实体与 Region 中识别。选择兼容且未达到 `max_layers` 的列；
  同列下一层依赖上一层放置完成。实际放置高度由实时感知计算。
- 多 Robot 可以并行执行资源不冲突的 Task；单 Robot 或 Robot 忙碌时由现有调度
  等待，不要为了并行而复制 Robot 或提前绑定忙碌设备。
- Proposal 只保存对象/槽位身份、必要位姿提示、允许 Skill 范围和完成标准。
  通过 `map.query` 选中实体后，Task input 必须把查询结果中的稳定业务 `name` 原样
  保存为来源 `object_ref` 和目标 `target_ref`；`entity_id` 只作可选规划来源。不能只
  保存 `r1-c2` 等局部行列标签，也不能让 Robot Agent 猜测或拼接引用。Map pose 不是
  Robot Skill 执行成功的证明。
## Task 字段

- Robot Task 的 `required_role` 固定写机器值 `robot`，不能写显示名称 `Robot Agent`。
- `resource_requirements` 只表达用户明确要求的可调度资源：`robot_ids`、`robot_models`、
  `backends` 和 `workspace_write`。`backends` 只能填写Robot实际目录中的实现标识，例如
  `mujoco` 或 `fake`；“仿真/simulation”是环境类别，不是backend约束，应当省略。箱体、
  槽位、工具和导航目标属于业务输入，必须写入 Task `input`。
- `required_capabilities` 只能列出 `approved_scope.allowed_skills` 中已经批准的 Robot Skill，
  工具组合、抓取方式等业务要求写入 Task `input`，不要发明新的能力名称。
- 常规拆码垛若明确使用现有三个 Robot Skill，应同时把
  `semantic-navigation`、`grasp-object`、`place-object` 写入
  `approved_scope.allowed_skills` 和 Robot Task 的 `required_capabilities`。版本仍由批准后
  的实际 Robot目录固定，不在 Leader Task中猜测。
- 如果计划有意保留 Robot Skill后绑定，就同时省略 `approved_scope.allowed_skills` 和
  `required_capabilities`。绝不能把“来源导航”“抓取并整理携物姿态”等业务步骤写成
  `required_capabilities`；这些步骤属于 Task目标或批准后由 Robot Agent生成的 SubTask。

## 完成标准

每个搬箱 Task 至少验证：目标对象位于目标列的正确层、放置稳定、Robot双工具
为空且恢复行走姿态。Workflow 完成后再由 Leader 汇总所有 Task 的真实结果和证据。
