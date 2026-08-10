# Spec 003：运行记录与结构化日志

- 状态：已实现
- 关联：Roadmap M1 第 2 片、ADR-002、ADR-005、design/data-model.md

## 问题

目前每次 `muxio import` 结束只留下三个计数，进程退出后什么都查不到：跑过几次、什么时候跑的、哪一条为什么失败，全都无法回答。

M3 的设置中心要展示的"运行历史"和"失败原因"，事实源就是本 Spec 建立的两张表。没有它们，面板上没有任何一行可显示。

## 行为

### 迁移

新增迁移，只增不改：

- `runs` 表：`id`、`source_id`、`trigger`、`status`、`started_at`、`heartbeat_at`、`finished_at`、`imported_count`、`duplicate_count`、`failed_count`、`attempt`、`last_error`。
- `run_events` 表：`id`、`run_id`、`level`、`message`、`detail_json`、`occurred_at`。
- `captures` 增加可空的 `run_id` 列，并建立指向 `runs` 的外键。已有记录保持 `NULL`。

### 运行生命周期

状态机遵循 data-model：`queued → running → succeeded | partial | failed | canceled | interrupted`。终态不可再转换。

- 每次 `import` 创建一条运行，`trigger = "manual"`。
- 运行期间按固定间隔更新 `heartbeat_at`。
- 进程启动时，把心跳已经停滞超过阈值的 `running` 与 `queued` 转为 `interrupted`，并为每条写入一条说明事件。

判定依据必须是心跳而不是"存在非终态运行"：后者会让同时启动的另一个进程把正在正常运行的记录误判为中断。阈值必须显著大于心跳间隔，使繁忙但存活的运行不会被误杀。

结束状态的判定：

- 全部输入行处理完毕且无存储错误：`succeeded`，失败行数记入 `failed_count`。失败的是行，不是运行。
- 存储错误中止，且此前已有记录成功提交：`partial`。
- 存储错误中止且没有任何记录提交：`failed`。
- 上下文取消：`canceled`。

### 运行事件

- 事件只追加，不修改，不删除（保留策略除外）。
- 记录运行开始、结束、状态转换、每个失败行的原因，以及导致中止的存储错误。
- 单次运行的事件上限为 1000 条；达到上限后停止写入并追加一条截断说明，避免一次异常输入刷爆事件表。
- 保留策略：写入前删除 30 天前的事件。运行记录本身不受影响。
- 写入事件前必须对凭据脱敏（ADR-005）。

### 结构化日志

- stderr 输出结构化日志，默认 `info`，`--log-level` 或 `MUXIO_LOG_LEVEL` 可调。
- 日志与运行事件是两层：调试明细只进 stderr，值得追溯的事件同时进库。
- 日志包含 `run_id`，便于把终端输出与库中记录对上。

### CLI

```sh
muxio runs [--limit N]      # 运行列表：id、来源、状态、时间、计数
muxio runs show <id>        # 运行详情与事件
```

这两个命令首期直接读数据库。M2 建立 `/api/v1` 后改为经 API 读取，本 Spec 的表结构与语义不因此变化。

### 事务

一次运行的写入必须与它产生的采集记录在同一事务中提交：写入或去重 capture、更新运行计数。任一步失败则整体回滚，运行计数不得领先于实际写入的记录。

## 边界与非目标

- 不实现调度、重试与 checkpoint 推进。
- 不实现连接器；唯一的运行来源仍是 `import`。
- 不实现配置系统；日志级别只经命令行与环境变量控制。
- 不通过 HTTP API 暴露运行数据，`api/openapi.yaml` 本 Spec 不变更。
- 不实现前端。

## 验收

- 一次 `import` 后，`muxio runs` 显示该次运行的状态、时间与三个计数，且与命令输出一致。
- 含失败行的导入：运行状态为 `succeeded`，`failed_count` 正确，`muxio runs show` 能列出每个失败行的行号与原因。
- 导入过程中 `SIGKILL`，心跳停滞超过阈值后再次运行，`muxio runs` 中该次运行为 `interrupted`，且带有说明事件；已提交的采集记录不受影响。
- 心跳新鲜的运行不会被同时启动的另一个进程判定为中断。
- 存储错误中止且已有记录提交时，运行状态为 `partial`。
- 新写入的 capture 带有 `run_id`，可由运行反查到它写入的全部记录。
- 单次运行产生超过 1000 条失败行时，事件表停在上限并留有截断说明。
- 30 天前的事件在下一次运行时被清理，运行记录仍在。
- `./scripts/selftest.sh` 全绿，含运行生命周期、中断恢复与事件上限的自动化测试。
