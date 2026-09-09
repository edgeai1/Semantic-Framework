# Docker 沙箱镜像前置条件提示

日期：2026-08-05

## 变更

- 显式固定 `execute` 使用的 Eino-ext DockerSandbox 默认镜像为 `python:3.9-slim`。
- 镜像未安装时返回稳定的 `DOCKER_IMAGE_MISSING`，并提示执行 `docker pull python:3.9-slim`。
- 不在模型调用过程中自动拉取镜像，也不回退到宿主执行，继续保持两个执行后端相互独立。

## 验证

- 单元测试覆盖镜像名称传递、缺少镜像的错误码、安装提示与失败清理。
- 安装固定镜像后，真实 DockerSandbox `data-profile` 集成测试通过。
