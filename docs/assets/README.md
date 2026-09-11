# README illustrations · 项目配图

| Asset / 文件 | Used by / 用途 |
|:--|:--|
| `logo.png` | Brand banner in both READMEs / 中英文 README 的品牌横幅。 |
| `architecture-zh-CN.svg` | System architecture in the Chinese README / 中文 README 的系统架构图。 |
| `architecture-en.svg` | System architecture in the English README / 英文 README 的系统架构图。 |

## Update the diagrams · 更新架构图

From the repository root / 在仓库根目录执行：

```bash
python3 docs/assets/generate_diagrams.py
```

The generator uses Python's standard library and produces the two architecture SVGs. Edit their text and layout in the script, keeping both languages aligned. The supplied `logo.png` is used directly.

生成器只使用 Python 标准库，生成中英文两张架构 SVG。文字与布局在脚本中维护，两种语言同步更新。`logo.png` 直接使用提供的原图。

The architecture diagrams follow the Framework components and the system model in `semantic-docs/docs/architecture/`. Each SVG includes an accessible title and description.

架构图依据 Framework 组件和 `semantic-docs/docs/architecture/` 中的系统模型绘制，每张 SVG 均包含无障碍标题与描述。
