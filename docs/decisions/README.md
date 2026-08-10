# 架构决策记录

ADR 一旦接受不得静默修改。需要改变方向时新增 ADR，声明被取代关系并解释原因。

| ADR | 状态 | 决策 |
|---|---|---|
| [001](001-lightweight-monorepo.md) | 已接受 | 核心与 Web 采用边界严格的轻量 Monorepo |
| [002](002-local-go-core.md) | 已接受 | Go 单二进制、本地 HTTP API 与 SQLite |
| [003](003-built-in-connectors-first.md) | 已接受 | 先用真实内置连接器验证接口，不冻结外部插件协议 |
