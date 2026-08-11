# Spec 005：只读 HTTP API

- 状态：已实现
- 关联：Roadmap M2 第 1 片、ADR-001、ADR-002、design/architecture.md

## 问题

来源、运行历史、运行事件和配置目前只能经 CLI 访问，而 CLI 直接读数据库与配置文件。

ADR-001 禁止 Web 读取 SQLite，唯一通道是 `/api/v1`。没有这些端点，M3 的设置中心拿不到任何数据可显示。

## 行为

### serve 接入存储

- `serve` 启动时打开数据库并应用迁移；数据库不可用时启动失败并说明原因。
- 启动时回收心跳停滞的运行，与 `import` 的行为一致。
- `/readyz` 反映数据库可达性，不再无条件返回就绪；`/healthz` 仍只表示进程存活。
- 与并发运行的 `import` 进程共存：SQLite 处于 WAL 模式且等待有界，读取看到的是一致性快照。

### 端点

```
GET /api/v1/sources
GET /api/v1/runs?limit=&before=&source_id=
GET /api/v1/runs/{id}
GET /api/v1/runs/{id}/events?limit=&before=
GET /api/v1/config
```

- 列表响应为 `{"items": [...], "next_before": <id|null>}`。`next_before` 为空表示没有更多。
- `limit` 默认 20，最大 100；超出上限按上限处理而不是报错。
- `before` 是游标：只返回 id 小于它的记录，配合按 id 倒序实现稳定翻页。
- 所有时间为 RFC3339 UTC；空值表达为 `null` 而不是空串。

`GET /api/v1/config` 返回每个字段的生效值与来源（`default` / `file` / `env` / `flag`），以及配置文件路径和是否存在。凭据不在配置文件中，因此不会出现在响应里。

### 错误

统一为一种形状，便于客户端一致处理：

```json
{ "error": "invalid_argument", "message": "limit must be a number", "field": "limit" }
```

- `400 invalid_argument`：参数不合法。
- `404 not_found`：资源不存在。
- `500 internal`：其余失败。响应中不得包含文件路径以外的内部细节或堆栈。
- 未知路径返回 404，方法不匹配返回 405。

### 契约

全部端点与 schema 写入 `api/openapi.yaml`，由 `selftest` 校验。Handler 与契约必须同一 PR 变更。

## 并发与限制

数据库句柄当前限制为单连接，因此 HTTP 请求串行访问存储。对本地单用户面板足够；出现真实并发证据前不放宽，以免与单写入者模型产生分歧。

## 边界与非目标

- 不实现任何写入端点；配置写入由下一片承载。
- 不实现认证与授权。服务仍只监听 loopback（ADR-002），远程访问需要先完成威胁模型。
- 不实现搜索、导出与采集触发。
- 不实现 capture 内容的读取端点：首期面板只需要运行与配置，暴露采集正文会引入分页、脱敏和体积问题，应由独立 Spec 承载。
- 不实现前端。

## 验收

- `serve` 在空目录启动后自动建库并迁移，`/readyz` 返回就绪；数据库被删除或不可读时 `/readyz` 返回非就绪且退出码与日志说明原因。
- `import` 写入的运行可立即经 `GET /api/v1/runs` 读到，字段与 `muxio runs` 输出一致。
- `GET /api/v1/runs/{id}` 对不存在的 id 返回 404 且响应体符合错误形状。
- `GET /api/v1/runs?limit=1` 配合返回的 `next_before` 可完整翻完全部运行，无重复、无遗漏。
- `limit=0`、`limit=abc`、`before=abc` 返回 400 并指出字段；`limit=9999` 按上限 100 处理。
- `GET /api/v1/config` 的来源标注与 `muxio config show` 一致，包括环境变量覆盖的情况。
- 未知路径返回 404，对已知路径使用 POST 返回 405。
- `serve` 运行期间并发执行 `import`，两者均成功且 API 能读到新运行。
- OpenAPI 校验通过，且每个已实现端点都在契约中描述。
- `./scripts/selftest.sh` 全绿，含端点行为、分页、错误形状与并发读取的自动化测试。
