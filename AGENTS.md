# AGENTS.md — Muxio 协作入口

Muxio 是本地优先的个人信息采集核心。当前第一目标是稳定采集、可搜索、可追溯和可导出；时间承诺、内容理解及 Agent 主动执行均后置。

## 语言

- 与维护者的对话、任务清单、Issue、PR、Spec、ADR 和设计文档使用中文。
- 代码与代码注释使用英文。
- Commit 使用中文 Conventional Commits。
- `README.md` / `README.en.md`、`CONTRIBUTING.md` / `CONTRIBUTING.en.md` 必须同一 PR 联动更新。

## 控制面

- GitHub Issues 管任务、状态和进度；仓库文档只保存长期有效的范围、合同和决策。
- `bug`：实现与已确认合同不符。
- `enhancement`：现有 Spec 和 Roadmap 范围内的改进。
- `spec-needed`：必须先补 Spec、ADR 或 Roadmap 的大型需求；它不是可直接实现的任务。
- 开工前先搜索已有 Issue；PR 使用 `Closes #N` 闭环。

## 入项阅读顺序

1. [docs/MAP.md](docs/MAP.md)
2. [docs/design/architecture.md](docs/design/architecture.md)
3. [docs/design/data-model.md](docs/design/data-model.md)
4. [docs/decisions/README.md](docs/decisions/README.md)
5. [docs/roadmap.md](docs/roadmap.md)
6. [docs/specs/README.md](docs/specs/README.md)

## 硬规则

1. 稳定采集优先；Roadmap 明确排除的高层产品能力不得顺手实现。
2. 改架构、持久化模型、配置 schema、公开 CLI 或 HTTP API 前，先更新 Design、ADR 或 Spec；接口变化先改 `api/openapi.yaml`。
3. 核心保持 Go 单二进制和 SQLite；不得引入消息队列、外部数据库或无真实需求的服务拆分。
4. `web/` 是独立工程，只消费 HTTP API；Go 构建不得依赖 Node，Web 不得读取 SQLite 或承载采集业务规则。
5. 本地 HTTP 服务只允许 loopback；远程监听必须先完成认证与威胁模型设计。
6. 连接器先以内置 Go 实现验证接口，不冻结外部 SDK、动态插件 ABI 或进程协议。
7. 时间入库和 API 输出统一使用 RFC3339 UTC；展示层负责时区转换。
8. 新增直接依赖必须在 Spec 或 ADR 说明收益、维护状态和替代方案。
9. 提交前必须通过 `./scripts/selftest.sh`；CI 红灯不得合并。
10. 小 PR、短分支、一个逻辑变更一个 Commit；不得把凭据、个人采集数据或运行数据库提交到仓库。

## 人与 Agent 协作

- 一份 Spec 或一个边界清晰的 Issue 是一个实现工作单元。
- 并行 Agent 必须先声明文件所有权；跨边界改动先回到主控协调。
- Spec 与 Design / ADR 冲突，或验收缺少关键语义时，停止实现并报告，不自行扩 scope。
- 完成定义：验收项通过、合同与实现同步、`selftest` 全绿、PR 可独立评审。
- 分支使用 `feat/<name>`、`fix/<name>`、`docs/<name>` 或 `chore/<name>`，不使用工具名称前缀，不直接 push `main`。

## 常用命令

```sh
make bootstrap
make check
go run ./cmd/muxio version
go run ./cmd/muxio serve
```

## 目录

- `cmd/muxio/`：唯一二进制入口。
- `internal/cli/`：CLI 命令与进程编排。
- `internal/api/`：本地 HTTP API。
- `internal/version/`：构建版本信息。
- `api/openapi.yaml`：跨核心与 Web 的公开接口合同。
- `web/`：独立 Web 工程边界；当前尚未引入工具链。
- `docs/design/`：当前架构与数据设计。
- `docs/decisions/`：不可静默推翻的 ADR。
- `docs/specs/`：可独立实现和验收的规格。
- `scripts/selftest.sh`：本地与 CI 共用的统一门禁。
