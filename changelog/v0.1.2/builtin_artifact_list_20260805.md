# 内置工具补全：artifact.list（产物列举）

> 日期：2026-08-05　模块：tool/builtin

## 为什么改

真实模型联调时发现能力缺口：用户问"查一下当前产物列表"，leader 如实回答"没有列举产物的能力"——artifact 命名空间只有 put/get，get 需要具体 ID，缺少最小完备的列举工具。store 层 `ListArtifacts` 早已存在（测试在用），补一个薄工具即可打通这条高频演示链路。

## 内容清单

- `internal/tool/builtin/builtin.go`：新增 `artifact.list` 工具（risk=low，幂等）——返回产物元数据清单（artifact_id/media_type/summary/metadata/size/created_at，按创建时间倒序），参数 `limit`（默认 20，上限 100）；**不返回内容本体**（内容仍只能经 artifact.get 按 ID 读取，保持引用传递纪律）。
- 包文档与注册表更新为 9 个内置工具。
- `internal/tool/builtin/builtin_test.go`：契约测试 8→9 + risk 映射补全；新增 `TestArtifactList`（空列表/倒序/无 content/limit 上限）。

## 影响面

- 模型侧新增净化名 `artifact_list`；安全门禁按 risk=low 直接放行（无审批）。
- 无跨模块接口变更；前端工具目录页自动可见（内置组 5→6 个）。

## 测试内容与标准

- `go test ./internal/tool/builtin/` 全过（含新用例）；全仓 `go test ./... -count=1` 26 包全绿；gofmt/golangci-lint 零告警。

## 遗留 TODO

- 无。下一步真实模型复测"查一下当前产物列表"应直接命中 artifact.list。
