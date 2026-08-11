# 面板设计

面板的结构与状态在这里收口，然后才进实现。`web/src/` 是能力事实源，`prototypes/*.pen` 是视觉与结构事实源，PNG 只是评审快照。

## 顺序

1. [contract-gaps.md](contract-gaps.md) — API 能支撑什么、缺什么。缺的不进画板。
2. [flows/](flows/) — 用户要回答的问题与完整状态矩阵。
3. [fixtures.md](fixtures.md) — 真实规模内容。所有画板与浏览器验收一律使用。
4. [screen-inventory.md](screen-inventory.md) — 画板清单、优先级与交付门槛。
5. [exploration-brief.md](exploration-brief.md) — 探索目标、非目标与阶段门禁。
6. `pencil-task-*.md` — 给 Pencil 编辑者的任务书。
7. `prototypes/*.pen` — 设计源文件。
8. `reviews/` — 评审记录与快照。

## 纪律

- 画板不得出现后端尚不存在的能力，包括 disabled 占位。
- 不得用短样例代替 `fixtures.md` 的真实规模内容。
- 每个状态可从 `.pen` 直接定位，不靠"想象另一个状态"。
- 结构探索先于视觉；维护者选定方向前不建设计系统。
