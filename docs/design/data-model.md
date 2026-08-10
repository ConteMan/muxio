# 首期数据模型

本文定义 SQLite 首期最小合同。具体 SQL 由存储 Spec 和只增不改的 migration 落地。

## 表

### `sources`

一条记录表示一个已配置采集源：`id`、`name`、`connector_kind`、`config_json`、`checkpoint_json`、`enabled`、`interval_seconds`、`next_run_at`、`last_success_at`、`last_error` 和审计时间。

### `runs`

一条记录表示一次逻辑采集运行：`id`、`source_id`、`trigger`、`status`、`started_at`、`heartbeat_at`、`finished_at`、计数、尝试次数和最后错误。

状态机：

```text
queued → running → succeeded
                 → partial
                 → failed
                 → canceled
                 → interrupted
```

进程启动时，遗留 `running` 必须转为 `interrupted`。只有已有记录成功提交而运行随后失败时使用 `partial`。

### `captures`

不可变采集记录：`id`、`source_id`、`run_id`、`external_id`、`content_hash`、`title`、`body`、`mime_type`、`canonical_url`、`occurred_at`、`captured_at` 和 `metadata_json`。

唯一约束为 `source_id + external_id + content_hash`：同一版本重复观察不重复写入，内容变化保留新版本和旧版本。

### `captures_fts`

FTS5 索引标题与正文。它是可重建投影，不是事实源。

### `schema_migrations`

记录已应用的顺序迁移。已发布 migration 只增不改；迁移前建立一致性备份。

## 事务不变量

一次成功提交必须在同一事务中完成：写入或去重 captures、更新运行计数、推进 source checkpoint。任一步失败则不得推进 checkpoint。

## 标识与时间

- 内部 ID 必须稳定、不可从可变标题推导。
- `external_id` 由连接器定义：文件使用规范化相对路径，Feed 优先使用 GUID。
- `content_hash` 对规范化后的采集内容计算。
- 所有持久化时间与 API 时间使用 RFC3339 UTC。

## 首期限制

- 正文只保存文本；附件和大型二进制不入库。
- 单条正文默认上限 5 MiB。
- 连接器配置首期不得保存凭据。
- 删除与保留策略在有真实数据量证据后单独设计。
