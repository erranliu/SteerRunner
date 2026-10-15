# SteerRunner 首版独立化专属验收

后续安装期 Windows broker 权限修复单独记录在 [Windows 启动验收](windows-broker-install.md)；空闲服务的通过不改变本页产品真机/R1 的既有失败边界。

> 当前建设范围已恢复远端 Orca，采用 [独立三级测试契约](../testing.md)。L2/L3 均从冻结
> Spec/Plan 开始，经过真实模型实施、检查、PR、反馈、合入、合并事实核验及停止收尾。
> L3 是持续扩展的真机场景集。以下为历史验收原始记录；旧通过与旧取消结论不替代新套件结果。

日期：2026-10-15。[Spec](../spec/steerrunner.spec.md)是产品行为依据，[Plan](../spec/steerrunner.plan.md)保留首版拆分与跨仓切换映射。当前本仓独立测试结果见 §10；§1–9 保留历史版本、当时的范围调整和失败证据，不作为新套件的通过依据。来源见 [source-map](../extraction/source-map.md)。

## 1. 最小验证范围与环境

下面的场景 ID 是本次规划标识。基础入口 B1/U1/U1-S/U1-H/C1/C2 已建设，其他入口按 Plan P2–P7 建设。Plan §5为每个自动项给出测试级别、组/场景、具体命令、环境、时点、通过条件及本文登记位置；不能把后续规划命令当作当前已有能力。同一场景可覆盖多个项。

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
| SR-A01 / §1–4、10 | `package.standalone`：干净目录只有一个 EXE，可离线查询版本、启动服务、查询协议/schema；默认资源内嵌，不读 SteerLine 文件，不要求 Python/Node。检查依赖与迁出清单，Runner 不导入 Task/DAG/产品实现 | unit + controlled；package.log、dependency-audit.md | 部分通过，详见§6；完整项待验 |
| SR-A02 / §5 | `cli.contract`：每个公开命令的成功/拒绝/未知输出符合协议；stdout 单 JSON、stderr 分离、UTF-8/中文正确；版本不支持、重复键、尾随对象、超限、未知字段在动作前拒绝；exit 0 不被解释为执行完成 | unit + controlled；cli-contract.jsonl、cli-stdout.log、cli-stderr.log | 部分通过，详见§6；完整项待验 |
| SR-A03 / §3–4 | `service.ownership`：同目录及路径别名并发启动只有一个写者；独立目录各自隔离；CLI/父Job退出不杀服务，无法安全独立启动时明确失败且零Run接受；陈旧实例/未知旧写者不可接管；查询不自动启动，submit 可幂等启动 | controlled；service-identity.json、service-ownership.log、write-counts.json | 部分通过，详见§6；完整项待验 |
| SR-A04 / §5.3、6 | `input.freeze`：inline/git 两种文档保存同一实际快照；变更/移除原文档、改变模板/Profile后不改变旧 Run；错 remote/base、非法路径、材料缺失不接受业务执行；重复 request 返回原 Run，变更正文或命令返回冲突；并发提交只有一次接受；响应丢失后可凭原request_id查询回执与Run | unit + controlled；input-manifest.json、request-receipts.jsonl、side-effects.json | 部分通过，详见§6；完整项待验 |
| SR-A05 / §7 | `workflow.development`：实施→报告→发布及必要补报在原会话按绑定推进；提示词覆盖不删除固定材料/机器格式；格式修正有界、源失败不被 done 掩盖、未知/截断/错回合正文不推进；反馈更新原分支及同一 PR | unit + controlled；workflow-events.jsonl、prompt-digests.json、report-cases.json | 待实施/待验 |
| SR-A06 / §5.3、8 | `reliability.reconcile`：保存意图失败零外部动作；外部成功但回执提交失败只查原请求；CLI丢响应/服务换代不重复输入、推送或建 PR；服务重启只读对账，未消费输入/旧授权不自动发送；新 resume 按原身份且仅一次执行 | controlled；operation-journal.jsonl、fault-points.json、side-effects.json | 待实施/待验 |
| SR-A07 / §3、7.2、8 | `lifecycle.original-session`：stop先持久撤销新输入授权，原生停止unsupported/unknown及服务重启也不恢复后继输入；停止与观察退出分别表达，原会话/原问题/Attempt绑定核验；旧写者未结束、原目标未知、能力 unsupported/unverified时零替代会话/错目标回复；合法恢复复用同thread；迟到事件不推进其他Run | unit + controlled；bindings.json、lifecycle-cases.json、side-effects.json | 待实施/待验 |
| SR-A08 / §4–5、7.2 | `storage.logs-events`：所有本地执行日志落本次state-dir，远端可取得记录和缺口归原Run；已知凭据字段不入快照/日志；事件游标分页重读可去重；inspect返回完整持久事实和真实缺口；日志变化不驱动业务状态 | controlled；directory-audit.json、events-pages.json、redaction-cases.json | 部分通过，详见§6；完整项待验 |
| SR-A09 / §7–9 | `publication.identity`：真实隔离Git commit/push配受控provider；正确PR归属才pr_ready；错仓库/base/head/URL拒绝，head推进/缺可选HeadRevision不成为成果锁；已有PR反馈复用；发布未知仅查询，补报不再次发布；refresh-pr只读刷新，查询失败不冒充最新或撤销原绑定 | controlled；git-facts.json、provider-queries.jsonl、pr-bindings.json、side-effects.json | 待实施/待验 |
| SR-A10 / §2、9–10 | `integration.steerline`：SteerLine只经CLI提交/查询/反馈/恢复/停止；原request幂等绑定候选；重复/丢失事件可补查询；正确绑定候选真实合并在SteerLine自有事务关闭一次并释放后继，其他候选未知/Runner阶段不成为门槛；Task终态待stop及迟到Run接受可重启对账，撤销回执后零新输入，确认窗口/native停止缺口不虚报；closed后零PR查询；Runner无任务图/TaskDelivery写权，原执行实现删除 | integration + controlled；caller-commands.jsonl、task-delivery.json、stop-receipts.jsonl、deletion-audit.md | 待实施/待验 |
| SR-A11 / §1、6–9 | `native.local-codex`：独立CLI在本次真实本地Codex环境完成最小Spec/Plan、必要检查、原生final及真实测试PR核验；材料/工作区/会话/PR来源明确；其他环境或模型不自动扩大矩阵 | live（先过相关controlled）；source-identity.json、native-facts.json、checks.log、pr-facts.json | 本机已通过，见§8；后续目录修正须新包复验 |
| SR-A12 / §1、6–9 | `native.orca-targets`：实际承诺的Orca本机和environment分别证明准备/原身份/完成final来源及最小真实PR闭环；可复用一个很小任务定义，两个目标各有独立Run/分支/PR。未核实的停止、替代终端恢复、其他agent不声明支持，受控environment不冒充远端真机 | native + 最小live（先过相关controlled）；按target分目录的native-facts.json、checks.log、pr-facts.json、capabilities.json | 用户取消；未计通过 |

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
| Spec / Plan / acceptance覆盖与示例检查 | 契约文档已固定；实施补充了 Windows 启动机制、settings schema、当前能力和定向入口 |
| Runner构建及独立执行 | P0/P1 可构建一个 EXE；干净目录版本→服务→schema通过。业务执行尚未接通 |
| CLI / IPC / 服务 / 幂等 / 恢复 | 已验证基础 CLI/命名管道、唯一写者、独立进程和停服命令回执；Run 幂等、原生恢复未实现/未验 |
| SteerLine切换及删码 | 未实施/未验 |
| 本地Codex / Orca本机 / Orca远端真实PR | 分别未验 |

验收失败保持失败及原证据，不重跑掩盖。历史SteerLine或第三方项目的能力说明只能作为设计参考，不能记作本次通过。

## 6. P0/P1 实施交付

已建设 `cmd/steerrunner`、`internal/cli`、`contracts`、`host`、`storage`、`resources` 和真实 EXE 进程夹具；无 SteerLine Go 导入，没有动原产品工作树。默认 workflow/提示词/schema 内嵌；依赖 go-winio、x/sys、纯 Go SQLite，没有引入额外执行引擎。Windows 自动启动的系统 broker 使用系统自带 PowerShell/CIM，不读取外部脚本文件。

| 验收项 | 此阶段实际覆盖 | 完整项结论 |
| --- | --- | --- |
| SR-A01 | B1/C1：只复制 EXE 到干净目录、中文状态路径、离线 version、服务与 capabilities/schema；声明 execution unsupported；合法形状 submit 明确拒绝且没有持久回执 | 部分通过，执行规则/原生提取依赖审计待 P2 |
| SR-A02 | U1：精确字段/重复键/非法UTF-8/null/控制字符/超限/多对象/未知设置/不支持版本/参数错误；单JSON和退出码。C1/C2：真实CLI成功、拒绝、持久停服回执 | 部分通过，Run 命令成功与未知结果路径待 P3/P4 |
| SR-A03 | C2：别名并发一个实例、不同目录隔离、CLI退出存活、允许父Job关闭仍活、禁止breakaway明确失败且无DB、只读查询不启动、原写者持锁拒绝第二serve、旧进程死亡后换代、旧实例stop拒绝；U1-H：原启动未知保留意图且不盲重建 | 基础宿主通过；submit幂等启动/Run接收待 P3/P4 |
| SR-A04 | U1/U1-S/C2：纯输入形状及路径/已知秘密配置反例；停服ID幂等、不同正文冲突、持久回执查询、旧ID不停止新服务 | 部分通过，未实现文档物化/快照及Run接受，不冒充 input.freeze 通过 |
| SR-A08 | C1/C2：服务日志、DB、发现及启动痕迹归本次 state-dir；CLI不打开DB | 仅服务目录部分通过；Run/远端日志/事件/完整凭据处理待 P3/P4 |
| SR-A05–SR-A07、SR-A09–SR-A12 | 未进入执行规则、原生或跨仓场景 | 未实现/未验 |

Windows 启动层的早期验证曾失败：原生 breakaway 创建成功后仍有关联 Job，保留 `.evidence/foundation-20261014-105427/integration.log` 和 result.json（exit 1）。后续加入系统 broker 和身份/权限核验，`.evidence/foundation-20261014-111246`、`111559`、`111759` 对应版本两组均 exit 0；这些是递进版本记录，不替代最终来源版本证据。

该阶段构建和验证来源为干净提交 `7cfad777b0e41e19000cc6fda6cc21973109e0ad`。5 个定向 unit 入口、2 组真实 EXE 进程场景、定向 go vet 通过，无skip；详细命令/摘要/原始证据位置见[基础层验收记录](evidence/steerrunner-foundation-20261014.md)。当时 P2 主干前置未满足；2026-10-15 已核实 `d20a72284705b4daac476d4fa5770151dca03e56` 完成三层交接并继续提取。历史结论不替代下面的新执行验证。

## 7. P2/P3/P4 继续实施

已从固定主干提取三层及原生/Git/PR owner，独立 application/runstate/SQLite 提交 Run、命令、原意图与事件。默认流程到 pr_ready；不含 Task/DAG/Delivery。恢复不新建替代原生会话，重启零自动输入，旧未发送答案/反馈需要新用户请求。Orca 采用输入内冻结正文，已做不访问本机路径的单元反例，尚不计原生闭环通过。

| 当前执行证据 | 实际结果与范围 |
| --- | --- |
| `.evidence/extraction-20261015/ported-tests.jsonl` | 来源执行子树 341 个 pass 节点；5 个 skip 分别为两个 subprocess helper 和三个 native opt-in，不计真机通过。新增工作流正文/依赖守卫另行通过 |
| `.evidence/foundation-20261014-232904` | exit 0 / 5 组：单 EXE、服务所有权、三轮开发+反馈、原输入回执丢失零重发+停止、服务重启零自动输入+当前原 session resume |
| `.evidence/foundation-20261014-233133` | exit 0 / 1 组：并发同 ID 只接受一个 Run，inline/git 固定快照、源文件变更不影响执行、改正文冲突，实际 native 一会话三轮 |
| `.evidence/foundation-20261014-233246` | exit 1 / 2 组：缺原 final 阻止后继通过；PR 刷新测试解码到复用对象，遗漏字段未清零而误报失败，保留原失败。修正测试使用新对象，未放宽产品归属核验 |
| `.evidence/foundation-20261014-233540` | exit 0 / 1 组：发布正文只补报一次（共四轮）；错误 PR head 刷新记录 query_error 且保留旧事实时间，恢复正确 provider 后刷新；查询零原生输入 |
| `TestSRRunAtomicCommit` | 事件提交触发失败，Run/命令/事件全部回滚；重复提交零额外事件、跨 Run 游标拒绝、整数指纹不丢精度 |
| `TestSROrcaFrozenMaterials` | 无远端/本机共享目录下，固定正文、commit、摘要及引用完整进入实施输入，覆盖正文不能删除固定材料 |

以上进程证据记录了各次构建的 source_dirty、EXE 摘要和调用输出；是递进工作树验证，最终清洁来源验证另行登记。完整 SR-A01–SR-A12 仍按对应项逐项完成，不将这些局部通过外推为全部验收。

### 2026-10-15 继续验证

| 证据 | 结果与限制 |
| --- | --- |
| `.evidence/foundation-20261014-234128` | 干净提交 `f5c99cadfc646ba87a252edea26883eb86e336d3`，exit 0 / 8 组实际 EXE 场景；构建 SHA `ac60ec162bc54f5edc9d354829244035b0783db8a7744d8979110e882ab1be73` |
| `.evidence/foundation-20261015-001224` | 新增发布报告尚未完成时先绑定已核实 PR，仍保持 running；fresh refresh 可暴露真实 merged；stop 后零新输入/后台 PR 查询。exit 0 / 1 组 |
| `.evidence/foundation-20261015-000958` | 保留失败：受控 bare remote 的 Windows 路径过长导致 Git `$GIT_DIR too big`，原生结果未知未重发；夹具改为自身短临时目录后通过，未放宽生产核验 |
| `.evidence/foundation-20261015-003511` | 加入固定权限资源构建与提交后，exit 0 / 5 组：包/所有权/材料/PR身份/提前绑定 |
| `.evidence/foundation-20261015-003706` | 10 组中既有 9 组全部通过；新增权限资格场景因测试记录器将空调用日志当非法 JSON 失败。原生版本不合格实际上已在零 thread/turn 创建处阻断。完整运行 exit 1，不能写作全通过 |
| `.evidence/foundation-20261015-004040`、`004158` | 空日志文件处理的第一次修复仍因空行解码失败；修复空行后权限资格场景 exit 0 / 1 组。两份失败证据均保留 |
| SteerLine `.evidence/steerrunner-cutover/` | 新 CLI/桥接及 SQLite 原子候选/交付定向通过：正确合并候选不要求 pr_ready，兄弟未知不阻塞；故障全回滚。实际 app 切换与旧执行域删除仍未完成，SR-A10 待验 |

本地真机 SR-A11 已执行但未通过：独立包 `C:\SteerlineDebug\dev-20261015-075410-d20a7228-dirty` 的桌面 Codex `0.162.0-alpha.17.2` 和 `dev-20261015-080121-d20a7228-dirty` 的固定 native `0.159.2` 均遇到 `windows unelevated restricted-token sandbox cannot enforce split writable root sets directly; refusing to run unsandboxed`。前者撤销并停止原 Run；后者原生 final 核实为 blocked，不能算实施成功。没有测试 PR。证据在各包 `evidence/native-*`、`evidence/runner-native-calls.jsonl` 及原 Run app-server 日志。桌面不同版本在补齐权限资源资格检查后应于原生启动前拒绝，不再将它当固定版本的可用替代。

远端 SR-A12 使用用户授权 Orca「台式机64G」，注册仓库 `C:\Projects\SteerlineTest`、远程 URL 核实为 `erranliu/SteerlineTest`。首次包 `C:\SteerlineDebug\sr\dev-20261015-083903-d20a7228-dirty` 的 Run `bdd3f96ba780860491ca879a4d7af59b` 在 `git worktree add` 因远端未获取固定 base commit 失败。Runner 保留原操作未知，不新建替代执行；已接受撤销新输入并请求停止自有服务。原生工作区/代理和 PR 均未完成。只读库存、专属 base fetch 的自有终端及关闭回执留在包/`.evidence/remote-base-*`。之后新包应先完成明确环境预检；不将手动 fetch、helper、终端空闲或旧失败写作真机闭环通过。未修改安装环境、用户认证或原生权限策略。

后续包 `C:\SteerlineDebug\sr\dev-20261015-085549-d20a7228-dirty` 使用干净 Runner `20fd3f89188e6fdede2a581dca05e054b6580ac8`。远端预检保留一次无输出及一次 GitHub 网络失败；新预检通过后，Run `693e2b7f86804d701b4f694a39ebd825` 创建了正确 base 的原工作区和终端，但 Codex TUI 报 `account/read failed … workspace routing discovery timed out`，未发送业务输入，无 PR。还暴露应用层未持续对账 accepted+ResourcePending 的遗漏，产生多份内部启动意图；适配器保留 LaunchAttempted，未再次发送启动命令。准备期限后 blocked；native harness exit 1，Run stop 和服务 stop 回执已留存。停止服务后另按原 runtime/handle/incarnation 核实并关闭本次终端，`ptyKilled=true` 的 CLI 回执在 `.evidence/native-remote-2-terminal-close.json`，不冒充 Runner 原停止动作已结算。

该应用遗漏已补成：accepted 且原资源仍 pending 时只对账原 Operation、暂停即时准备循环、无变化不增加 revision；准备超时撤回输入授权，迟到就绪须新 resume。`TestSRAcceptedResourceUsesOriginalOperation` 的正常/超时两个反例通过；`.evidence/foundation-20261015-011213` 的 workflow/unknown-stop/restart 三组 exit 0。这些验证不解决远端原生账户路由故障，SR-A12 仍未通过。

本地沙箱能力探针在独立包 `C:\SteerlineDebug\cap\dev-20261015-091408-d20a7228-dirty` 执行：已安装固定 `0.159.2` 和桌面版本分别显式选择 `mxc` 运行只读 echo 成功；固定版本对非法内核配置明确拒绝，证明选项被实际解析。证据为该包 `evidence/mxc-*.log`、`invalid-kernel-*.json`。据[官方 Windows sandbox 说明](https://learn.chatgpt.com/docs/windows/windows-sandbox)，改用显式 MXC；本地固定权限契约更新为 v4，继续固定 native 版本、workspace-write、原写入根、on-request/auto_review、网络和临时目录限制，不添加内核回退。探针不是 SR-A11 闭环通过，linked worktree 的真实读写/PR 仍须新包验收。

`.evidence/foundation-20261015-011953`：含 ResourcePending 修正与 MXC 的实际 EXE，8 组 exit 0；覆盖材料固定、权限资格、完整受控工作流、日志/事件、PR 身份/提前绑定、未知原输入/停止及重启显式恢复。`.evidence/foundation-20261015-013358` 的服务所有权组 exit 0，新增同版本号但不同 EXE SHA 的冲突测试：不能接管原服务，实例不变。

本地新包 `C:\SteerlineDebug\sr\dev-20261015-092541-d20a7228-dirty` 的 Run `be918ac793bab3b34581e624fb3c36c9` 使用 MXC，已实际读取冻结 Spec/Plan，写入并提交 result.txt，但原始 sandbox-check 首次报告 TEMP 不在指定目录；后一次探针被原生代理中断，仍为失败。12 分钟时处于 implement.report，缺发布/PR，harness exit 1（726.98 秒），服务已核验 not_running。完整 final 保留真实探针失败，不能将原生 turn completed 或 Git commit 当作任务验收成功。后续启动显式添加 `--no-daemon --strict-config`，避免会话被路由到其他环境的原生 daemon；这是待真机验证的修正，不声明已解决 TEMP/探针问题。没有测试 PR。

用户已确认 Apache-2.0；仓库加入完整 LICENSE、NOTICE，保留来源提交及第三方说明。许可证选择不改变当前尚未通过的真机与跨仓切换验收状态。

干净构建 `bd6353377e08e9aace7660fcc5a68acfc194ac4f`（EXE SHA `8247cb16ef4707556aa95cb21851e5badf4723f70cc8441bb36cce9c8171cb48`）在新包 `C:\SteerlineDebug\sr\dev-20261015-095009-d20a7228-dirty` 复验。Run `e27398eeadcb2c6cfd686cdac8b482fe` 的原始探针仍因 TEMP 不匹配失败，证明 no-daemon 单独不足；12 分钟时尚在 publish、无已核实 PR，harness exit 1（728.69 秒）。新 cleanup driver 在停止原服务前等待原 Run，已记录 `stop_status=stopped`，不再用服务 stop 回执代替原生停止事实。

按[固定版本原生环境构造实现](https://github.com/openai/codex/blob/rust-v0.159.2/codex-rs/protocol/src/shell_environment.rs)，新增任务公开运行值的 shell_environment_policy.set 覆盖：TEMP/TMP、已过滤 PATH、维护禁用的 Git count/新增索引；不输出或复制继承 Git 认证配置，不修改用户配置或权限根。权限资源契约为 v5。原生效果待新包复验；`.evidence/runtime-tool-env.log` 的 Codex/application 定向通过，`.evidence/foundation-20261015-020258` 的权限资格和完整受控工作流两组 exit 0。原生 driver 加强为核验首次探针实际 item/completed 的退出码和 temp/兄弟写入拒绝 proof，模型总结、提交或 PR 本身不满足该验收项。

v5 新包 `C:\SteerlineDebug\sr\dev-20261015-100648-bdd894a8-dirty` 的 Run `4b222526ce17594724cc52cd5eec5614` 仍失败，日志首次输出实际路径，确认 MXC 将 TEMP 设为原生 `Packages/sandbox.{…}/AC/Temp`。强制等于 Git admin 是旧验收与原生内核语义冲突；移除工具 TEMP/TMP 强制值，改为 v6 的宿主临时目录/原生工具临时目录分工，固定写入根不变。保留公开 PATH/Git 禁用维护配置；不关闭沙箱或添加内核回退。

原生能力包 `C:\SteerlineDebug\cap\dev-20261015-101834-bdd894a8-dirty` 的 `native-kernel-probe-2.log` 与 result.json 已核验 exit 0：实际 TokenIsAppContainer、TokenAppContainerSid 与 GetAppContainerFolderPath，TEMP 位于其 OS 容器目录，临时文件写读成功，本包兄弟 sentinel 的写访问为 PermissionDenied。原始 native-kernel-probe.log 的 `--strict-config` 不适用 sandbox 子命令错误保留，第二次使用该命令支持的参数，没有更改内核或权限范围。实现依据：[Windows token 信息](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ne-winnt-token_information_class)、[容器目录 API](https://learn.microsoft.com/en-us/windows/win32/api/userenv/nf-userenv-getappcontainerfolderpath)。此探针不是 Spec→PR 闭环通过。

v6 `.evidence/native-kernel-temp-unit.log` 的 Codex/application 定向通过；`.evidence/foundation-20261015-021835` 权限资格/完整受控工作流两组 exit 0。原生 driver 在实际首次探针失败时立即保存 Run 并清理，避免把已知失败拖成发布超时。完整本地和 Orca 真机验收仍待对应新包运行结果。


2026-10-15 后续本地复验：干净 `4bb484d` 的新包 `C:/SteerlineDebug/sr/dev-20261015-103740-bdd894a8-dirty`（Run `5f0cdc218311df889d4c7bca696393ad`）首次真实探针 exit 0，实际 AppContainer/TEMP/兄弟写拒绝均通过，提交 `212a761`；发布在 MXC 中因 `gh` HTTP 401 阻塞，完整 harness exit 1（454.01 秒）。宿主已有登录有效；没有复制认证/配置。此结果只解决真实沙箱检查，不是 Spec→PR 验收通过。

干净 `9b5b527`、EXE SHA `152f4b191e746e36f2b5bb790e787372bb3b14aeb56fb2e468fc7f5f579b508a` 的新包 `C:/SteerlineDebug/sr/dev-20261015-110134-bdd894a8-dirty`，Run `23206cc624f4651d02c6863149f9d1e9`：首次沙箱检查通过，原生提交 `d95d57d`。固定发布命令普通沙箱返回 401；同一命令的原生自动审核明确 approved（review `f3cdd9a5-0d6e-496b-868c-48eb84025bce`），之后只见原命令开始与轮询，无完成回执。模型终答将其描述为“审核待返回”，该文本不能代替审核事实。harness exit 1（443.32 秒），原 Run stop_status=stopped；停止后按专属 base/head 查询 GitHub 为 `[]`，证据 `.evidence/native-auth-provider-after-stop.json`。完整真机发布仍未通过，不声明审核拒绝或发布成功。

SteerLine 实际服务接入在新模拟包 `C:/SteerlineDebug/caller/dev-20261015-111825-bdd894a8-dirty` 暴露其 managed Job 禁止 BREAKAWAY（flags 0x2000），常驻启动在任何 Run 前拒绝。Runner 扩展已有的暂停子进程 broker：仅在已核实父 Job 且 CreateProcess 明确 AccessDenied 时尝试；仍拒绝受限/提升 token，核对同 SID、integrity、授权组/权限、EXE、创建时间，并在服务执行存储前验证无 Job。`.evidence/foundation-20261015-032334` 服务归属组 exit 0；加强为 ordinary allow/deny Job 都必须成功且父 Job 关闭后同实例存活的 `.evidence/foundation-20261015-032530` exit 0。实际 SteerLine 包须使用新 EXE 重建复验，原失败包保持冻结。


2026-10-15 后续：SteerLine 干净 `849f8737` 的实际 CLI 接入暴露 Windows 深目录加专属 base 分支路径超过 Git 默认限制。Runner `5f664a9` 对自有 Git 进程和原生工具环境设置 scoped `core.longpaths=true`，固定运行契约 v7，不修改全局/仓库配置或权限根。`.evidence/git-longpaths-integration.log` 真实长 ref clone 通过；`.evidence/foundation-20261015-042423/result.json` 两组（权限资格、完整工作流）exit 0。

台式机64G 的新包 `C:/SteerlineDebug/srremote/dev-20261015-122505-849f8737` 已仅推送专属 base `314d2d10bbefdd281ba601d5e17dfaa9e1a3c5c5`，经独立准备终端验证 fetch/cat-file 后关闭原终端。原生 Run `f8cec0f0d039831fbed5c22025eb270c` 创建工作区/终端并接受启动命令，尚无业务输入；对账进入 unknown，诊断 pending lifecycle resource。发现应用层对账传回创建前快照，丢掉已提交的 LaunchAccepted 事实。修正为在同 Attempt/generation/config 下使用当前已提交事实，保留原 operation、输入、binding digest 和原 intent snapshot；不重发启动。`.evidence/reconcile-current-facts.log` 定向通过，新增检查原操作不变、零执行重发且旧快照未重写。实际远端修正需要新包复验，当前包保持冻结，不计 SR-A12 通过。


`ede6dc5` 的 `.evidence/foundation-20261015-044428/result.json` 四组实际 EXE 回归 exit 0：权限资格、完整工作流、未知输入/停止、服务重启显式恢复。原远端第三轮 harness exit 1、1 场景，原 Run 最终 stop_status=stopped，未创建测试 PR；不得将修正后的受控通过写作该冻结旧包的真机通过。

SteerLine 后续完整产品包 `C:/SteerlineDebug/caller/dev-20261015-124412-ef780c60` 使用此版，已在原会话完成实现/报告；受控 helper 发布时暴露原 Git 缓存布局的 linked-worktree admin 路径过深，Git 报 `$GIT_DIR too big`。改为私有 `repos/<128-bit SHA-256 prefix>`，缓存命中仍验证精确 remote/冻结提交，不改变原完整输入摘要、分支/Run 身份或日志归属，不读取旧布局。`.evidence/compact-cache-publish.log` 真实长分支 clone 与较深 state-dir 的 linked worktree push 两项通过。必须新包复验实际产品链路，旧包不替换运行文件。


2026-10-15 后续验收：

- SteerLine 实际受控包 `C:/SteerlineDebug/caller/dev-20261015-130023-025de7fc` 使用干净 Runner `6b18f3e` / EXE SHA `c27e84a1c30f5a567bfd76beeef1c0b6134ce2ca182ede15db53556f3560c65b`，独立产品 CLI 完成三阶段、真实 Git push、受控 PR、合并交付及后继、停止与重置；result-13 exit 0 / 1 场景，137.44 秒。基础跨仓闭环通过，SR-A10 的恢复和固定最终版本子项尚未全部完成。
- 远端第四轮 `C:/SteerlineDebug/srremote/dev-20261015-130412-7b42cd7f`，Run `ad72d46669e9520db416c53739aa204d`，1 场景 exit 1（191.54 秒）。原启动接受保持、没有重复发送；业务输入前准备期限耗尽。另有 Orca CLI 本地 environments 缓存 EPERM rename。停止原 Run 后，只读同一 runtime/terminal/incarnation 的终端事实证实 Codex TUI 报 `account/read failed: workspace routing discovery timed out` 并退出；证据 `.evidence/native-remote4-show.json` / `read.json`。原 Run stop_status=stopped，原终端 operator_close；不把 CLI 缓存错误当作唯一根因。
- 用户曾选择远端待验，随后明确通知原生环境已修复并恢复验收。下一轮必须生成新 debug 包、专属 base/head，先原生准备，再重跑。第四轮冻结文件不替换，不计通过。
- Apache-2.0 为用户已选定许可证，根 LICENSE 及第三方声明已提供。当前仅草稿 PR，无正式 Release；所有未验/失败项保留。


远端第五轮：用户修复原生环境后，`C:/SteerlineDebug/srremote/dev-20261015-134657-ed44a08b` / Run `76b4e1992e576c0964ada38a398dda0d` 已确认原 agent/session/turn_started，完成实施原轮次。进入 implement.report 时原生策略资格拒绝：应用层传空 reportPolicy，Workflow 采用本机默认值，与 Orca `host_managed_permissions` 不一致。不是环境失败，也不能跳过资格门槛；harness exit 1 / 1 场景（157.86 秒）。修正应用层在处理原结果时，使用该原 worker port 的实际 PolicyCapabilities 冻结报告策略。`.evidence/report-origin-policy.log` 定向通过，覆盖原结果→下一 report→同策略资格通过、换策略拒绝，以及原资源/远端材料回归。下一轮须用新构建/新包。

2026-10-15 用户明确放弃本机 Orca 路径：本次支持/自动验收范围为本机 Codex 与远端 Orca environment；本机 Orca 不再列为待完成目标，也不得记为通过。原本跨仓 R1 的本机 terminal 场景从本次必验清单移除，其历史失败仍保留。


## 8. 最新验收范围与本机真机结果

用户已明确取消本机和远端 Orca 路径验收，本次仅本机 Codex 为真机门槛。SR-A12 记为「用户取消，不计通过」；保留所有旧失败证据，不继续启动 Orca 场景。第六轮远端包 `C:/SteerlineDebug/srremote/dev-20261015-135928-d045c509` 原 Run `66c944425084d05d2401eeea5961cf82` 在通知前已 exit 1（148 秒）且 stop_status=stopped，没有 PR；后续查询服务不可用，不将它记作本次取消请求的停止回执。

SR-A11 本机真机已通过：干净 Runner `c85f4e71e88e1e9c51d73284a12d3034e6778f4e`、EXE SHA `f58aa9eb850df89d768957b740490eafc7cb89312ab762b66d53740e39ebe8dd`，干净 SteerLine `560ce11e` 生成包 `C:/SteerlineDebug/srlocal/dev-20261015-141218-560ce11e`。`evidence/native-20261015-061410/result.json` exit 0 / 1 场景、235.62 秒。固定 Codex 0.159.2、gpt-6-luna/medium；原生 AppContainer/TEMP/项目检查/相邻写拒绝探针 exit 0。实施→报告→发布来自同一原会话与实际 final，指定两行 result.txt 提交 `5d74b109c9f6035029ac8152d360d8fb36039b1b`。

原生普通发布命令先返回 HTTP 401，原生自动审批批准同一命令后成功推送并创建 [SteerlineTest PR #253](https://github.com/erranliu/SteerlineTest/pull/253)。模型后续普通查询仍 401；Runner 自己的只读 provider 核验正确仓库、专属 base/head/提交后才进入 pr_ready，不以模型声明替代 PR 事实。PR 为 OPEN draft，未合并。原 Run `f292bda278eb655e6854b388067bdca8` 已确认 stop_status=stopped。原始公开命令、快照及清理证据为包内 `evidence/runner-native-calls.jsonl`、`runner-native-local-codex.json`、`runner-native-local-codex-cleanup.json`；没有宿主代发布或认证复制。

后续 SteerLine 本机 waiting-merge-restart 暴露更深 state-dir 的 Git `$GIT_DIR too big`，外部夹具原错误已保存；不改写历史失败。私有缓存改为 `r/<128-bit base32>`，工作区叶目录为 Run 身份的 128-bit base32；原 Run ID、branch、完整输入摘要和精确仓库/提交核验不变，命中已有工作区仍拒绝。更深管理路径的真实 Git push 必须复验，新构建须新包，不覆盖已验冻结包。


Runner dc24205 的更深路径回归 `compact-cache-deep-publish.log` 两个场景通过（真实 Git clone/ref/linked-worktree push）；`.evidence/foundation-20261015-063029/result.json` exit 0 / 2 组，实际 EXE workflow 与 publication identity 通过。对应构建 SHA `59801148b84cd7eff52416eb2afd11cf7749cf36ec3c083acc05bb93f603f762`。SteerLine [切换 PR #325](https://github.com/erranliu/SteerLine/pull/325) 已创建；用户保留的 delivery.basic、codex-resume、waiting-merge-restart 三个本机 R1 场景分别通过，后者在干净 ca5d8563 + dc24205 下 319.75 秒，清理通过。批次中已取消 Orca 场景的失败不冒充通过。

SteerLine 最新实际产品真机包另在补充验证：一包版本预检超时，零模型输入；同源新包随后已完成权限资格、沙箱、实施和报告，发布普通命令及查询 401，代理直接给出 permission blocker，未申请原生自动审核，无 PR，清理通过。对齐前述独立 L1 的已验证提示规则后，正在以新冻结包复验。这里没有原生审核拒绝，不将其写成拒绝或全链通过。


后续本机产品包 `dev-20261015-145352-9e195790` 再次在会话创建前资格版本探测的 10 秒预算超时（108.53 秒，零模型输入，无 PR，清理通过）。同 EXE 的直接版本探测 0.427 秒，使用 Runner 自己的 managed-process、相同包 TEMP/TMP 的只读探测 0.662 秒。保留两次超时事实；资格探测的独立冷启动预算统一为现有外部命令的 30 秒，不自动重试、不跳过或放宽版本/资源校验。必须新构建和新包复验，不能把快速独立探测记作业务通过。

## 9. 双仓 PR 审计与最新失败边界

本轮为当前聊天自查，不宣称独立复审。已修复 Runner 原生多题回答校验遗漏：额外题目、固定选项后的第二个答案和重复 JSON 键曾可进入原生意图；现在要求完整且精确的原题集，拒绝空回答和未提供的选项，并把合法选项 ID 规范为原生标签。`TestSRNativeAnswerRejectsAmbiguityBeforeIntent` 使用真实存储和 nil native port，证明全部六类非法回答零原生意图、原问题保留、原命令失败；合法回答不改原题。`.evidence/audit-answer-native.log` 的四个定向测试通过；`.evidence/audit-contract.log` 的公开契约检查通过。SteerLine 同时修复文本回答重复键和已绑定 PR 消失，详见其 [接入验收](https://github.com/erranliu/SteerLine/blob/codex/steerrunner-cutover/docs/acceptance/steerrunner-cutover.md)。

Runner `7bede6e` 的 `.evidence/foundation-20261015-070142/result.json` exit 0 / 1 场景，只证明实际 EXE 的权限资格反例。最新 SteerLine `bba00436` + Runner `7bede6e` 包 `C:/SteerlineDebug/slcodex/dev-20261015-150152-bba00436` 未通过真实产品链路：SteerLine 服务退出，原清理 Git 子进程报 `0xc000026b`；原候选提交状态为 unknown，不能据未绑定 Run 猜测零接受。随后通过冻结包原 helper/公开 CLI，在外部独立宿主核清原请求并停止，`evidence/original-public-cleanup-external-result.json` exit 0、`original-public-cleanup-external.log` confirmed=true。没有业务 resume 或新发布，也未修改冻结配置/二进制；工具 Job 内清理曾安全拒绝 `detach_unsupported`，未放宽 token 校验。

同版本 SteerLine `local-final` 受控补验在 TestBasicDelivery 报 Git `0xc000026b`，批次无完成汇总，不计通过。此前 §8 的真实 Codex/PR #253 和三个 R1 通过记录仍只归属各自版本。当前不宣称最新版产品真机、最终固定版本 R1 或完整跨仓交付完成；用户已要求本轮审计后分别提 PR，保持草稿，不合并、不创建 Release。

审计修复实现提交 `41908e641b34a890448158d2f21a9edc597d08cb` 已推送远端。干净 Windows/amd64 构建版本 `0.1.0-dev.1`，EXE SHA-256 `5eb9c3a9162d13449d55d363dd136e0ee50dea0d1e06ac5afaa8b439b73e5a71`，资源摘要不变。`.evidence/audit-standalone/result.json` exit 0 / 1 场景，TestSRStandalonePackage 通过（15.41 秒）：从实际 EXE 验证独立包与公开服务契约；其 unit-foundation.log 的契约/存储/启动定向检查也通过。它不是模型或最新版 SteerLine 产品真机通过。

## 10. 本仓独立三级测试（2026-10-15）

用户重新明确本次验收范围：L1 为规则/组件测试，受控集成为真实测试的前置；L2 为本机原生 Codex + SteerlineTest；L3 为持续扩展的真机真实环境集合，首个场景为远端 Orca「台式机64G」。L2/L3 均须从可执行 Spec/Plan 到实际 PR、反馈复用、合入专属 base、Runner 核验 merged、停止原 Run 与服务。此定义取代上文历史层级编号及 Orca 取消范围。

统一入口、catalog、Spec/Plan、checker/publisher、现场生成器、Go live driver 和结果判据均已归属本仓，详见 [三级测试指南](../testing.md)。独立现场使用 `C:/srt/<ID>`，不调用 SteerLine 代码、测试或配置。L3 以场景 ID 和 driver 注册表扩展，不将启动成功、零场景、skip 或残留进程视为通过。

首轮独立 L2 完整通过：`.evidence/three-level-live-20261015-r3/summary.json`，现场 `C:/srt/60ffacd6e2a6`，Run `5ecfbecf883eb925cdd0a0921b47eb02`，固定 Codex 0.159.2 / gpt-6-luna / medium。[SteerlineTest PR #254](https://github.com/erranliu/SteerlineTest/pull/254) 从原 head `c7cc1a29de5edf4e5f79b047facb2055e604aa5e` 经原会话反馈更新为 `8e73e061d99078109f6d8cd5f72d5cd8ceff1107`，合入 `codex/srtest/60ffacd6e2a6/base`，merge commit `8146628855b6e62c67a2efbf319a7db4f3dd6029`。原生 final、实际 fetch 内容、合入后 checker、Run stop、服务退出全部通过。该现场对应 Runner EXE SHA `9a215f39112e8bfc5807ed2c923df7481daaa2a339e1fa6f6ef67ecb651a815e`，来源 `af545563` + 工作区测试变更。

失败证据按原现场保留：初次 manifest 的时间解码及 Windows PowerShell 日期解析问题已修复；`C:/srt/8ee74d168d25` 实际发布暴露 PowerShell 5 空 JSON 数组计数问题，修复后新增空列表/已有 PR/冲突/查询失败反例，再完成上述 L2。`C:/srt/f45f2aefa9ba` 远端启动报账户路由 discovery timeout；`C:/srt/ecc1bec8965d` 原生 shell 报 `helper_unknown_error: setup refresh had errors`，均失败且已确认停止，没有 PR。

按用户在 Orca terminal 中裸启动 `codex` 的方式，`C:/srt/36f0b13817be` 使用远端原生默认模型复验。Codex 0.162.1 仍在 shell 工具初始化失败后请求交互权限确认。诊断过程中对四个具体、已授权的仓库读取/单文件写入检查/本地提交命令进行了单次确认，没有永久规则；原生最终记录提交 `b855397912a47d09fd4d88a881ee9c908659c35b` 与 checker exit 0。切换报告时触发 `preparation_deadline`，没有发布/PR；该次 exit 1，Run 与服务均停止。确认介入证据和终端快照保存在现场 `evidence/`，不能作为无人值守通过。

该真机失败暴露准备预算误用于后续阶段：长业务回合结束后报告准备仍沿用首次启动期限。已修复为新阶段输入获得新准备预算，原未决操作继续使用原期限。`TestSRLongTurnCanPrepareNextStage` 验证实际报告准备可在原 session 执行，原资源超时/迟到就绪反例继续通过。远端同一工作区执行官方 MXC 兼容探针返回 `MXC_OK` / exit 0，诊断终端已核实 `ptyKilled=true`。新批次使用进程级 MXC 与原生自动审核配置复验，旧失败现场不覆盖。

修复后的批次 `.evidence/three-level-live-20261015-r6`：L1 顶层测试 149 项通过，5 项明确 helper/opt-in skip，语句覆盖率 44.2%；9 项脚本 guard、live driver 编译/零匹配拒绝及 10 项受控集成均通过。该覆盖率仅是 L1 Go 语句覆盖，不包含独立 EXE 与真机执行，也不表示所有故障已做真机验证。

按包覆盖率：contracts 59.0%、Codex adapter 51.8%、Orca transport 73.1%、publication 60.5%、storage 62.8%；application 22.8%、host 12.7% 仍是单元覆盖薄弱处。当前 `go test -coverprofile` 按各包自己的测试计数，跨包调用及独立 EXE 的执行未汇总为联合覆盖；例如 resources/runstate 的 0% 不能解释为完全未被集成场景执行。后续应补关键分支测试及跨进程覆盖采集，不能把“三级入口齐备”表述为“全部代码已覆盖”。

同批次 L2 已在新构建上再次完整通过：EXE SHA `04c9f6e08c413f8febb0481b3fcff0625628a080b02b37639ce435483e5b4492`，现场 `C:/srt/44e2387b7042`，Run `d9b3fb6a0c4b69b456b200456bb86138`，[PR #255](https://github.com/erranliu/SteerlineTest/pull/255)。原 head `f5e01e400207eee5423065f1bc1018daff9216ff`，原会话反馈 head `6dad91936b0ee4e3276679733b9653e004882d75`，合入专属 base 的 merge commit `6c32ee35dbea7a32b8973cd31a90ec7acfd20a24`。`scenario.json` 和 `result.json` 确认完整验证通过、Run 停止、服务退出。

同批次远端 `C:/srt/c9d63e387b4b` 在业务输入前再次出现账户路由 discovery timeout，exit 1，原 Run 与服务停止已核实。因此 r6 混合批次整体为 failed，不能用 L2 成功覆盖 L3 失败。后续新现场使用 `--no-daemon --strict-config` 隔离原生后台路由，再验证 MXC/原生自动审核配置。

远端 L3 在新批次 `.evidence/three-level-live-20261015-r7/summary.json` 完整通过（exit 0 / 1 个真实场景），其 L1、9 项脚本 guard、10 项受控前置也全部通过。现场 `C:/srt/beceb027f358` 使用上述同 SHA 的 Runner；远端 Codex 0.162.1，原生默认 GPT-6.1-Sol/high，启动参数为 `--no-daemon --strict-config -c windows.sandbox=mxc -c approvals_reviewer=auto_review`，未修改主机全局配置。该配置消除了本次运行中的路由及 shell 初始化阻塞，不声称已修复远端默认启动配置。

Run `406c7e4fe039276d6be62ab62af172ba` 在固定远端 runtime `f8616747-34af-4543-8487-ec6b5d8de4ff` 的原会话完成实施、报告、发布及反馈。[PR #257](https://github.com/erranliu/SteerlineTest/pull/257) 原 head `d4ffa4818da4f3006dc084ad1092917ad4d9430e`，反馈 head `33d6c0f57933b8e39c8206280d24302c27b29b68`，merge commit `eb101e2f55c34f96e3c205d0ee987aef105c9f77`，base `codex/srtest/beceb027f358/base`。本轮没有手动权限确认或宿主代发布；GitHub 实际对象、原 session/attempt/branch/PR 复用、merged 事实、合入 checkout checker、Run stop、服务退出全部验证通过。

当前结果汇总：L1 149 项、脚本 guard 9 项、受控前置 10 项通过；L2 采用 r6 的 PR #255 完整通过，L3 采用 r7 的 PR #257 完整通过。两者使用相同 Runner EXE；各批次原始状态保持原样。现有真机场景各 1 个，L3 后续通过 catalog/driver 持续增加环境和流程，不将此结果扩大为未注册场景的通过。
