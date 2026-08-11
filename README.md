# Muxio

**中文** | [English](README.en.md)

Muxio 是一个本地优先的个人信息采集核心。它的首要目标是从本机、局域网和在线来源稳定取得信息，并让每条记录可搜索、可追溯、可导出。

> 当前状态：项目基础已建立，采集与 SQLite 尚未实现。范围以 [Roadmap](docs/roadmap.md) 为准。

## 原则

- 稳定采集优先于内容理解和自动化。
- Go 单二进制、SQLite、本地 HTTP API。
- 连接器模块化，但不提前冻结外部插件协议。
- Web 与核心同仓、工程独立，只通过 OpenAPI 合同协作。
- 个人数据默认留在本地，服务默认只监听 loopback。

## 为什么不用现成工具

RSS 阅读器解决订阅，笔记插件解决单一来源，一段脚本也能跑通一次采集。它们各自成立，但都不提供一个跨来源统一的记录底座：同一套幂等、版本化、可追溯和运行可观测的语义，覆盖所有来源。

Muxio 首期只做这个底座。上层能力（搜索、内容理解、自动化）都建在它之上，而不是每接一个新来源就重做一遍。

这也是范围判断标准：一个功能该不该做，取决于它是否强化这个底座。

## 快速开始

要求 Go 1.25 或更高版本。

```sh
make bootstrap
go run ./cmd/muxio version
go run ./cmd/muxio serve
```

另一个终端验证。`/readyz` 反映数据库是否可达，`/healthz` 只表示进程存活：

```sh
curl http://127.0.0.1:8080/readyz
curl http://127.0.0.1:8080/api/v1/runs
curl http://127.0.0.1:8080/api/v1/config
```

`/api/v1` 是客户端读取 Muxio 数据的唯一通道，接口合同见 [api/openapi.yaml](api/openapi.yaml)。

导入记录并确认重复导入不会产生重复数据：

```sh
muxio db path
echo '{"external_id":"note-1","title":"标题","body":"正文"}' | muxio import --source notes
echo '{"external_id":"note-1","title":"标题","body":"正文"}' | muxio import --source notes
```

第二次输出 `imported=0 duplicate=1 failed=0`。数据库默认位于平台数据目录，`MUXIO_HOME` 可覆盖。

查看历次运行，以及某次运行里哪一行为什么失败：

```sh
muxio runs
muxio runs show 1
```

查看和修改设置。`config show` 会逐项标注值来自默认、文件、环境变量还是命令行：

```sh
muxio config init
muxio config show
muxio config set log.level debug
```

配置文件由程序整份重写，其中的注释是生成的；**你自己添加的注释在写入时会丢失**。凭据不放在这里。

运行全部质量门禁：

```sh
make check
```

## 仓库地图

- `cmd/muxio/`：Go 单二进制入口。
- `internal/`：核心实现，不作为公共 SDK。
- `api/openapi.yaml`：核心与 Web 的公开合同。
- `web/`：独立 Web 工程边界。
- `docs/`：设计、ADR、Roadmap 和 Specs。
- `scripts/selftest.sh`：本地与 CI 共用门禁。
- `scripts/status.sh`：`make status`，全局进度与待决策事项。

详细入口见 [项目地图](docs/MAP.md) 和 [架构设计](docs/design/architecture.md)。

## 参与贡献

请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 和 [AGENTS.md](AGENTS.md)。任务与进度使用 GitHub Issues；长期合同保存在 `docs/`。

## 许可证

[MIT](LICENSE)
