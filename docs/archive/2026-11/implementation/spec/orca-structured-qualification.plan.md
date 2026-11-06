# Plan A：结构化真机验收接线

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-06。状态：P0–P4 已完成实现与受影响受控验收；真实 L3 未执行；依据 [Spec](orca-structured-qualification.spec.md)，必须先独立验收 A 后再实施 B。

1. P0 核对 config → fixture → manifest → driver → result 的实际注册链；逐函数列 shared/current/历史消费者，检查旧验收要求与当前 scope 的差异。审计 Spec/Plan 后记录确认和取舍。
2. P1 新建测试侧集中原结构化身份/完整历史核验，配置/manifest/CheckPR 改接；增加错误路径和反例，所有私有编码解释限测试支持边界。
3. P2 当前 driver 的诊断、原反馈/重启/模型/停止改接；本机 named-pipe 原样转发、凭据隔离、原回执丢失/零重发。远端不改 host key；显式区分原历史语义计数与实际 wire 计数。旧 helper 独占探针删除，共享 stability 工具不动。
4. P3 受影响组件与 framework、正式构建/live 编译、公开受控定向；真实配置先检查可用性，再运行具备前提的正式真机入口，未具备资格明确列缺口。不把模拟实际模型或 Job 清场当当前 Orca 证明。
5. P4 独立审计消费者、共享夹具、格式、结果守卫和零匹配；写 docs/acceptance/orca-structured-qualification-proof-cleanup.md，固定 A 验证点；此后才开始 B。

设计审计及实施更正：Orca audited 没有当前注册真机入口，不虚构 orca-local.audited；相应模型证明 helper/脚本只保持当前解释和框架反例。close 后不强求历史继续可读，核对冻结原输入集和原 close 回执。本批目录注册文件不变。A 组件、完整 live/helper 编译、目录/结果框架与两个公开受控顶层（创建恢复七窗口及 cold stop）通过；不等于完整 Controlled 或实际 L3。
