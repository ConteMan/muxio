# Spec 004：配置系统

- 状态：已实现
- 关联：Roadmap M1 第 3 片、ADR-005、design/architecture.md

## 问题

监听地址、日志级别、正文上限和事件保留期目前全部硬编码或只能逐次通过命令行传入，进程之间不留痕迹。维护者无法回答"这台机器现在按什么设置在跑"。

M3 设置中心要编辑的对象，事实源就是本 Spec 建立的配置文件。没有它，设置页没有任何字段可绑定。

## 行为

### 位置

- 配置位于用户配置目录的 `muxio/config.toml`；`MUXIO_HOME` 设置时位于 `$MUXIO_HOME/config.toml`。
- 文件不存在时全部使用默认值，不自动创建：一个没配置过的 Muxio 必须能直接跑起来。
- 文件权限为 `0600`。

### 字段

首期只承载已经真实存在的设置：

```toml
[server]
addr = "127.0.0.1:8080"

[log]
level = "info"

[capture]
max_body_bytes = 5242880

[retention]
run_event_days = 30
```

### 优先级

命令行 > 环境变量 > 配置文件 > 内置默认值。

单个字段独立解析：命令行只覆盖它显式给出的字段，不影响其余字段的来源。

### 校验

配置在加载时整体校验，任何一项不合法都拒绝启动：

- `server.addr` 必须是 loopback 地址，与 ADR-002 的约束一致。
- `log.level` 必须是 `debug`、`info`、`warn`、`error` 之一。
- `capture.max_body_bytes` 必须为正且不超过 64 MiB。
- `retention.run_event_days` 必须为正。

错误必须指出字段路径、当前值和期望，并在来自文件时给出行号。不接受未知字段：拼错的键会被静默忽略，而用户会以为设置生效了。

### 写入

- 写入是整份重新渲染，不是就地修改：先写同目录临时文件，`fsync` 后原子 rename。
- 渲染模板由程序提供并始终带注释，因此程序生成的注释不会丢失。**用户自行添加的注释与字段顺序会在写入时丢失**，必须在 README 与后续设置界面明确告知，不得暗示保留。
- 写入前重新读取文件的修改时间，与读取时不一致则拒绝写入并报告冲突，由人决定如何合并（ADR-005）。

### CLI

```sh
muxio config path              # 打印配置文件路径，不创建文件
muxio config show              # 打印生效配置，逐项标注来源
muxio config init              # 写入一份带注释的默认配置，已存在时拒绝覆盖
muxio config set <key> <value> # 修改单个字段，键形如 server.addr
```

`config show` 必须标注每一项的来源（默认 / 文件 / 环境 / 命令行），否则"为什么这个值没生效"依然无法自查。

### 接入

`serve` 与 `import` 从配置读取监听地址与日志级别，命令行标志继续优先。`capture.max_body_bytes` 与 `retention.run_event_days` 取代 `record` 与 `run` 包中的对应常量。

## 依赖

本 Spec 需要 TOML 解析。标准库不提供，写入侧可以自行渲染，读取侧不应自造解析器——用户写出的合法 TOML 若被自造解析器拒绝，错误信息会极难理解。

选定 `github.com/BurntSushi/toml`：零外部依赖，API 面很小，`ErrorWithPosition` 能给出行列以支撑校验提示，长期稳定且没有大版本迁移史。备选 `github.com/pelletier/go-toml/v2` 更快，但配置文件只有几十行，性能不构成理由。

影响面限制在 `internal/config` 一个包内，因此不单独立 ADR；若日后需要更换，改动不外溢。

## 边界与非目标

- 不定义采集源。来源的名字、连接器与调度属于配置，但首期尚无连接器，`import --source` 仍隐式创建来源；来源进配置文件由 M4 承载。
- 不实现凭据读写；`credentials.toml` 在出现需要认证的来源前不创建。
- 不通过 HTTP API 暴露配置，`api/openapi.yaml` 本 Spec 不变更。
- 不实现热重载与文件监听；配置在进程启动时读取一次。
- 不实现前端。

## 验收

- 无配置文件时 `muxio import` 与 `muxio serve` 使用默认值正常运行，且不创建配置文件。
- `muxio config path` 打印路径且不创建文件。
- `muxio config init` 生成带注释的默认配置，权限为 `0600`；再次执行时拒绝覆盖。
- `muxio config show` 标注来源：未配置项显示默认，文件中的项显示文件，被 `MUXIO_LOG_LEVEL` 覆盖的项显示环境，被命令行覆盖的项显示命令行。
- 文件中写入 `server.addr = "0.0.0.0:8080"` 时，`serve` 拒绝启动并指出字段与原因。
- 文件中写入未知键或非法层级时，加载失败并给出行号。
- `muxio config set log.level debug` 后，文件中其余字段与注释保持不变。
- 读取后、写入前被外部修改的文件不会被覆盖，命令报告冲突并以非零码退出。
- `./scripts/selftest.sh` 全绿，含优先级、校验、原子写入与冲突检测的自动化测试。
