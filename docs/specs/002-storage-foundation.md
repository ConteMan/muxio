# Spec 002：存储地基与幂等导入

- 状态：草稿
- 关联：Roadmap M1、ADR-002、design/data-model.md

## 问题

Muxio 当前没有持久化。在实现调度、连接器和搜索之前，需要先用一条最短的端到端路径验证数据模型最核心的假设：**同一份内容重复写入不产生重复记录，内容变化保留新版本**。

这条假设错了，后面所有采集能力都建在流沙上。因此本 Spec 只做"把一条记录可靠地写进去"，不做读侧能力。

## 行为

### 数据位置

- 数据库默认位于平台数据目录的 `muxio/muxio.db`。
- `MUXIO_HOME` 设置时，数据库位于 `$MUXIO_HOME/muxio.db`，用于测试与便携运行。
- 首次使用自动创建目录与数据库文件；目录权限限制为仅当前用户可访问。
- `muxio db path` 打印当前生效的数据库路径，不创建文件。

### 迁移

- 迁移以顺序编号的嵌入式 SQL 承载，通过 `schema_migrations` 记录已应用版本。
- 启动时自动应用未应用的迁移，全部在单个事务内完成。
- 数据库 schema 版本高于当前二进制已知版本时拒绝启动并明确报错，不做降级。
- SQLite 使用 WAL、`foreign_keys=ON` 和有界 busy timeout。

### 表

本 Spec 只建立 `schema_migrations`、`sources` 和 `captures`。

`sources` 首期只需支撑归属与去重：`id`、`name`（唯一）、`connector_kind`、`config_json`、`checkpoint_json`、`enabled`、`created_at`、`updated_at`。调度相关列由后续 Spec 通过新增迁移添加。

`captures` 承载不可变记录：`id`、`source_id`、`external_id`、`content_hash`、`title`、`body`、`mime_type`、`canonical_url`、`occurred_at`、`captured_at`、`metadata_json`。唯一约束为 `(source_id, external_id, content_hash)`。

`run_id` 不在本 Spec 引入，由 runs Spec 通过新增迁移添加为可空列。

### 导入

```sh
muxio import --source <name> < records.jsonl
```

- 从 stdin 逐行读取 JSONL，每行一条候选记录。
- 记录字段：`external_id`（必填）、`title`、`body`、`mime_type`、`canonical_url`、`occurred_at`、`metadata`。
- `--source` 指定的来源不存在时，自动创建 `connector_kind = "manual"` 的来源。
- `content_hash` 由核心对规范化后的内容计算，不接受调用方提供。
- `occurred_at` 接受 RFC3339；缺省时留空，不猜测。`captured_at` 由核心写入。
- 单条正文超过上限（默认 5 MiB）时该行失败，不中断整批。
- 单行解析失败或校验失败记为该行失败并继续；进程级错误（数据库不可写）立即中止。
- 结束时向 stderr 输出 `imported`、`duplicate`、`failed` 计数。
- 全部行成功退出码 0；存在失败行退出码 1；用法错误 64。

## 依赖

本 Spec 引入首个外部直接依赖：SQLite 驱动。

推荐 `modernc.org/sqlite`（纯 Go，无 cgo）。理由：ADR-002 承诺跨平台单文件发布，纯 Go 驱动让 M4 的交叉编译无需为每个目标平台准备 C 工具链，并且内置 FTS5，可支撑后续搜索 Spec。代价是性能低于 `mattn/go-sqlite3`，对个人规模数据可接受。

选定后需补 ADR 记录该决策；实现前由维护者确认。

## 边界与非目标

- 不实现 `runs` 表、运行状态机与崩溃恢复。
- 不实现 FTS5、搜索命令与 JSONL 导出。
- 不实现调度、重试、连接器与 checkpoint 推进。
- 不通过 HTTP API 暴露任何存储能力；`api/openapi.yaml` 本 Spec 不变更。
- 不实现来源的增删改查命令。

## 验收

- `MUXIO_HOME=$(mktemp -d) muxio db path` 输出该目录下的 `muxio.db` 且不创建文件。
- 首次 `muxio import` 自动建库、应用全部迁移，`schema_migrations` 记录完整。
- 同一份 JSONL 连续导入两次：第二次 `imported=0`、`duplicate=N`，`captures` 行数不变。
- 修改某条记录的 `body` 后再次导入：新增 1 行，旧行仍在且内容未被修改。
- 缺少 `external_id` 的行计入 `failed`，同批次其他行仍成功写入，退出码为 1。
- 导入过程中断（进程被杀）后重跑，不产生重复记录且数据库可正常打开。
- `./scripts/selftest.sh` 全绿，含针对幂等与并发写入的自动化测试。
