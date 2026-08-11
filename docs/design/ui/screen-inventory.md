# M3 画板与状态清单

本清单以 `web/src` 与 `internal/api` 的真实行为为起点。Pencil 画板用于收口尚未确定的体验与状态，**不以评审文字或单张 PNG 代替可编辑 `.pen` 中的节点、组件和状态**。

当前实现是直接从 Spec 007 写成的，没有经过设计阶段。它是结构探索的输入事实，不是目标结构。

## 优先级清单

| 优先级 | 画板 | 用户任务 | 关键状态 | 合同依赖 |
|---|---|---|---|---|
| P0 | `00 Foundations` | 理解界面的语义层级与反馈 | 中性、成功、注意、失败、进行中、禁用、焦点 | 无；需先统一本文件与 `web/src/index.css` 的颜色、间距、圆角与内容宽度 |
| P0 | `01 Core Components` | 使用一致、可键盘操作的基础控件 | Button / StatusBadge / Table row / Empty / Alert / Skeleton 的 normal、hover、focus、disabled、error | Base UI 组件边界见 Spec 007 依赖章节；现有组件在 `web/src/components/` |
| P0 | `02 Runs` | 判断最近采集是否成功（Q1） | 加载中、空、正常、单页到底、多页、请求失败、存储不可达 | CG-01（事件游标语义待修，列表游标本身正确）；`GET /api/v1/runs` |
| P0 | `03 Run Detail` | 定位失败原因与具体行号（Q2、Q3、Q4） | 七种运行状态、进行中、有 last_error、无事件、事件截断、长消息、运行不存在 | CG-02；`GET /api/v1/runs/{id}` 与 `/events` |
| P0 | `04 Settings` | 知道当前按什么设置在跑，以及为什么某项没生效（Q5、Q6） | 文件不存在、四种 origin、env 覆盖警告、长路径 | CG-07、CG-08；`GET /api/v1/config` |
| P1 | `05 Narrow / Runs` | 在 390px 判断采集结果 | 与桌面相同的加载/空/错误状态；七列的取舍或折叠 | 不能只靠 CSS 让表格横向滚动；需先确定列优先级 |
| P1 | `06 Narrow / Run Detail and Settings` | 在 390px 读事件与设置 | 长事件消息、长配置路径、env 警告 | CG-02、CG-08 |
| P2 | `07 Settings / Edit` | 修改设置（下一片） | dirty、保存中、成功需重启、412 冲突、字段错误 | CG-07、CG-09；`PUT /api/v1/config`，由 M3 第 2 片承载 |

`07` 列在此处只为让结构探索预留位置，本轮不产出该画板的完成态。

## 真实规模样本

所有业务画板使用 [fixtures.md](fixtures.md) 的内容：六位数计数、32 字符来源名、96 字符事件消息、58 字符配置路径、1001 条事件的截断运行。**不得用三条短记录代替。**

## Desktop 验收

- 视口 1440px。六位数计数右对齐且不换行；32 字符来源名与 96 字符事件消息不裁切主信息。
- 每个页面只有一个主任务。运行列表的主任务是判断成败，不是浏览全部字段。
- `failed_count > 0` 的 `succeeded` 运行必须一眼看出"成功但有失败行"——这是最容易被误读的一行。
- `interrupted` 必须强调已提交计数仍然有效，否则用户会以为数据丢了。
- 事件截断必须显式，不能让读者以为只有 1000 个失败行。
- `env` 覆盖必须是显著状态而非附注。
- 所有 normal、focus、disabled、error 状态可从 `.pen` 直接定位；业务组件使用 reusable component/ref，不以导出 PNG 反推组件结构。

## Narrow 验收

- 视口 390px。运行列表不造成页面级横向溢出。
- 七列必须先做取舍：哪些保留、哪些进入次级行或折叠，而不是整表横向滚动。
- 长事件消息与长配置路径可读，允许自身换行或滚动，不撑宽页面。
- 导航在窄屏可发现、可达。

## 原型交付门槛

唯一源文件为 `docs/design/ui/prototypes/muxio-ui.pen`。交付前逐项核对：

1. 顶层画板名称与本清单一致，且包含非空业务节点；
2. Foundations、Core Components、Desktop 与 Narrow 均可从源文件独立定位；
3. reusable components/ref 与具体页面的引用关系可检查；
4. 对具体页面运行布局检查，确认无裁切和意外溢出；
5. PNG 仅作为评审快照，命名为 `<画板名称 slug>-<顶层 Pencil 节点 ID>.png`，并在评审记录中回填视口与状态。
