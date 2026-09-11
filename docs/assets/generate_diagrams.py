#!/usr/bin/env python3
# Copyright 2026 InsightOS
# SPDX-License-Identifier: Apache-2.0
"""Generate the bilingual system architecture diagrams using only the Python standard library."""

from html import escape
from pathlib import Path

ROOT = Path(__file__).resolve().parent
INK = "#172c40"
MUTED = "#52677a"
BLUE = "#3268d8"
GREEN = "#14796b"


class SVG:
    def __init__(self, height, title, description):
        self.parts = [f'''<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="{height}" viewBox="0 0 1200 {height}" role="img" aria-labelledby="title desc">
<title id="title">{escape(title)}</title><desc id="desc">{escape(description)}</desc>
<defs>
  <marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto-start-reverse"><path d="M0 0 L8 4 L0 8 Z" fill="#72899f"/></marker>
  <linearGradient id="background" x2="1" y2="1"><stop stop-color="#f3f7ff"/><stop offset="1" stop-color="#eef9f4"/></linearGradient>
</defs>
<style>text {{ font-family: Inter, 'Noto Sans CJK SC', 'Microsoft YaHei', Arial, sans-serif; }} .title {{ font-weight:700; }} </style>
<rect width="1200" height="{height}" rx="24" fill="url(#background)"/>
''']

    def rect(self, x, y, w, h, fill="white", stroke="#d6e1ec", radius=16):
        self.parts.append(f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="{radius}" fill="{fill}" stroke="{stroke}"/>')

    def text(self, x, y, text, size=22, color=INK, bold=False, anchor="start"):
        self.parts.append(f'<text x="{x}" y="{y}" font-size="{size}" fill="{color}" text-anchor="{anchor}" class="{"title" if bold else "body"}">{escape(text)}</text>')


    def path(self, d, dashed=False, both=False, color="#72899f", arrow=True):
        extra = (' stroke-dasharray="7 6"' if dashed else '') + (' marker-start="url(#arrow)"' if both else '') + (' marker-end="url(#arrow)"' if arrow else '')
        self.parts.append(f'<path d="{d}" fill="none" stroke="{color}" stroke-width="2"{extra}/>')

    def save(self, name):
        (ROOT / name).write_text("".join(self.parts) + "</svg>\n", encoding="utf-8")


def architecture(zh):
    title = "Semantic 系统架构" if zh else "The Semantic system"
    s = SVG(910, title, "Studio → Server → Pilot ↔ Robot Skill; Pilot → AbilityFramework → Robot SDK → MuJoCo / hardware.")
    s.text(48, 64, title, 34, bold=True)
    s.text(48, 101, "从目标规划到物理执行，共享任务状态与执行反馈。" if zh else "From goal planning to physical execution, with shared state and feedback.", 20, MUTED)
    s.rect(48, 132, 744, 99)
    s.text(72, 173, "Semantic Studio", 27, BLUE, True)
    s.text(72, 207, "对话 · 计划 · 工作流 · 场景 · 执行记录" if zh else "Conversation · Plans · Workflows · Scenes · Execution", 20, MUTED)
    s.rect(840, 132, 312, 99, "#f4f0ff", "#ded4f5")
    s.text(864, 173, "模型服务" if zh else "Model providers", 25, "#7251a3", True)
    s.text(864, 207, "OpenAI-compatible · Claude", 18, MUTED)
    s.path("M420 234 V273", both=True)
    s.path("M996 234 V273", both=True)
    s.rect(48, 280, 1104, 187, "#e9f0ff", "#bcd0f5")
    s.text(72, 321, "Semantic Server", 27, BLUE, True)
    names = ["Leader", "Plan → Workflow", "Robot Agent"]
    details = ["理解目标与环境", "编排任务与依赖", "选择技能、组织子任务"] if zh else ["Goal & environment reasoning", "Tasks & dependencies", "Skill selection & subtasks"]
    for i, (name, detail) in enumerate(zip(names, details)):
        x = 72 + i * 360
        s.rect(x, 341, 336, 77, "white", "#ccdbf5", 10)
        s.text(x + 16, 371, name, 23, bold=True)
        s.text(x + 16, 401, detail, 18, MUTED)
    s.text(72, 445, "项目 · 语义地图 · 技能注册 · Runtime 管理 · 事件与产物" if zh else "Projects · Semantic maps · Skill registry · Runtime lifecycle · Events & artifacts", 19, MUTED)
    s.text(209, 500, "任务 / 执行事件" if zh else "Tasks / execution events", 17, MUTED)
    s.rect(48, 524, 744, 288, "#f3faf7", "#b9dbce")
    s.path("M182 470 V545", both=True)
    s.rect(72, 552, 220, 78)
    s.text(92, 585, "Pilot", 26, GREEN, True)
    s.text(92, 614, "机器人执行桥梁" if zh else "Execution bridge", 18, MUTED)
    s.rect(412, 552, 356, 78)
    s.text(432, 585, "Robot Skill", 26, GREEN, True)
    s.text(432, 614, "阶段 · 观察 · 动作 · 检查" if zh else "Stages · Observe · Act · Check", 18, MUTED)
    s.path("M295 590 H405", both=True)
    s.text(350, 572, "SDK", 16, MUTED, anchor="middle")
    s.path("M182 634 V686", both=True)
    s.text(207, 667, "Action", 17, MUTED)
    s.rect(72, 694, 292, 78)
    s.text(92, 727, "AbilityFramework", 25, GREEN, True)
    s.text(92, 756, "运行 Ability 能力实例" if zh else "Runs Ability instances", 18, MUTED)
    s.rect(448, 694, 320, 78)
    s.text(468, 727, "Robot SDK", 25, GREEN, True)
    s.text(468, 756, "状态 · 控制 · 传感器" if zh else "State · Control · Sensors", 18, MUTED)
    s.path("M368 733 H440", both=True)
    s.text(72, 797, "机器人执行环境 · Robot Bundle" if zh else "Robot execution environment · Robot Bundle", 17, GREEN)
    s.rect(840, 524, 312, 288, "#fff9ee", "#ead3a8")
    s.text(864, 565, "环境与设备" if zh else "Environment & devices", 23, "#926423", True)
    s.rect(860, 588, 272, 79, "white", "#eadfc9", 10)
    s.text(878, 620, "MuJoCo Runtime", 23, bold=True)
    s.text(878, 649, "场景 · 资产 · 物理仿真" if zh else "Scenes · Assets · Physics", 18, MUTED)
    s.rect(860, 696, 272, 79, "white", "#eadfc9", 10)
    s.text(878, 728, "真实机器人" if zh else "Physical robots", 23, bold=True)
    s.text(878, 757, "型号适配器 · 硬件 Backend" if zh else "Model adapters · Backends", 18, MUTED)
    s.path("M772 733 H852", both=True)
    s.path("M814 733 V627 H852")
    s.text(48, 863, "反馈沿执行链返回；Studio 汇集场景状态、技能进度和执行产物。" if zh else "Feedback returns along the execution chain; Studio brings scene state, skill progress, and artifacts together.", 19, MUTED)
    s.save(f"architecture-{'zh-CN' if zh else 'en'}.svg")


if __name__ == "__main__":
    for chinese in (False, True):
        architecture(chinese)
    print("Generated two system architecture diagrams.")
