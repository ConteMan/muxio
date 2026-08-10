# 维护与发布

## 统一质量门禁

提交前运行：

```sh
./scripts/selftest.sh
git diff --check
```

`selftest` 与 CI 使用同一入口，检查文档基线、ADR/Spec 索引、OpenAPI、`go mod tidy`、格式、构建、vet 和 race tests。后续加入 Web 工具链时，也必须接入同一入口。

## 依赖

- Go 直接依赖保持少且可解释；新增依赖必须在 Spec 或 ADR 记录理由。
- 工具以固定版本运行，避免本地与 CI 漂移。
- 不为尚未实现的模块提前引入依赖。

## 文档同步

- 改 HTTP API：先改 `api/openapi.yaml`。
- 改架构或边界：更新 Design 或新增取代型 ADR。
- 改用户可见行为：更新 README 双语镜像和 `CHANGELOG.md`。
- 改范围：更新 Roadmap，并由对应 Spec 承载验收。

## 版本

- 遵循 Semantic Versioning，tag 为 `vX.Y.Z`。
- `v1.0.0` 前仍必须在 Changelog 声明破坏性变更。
- Core 与未来 Web 独立发布；API 使用路径版本 `/api/v1`。
- 版本由 Git tag 注入二进制，不维护第二份硬编码发布版本。

## 发布前最低检查

1. `main` CI 为绿。
2. Changelog、双语 README 和相关合同已同步。
3. 本地 `make check` 通过。
4. 发布产物包含 macOS、Linux、Windows 二进制与校验和。
5. 在干净目录验证 `version`、`serve` 和一次最小采集流程。

首个正式发布流程在 M4 通过独立 Spec 建立，不提前维护未验证的发布脚本。
