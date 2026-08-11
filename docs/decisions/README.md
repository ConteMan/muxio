# 架构决策记录

ADR 一旦接受不得静默修改。需要改变方向时新增 ADR，声明被取代关系并解释原因。

| ADR | 状态 | 决策 |
|---|---|---|
| [001](001-lightweight-monorepo.md) | 已接受 | 核心与 Web 采用边界严格的轻量 Monorepo |
| [002](002-local-go-core.md) | 已接受 | Go 单二进制、本地 HTTP API 与 SQLite |
| [003](003-built-in-connectors-first.md) | 已接受 | 先用真实内置连接器验证接口，不冻结外部插件协议 |
| [004](004-pure-go-sqlite-driver.md) | 已接受 | 采用纯 Go SQLite 驱动，构建不依赖 cgo |
| [005](005-config-and-credential-storage.md) | 已接受 | 配置以文件为事实源，凭据独立存放且永不入库 |
| [006](006-config-fingerprint.md) | 已接受 | 用内容指纹而非修改时间检测配置并发修改（细化 ADR-005）|
