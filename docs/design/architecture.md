# 架构设计

## 目标

Muxio 首期只解决一件事：从不同来源稳定取得信息，并让每条信息可搜索、可追溯、可导出。系统优先减少丢失、重复和不可解释状态，而不是提前增加内容理解能力。

## 系统边界

- `muxio`：Go 单二进制，承载 CLI、本地 HTTP API、调度、连接器、SQLite、搜索和导出。
- `web/`：同仓独立前端，通过 HTTP API 管理采集源并查看运行状态。
- 浏览器扩展、桌面端和移动端在出现真实需求时再加入，不创建空工程。
- MCP / Agent 接口复用应用服务和稳定 ID，首期不实现独立协议层。

## 运行形态

`muxio serve` 是唯一长期运行进程，也是 SQLite 的唯一长期写入者。其他在线 CLI 命令通过 loopback HTTP API 调用服务。迁移、修复或恢复类离线命令必须在服务停止后执行。

HTTP 默认监听 `127.0.0.1`。在认证、来源校验和威胁模型完成前，不允许监听非 loopback 地址。

## 目标模块

```text
cmd/muxio
  → cli
  → app                  use cases and orchestration
    → collector          scheduling, retries, run lifecycle
      → connector        minimal built-in connector contract
    → store/sqlite       transactions, migrations, queries
    → search             FTS5 projection and queries
    → export             portable JSONL export
  → api/http             versioned external contract
```

包只向内依赖应用合同；HTTP、CLI 和连接器适配器不得直接承载领域规则。

## 连接器边界

首期连接器是进程内 Go 实现，最小行为是：声明类型、校验配置、读取 checkpoint、流式提交候选记录并返回新 checkpoint。

调度、重试、日志、去重、事务和运行状态属于核心引擎。只有 file 与 RSS/Atom 两个真实实现验证稳定后，才评估外部 SDK 或进程协议。

## Core 与 Web

- OpenAPI 是唯一跨工程类型合同。
- Go 构建、测试和发布不依赖 Node。
- Web 不访问 SQLite、采集目录或连接器实现。
- Core 与 Web 允许独立版本和独立发布。
- 当前不建立共享源码包；生成客户端只能来自发布的 OpenAPI。

## 配置与数据目录

默认使用平台原生目录：配置位于用户配置目录的 `muxio/config.toml`，数据库位于用户数据目录的 `muxio/muxio.db`。`MUXIO_HOME` 可覆盖全部路径，用于测试和便携运行。

运行日志默认写 stderr；导出路径必须显式指定。任何凭据、数据库和个人采集内容都不得进入 Git 仓库。

## 稳定性约束

- SQLite 使用 WAL、有界等待和显式事务。
- checkpoint 仅在对应采集记录成功提交后推进。
- 采集记录不可变；来源内容变化产生新版本。
- 所有网络读取都有超时、重定向、响应大小和重试上限。
- 所有时间在入口转换为 RFC3339 UTC。
