# 实现规格

一份 Spec 是一个可独立实现、评审和验收的工作单元。实际任务状态使用 GitHub Issue / PR；此处只记录长期有效的行为合同与实现状态。

| 编号 | 标题 | 状态 |
|---|---|---|
| [001](001-foundation.md) | 可维护项目基础 | 已实现 |
| [002](002-storage-foundation.md) | 存储地基与幂等导入 | 草稿 |

新增规格复制 [template.md](template.md)，编号递增。

## 执行规则

1. 开工前通读本 Spec、`AGENTS.md` 及 Spec 的关联文档。
2. 一份 Spec 对应一个聚焦 PR；实现 Agent 只拥有被分配的文件范围。
3. 接口合同调整必须回写 Spec，并检查依赖方；公开 HTTP 行为先改 OpenAPI。
4. Spec 与 Design / ADR 冲突或遗漏关键行为时停止实现，由维护者裁决。
5. 完成定义为验收项全过、文档与代码联动、`selftest` 全绿、状态改为“已实现”。
