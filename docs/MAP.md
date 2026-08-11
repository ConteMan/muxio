# 项目地图

这是一份给人和 Agent 的快速入口，只描述主路径和边界；行为细节以 Design、ADR、Spec 和 OpenAPI 为准。

## 当前可运行路径

```text
cmd/muxio
  → internal/cli
    → version
    → serve
      → internal/store/sqlite     opened for the process lifetime
      → internal/webui            embedded panel, serves unmatched paths
      → internal/api
        → GET /healthz                        process liveness
        → GET /readyz                         storage reachability
        → GET /api/v1/status
        → GET /api/v1/sources
        → GET /api/v1/runs[/{id}][/events]
        → GET /api/v1/config
        → PUT /api/v1/config          conditional write, If-Match required
    → db path
      → internal/paths            data directory resolution
    → import
      → internal/app              import use case
        → internal/record         normalization and content hash
        → internal/run            run state machine and events
        → internal/logging        structured logs to stderr
        → internal/store/sqlite   migrations and transactions
    → runs [show <id>]
      → internal/store/sqlite     run history and events
    → config path|show|init|set
      → internal/config           file-backed settings and validation
```

`api/openapi.yaml` 是 HTTP 行为的公开合同。Handler 与契约必须在同一 PR 变更。读路径与配置写入已进入 HTTP API；采集数据的写入仍只经 CLI。

## 目标采集路径

```text
source configuration
  → scheduler / manual trigger
  → built-in connector
  → candidate stream
  → transaction + deduplication
  → immutable capture + checkpoint
  → search / API / CLI / export
```

该路径尚未实现，范围和顺序见 [roadmap.md](roadmap.md)。连接器不负责调度、重试、运行状态或数据库事务。

## Web 边界

`web/` 与 Go 核心同仓，是独立工程：独立依赖、构建、类型检查与测试。它只能消费 `/api/v1`，不得读取 SQLite、本地文件或 Go 内部类型。

构建产物输出到 `internal/webui/assets/` 并提交进仓库，随二进制嵌入交付（[ADR-007](decisions/007-embedded-web-ui.md)）。因此 `go build` 不需要 Node，而改动前端后必须重建并提交产物——`selftest` 会校验产物与源码一致。

客户端类型由 `openapi-typescript` 从 `api/openapi.yaml` 生成，门禁校验其与合同同步。

## 文档控制面

- Design：系统当前应该怎样工作。
- ADR：为什么选择某个难以轻易逆转的方向。
- Spec：一个可独立实现、评审和验收的变更单元。
- Roadmap：范围和里程碑，不记录逐项任务状态。
- GitHub Issue / PR：实际任务、进度、证据和协作状态。
