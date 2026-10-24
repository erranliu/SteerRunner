# Orca CLI 等待与重试：审计与验收规划

日期：2026-10-25。状态：设计已审计，功能待实施、待验收。关联 [Spec](../spec/orca-cli-wait-retry.spec.md) / [Plan](../spec/orca-cli-wait-retry.plan.md)。本轮授权审计文档、提交和 push，不授权实施或部署。

## 设计审计结论

Runner 有独立收益：直接使用其 CLI 的用户，同样可能因通道竞争耗尽准备读取预算；本仓独立修正等待/执行期限混用、有界读取重试和等待可见性，不需要 SteerLine 才产生价值。验收只依赖 Runner 服务/CLI、SQLite、隔离夹具与 Orca 目标，调用方 UI 联验不是门槛。

| 原问题 | 修订决定 |
| --- | --- |
| 三类加权 dispatcher、强制合并器与统一采样退避超出必要范围 | 只在 SerializeCLI 内增加同进程 FIFO，保留原命名互斥；不调整采样周期 |
| 为写操作加 120 秒窗口与核验子窗口，重叠原节点策略 | 仅为 remote repo list / local status 定义单层读取窗口；写操作、unknown 与停止保持 |
| disposition/effect/dispatch_state 等重复 operation/status/controls | 最小 execution_wait 仅描述准备查询；未知副作用继续用已有事实 |
| Runner 完成依赖 SteerLine 联验 | 本仓独立测试、原生资格与公开契约形成自身交付，调用方另验 |
| 等待事件推进 revision，可能被原 preparationQuery 判 stale 或被旧 clone 覆盖 | Plan 要求区分自有进度与真实控制/身份竞争、合并当前聚合；不另建 revision/store |
| 原调用链已有 90 秒 context，单独承诺 120 秒会暴露虚假等待期限 | 删除独立 120 秒预算，复用原 90 秒上限并取原准备/caller context 更早期限；不延长父操作/写操作预算 |

以上是静态设计与源码边界审计，不是实现/原生能力通过。未采用通用队列/重试框架、迁移兼容层或替代 owner。

## 自动验收计划

| 项目 | 最小条件 | 状态 |
| --- | --- | --- |
| OCW-A01 | >15 秒排队后有效窗口内自动继续一次 | 待验收（未实施） |
| OCW-A02 | 两项准备读取的单层预算/分类/次数，空 JSON 错误修正 | 待验收（未实施） |
| OCW-A03 | 同进程 FIFO/取消、原命名互斥、owner 无重入死锁 | 待验收（未实施） |
| OCW-A04 | 原身份/预算跨重启、自有 revision 和旧 clone/停止竞争正确 | 待验收（未实施） |
| OCW-A05 | capability/schema/同快照/事件及清除一致 | 待验收（未实施） |
| OCW-A06 | 原写操作期限、unknown 和资源义务不变，stop 后零新输入 | 待验收（未实施） |
| OCW-A07 | 独立 local / remote 竞争/恢复/取消和清理 | 待验收（未实施），分目标记录 |

按 Plan 执行，输出 `C:/SteerlineDebug/runner-cli-wait-retry/<唯一执行ID>/`。记录 source/dirty、EXE SHA/version/resource、原命令、选中 ID、Run/operation、结果和清理；失败不覆盖。禁止向 SteerLine 安装 home、共享 state-dir 或 Task 18 注入测试。当前不安排人工签收，本仓完成不依赖调用方。

## 文档验证记录

首轮原稿曾通过两仓本地链接及空白/示例一致性检查，仅适用于原稿。审计修订在提交前重新检查并登记，不继承原稿功能资格。全部功能项仍待验收。

审计修订静态检查：两仓 8 份新增/修改文档、225 个本地链接、代码围栏/UTF-8/尾随空白和最小公开字段/枚举/JSON 示例检查通过；SteerLine 全库链接检查 576 份 Markdown、3605 个本地链接，0 错误；两仓 diff 空白检查通过。修改仅限文档，未运行产品 L1/Controlled/原生测试。推送前另核对暂存文件仅为本 Spec/Plan、专属审计验收记录与文档索引。
