# Muxio Web

`web/` 是 Muxio 轻量 Monorepo 中的独立 Web 工程边界。

当前没有已确认的 Web 实现 Spec，因此不提前引入 React、Vite、Node 依赖或生成空应用。这样可以避免无用户价值的依赖升级和脚手架腐化。

开始实现前必须满足：

1. Core 的 `/api/v1` 已覆盖连接器状态、运行历史和错误详情。
2. `api/openapi.yaml` 是唯一接口合同，并能生成客户端类型。
3. Web 独立构建、运行和发布；Go 构建不得依赖 Node。
4. Web 不得读取 SQLite、本地采集目录或承载采集业务规则。
5. 前端工具链与最小状态面板由独立 Spec 决定并接入根 `selftest`。
