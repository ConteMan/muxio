# 参与贡献 Muxio

**中文** | [English](CONTRIBUTING.en.md)

Muxio 采用文档先行、对人和 Agent 使用同一规则的协作方式。

## 开发环境

当前只要求 Go 1.25.9。Web 工具链将在对应 Spec 确认后加入。

```sh
git clone git@github.com:ConteMan/muxio.git
cd muxio
make bootstrap
make check
```

## 动手之前

1. 按 [AGENTS.md](AGENTS.md) 的顺序阅读项目地图、Design、ADR、Roadmap 和 Specs。
2. 搜索已有 Issue；重复问题回到原 Issue 补充证据。
3. Roadmap 范围内的小改进使用 `enhancement`；需要新合同的大型需求使用 `spec-needed`，先由维护者确认方向。
4. 架构、数据模型、配置、公开 CLI 或 API 变化必须让文档合同与代码在同一 PR 联动。

## 分支与 Commit

从 `main` 创建 `feat/<name>`、`fix/<name>`、`docs/<name>` 或 `chore/<name>` 分支。Commit 遵循 Conventional Commits：

```text
feat(connector): 增加 file 连接器
fix(store): 避免失败运行推进 checkpoint
docs(adr): 记录连接器进程边界
```

一个 Commit 和一个 PR 只承载一个逻辑变更。标题和正文使用中文；type、scope、代码、标识符和路径保留英文。

## Pull Request

- 使用 `Closes #N` 关联 Issue；无 Issue 时说明原因。
- 说明变更、原因、验证证据、风险与回滚方式。
- 提交前运行 `make check`，CI 红灯不得合并。
- 用户可见变化更新 `CHANGELOG.md`。
- 改动主 README 或贡献指南时同步中英文镜像。

## 新增依赖

不要为了可能的未来需求提前引入依赖。新增 Go 直接依赖或 Web 工具链必须在 Spec 或 ADR 说明用途、维护状态和不采用标准库/现有能力的原因。

## 许可证

提交贡献即表示同意以 [MIT License](LICENSE) 授权。
