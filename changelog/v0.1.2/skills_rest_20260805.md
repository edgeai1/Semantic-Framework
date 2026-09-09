# R11 技能库 REST 端点：GET /skills 清单 + GET /skills/{name} 详情

日期：2026-08-05
里程碑：v0.1.2 / R11（前端技能库页 · 后端数据面）
分支：feature/skill-system

## 为什么做这次变更

R9 落地了 skill store（SKILL.md 快照 + 热更），但技能只经内核 middleware
面向模型注入，用户无法在 UI 看到"系统当前有哪些技能、技能内容是什么"。
R11 前端技能库页需要只读数据面，本 MR 在 HTTP 网关补两个只读端点：
清单（frontmatter 四标准字段，不含正文）与详情（含正文与扩展字段透传）。
严格最小必要：只读、不暴露服务端文件系统路径（Dir）、不做分页（技能量
为几十量级，与 agents 端点同形态）。

## 包含内容

### 1. skills 处理器（`internal/server/http/handlers/skills.go`，新）

- `SkillsHandler{skillsFn func() *skill.Store}`：**provider 间接取值而非
  固定指针**——skills.dir 热重载的补建路径（启动时技能目录不可用 →
  运行中经配置热重载补建，见 `bootstrap.applySkillsDir`）会替换 store
  实例，固定指针会永久读到 nil 快照；provider 保证端点始终读最新一份。
- `GET /api/v1/skills` → `{skills: [{name, category, description,
  when_to_use}]}`：按 category 分组排序（组间 category 升序、组内 name
  升序，渲染稳定）；**清单不含正文**（列表页摘要展示，正文按需经详情
  获取）。无技能形态返回 `{"skills": []}` 而非错误（与 agents 未配置
  Team 同语义）。
- `GET /api/v1/skills/{name}` → `{skill: {name, category, description,
  when_to_use, body, extensions?}}`：frontmatter 标准字段 + markdown
  正文 + **Extensions 扩展字段原样透传**（无扩展时 omitempty 省略）；
  `Dir` 是服务端文件系统路径，不暴露。未命中（含无技能形态）404
  `SKILL_NOT_FOUND`（统一错误格式）。

### 2. runtime 读取面（`internal/agent/runtime/service.go`）

- 新增 `Service.SkillStore()` 导出 getter：与 `SetSkillStore` 同一把锁，
  运行中热重载替换实例后读者始终拿到最新一份。这是 skills REST 的
  数据源（handler 的 provider 即 `agentRT.SkillStore` 方法值）。

### 3. 路由与装配

- `internal/server/http/router.go`：`NewRouter` 增加 `skillsH` 参数；
  `/skills` 注册进**受保护路由组**（Bearer 鉴权，无 token 401）。
- `internal/bootstrap/wire_access.go`：装配
  `handlers.NewSkillsHandler(agentRT.SkillStore)` 并传入路由。
- `internal/server/http/handlers/doc.go`：包文档补技能库域说明。
- `docs/api/rest.md`：新增 `## skills` 段（两端点契约 + 空态/404 语义）。

### 4. 单测（`internal/server/http/handlers/skills_test.go`，httptest）

真实 skill store（临时目录三技能夹具：跨 embodied/general 两 category，
含扩展字段与正文）+ chi 路由，覆盖：清单分组排序与字段裁剪（无 body/
extensions）、无技能形态空数组（非 null）、详情字段完整性与扩展透传
（goal 字符串 + safety_rules 列表）、无扩展技能详情省略 extensions、
404 SKILL_NOT_FOUND、无技能形态详情 404、**provider 换实例**（nil →
真实 store，补建路径缩影，端点无需重装配即读到新快照）。
`router_test.go` 的装配调用同步更新。

## 影响面

- `internal/server/http/handlers`：+skills.go/skills_test.go，doc.go 同步。
- `internal/server/http`：NewRouter 签名 +skillsH（仅 wire_access 与
  router_test 两个调用点，已同步）。
- `internal/agent/runtime`：+SkillStore() getter（纯新增，存量路径不变）。
- `internal/bootstrap`：wire_access 一行装配 + 一行注释。
- `docs/api/rest.md`：+skills 段。
- 无新增第三方依赖；无配置项变更。

## 测试内容与标准

- 单测（-race）：handlers 与 http 包全绿（含上述 7 个 skills 用例）；
  runtime 包全绿。
- 门禁：gofmt 无输出、go vet 通过、golangci-lint 零告警、
  `go test ./... -race` 全绿、`make build` 成功。
- 联调实测（与前端 R11 同轮，`SEMANTIC_LLM_DEFAULT=mock` 起 server +
  vite 代理 :3000，admin/admin123 登录取 token）：
  - 无 token GET /skills 与 GET /skills/echo-guide → 均 401
    `AUTH_TOKEN_REQUIRED`（受保护路由佐证）；
  - GET /skills 200：两个种子技能（artifact-usage、echo-guide），
    general 组内 name 升序，条目仅四标准字段无 body；
  - GET /skills/echo-guide 200：frontmatter 四字段 + body 原文完整，
    无 extensions 键（种子技能无扩展字段，omitempty 生效）；
  - GET /skills/artifact-usage 200；GET /skills/no-such → 404
    `SKILL_NOT_FOUND`；
  - server 启动日志确认"技能存储已加载 skills=2"与热更监听启动；
  - 实测完毕 server 与 dev 进程均已停止。

## 遗留 TODO

- 技能清单无分页/搜索：技能量为几十量级，后续量级增长或匹配信号
  落地后再评估（skill.search 工具已是模型侧检索面）。
- `extensions` 原样透传的展示消费在前端 R11（字段表 JSON 渲染）；
  具身扩展字段的框架消费仍是 Phase 3（与 R9 同）。
- 前端页面级浏览器手测（选中高亮、markdown 排版）留待验收轮
  （本轮联调为 API 级 + vite 模块编译级，环境无 headless 浏览器）。
