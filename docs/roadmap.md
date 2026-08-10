# Roadmap

> 本文维护范围和里程碑完成标准；具体任务与进度使用 GitHub Issues / Milestones。

## v0.1 范围

### In

- Go 单二进制、SQLite、CLI 和本地 HTTP API。
- 采集源配置、手动运行、固定间隔调度和有界重试。
- file 与 RSS/Atom 两个内置连接器。
- 幂等采集、不可变版本、来源追溯和崩溃恢复。
- 标题与正文全文搜索。
- JSONL 导出。
- CLI 状态表、运行历史、健康检查和结构化日志。
- 独立 Web 的 OpenAPI 边界；核心稳定后实现最小状态面板。

### Out

- Clipboard、ICS、通用 HTTP JSON 和浏览器自动化。
- 外部连接器 SDK、动态插件 ABI。
- MCP Server、Agent 主动执行。
- 摘要、标签、承诺识别等内容理解。
- 附件、大型二进制、多用户、云同步和远程管理。
- 浏览器扩展、桌面端和移动端。

## 里程碑

| 里程碑 | 内容 | 完成标准 |
|---|---|---|
| M0 基础 | 仓库、文档合同、CLI、健康 API、统一门禁 | `muxio version`、`muxio serve` 可用，`selftest` 与 CI 全绿 |
| M1 存储纵切 | migration、sources/runs/captures、stdin 导入、搜索、JSONL 导出 | 重复导入幂等，内容可搜索、追溯和导出（分四片交付，见下） |
| M2 真实采集 | 连接器合同、file、RSS/Atom、调度、重试、checkpoint | 两个连接器重复运行幂等，失败不推进 checkpoint，重启无悬挂运行 |
| M3 API / CLI | 完整 `/api/v1`、OpenAPI、source/run/search/export/status CLI | CLI 不依赖表结构，主要能力均可由 API 完成 |
| M4 稳定发布 | 故障测试、迁移备份、跨平台构建、GoReleaser | 干净机器可完成采集、搜索、导出和升级验证 |
| M5 Web | 独立 Web 状态面板 | 不访问 SQLite，仅消费已发布 OpenAPI |

## M1 切片顺序

M1 不作为单个 PR 交付。按下列顺序切片，每片一个 Spec、一个可独立评审和回滚的 PR；前一片验收通过后才开下一片。

| 顺序 | 切片 | 承载 | 验证的核心假设 |
|---|---|---|---|
| 1 | 存储地基与幂等导入 | [Spec 002](specs/002-storage-foundation.md) | 重复写入不产生重复记录，内容变化保留新版本 |
| 2 | 运行记录与崩溃恢复 | 待补 Spec | 运行状态可观测，重启不留悬挂运行 |
| 3 | 全文搜索 | 待补 Spec | FTS 是可重建投影，重建不需要重新采集 |
| 4 | JSONL 导出 | 待补 Spec | 数据可完整带走 |

"待补 Spec"在前一片验收通过后再编写，避免为尚未被真实实现检验的行为提前冻结合同。

修改 v0.1 In / Out 边界必须由维护者确认，并同步 Roadmap 和相关 Spec。
