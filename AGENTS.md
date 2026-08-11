# AGENTS.md — Muxio 协作入口

Muxio 是本地优先的个人信息采集核心。当前第一目标是稳定采集、可搜索、可追溯和可导出；时间承诺、内容理解及 Agent 主动执行均后置。

## 语言

- 与维护者的对话、任务清单、Issue、PR、Spec、ADR 和设计文档使用中文。
- 代码与代码注释使用英文。
- Commit 使用中文 Conventional Commits。
- `CHANGELOG.md` 面向发布受众，使用英文。
- `README.md` / `README.en.md`、`CONTRIBUTING.md` / `CONTRIBUTING.en.md` 必须同一 PR 联动更新，`selftest` 会校验两者章节结构一致。

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
4. `web/` 是独立工程，只消费 HTTP API；Web 不得读取 SQLite 或承载采集业务规则。构建产物提交至 `internal/webui/assets/` 并随二进制嵌入（ADR-007），因此 `go build` 仍不得依赖 Node。
5. 本地 HTTP 服务只允许 loopback；远程监听必须先完成认证与威胁模型设计。
6. 连接器先以内置 Go 实现验证接口，不冻结外部 SDK、动态插件 ABI 或进程协议。
7. 时间入库和 API 输出统一使用 RFC3339 UTC；展示层负责时区转换。
8. 新增直接依赖必须在 Spec 或 ADR 说明收益、维护状态和替代方案。
9. 提交前必须通过 `./scripts/selftest.sh`；CI 红灯不得合并。
10. 小 PR、短分支、一个逻辑变更一个 Commit；不得把凭据、个人采集数据或运行数据库提交到仓库。

## 决策卡点

下列变更 Agent 不得自行决定。必须停止实现，创建 `spec-needed` Issue 写清选项、权衡和推荐，等待维护者裁决：

1. 新增任何外部直接依赖（Go module 或 Web 工具链）。
2. 修改或取代任何已接受的 ADR。
3. 修改 `docs/design/data-model.md` 中的不变量。
4. 调整 Roadmap 的 v0.1 In / Out 边界，或改变 M1 切片顺序。
5. 改变 `api/openapi.yaml` 已发布端点的行为或字段语义。
6. 引入新的网络出站行为、凭据存储位置或数据落盘位置。
7. 放宽任何既有安全约束，包括监听地址、文件权限和超时上限。
8. 删除、跳过或放宽 `scripts/selftest.sh` 中的任何检查。

卡点之外的实现细节不需要等待。停下来问的成本，远低于事后回滚一个已被依赖的错误决策。

`make status` 输出当前里程碑、Spec 与 ADR 的真实状态，用于开工前对齐和交接。

## 人与 Agent 协作

- 一份 Spec 或一个边界清晰的 Issue 是一个实现工作单元。
- 并行 Agent 必须先声明文件所有权；跨边界改动先回到主控协调。
- Spec 与 Design / ADR 冲突，或验收缺少关键语义时，停止实现并报告，不自行扩 scope。
- 完成定义：验收项通过、合同与实现同步、`selftest` 全绿、PR 可独立评审。
- 分支使用 `feat/<name>`、`fix/<name>`、`docs/<name>` 或 `chore/<name>`，不使用工具名称前缀，不直接 push `main`。

## 常用命令

```sh
make bootstrap        # Go 依赖 + web 依赖
make status
make check            # 统一门禁，含面板产物与生成类型的时效校验
make web-dev          # 前端开发服务器，API 代理到本地 muxio serve
make web-build        # 重建嵌入产物，改前端后必须执行并提交
make panel-smoke      # 用真实二进制跑浏览器冒烟
go run ./cmd/muxio serve
```

## 目录

- `cmd/muxio/`：唯一二进制入口。
- `internal/cli/`：CLI 命令与进程编排。
- `internal/api/`：本地 HTTP API。
- `internal/app/`：用例编排，不依赖任何适配器。
- `internal/config/`：配置的读取、校验与原子写入。
- `internal/record/`：采集记录的规范化与内容哈希等领域规则。
- `internal/run/`：运行状态机、事件与相关上限。
- `internal/logging/`：写往 stderr 的结构化日志。
- `internal/store/sqlite/`：迁移、事务与查询，SQLite 相关代码只在这里。
- `internal/paths/`：配置与数据目录解析。
- `internal/version/`：构建版本信息。
- `api/openapi.yaml`：跨核心与 Web 的公开接口合同。
- `web/`：Web 面板源码，独立工程；构建输出到 `internal/webui/assets/`。
- `internal/webui/`：随二进制嵌入的面板产物与静态托管。
- `docs/design/`：当前架构与数据设计。
- `docs/decisions/`：不可静默推翻的 ADR。
- `docs/specs/`：可独立实现和验收的规格。
- `scripts/selftest.sh`：本地与 CI 共用的统一门禁。
- `scripts/status.sh`：从仓库真实状态推导的全局视图，不落盘。
