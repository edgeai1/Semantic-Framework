> Historical technical reference / 历史技术参考。For current build and usage instructions, see [English](README.md) / [中文](README.zh-CN.md). Version-specific examples below are not a current release manifest.

# semantic-framework

Semantic Framework 是 Semantic 系统的 Server、Agent Runtime、CLI 与 Pilot 运行基座。

当前 v0.5 功能分支建立在已合入 develop 的 v0.4 仿真基线上。场景和 Simulation
Runtime 仍由 SimulationService 管理；Robot 执行链不创建第二套场景生命周期。

## 二进制

| 二进制 | 说明 |
|---|---|
| semantic-server | HTTP/WS、Agent Runtime、模型、工具、Skill、Project、Conversation 与运行数据 |
| semantic-pilot | 每台 Robot 的 Skill Worker、Ability 精确路由、停止、状态与 Artifact 桥接 |
| semantic | 初始化、诊断、Runtime 安装清单和本地管理 CLI |

## 快速开始

~~~bash
make build
make test
make lint
make run
~~~

构建和测试产物统一写入 `.output/`。`make run` 使用 `.output/` 安装实例启动
Server；另开终端执行 `make logs` 可以持续查看
`.output/logs/semantic-server.jsonl`。终端仍会同时输出相同的结构化日志。

## v0.5 Robot 执行

`robot.run` 通过 Server 将已安装的 Robot Skill 交给目标 Pilot。Pilot 启动隔离
Worker，并按 Robot ID、语义 Action 和 Ability instance UUID 精确调用该 Robot
自己的 AbilityFramework。Ability 通过环境中安装的 Robot SDK Wheel 控制真机或
虚拟 Robot；Robot Skill 和 Server 都不直接调用 Robot SDK。

Robot 正式接入不再使用管理员登录 Token。设备中心创建 5 分钟有效的一次性加入码，类型包启动器通过局域网 mDNS 自动发现 Server，并把加入码换成绑定 Pilot ID 的专用 credential。首次成功后 credential 只保存在 Robot 实例的 `connection.yaml`；后续启动不再配对。RobotDeployment 同时声明 Robot SDK、AbilityFramework、七类 Ability、Pilot 和 desired Robot Skill，启动器负责完整装配，不要求逐项上传和安装。

Server 广播 `_semantic-server._tcp.local.` 时只包含 Server ID、显示名、API 版本和 HTTP/WS 端口，不包含凭据。mDNS 不可用时，启动器仍可显式指定 Server HTTP/WS 地址。

`semantic-robot-instance stop` 只关闭一台 Robot 的运行实例，不停止场景：

1. Pilot 锁存停止，禁止新 Action，并同时停止 Worker、活动 Ability 和 Robot SDK 命令；
2. Robot SDK 返回命令已停止和 hold 证据；
3. Pilot 保存结构化安全退出结果并正常退出；
4. 启动器再停止七个 Ability，等待 heartbeat 消失，最后停止 AbilityFramework；
5. 任何一层无法确认时实例进入 `interrupted`，不伪报 `stopped`。

这条流程由具体 Robot SDK 后端完成物理差异，因此 Fake、真机和仿真共用上层流程。
场景 reset、stop、切换与 Project 释放仍由 v0.4 SimulationService 管理；它先协调
Robot 到安全边界，再调用已有场景生命周期，不建立第二套 Runtime stopper。

本地完整 Fake 产品 Gate：

~~~bash
make test-v050-real-gate
~~~

Gate 使用真实 Server、生产构建的 Web、两个 Pilot、两个 AbilityFramework、十四个
Ability 进程、三个 Robot Skill Worker 和两个隔离 Fake Robot。证据写入
`.output/v050-real-gate/`。本 Gate 不启动 MuJoCo；仿真模型确认后再接
SimulationService 与虚拟 Robot 描述。

## 配置

仓库 configs/ 是不可变模板。semantic init 默认把运行配置安装到 ~/.semantic/；Server 使用 -c 或 SEMANTIC_CONFIG 选择配置文件，设置接口只修改 Server 实际加载的文件。

仿真运行环境使用 `simulation.runtimes_dir` 下的严格 YAML 清单登记。
`RuntimeInstallation` 描述这台机器已经准备好的运行环境，`RuntimeProfile`
描述能力；浏览器不能传入 Shell 命令、宿主路径或安装依赖。

~~~bash
# 第一次安装 Semantic Server
semantic init

# 在线 Registry 或离线 Pack 安装 Native MuJoCo（二选一）
semantic runtime install native-mujoco@0.4.0 \
  --asset-root /data/semantic/mujoco-assets
semantic runtime install \
  --pack semantic-native-mujoco-0.4.0.runtime.tar.zst \
  --asset-root /data/semantic/mujoco-assets

# 静态检查全部安装；--smoke 会真实启动最小场景后再停止
semantic runtime doctor --all
semantic runtime doctor --id local-native-mujoco --smoke
semantic runtime doctor --all --release
~~~

一次安装会完成以下工作：校验 Pack 与每个 Wheel 的 SHA256、建立固定 Python
版本的隔离 uv 环境、安装离线 Wheelhouse、登记外部内容、生成
`~/.semantic/runtimes.d/*.yaml`、运行 health 与最小场景 smoke。成功后
Server 重启不再需要导出 Runtime 环境变量。

~~~bash
# robosuite 只登记正式 Franka Model Bundle
semantic runtime install robosuite-1.5@0.4.0 \
  --model-root /data/semantic/models/franka-panda

# LIBERO/Pro 内容不下载、不复制，只在安装时确认一次路径和许可
semantic runtime install libero-robosuite-1.4@0.4.0 \
  --libero-root /data/LIBERO \
  --libero-pro-root /data/LIBERO-Pro \
  --model-root /data/semantic/models/franka-panda \
  --accept-license LIBERO --accept-license LIBERO-Pro
~~~

常用管理命令：

~~~bash
semantic runtime list
semantic runtime test-start --id local-native-mujoco
semantic runtime upgrade --id local-native-mujoco native-mujoco@0.4.1
semantic runtime uninstall --id local-native-mujoco
~~~

`upgrade` 自动复用当前 installation 已登记的内容路径和许可确认，先建立新环境并
smoke，成功后才切换清单；失败时旧版本保持可用。`doctor --release` 会拒绝
开发源码安装，可直接用于 RC/正式制品门禁。
`uninstall` 在任何 Project（包括归档 Project）仍永久绑定时拒绝执行，并且永远
不删除外部资产、Robot Model 或 benchmark 数据。目录移动后使用 `doctor`
获得明确诊断，再用同一 installation ID 修复。

remote Runtime 不创建 Python 环境，管理员仍可用严格 manifest 执行
`semantic runtime register --file remote.yaml`。源码只用于开发：

~~~bash
semantic runtime install \
  --dev-source /work/plugin-mujoco --profile native-mujoco \
  --asset-root /work/mujoco_asset \
  --scene-catalog /work/plugin-mujoco/runtime-packs/native-mujoco/catalog
~~~

开发 installation 带有 `development: true`，不允许作为 RC/正式制品验收。

Project 打开后，Server 为其取得 Runtime 租约并在后台启动或连接 Runtime；场景仍由用户显式启动。退出 Project 后，Server 先停止 Scene、位姿流、Sensor 和 Robot 命令，再回收本地受管 Runtime。remote Runtime 只解除 Project 关联。

Runtime Pack 是 CI 从 `plugin-mujoco` 固定 Tag 构建的安装制品，不是源码包。
Pack 不包含 Project、未确认分发的 MuJoCo 资产或 LIBERO 数据，也不允许提供
任意 Shell。场景公共索引与 Layout authoring 模板随 Pack 安装，因此 Runtime
离线时 Studio 仍能浏览兼容场景；Runtime 未安装时干净 Server 也可以正常启动。
