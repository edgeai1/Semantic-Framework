# Semantic Framework

[English](README.md) | [简体中文](README.zh-CN.md)

🧭 The control plane of Semantic. Server manages projects, tasks, registries, and runtime coordination; Pilot connects a Robot execution environment to Server. The `semantic` CLI handles configuration and runtime management.

## Structure

- `cmd/` — Server, Pilot, and CLI entry points.
- `internal/` · `pkg/` — orchestration, services, contracts, and shared libraries.
- `configs/` — configuration templates; `scripts/` — build and bundle workflows.
- `tests/` · `ci/` — unit/integration gates and build automation.

## 🛠 Build and run

Requires Go **1.23+**, Make, and a Linux development environment.

```bash
make build
make init
go test ./...
make run
```

`make build` produces `.output/bin/semantic-server`, `semantic-pilot`, and `semantic`. Initialization writes configuration under `.output/configs/`. Set `SEMANTIC_ADMIN_PASSWORD` in a local environment or untracked `.env` before initialization; do not publish passwords.

Edit the generated `.output/configs/semantic-server.yaml` for your instance rather than changing shared templates. Default development endpoints are HTTP `127.0.0.1:8080` and WebSocket `127.0.0.1:8081`; the Studio is a separate project.

## Use the outputs

A server build alone does not provision a Robot. For the complete stack, follow quick-start's workspace layout and pinned manifest: register native MuJoCo, build AbilityFramework/SDK Wheels, then assemble and activate a Robot Bundle.

From this repository, after those inputs exist:

```bash
python3 scripts/refresh_v050_mujoco.py --help
```

Use its `build` workflow with the Robot Python **3.13** interpreter. Publish the required Skill packages through its separate `publish` workflow after Server starts. Stop affected scenes and Robots before replacing active bundles.

## Troubleshooting

- Missing or offline Robot: verify Pilot connectivity, active catalog, Runtime registration, and published Skill versions.
- Configuration changes do not apply: check the generated configuration used by the running process.
- Unit tests do not cover the entire product: simulation gates require assets and sibling repositories; model-provider gates can make billable calls.
- Bind development ports to trusted interfaces and keep tokens out of commits.

[Detailed technical reference](README.reference.md) · [Configuration templates](configs/) · [Build workflows](scripts/)

## License

Copyright 2026 InsightOS. First-party code: [Apache-2.0](LICENSE). See [NOTICE](NOTICE) and [license scope](LICENSE_SCOPE.md) for third-party components and assets.
