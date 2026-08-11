# Muxio 文档

| 文档 | 作用 |
|---|---|
| [MAP.md](MAP.md) | 给人和 Agent 的快速项目地图 |
| [design/architecture.md](design/architecture.md) | 系统边界、运行形态和模块职责 |
| [design/data-model.md](design/data-model.md) | 首期 SQLite 数据合同 |
| [design/ui/](design/ui/) | 面板的流程、状态、合同缺口与 Pencil 原型 |
| [decisions/README.md](decisions/README.md) | 已接受的长期架构决策 |
| [roadmap.md](roadmap.md) | 产品范围、里程碑和完成标准 |
| [specs/README.md](specs/README.md) | 可实现规格索引与协作规则 |
| [maintenance.md](maintenance.md) | 质量门禁、版本和发布约定 |

任务进度只放在 GitHub Issues 和 Pull Requests，不在文档中维护第二套任务列表。

`make status` 从上述文档与 Git 的真实状态推导全局视图并直接输出，不落盘、不需要同步，因此不会过期。需要人裁决的决策卡点清单见 [AGENTS.md](../AGENTS.md)。
