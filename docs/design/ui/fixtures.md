# M3 面板真实规模内容

画板与浏览器验收一律使用本文的内容。**不得用三条短记录、短来源名或空白控件代替**——那样验收的是示意图，而不是这个界面真实运行时的样子。

这些数值来自当前实现的真实上限与语义，不是编造的极端值。

## 来源

| 名称 | 说明 | 为什么在这里 |
|---|---|---|
| `notes` | 最常见的短名称 | 基准 |
| `feeds` | 第二个来源 | 验证列表与过滤 |
| `obsidian-daily-notes-2026-archive` | 32 字符 | 来源名无长度限制，列宽必须容得下或明确截断 |

## 运行

| # | 状态 | imported / duplicate / failed | 特征 |
|---|---|---|---|
| 7 | `running` | 1 240 / 0 / 0 | `finished_at` 为 null，界面必须显示"进行中"而非空白或 1970 年 |
| 6 | `succeeded` | 0 / 3 / 0 | 全部重复，"什么都没导入"但这是成功 |
| 5 | `succeeded` | 298 795 / 21 205 / 0 | 六位数计数，数字列必须对齐且不换行 |
| 4 | `interrupted` | 16 287 / 0 / 0 | 进程被杀，`last_error` 为 `process exited without finishing this run` |
| 3 | `failed` | 0 / 0 / 0 | 未提交任何记录即中止 |
| 2 | `partial` | 1 / 0 / 0 | 已提交部分后中止，`last_error` 非空 |
| 1 | `succeeded` | 2 / 0 / 1 | 有失败行但运行成功——最容易被误读的一行 |

七条运行超过一页（默认 20 条时不超，但验收分页需构造 25 条以上）。

## 运行事件

**普通运行**（运行 1，4 条）：

```
info   import started
error  line 3 rejected: external_id is required
error  line 5 rejected: invalid JSON: invalid character 'è' looking for beginning of value
info   import finished
```

**截断运行**（1 001 条）：前 1 000 条为 `line N rejected: external_id is required`，第 1 001 条为：

```
warn   event log truncated: this run reached its event limit
```

界面必须让读者看出后面还有 499 个失败行没有对应事件——这正是 CG-02。

**无事件运行**：0 条。空态文案不得与"加载中"或"出错"混淆。

**长消息事件**：

```
error  line 842 rejected: invalid JSON: invalid character 'x' looking for beginning of object key string
```

96 字符，不换行会撑破布局。

## 配置

| 键 | 值 | origin | 特征 |
|---|---|---|---|
| `server.addr` | `127.0.0.1:8080` | `default` | 基准 |
| `log.level` | `debug` | `env` | **被环境变量覆盖**，文件里写的是 `info`，界面必须让用户看懂改文件无效（CG-08） |
| `capture.max_body_bytes` | `5242880` | `file` | 七位数字 |
| `retention.run_event_days` | `30` | `default` | 基准 |

配置文件路径使用真实长度：

```
/Users/conteman/Library/Application Support/muxio/config.toml
```

58 字符，单行显示会溢出窄屏。

## 错误响应

面板必须能渲染这四种，全部来自真实端点：

```json
{"error":"not_found","message":"no run with id 999"}
{"error":"invalid_argument","message":"limit must be a whole number","field":"limit"}
{"error":"precondition_failed","message":"the configuration changed since it was read; reload and reapply your edit"}
{"error":"internal","message":"the request could not be completed"}
```

## 生成命令

```sh
export MUXIO_HOME=$(mktemp -d)/fixtures

# 运行 1：有失败行但成功
printf '%s\n' \
  '{"external_id":"note-1","title":"第一条","body":"内容 A"}' \
  '{"external_id":"note-2","title":"第二条","body":"内容 B"}' \
  '{"body":"缺少 external_id"}' \
  | muxio import --source notes

# 运行 6：全部重复
printf '%s\n' \
  '{"external_id":"note-1","title":"第一条","body":"内容 A"}' \
  '{"external_id":"note-2","title":"第二条","body":"内容 B"}' \
  | muxio import --source notes

# 长来源名
echo '{"external_id":"a","body":"x"}' \
  | muxio import --source obsidian-daily-notes-2026-archive

# 截断运行：1499 个失败行，事件停在 1000 + 1 条截断说明
python3 -c 'print("{\"body\":\"no id\"}\n" * 1499, end="")' \
  | muxio import --source notes

# 环境覆盖
MUXIO_LOG_LEVEL=debug muxio serve
```

`interrupted` 与 `partial` 需要真实中断，见 `docs/specs/003-runs-and-logging.md` 的验收步骤。
