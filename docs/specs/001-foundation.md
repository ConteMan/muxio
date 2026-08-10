# Spec 001：可维护项目基础

- 状态：已实现
- 关联：Roadmap M0、ADR-001、ADR-002

## 问题

Muxio 需要一个能够持续保持可编译、可验证、可交接的起点，让人和 Agent 在共同合同下增量开发。

## 行为

- `muxio version` 输出当前构建版本。
- `muxio serve` 默认在 `127.0.0.1:8080` 启动服务，并拒绝非 loopback 地址。
- `/healthz`、`/readyz` 与 `/api/v1/status` 返回 JSON 状态。
- `api/openapi.yaml` 描述全部已公开端点。
- 根目录 `selftest` 是本地和 CI 的统一质量门禁。
- README、Design、ADR、Roadmap、Spec、Issue 和 PR 模板形成项目协作控制面。

## 边界与非目标

- 不实现 SQLite、采集、搜索、导出和连接器。
- 不初始化前端依赖，只建立 `web/` 工程边界。
- 不建立发版、部署或远程访问能力。

## 验收

- `go run ./cmd/muxio version` 输出 `muxio dev`。
- 启动后三个已声明 HTTP 端点返回 200，OpenAPI 校验通过。
- `./scripts/selftest.sh` 在干净仓库通过。
- GitHub Actions 在 push 与 pull request 上运行同一门禁。
