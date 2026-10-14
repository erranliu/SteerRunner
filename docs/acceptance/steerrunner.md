# SteerRunner 首版独立化专属验收

日期：2026-10-14。仅归属「SteerRunner 首版独立化及 SteerLine 接入」这一交付，不覆盖其他规划/运行时集成任务。[Spec](../spec/steerrunner.spec.md)是行为依据，[Plan](../spec/steerrunner.plan.md)给出实施与完整测试映射。状态：本轮契约自查及实施规划完成，尚无可运行程序或运行验收证据。

## 1. 最小验证范围与环境

下面的场景 ID 是本次规划标识，尚未成为现有测试入口。Plan §5已为每个自动项给出测试级别、组/场景、具体命令、环境、时点、通过条件及本文登记位置；Plan P1–P7明确其入口建设步骤。工具/驱动落实后才能执行，不能把规划命令当作已有能力。同一场景可覆盖多个项。

- unit：协议解析、请求去重、流程规则与依赖边界，使用定向测试。
- controlled：真实 EXE/CLI/服务/IPC/持久化、隔离 Git 仓库，使用受控代理和 provider，并记录实际动作计数。
- native：真实 Git/所选代理或 Orca 的能力、身份和准备核验；不以它代替真实模型/PR闭环。
- live：仅实际承诺的执行路径做最小真实实现→PR闭环，未选择的能力如实保持未验。
- integration：SteerLine 客户端经真实 CLI 契约接入独立 Runner，使用隔离任务数据库和受控 provider 验证任务交付。

SteerLine 临时调试与真实 PR 验证先使用其 `scripts/new-debug-release.ps1` 生成 `C:/SteerlineDebug` 独立环境。不得使用、修改或重置 `C:/SteerlineRuntime`；测试项目统一为 SteerlineTest，真实 PR 仅针对 `erranliu/SteerlineTest` 本次专属 base/head 分支。模拟方案显式选择 `--simulate`；不复制安装目录的数据库、认证或配置。原有分级夹具继续隔离使用。

Runner 自身受控测试使用本次新建的临时目录和独立 Git 夹具，不连接用户状态目录。live 使用本次专属运行目录/材料和明确的目标主机。执行环境的计划冻结，改变方案创建新现场。

## 2. 自动验收项

| ID / Spec 范围 | 最小场景与通过条件 | 环境及证据 | 状态 |
| --- | --- | --- | --- |
| SR-A01 / §1–4、10 | `package.standalone`：干净目录只有一个 EXE，可离线查询版本、启动服务、查询协议/schema；默认资源内嵌，不读 SteerLine 文件，不要求 Python/Node。检查依赖与迁出清单，Runner 不导入 Task/DAG/产品实现 | unit + controlled；package.log、dependency-audit.md | 待实施/待验 |
| SR-A02 / §5 | `cli.contract`：每个公开命令的成功/拒绝/未知输出符合协议；stdout 单 JSON、stderr 分离、UTF-8/中文正确；版本不支持、重复键、尾随对象、超限、未知字段在动作前拒绝；exit 0 不被解释为执行完成 | unit + controlled；cli-contract.jsonl、cli-stdout.log、cli-stderr.log | 待实施/待验 |
| SR-A03 / §3–4 | `service.ownership`：同目录及路径别名并发启动只有一个写者；独立目录各自隔离；CLI/父Job退出不杀服务，无法安全独立启动时明确失败且零Run接受；陈旧实例/未知旧写者不可接管；查询不自动启动，submit 可幂等启动 | controlled；service-identity.json、service-ownership.log、write-counts.json | 待实施/待验 |
| SR-A04 / §5.3、6 | `input.freeze`：inline/git 两种文档保存同一实际快照；变更/移除原文档、改变模板/Profile后不改变旧 Run；错 remote/base、非法路径、材料缺失不接受业务执行；重复 request 返回原 Run，变更正文或命令返回冲突；并发提交只有一次接受；响应丢失后可凭原request_id查询回执与Run | unit + controlled；input-manifest.json、request-receipts.jsonl、side-effects.json | 待实施/待验 |
| SR-A05 / §7 | `workflow.development`：实施→报告→发布及必要补报在原会话按绑定推进；提示词覆盖不删除固定材料/机器格式；格式修正有界、源失败不被 done 掩盖、未知/截断/错回合正文不推进；反馈更新原分支及同一 PR | unit + controlled；workflow-events.jsonl、prompt-digests.json、report-cases.json | 待实施/待验 |
| SR-A06 / §5.3、8 | `reliability.reconcile`：保存意图失败零外部动作；外部成功但回执提交失败只查原请求；CLI丢响应/服务换代不重复输入、推送或建 PR；服务重启只读对账，未消费输入/旧授权不自动发送；新 resume 按原身份且仅一次执行 | controlled；operation-journal.jsonl、fault-points.json、side-effects.json | 待实施/待验 |
| SR-A07 / §3、7.2、8 | `lifecycle.original-session`：stop先持久撤销新输入授权，原生停止unsupported/unknown及服务重启也不恢复后继输入；停止与观察退出分别表达，原会话/原问题/Attempt绑定核验；旧写者未结束、原目标未知、能力 unsupported/unverified时零替代会话/错目标回复；合法恢复复用同thread；迟到事件不推进其他Run | unit + controlled；bindings.json、lifecycle-cases.json、side-effects.json | 待实施/待验 |
| SR-A08 / §4–5、7.2 | `storage.logs-events`：所有本地执行日志落本次state-dir，远端可取得记录和缺口归原Run；已知凭据字段不入快照/日志；事件游标分页重读可去重；inspect返回完整持久事实和真实缺口；日志变化不驱动业务状态 | controlled；directory-audit.json、events-pages.json、redaction-cases.json | 待实施/待验 |
| SR-A09 / §7–9 | `publication.identity`：真实隔离Git commit/push配受控provider；正确PR归属才pr_ready；错仓库/base/head/URL拒绝，head推进/缺可选HeadRevision不成为成果锁；已有PR反馈复用；发布未知仅查询，补报不再次发布；refresh-pr只读刷新，查询失败不冒充最新或撤销原绑定 | controlled；git-facts.json、provider-queries.jsonl、pr-bindings.json、side-effects.json | 待实施/待验 |
| SR-A10 / §2、9–10 | `integration.steerline`：SteerLine只经CLI提交/查询/反馈/恢复/停止；原request幂等绑定候选；重复/丢失事件可补查询；正确绑定候选真实合并在SteerLine自有事务关闭一次并释放后继，其他候选未知/Runner阶段不成为门槛；Task终态待stop及迟到Run接受可重启对账，撤销回执后零新输入，确认窗口/native停止缺口不虚报；closed后零PR查询；Runner无任务图/TaskDelivery写权，原执行实现删除 | integration + controlled；caller-commands.jsonl、task-delivery.json、stop-receipts.jsonl、deletion-audit.md | 待实施/待验 |
| SR-A11 / §1、6–9 | `native.local-codex`：独立CLI在本次真实本地Codex环境完成最小Spec/Plan、必要检查、原生final及真实测试PR核验；材料/工作区/会话/PR来源明确；其他环境或模型不自动扩大矩阵 | live（先过相关controlled）；source-identity.json、native-facts.json、checks.log、pr-facts.json | 待实施/待验 |
| SR-A12 / §1、6–9 | `native.orca-targets`：实际承诺的Orca本机和environment分别证明准备/原身份/完成final来源及最小真实PR闭环；可复用一个很小任务定义，两个目标各有独立Run/分支/PR。未核实的停止、替代终端恢复、其他agent不声明支持，受控environment不冒充远端真机 | native + 最小live（先过相关controlled）；按target分目录的native-facts.json、checks.log、pr-facts.json、capabilities.json | 待实施/待验 |

SR-A01–SR-A10 优先复用既有规则、协议样本和隔离夹具，新增只覆盖新的CLI/宿主/跨仓边界。运行源码实际采用哪一份三层产物、哪些既有场景可复用，由实施 Plan 固定，不能引用不存在的命令作可用入口。

SR-A11/SR-A12 只做支持声明所需的最小真实闭环，不把全部故障点扩展成真实模型矩阵。故障行为主要由SR-A06/SR-A07受控反例覆盖。原生回执与最终正文必须来自实际公开协议，测试报告自身生成的文本不计为原生证据。

## 3. 人工验收

| ID | 内容 | 参与窗口与阻塞范围 | 状态 |
| --- | --- | --- | --- |
| SR-M01 | 用户按README在独立目录使用CLI，随后从SteerLine查看同一Run的反馈、PR和日志位置，确认产品表达可理解 | 单EXE及CLI接入可用后；参与者、时间和主机待定。体验检查可后置，不阻塞无关开发；发现执行/归属/恢复缺陷按对应自动项处理，不以人工后置豁免 | 待安排 |

## 4. 本轮契约审阅

本次为当前聊天自查，不宣称独立复审。已核对Spec、旧代码与另一工作区三层产物，修正以下确认问题：服务脱离CLI/父Job生命周期；stop先撤销新输入授权且保留原生停止缺口；跨仓Task终态的停止确认窗口；合法SSH用户身份。具体修订与实施安排见Plan §2。

三层产物已有固定代码提交，但主工作区调查HEAD尚无新包，另一工作区仍有验收/夹具变更。仅作为来源核对，未把它标作主干已交付。Plan P2要求最终主干交接，P1纯协议/宿主工作可先建设。

## 5. 证据和结果登记

实施 Plan 必须给出具体命令、超时、执行时点、工具补齐和证据根；不能只列本表的场景名称。每次验证记录两个仓库代码commit、Spec/Plan实际commit/path、EXE版本/摘要、CLI协议/资源版本、adapter/provider版本、主机、时间和场景数量。工作树材料尚未提交时登记其摘要，不能冒充Git版本。

测试诊断和导出结果可存于本次临时证据目录；本次真实执行的Runner业务数据/日志仍在明确state-dir。专属文档登记可定位的证据及结论，不要求把完整运行数据库、原生日志或凭据提交到仓库。

| 日期 | 项目 | 代码/文档版本与环境 | 实际命令/入口 | 结果 | 证据 |
| --- | --- | --- | --- | --- | --- |
| 2026-10-14 | 本轮文档检查 | SteerRunner初始空仓库；本文与Spec工作树；SteerLine源码调查HEAD a81341a515251e23b4fa19d7a09096c11d0db8dc | PowerShell逐文件核验相对链接、ConvertFrom-Json解析示例、验收ID覆盖、代码围栏/尾部换行/空白；git diff --check | 通过：3份文档、5个本地链接、3个JSON示例、12个自动项及1个人工项；新文件另经逐文件检查 | 本轮工具输出；不计运行验收 |
| 2026-10-14 | 契约审阅与Plan文档检查 | 新仓库4份工作树文档；来源与在途三层commit见Plan §1 | PowerShell核对本地链接/锚点、ConvertFrom-Json解析示例、逐项展开测试映射、对照SteerLine suites.json、表格列数/代码围栏/空白；git diff --check | 通过：4份文档、12个链接/锚点、3个JSON示例；12个自动项映射14组入口，5个旧场景已登记；只证明文档与规划一致，不计运行验收 | 本轮工具输出 |

| 项目 | 当前结论 |
| --- | --- |
| Spec / Plan / acceptance覆盖与示例检查 | 2026-10-14通过本轮文档检查与契约自查；运行能力尚未实施 |
| Runner构建及独立执行 | 未实施/未验 |
| CLI / IPC / 服务 / 幂等 / 恢复 | 未实施/未验 |
| SteerLine切换及删码 | 未实施/未验 |
| 本地Codex / Orca本机 / Orca远端真实PR | 分别未验 |

验收失败保持失败及原证据，不重跑掩盖。历史SteerLine或第三方项目的能力说明只能作为设计参考，不能记作本次通过。
