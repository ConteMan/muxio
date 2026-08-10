# 项目地图

这是一份给人和 Agent 的快速入口，只描述主路径和边界；行为细节以 Design、ADR、Spec 和 OpenAPI 为准。

## 当前可运行路径

```text
cmd/muxio
  → internal/cli
    → version
    → serve
      → internal/api
        → GET /healthz
        → GET /readyz
        → GET /api/v1/status
```

`api/openapi.yaml` 是 HTTP 行为的公开合同。Handler 与契约必须在同一 PR 变更。

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

`web/` 与 Go 核心同仓，但独立构建、运行和发布。它只能消费 `/api/v1`，不得读取 SQLite、本地文件或 Go 内部类型。当前尚未创建前端工具链，避免未使用依赖提前老化。

## 文档控制面

- Design：系统当前应该怎样工作。
- ADR：为什么选择某个难以轻易逆转的方向。
- Spec：一个可独立实现、评审和验收的变更单元。
- Roadmap：范围和里程碑，不记录逐项任务状态。
- GitHub Issue / PR：实际任务、进度、证据和协作状态。
