# Orca 多行输入屏幕草稿依赖修复 Spec

日期：2026-10-30。生产源码基线：`a086a9984dc0bd32b3f02b9698cd26c9ba313d90`。
状态：窄范围代码及后续授权修正已实施。最新冻结源的7050次稳定回归、本机3/3、远端3/3全新原生会话通过；完整L1与最终独立归档复审均通过，无未修P0–P2。见[最新保真验收](../acceptance/orca-native-submit-integrity.md)。此前[持续读取批次](../acceptance/orca-native-submit-control-stream.md)的本机0/3保持失败，不混用构建资格。原实数与边界见[实施记录](../acceptance/orca-native-submit-implementation.md)及[远端最终验收](../acceptance/orca-native-submit-remote-final.md)。配套 [Plan](orca-native-submit.plan.md)、[调查记录](../acceptance/orca-native-submit-reassessment.md)。

原存档构建的本机补验为 3/3，原请求、原文/模型/final及正式停止认证均通过；当时用户新指定的远端
`192.168.1.151` 已达 3/3，保留原 NativeProof/原输入/final及 `orca_terminal` 正式停止范围。
见[本机最终记录](../acceptance/orca-native-submit-local-final.md)。本机测试目标须具备与正式
workflow 相同的既有 writer/audit 冻结配置，不清空 Turn.Model 或放宽模型凭据校验。

远端测试适配仅补独立身份、原 operation/input/receipt 归档和既有正式 stop，未修改生产链路。
准备与业务共用现有本机同等六分钟 fixture 总期限，删除测试专属 90 秒上限；只读等待原 READY，
不增加发送或重试预算，生产时限不变。此前三份准备失败仍不计资格，也不声称冷启动 SLA 达标。

本版替代上一版整条执行/完成链路统一方案。M1本机模型控制、长final迁移退出本次范围，不再作为修复前置；这些能力并未迁移或验证等价。旧审计结论仅适用于对应旧版。2026-10-30，design_audit与final_review独立只读复审本版均无P0–P2文档发现；此结论不代表生产实施或真实验收完成。

## 1. 修改边界

只修当前远端普通原生Codex多行输入依赖屏幕草稿的问题：移除submitNativePaste分支，让它进入lifecycle.go已经存在的通用terminal send路径。单行、多行、本机、远端使用同一种Orca通用提交能力，不再按多行选择Runner手动paste/Enter算法。

**保留进入提交前的正文编译，保留提交后的原有执行与完成流程。** 本机native-command/model包装、模型/effort及clientUserMessageId传递、observeRecorded/history reader和32KiB结果能力不迁移；远端NativeProof、queryOriginalInput/新done补证也不替换。本机与远端既存的准备、认证、正文包装及完成证据来源不要求在此修复中重构为同一实现。保留这些已有流程不代表新增本机旁路、runtime或凭据。

不修改Orca，不用实验开关、私有API或安装包内部函数作生产依赖。不增加提交重试、重试预算、固定sleep、request-show生产恢复、状态机、codec字段、通用框架或兼容回退。其它agent extension、业务脚本、SteerLine/Unity和环境检测不在修改范围；不部署安装环境、不改全局设置或历史证据。

2026-10-30 用户追加授权本机补验发现的最小读取修正：`controlLines` 和
`readLocalCodexReadiness` 显式使用公开 cursor 原始流，不拼接屏幕行或回退默认读取。
2026-10-31 复审发现：不能把准备阶段的全历史完整门禁用于持续控制查询，否则正常累积
超过 1000 行或淘汰旧前缀后会永久拒收。此次修正区分读取目的：

- control READY 与本机 launch READY 仍从 cursor 0、limit 1000 读取，要求保留起点、
  `source=stream` 和明确未截断/未受限；空的完整流仍按原 pending 处理。
- 持续 original-turn/workspace-audit/authentication-stop 查询从 cursor 0 读取当前保留窗口，
  允许第一次读取报告旧前缀淘汰；若 limited，按公开 nextCursor 读取后续页，最多扫描到
  首次读取的 latestCursor，不追赶新增输出。每页验证原 runtime/terminal/incarnation、
  来源与 cursor 元数据；中途丢页、cursor 不前进/倒退、身份变化或取消均失败并保留原因。
  仍由原解码、请求 ID/binding/request digest 和各类正式证据决定是否采纳响应。

分页只在这一次只读查询内使用局部变量，不新增持久 cursor、状态机、重试预算或发送。
保留原 checkpoint、未知处理、提交/模型/完成/停止算法；此前真实会话仅证明其存档构建，
不能自动成为本次读取增量的真实资格，也不将此前失败改判为通过。

## 2. 当前事实与提交契约

基线lifecycle.go在Environment非空、Terminal为空、agent=codex、WireFormat为空且正文含LF时调用submitNativePaste。该路径做text-only手动bracketed paste、屏幕草稿比较/双稳定/可能resize，最后单独Enter；其余路径已经调用通用text+enter。普通本机Codex不进入这一草稿分支。

Orca CLI只有同次text+enter且非interrupt才启用agentPrompt及durable请求绑定。修复后复用既有调用：

```text
terminal send --terminal <原终端> --text <既有编译后的正文>
              --enter --retry-request <原请求ID> --json
```

保留wirePrompt/nativePromptText与输入预算的先后关系。普通Codex正文继续CRLF→LF、去尾CR/LF，不改变内部空白和空行；本机既有wire不改成普通正文。移除的是Runner附加的ESC bracketed框架，不能把框架再交给Orca净化后包装。原草稿分支NUL/ESC拒绝检查保留在该受影响输入的发送前，不顺手改变其它agent的输入范围。

既有通用调用的等待规则保持：NativeRemoteProfileVersion使用现有--wait-submit 20和50秒CLI上限，其它既有路径保持原参数，调用方更短期限优先。不统一等待策略、不增加预算；受影响多行改走此入口后会继承远端既有20秒开始观察，区别于原40秒草稿流程。该观察不重发原输入，不保证一定取得turn_started。CLI整次调用持共享锁，新增多行使用它可能延迟其它Run的观察/停止，甚至耗尽其原期限；必须通过Plan的多Run锁与取消回归。排队和执行共用原deadline，取锁不刷新期限；排队取消的CLIQueueError表示回调未执行，不保证stop抢占或所有排队调用成功。若暴露死锁、取消后派发、期限刷新、错误成功或违反既有停止约束，修订提交范围再审计，不临时造锁、轮询器或分支回退。

提交回执仅做必要的原生Codex校验：保留runtime/terminal handle/requestId/processIncarnation绑定、CallUncertain及错误检查；确认provider=codex、observation=supported及阶段一致后才采纳受理。Codex 的 turn_started 阶段仅保留为原回执提示，不能直接发布原业务 Started；原输入和原 turn 的正式查询才建立开始事实。复用现有解析和Turn事实，不建立第二个回执层，也不改变其它agent契约。unsupported、old-host、缺字段、incarnation_replaced或未知值不升级为成功；permission沿原阻断语义。明确拒绝与未知保留区别，未证明调用前无副作用时不能报成“未发送”。

能力预检不能排除宿主未识别前台agent时内部raw发送。因此Runner不主动选择raw或fallback，并不保证Orca内部不存在raw或held-Enter恢复；异常回执可能已有副作用，禁止追加正文、补Enter或换ID重试。已发布通用入口是提交顺序的承担者；当前没有独立的原子消费全文/任意人工写者排他证明，不能把事后原输入核验冒充执行前保证。

## 3. 五种事实及权威来源

| 事实 | 权威来源与能证明什么 | 不能证明什么 |
| --- | --- | --- |
| 传输写入成功 | 原传输/调用回执，按实际settlement解释 | bytesWritten不证明TUI完整消费、开始或完成 |
| 完整输入提交 | Runner向Orca通用入口一次交付既有编译后的完整正文及Enter，由Orca负责framing、处理与提交顺序 | 无新增committed或全文消费摘要；不能承诺原子消费或所有人工写者排他 |
| 原请求accepted | 原runtime/terminal/incarnation/request绑定、可验证provider/observation的send.prompt受理回执 | 不等于开始、完成或完整正文证明 |
| 原任务started | 原有history中的唯一匹配原输入及turn，或远端独立唯一原输入与新done补证 | accepted、Orca turn_started、普通working、idle、屏幕变化不能单独认领原任务 |
| 完成及结果归属 | 本机原recordedTurn/validateRecordedTurn；远端原NativeProof/正式原输入查询、主代理新done与完整final；保留各自唯一性、身份和冲突核验及workflow验证 | mutation completed不是模型完成，模型完成也不是业务交付成功 |

继续先冻结Turn.Prompt/InputDigest、wire与原operation intent，再执行唯一提交。保留ProcessIdentity、session、runtime、terminal、incarnation、pane/provider、command/request、authority/revision和回合基线；accepted、started、settled各自判定，不降低工具/旧结果排除和完整性围栏。

保留既有准备、权限、原资源检查、Runner串行化和已知并发/外来输入冲突阻断。删除草稿前置空检查与全文比较后，不能再宣称能通过屏幕排除已有草稿；idle也不能证明草稿为空。已知有他人输入或身份/会话冲突则阻断，不清空或合并。任意人工输入竞态/未知草稿能否在执行前完全排除仍未获证，真实验证须记录边界；本次不以修改Orca或恢复屏幕全文比较补造保证。

## 4. 未知回执沿原流程处理

发送超时、回执丢失或不可验证时，沿现有错误、operation journal、Reconcile和observer保留原身份、原因与未知事实，不新增查询/重试入口。不再次调用send，不补Enter，不换ID、不建替代资源或重派历史Run。不能因同ID曾有去重就重放send；mutation记录可能被清理。

不新增orchestration request-show接线、计时器或恢复循环。该接口已核实存在，仍保留为调查事实，不列为本次生产依赖或正常完成门禁。丢失accepted时，现有observer可能因缺受理证明而无法认领完成；本次不承诺恢复这种缺失回执，按原期限/错误处理明确保留未知，不能伪造accepted或改称确定未执行。

正常accepted后使用原有观察。首次只有accepted不能提前started/completed；远端既有queryCompletedOriginalStart仅在原身份、唯一正式原输入及新done充分时补证，本机沿原history读取。保持working依赖Started的既有规则，不新建开始轮询器。已有强事实/完整final/持久冲突不得因弱回执或后续观察缺口被清除。

## 4.1 2026-10-31 本机正文保真与凭据修正

用户授权修复只读调查发现的问题并重新验证。原三场失败保持失败：CLI 正文均 5093 字节，精确原 session 的 Codex 提交历史只剩 5079/5077/5077 字节，仅丢失 14/16/16 个 LF；原回执均有 turn_started、initial_start_confirmed=true，但正式历史没有对应业务 turn。不能据此唯一归因到 Orca、ConPTY 或 Codex 的具体函数，当时也不能宣称 LF→CR 已验证。旧16MiB桥接仍断连时，单独CR候选也丢失8个LF；只修128MiB并保留LF的后续本机三场为2通过/1缺2个LF。因此限额断线不能解释全部正文损坏，不将任一单独修正冒称完整修复。

最小增量只有四处：

- 模型桥对 rewrite 拒绝返回原 JSON-RPC id 的结构化错误，不把该请求转发给原 server，不断连伪装成传输未知，不重试。无 request id 的非法消息关闭连接，不能伪造请求身份。保留认证、单活动连接、原线程、正文摘要、冻结模型与 clientUserMessageId 规则；错误不包含正文或凭据。
- 原生 Codex 不再把 send 回执阶段直接提升为业务 Started；本机继续 observeRecorded，远端继续 queryCompletedOriginalStart。未得到既有正式证明时保持未证实，即使运行中只能等到 done；不新增开始查询、字段、状态机或回退。其他 agent/terminal extension 语义不变。
- 模型桥上游 WebSocket 消息上限从 16 MiB 对齐到原生 Codex remote client 已有的 128 MiB 有界上限；下游 2 MiB 输入边界保持。诊断原始流直接捕获 16777217 字节处 read-limit 错误，随后下游取消和重连；大元数据请求先于业务发送，首断发生在正文处理中。不是 Orca 提交 API 的缺口证据。冻结逻辑正文、wire、摘要和预算保持原规则，不增加启动等待、配置开关或重连逻辑。

- 所有原生Codex在同一通用send前将逻辑LF编译为终端粘贴CR；位于既有控制字符拒绝之后，两端/单多行/既有wire格式共用同一规则。Codex既有paste归一化恢复LF；不改逻辑wire/hash/预算或模型桥正文检查，不增加LF回退。分别记录实际CLI参数摘要与原逻辑wire摘要，正式history/query必须逐字等于逻辑wire，不能对损坏正文放宽归一化。

消息上限与CR传输编译的组合已完成大回复/同连接原提交确定性测试、原模型桥摘要核验和同一冻结源的双端各三个全新独立原生会话。24根235叶各30次=7050/0/0，双端各3/3；完整L1与最终独立归档复审均通过，按验收报告边界放行，逐场身份/原字节见最新保真验收；失败不重投、不补 Enter，保留失败记录，不保留先前LF实现的兼容路径、诊断探针或实验模式。不把成功归因扩写为已唯一定位下层缺陷或证明任意人工输入排他。原输入/模型/final/正式 stop 证明不变；不部署安装或修改 Orca/全局配置。

## 5. 删除与保留清单

最后L1另暴露既有RPC单元夹具listener.Close挂死，原12分钟超时保持失败。仅测试文件沿用仓库已有stop/wake停Accept顺序，失败仍报错并关闭活动连接/等待worker；无依赖升级或生产RPC改动。修后IPC990/0/0、controlled7050/0/0使用b3a642…5c91新测试源，L1通过721顶层/2730含子例、0失败，17明确跳过另列。六场c8资格保留，逐文件证明唯一差异为未参与live/agent的单元夹具，agent重建字节相同；不声称所有检查全源相同。

| 对象 | 本次处置 |
| --- | --- |
| submitNativePaste及native_paste.go | 删除手动paste/Enter、草稿全文/标签匹配、显示LF推断、双稳定、250ms轮询、40秒专用期限；迁出受影响输入的控制字符检查 |
| fitComposer/composerViewport | 删除Lifecycle字段、初始化和调用；删除仅为正文匹配存在的FitBoundComposer/ComposerViewport、ViewportRPC/Recorder、fit/restore回执与terminal_viewport.go专用实现/测试 |
| 专用测试、fixture、probe、脚本和文档 | 删除native_paste*、native_reflow及草稿/resize专用断言；live probe取消手动分段paste与segmented-shape门禁，改验实际通用提交；迁移内容、身份、未知/并发反例，保留正式原输入/final/清理断言 |
| wire与模型 | 保留prepareInput、freezeWireInput、nativeWireText、wirePrompt、native-command/model、remote_models及实际模型核验，不改正文身份与预算 |
| 原输入/完成/停止 | 保留NewLifecycle现有查询接线、native_start/native_state、observeRecorded/helper reader、reply上限、准备/认证/资源所有权、stop与清理，不重构或统一来源 |
| 状态与旧记录 | 复用Turn/operation receipt/现有codec，不新增字段或版本，不自动迁移旧未知为已受理；历史evidence保留原字节与失败结论 |

全仓按引用确认屏幕专用项无残留；共享WaitTimeout、通用terminal read能力、其它agent/extension测试不因同名误删。本次“旧路径清理完整”只指草稿提交路径，不包括本来就存在的模型和完成流程。只保留一条Runner通用提交算法，不新增旧host回退或实验模式。

## 6. 验收与交付

Plan的NS-T01–T10映射既有正式入口、条件与证据；实现时登记实际测试名/case ID。核心确定性故障各目标每叶例30次，报告执行/失败/跳过；夹具控制延迟、分段、顺序与故障点，按请求身份、实际调用及字节/摘要核验，不靠固定sleep或重放同一快照充数。

实施后完成项目规定检查、正式L3/稳定回归与独立复审。真实验证在全新隔离现场，以无副作用最小任务，本机/远端各至少3个独立native session，记录原请求、实际Runner通用提交次数、accepted/started/completed证据、正式原输入/final与精确停止清理。一次RPC不等于一次底层按键，宿主内部不可观察次数记不可观察；缺条件标未验证。

模型与长结果仅做既有能力不退化的回归，不要求先迁移、重新证明两端等价或用六个短任务替代这些既有测试。不跑Unity、业务完整交付或创建业务PR。原窄范围提交修正已有双端各三个真实会话及一次远端重跑记录；本次持续控制读取增量的检查、复审与未验边界另行登记，不宣称完整复现或修复历史原生故障。更新原单独 PR，不合并或部署。
