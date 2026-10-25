# 执行可见性验收

## 2026-10-25 缺失 working 通知的薄结束补证（验证中）

Task18 原配置第二轮 Run `27761aaae314c4bbcd00a7d7fae688d1` 已产生 Gitea PR419 / head `e51362e701f788295b3ec41ebf20258b87828ecd`，Runner 仍为 `publish.execute / observation_unverified`，不能计成功。独立只读现有协议探针核实原 terminal/incarnation/worktree/pane/provider、唯一原发布输入及不变 done/273 字符完整正文；探针仅用公开上一 final 观察时间作为诊断估计 baseline，不冒充该 Run 的持久证明。

冻结源码 `5df57b6` 的复现、原始探针和失败日志：`C:/SteerlineDebug/wt-507e-task18-retest/diagnostics/attribution-20261025T0943/`。扩展原薄投影 IPC 回归后，原代码中 `missed/original` 与 `missed-restored/original` 失败，已观察开始及重复输入拒绝保持通过；最小修正后六个变体通过。缺失开始仍只允许独立、精确进程绑定的唯一输入 query，不接受普通未绑定历史，不补造开始。owner/L1/L2 与新构建真机复验尚待登记；此项不宣称两轮真机成功，不代表 Task18 Unity/业务验收。

### 2026-10-25 前置门禁核查

独立固定 Runner 为干净 `f3dd4ec020134e1eb56625ddd670641a33229151`，EXE SHA256 `4a822c99fec734f296890b8d71a4a4013dfef46e3ab4c1395573130025a4e16e`。`l2-final-03/summary.json` 中干净源码 L1 的 673 项顶层、29 项扩展及编译/零匹配守卫通过。L2 仍未通过，Task18 连续成功数为零，第三轮尚未启动。

前次 `l2-final-02` 因 WMI 调用令牌的全部权限启用而跳过 broker 权限用例，严格门禁正确拒绝。新独立 launcher 仅将自己的四项权限降回普通调用方原本的禁用状态，原 broker 用例实际通过；没有放宽测试、启用额外权限或修改防护。

`l2-final-03` 的 Controlled 随后发现原 `TestSRContentGenerationRecovery/generate-graceful` 比较了整个 history 响应，其中包含实时 Run revision/updated_at。旧 Runner `5df57b6` 对同一原用例三次均失败；独立日志对照旧版和 f3 均证明只有 revision/updated_at 变化，会话、回合、原进程、控制资格、正文、命令归属和分页全部相同。验收测试因此只排除这两个可变元数据，继续精确比较其余整个响应，并保留原零重发、停止和身份断言；不改变产品代码或持久状态。原失败、两版对照及补测分别保留在上述诊断根的 `generation-baseline-01`、`generation-probe-{baseline,fixed}-01`、`generation-corrected-{baseline,fixed}-01`，后续正式 L2 必须重新执行，不能把原失败报告改记成功。

同一完整 Controlled 最终执行 24 场景，另有 `TestSRResourceCreationRecovery/owner-crash` 和 `missing-initial-plan` 失败。补测启动器据此拒绝运行 `l2-final-04`，该次只有启动检查失败，未执行原生 L2。两项原测试在旧 `5df57b6` 与 f3 均复现：原创建 operation 已是 `outcome_unknown`，停止已撤销授权，`phase=stop_requested/stop_status=pending` 保持权威，测试却仅等待 coarse status=reconciling。该停止优先级是等待契约的既有要求，不能为了通过测试改写产品状态。修正断言要求同一原 `prepare_workspace` operation ID 仍为 unknown、ResumeRequired=true、禁止新输入，同时接受原 reconciling 或明确的 pending stop 投影；延迟后的复核仍用完整相同谓词，原零重新创建/发送/认领/关闭和不能伪造 stopped 的断言保留。

两处原失败现场通过公开 service status 确认为 not_running；公开 verify-stopped 正确保留 resources_settled=false，原因是故意丢失的模拟创建身份，不冒充已结算。原目录继续保留，未读取或修改 Runner 私有数据库。旧/新原用例与修正后全部七变体补测分别位于同一诊断根的 `creation-original-{baseline,fixed}-01`、`creation-corrected-{baseline,fixed}-01`；公开收尾查询位于 `creation-original-inspection-01`。

`l2-final-05` 的干净 ebcda55 harness 完成 L1（673 顶层、29 扩展及全部守卫）和完整 24 项 Controlled，均通过。两个真实本机 L2 都完成原执行、结果采集、发布、provider 核验、测试 PR 合入和原 Run/服务停止，但末端 R2 验证器因共享 `.steerrunner/artifacts/` 已存在而失败；05 仍是 failed，不能改记成功。真实项目规则要求报告写入该忽略目录，工作器此前按规则产生了自己的报告；验证器不应要求共享根不存在，也不应将忽略文件当作 Git dirt。

仅修正 native 测试：在共享根下独占创建 `native-r2-<fixture-id>`，不覆盖或删除已有报告；使用 O_EXCL 创建、只含实际 checker 输出的独立临时文件验证 Git dirty/clean，采集路径仍是自有子目录下的真实报告。保留字节相等、Run/PR 不变、源删除后幂等重放与停止清理断言。两处已停止的原 fixture（`8f14a70183cb`、`302b091968cc`）经独立诊断实际通过全部 R2 断言及 offline resources_settled=true，零 submit/reply/resume、零新模型/PR；诊断不替代正式 L2 新现场。证据为上述诊断根 `r2-corrected-<fixture-id>-01`，原失败报告仍位于 `C:/srt/<fixture-id>/evidence`。最终正式补测待完成；Task18 仍为零次连续成功。

范围及命令见 [Plan](../spec/execution-observability.plan.md)。2026-10-15，Windows / Go 1.26.4；独立临时目录和受控原生协议，无真实模型、GitHub PR 或安装环境变更。

| 项 | 验收要求 | 结果 |
| --- | --- | --- |
| ROV-A01 | 真实流程/状态/诊断变化原子入库；错误回滚，历史不可重画 | 通过；最终定向测试及提交前复验通过 |
| ROV-A02 | 仅公开摘要，身份与乱序守卫、脱敏有界；查询时间不冒充活动时间，零业务状态写入；观察问答/终态继续 | 通过；最终定向测试及提交前复验通过 |
| ROV-A03 | 真实 EXE/服务 CLI 可读摘要与变化；零额外 native input；原反馈流程保持 | 通过；TestSRRunWorkflow 实际 EXE/常驻服务受控验证 |

## 最终自动结果

2026-10-15 完成 Plan 定向命令：storage/application/Codex adapter/CLI 合计 19 个测试及子用例 pass，0 fail/skip；host 包完成编译。随后自查补充引号凭据脱敏和测试错误核验，再复验 LastMessage/WorkflowTransitions 两项通过。Go 输出位于 `.evidence/observability/go-final.jsonl` 与 `final-review.jsonl`。

真实 EXE/服务受控 `TestSRRunWorkflow` 通过，场景耗时 20.04 秒（含构建总计 29.428 秒），结果 scenario_count=1、exit_code=0。覆盖公开 CLI 两次读取同一 observed_at、Run/Attempt/Session 正确、读取后 revision 与 native turn/start 次数不变、CLI workflow_changed 真实历史，以及后续反馈仍使用原 session/PR。证据为 `.evidence/observability/integration/integration.log`、`result.json`、`caller-commands.jsonl`；均为隔离夹具，不含用户认证或真实任务数据。

`go build -trimpath -o .evidence/observability/steerrunner.exe ./cmd/steerrunner` 和 `git diff --check` 通过。原始产物忽略入库，本文保留复现命令与最小摘要。


限制：摘要是进程内最佳努力缓存，服务重启后为空，只在收到新通知后更新；不补抓所有历史，不包含 reasoning/工具全文。Orca 未实现该摘要且本轮不验收。历史 event 缺少 transition 时保持原始事实，不能用当前投影补画。


## Orca 最后消息实施与自审（2026-10-23）

按 [Spec](../spec/orca-last-message.spec.md) / [Plan](../spec/orca-last-message.plan.md) 实施远端原生 Codex 的直接观察接入。用户明确要求先不部署生产；原生产 EXE、state-dir、Run/Attempt、原输入和持久冲突均未改动。以下记录为 PR 整合前的冻结构建验收；后续用户授权提交 PR 的主线整合见文末。

复用原 StateFeed，在源收到时固定样本种类和 UTC 时间；adapter 仅保留每原 command 一个有界瞬时候选，经过原资源前后核验后单独交给原 observer。消息投递成功才提交内存去重水位；业务 started、Implementation、completed 仍分别确认。application 的原摘要缓存、身份围栏、脱敏、UTF-8 限长及只读 CLI 保持。原观察诊断改为同一事务中的当前值比较，A→解除→A 不再被永久事件键吞掉；消息读取仍不推进 revision。

| 实现审计发现 | 已收敛的修复与验证 |
| --- | --- |
| 原生 tab 通知合并，不能以不断线证明边界完整 | 空 prompt 还要求已观察原始 working 起点和相同正值 turnStartedAt；标记变化、缺口、query-only、重连、重启均不能建立该完成证明 |
| Reconcile 或同步首读先取得消息但没有投递渠道 | 私有观察结果移交正式 observer；未确认候选不消耗摘要水位，重附着 initial 不吞掉仍有效候选 |
| 消息和事实混发，或 observer Done 早于资源清理 | 独立消息事件；清理原 feed/context 后才发布 Done，取消可打断消息确认等待；所有权检查防止旧观察清理新上下文 |
| 已证结束正文暂缺，或新查询混用不同主记录 | 有界查询仅能为已证的同一结束、同一标记补正文；独立主记录和 session 元数据的 state/stateStartedAt 必须一致，不能据元数据替换原正文 |
| 更新时间心跳重复触发 done 查询 | hint 只使用 pane/state/stateStartedAt；缺证据沿原 30 秒重试预算读取 |
| 临时问题在无新样本的轮询或重附着时被误清 | 瞬时诊断保留到补证；initial 缺归属仍返回未核实，匹配原输入补证后解除；旧持久冲突不清除 |
| 诊断被 SeenNative 永久消费，或覆盖 stop/问答 | 原 Session/Turn 围栏和独立当前值投影；只有真实可见变化提交，优先级遮挡/重复更新不写 revision/SeenNative；merge 保留本模块拥有的 phase |
| 正式 L1 入口遗漏已有用量 opt-in 测试的默认 skip | 逐名补齐八项既有入口，保留全部 opt-in 环境拦截和其它 skip 失败门槛；首次失败报告保留，不将默认 skip 计作真机通过 |

本次为主会话自审，没有使用独立审计 agent。未新增消息表、持久诊断计数器、后台轮询器、旁路队列、helper/reader 终端或模拟生产参数；profile/state/CLI 版本及业务结果准入、授权和停止范围不变。旧泛化 provenance 冲突与 outcome 保留，原故障 Run 不承诺自动恢复或回填消息。

本轮证据唯一保存于 [orca-last-message-20261023](evidence/orca-last-message-20261023/)，远端业务资格引用[原生验收记录](orca-remote-native.md)。完整 internal 回归 28 包、2052 个测试及子例通过；审计修复后 adapter/protocol 全包、定向 application/CLI/Codex、独立查询记录一致性及 go vet 通过。真实 EXE 三场景早期受控验证通过；最终构建正式 L1 已通过：598 个顶层 Go 测试、29 个终端扩展测试，默认 opt-in skip 逐名审查，driver 编译及零匹配守卫通过；完整 Controlled 22/22 通过，退出0、零 skip；公开远端四个 case 的最终资格均已取得，首次 smoke 失败与后续串行复验分别保留。完整受控的实际 EXE SHA 与最终构建一致。L1 后补验 `TestLastMessageSameTextFromNewTurnRetainsOriginalIdentities` 及原 LastMessage/诊断测试通过，覆盖跨轮同文、旧摘要真实 Turn、同轮时间去重和 ResourceGeneration 不能代替 Session.Generation。该增量仅为测试，未改变最终 EXE。

原生定向测试：普通通知两个回合 68.98s 通过，query-only 两个回合 109.22s 通过，原终端关闭已确认。普通观察产生两条独立公开消息，未混入完成事实；本次字段在主 agent done 时发布。两模式的结束 prompt 均保留，因而只证明匹配 prompt 路径；不将它们记作空 prompt 真机正向/反向或主 agent working 期间字段发布的资格。空 prompt 窄化、边界撤销、旧冲突、重附着、正文补读与诊断事务均已由受控正反例验证。真实来源未触发的字段形状属于验证限制，不通过新增模型提示或测试开关伪造。


### 最终构建与远端结果

最终构建 0.2.0-dev.3-r2，SHA256 `61C8B89110DAB644D03B5AE74A77304CF98FCA0859709D92CA9AFBB7D9B38DE4`，resource digest `84d57119b0163f88c84519b921fe97db2bd194f3d9a06ffad7bf27c4287fa3d8`。所有最终公开链路现场与完整 Controlled 的主 EXE 相同；生产未部署。L1 598 顶层测试（2065 测试及子例）、29 个终端扩展测试通过，完整 Controlled 22/22 通过；最终静态检查、文档链接和差异检查通过。L1 后新增的跨轮同文专项单独通过，未改变运行构建。

| 原 catalog case / 最新现场 | 实际结果 | 原身份与收尾 |
| --- | --- | --- |
| smoke/default，`C:/srt/08cb8fae5e48` | 200.81s，通过；测试 PR [#650](https://github.com/erranliu/SteerlineTest/pull/650) | Run `a2ef579420410d776f2ce00b5a0b7c29`；消息、原生 unreported/verified、业务 pr_ready、精确测试 base 合入、Run/服务 stopped、offline resources_settled=true |
| connection-reconnect/active-disconnect，`C:/srt/b2b0990dc19e` | 548.42s，通过；测试 PR [#646](https://github.com/erranliu/SteerlineTest/pull/646) | 原服务/Attempt/资源保持；直接状态与 CLI 双通道确实被隔离，观察重试，零原输入重发、无 resume；原生资源、服务及网络设施全部收尾 |
| connection-reconnect/pre-send-disconnect，`C:/srt/d3e7db32713f` | 451.98s，通过；测试 PR [#647](https://github.com/erranliu/SteerlineTest/pull/647) | 原发送前 gate、连接重试与输入授权保持；原资源、服务及网络设施全部收尾 |
| multirun/default，`C:/srt/bbc3990ab2a0` | 197.17s，通过；测试 PR [#648](https://github.com/erranliu/SteerlineTest/pull/648) / [#649](https://github.com/erranliu/SteerlineTest/pull/649) | 两 Run `5dfd231961d10dababe5f59b43fbd50a` / `4590563a1600576b5b1c25191bb8a47c` 的消息、会话、工作区、分支和 PR 隔离；两 Run/服务停止；两个 PR 在完成隔离验证后关闭，未合并 |

首批正式报告真实结果为 **3/4，通过率75%，batch=failed**，没有改写为全绿。首次 smoke `C:/srt/0588bad84334` 的消息、可信业务结果、测试 PR [#645](https://github.com/erranliu/SteerlineTest/pull/645) 合入通过，但原 terminal close 无回执，后续原清单仍见活动终端，停止保持 unknown，故整项失败。不能仅据现有证据确定原因，未修改产品停止门槛或增加自动重发。随后先核对同一 runtime/handle/incarnation，再明确关闭该测试原终端；重启该已撤权测试服务仅对账原停止意图，确认 stopped，关闭服务并由离线核验得到 resources_settled=true / scope=orca_terminal。其失败快照与后续收尾证明分开保存。

串行重验沿原 `new-test-fixture.ps1` + `test-native.ps1`，前置断言本轮正式 L1/22 项 Controlled 已通过且首次资源已结算，使用同一冻结配置、同一 EXE 和全新 fixture；没有替换原失败结果，也没有重跑模型输入到原 Run。最终四 case 的资格索引为 [qualification.json](evidence/orca-last-message-20261023/qualification.json)，首次报告为 [initial-native-batch-summary.json](evidence/orca-last-message-20261023/initial-native-batch-summary.json)。

公开 CLI 在前序 inspect 为 running 的观察期间和 pr_ready 时均实际返回消息，重复读取时间稳定；实际中文缓存无替换字符。直接来源事件与业务完成事件分开已由原生定向测试验证；CLI 响应本身不暴露采样路径，不能据它区分直接采样或 final 补采样。OLM-02/03/04/05/07/08/09 的受控及对应真机范围通过。**OLM-01 的原生 working 期间新字段发布、OLM-06 的真实空结束 prompt 正反向仍未触发，保留未验证**；实现边界由确定性正反例通过，不为此增加模型提示、模拟生产参数或上游改动。

本次管理员代理已按同一 host_id 正常结束，两个网络故障现场设施 restored；测试 PR #645/#646/#647/#650 仅合入各自 codex/srtest 专属 base，#648/#649 已关闭。短 state-dir、业务测试工作区和证据文件保留供复核；这不扩大原生停止的 orca_terminal 范围。归档配置是历史冻结输入，其中 admin_host 已结束，未来故障验证须提供新的在运行测试宿主。该冻结阶段尚未提交产品变更；未部署生产，原生产 Run 的冲突和 outcome 未改动。

### PR 主线整合（2026-10-23）

用户随后授权提交 PR。主线已合入 [#49](https://github.com/erranliu/SteerRunner/pull/49)，本分支整合 `1cfe2f9`：复用其 state v7、唯一 v6 解码兼容边界和 `ObservationStatus` 原子事件，删除 InputWait 并行诊断投影、重复的 envelope 状态投递及 applyObservation 重复 phase 保存逻辑。诊断变化通过原 Session/Turn/event 事务提交，PublicMessage 仍单独确认；同一事件幂等、冲突拒绝、失败可重放。

本机查询沿主线的已核实输入绑定规则；远端空预览结束继续要求本专项的连续可信通知和相同正值 turnStartedAt。已有本机恢复测试及新增远端 query-only、重连、重启、标记变化、补正文反例共同约束该差异。消息表、额外查询器、迁移分支或远端 helper 均未增加。

上面的四项真机资格只对应归档的 `61C8B891…B38DE4` 构建，不能移记为主线整合后构建的真机验收。原 `source-index.json` 是该冻结阶段的历史清单；整合后的检查与源码身份另行记录，生产环境保持未部署。

整合自审补正：工具输出属于可恢复缺证，但不能掩盖同时出现的非空外来输入预览；归属分类先检查明确输入冲突，并增加组合反例。冻结运行源码为 `b008f4d990bfc279356335a2e0d2e75538f26a83`。完整低并发复跑使用 `GOFLAGS=-p=2`，L1 为 **609 项顶层 / 2111 项含子测试 / 33 个包通过**，无失败；16 项既定 opt-in skip 不算现场资格。29 项终端扩展、native driver 编译与零匹配保护、`go vet -tags=live` 通过。

首轮整合 L1 真实结果保留：608 项顶层通过、1 项失败；既有 `TestOwnedJobReconnectAndStopIncludesStartedDescendant` 在 10 秒内未看到子进程启动标记，不能据此确定原因。该包独立复跑与后续完整复跑通过，未修改测试、跳过失败或放宽门槛。另一次早期 ad hoc 全 application 命令选取的 6 分钟总时限不足，正式 12 分钟门槛下全包已通过；两者分开记录。

整合后 **Controlled 22/22 通过**，正式报告整体 passed。实际受测 EXE SHA256 为 `3C1F95319EA4C8029540789FFD974A1F59123BD66645E391F9CF7433752AA338`，版本 0.2.0-dev.3-r2；运行源码保持上述 `b008f4d`，测试期间新增内容仅为文档和证据。L1 起点 clean，后续构建的 dirty 标记来自这些新增材料，运行目录树和 28 个变更源码文件哈希另行核验。整合前 36 个源码文件的历史索引也与提交 `e5aeddc` 全部吻合，原始 87 份证据字节校验通过。

最终结果与失败记录统一见 [PR 整合资格索引](evidence/orca-last-message-20261023/pr-integration/qualification.json)，对应 [PR #50](https://github.com/erranliu/SteerRunner/pull/50)。当前构建未重新执行上述远端真机四项资格；不把前一构建的真机结果改写为本构建结果。未部署生产。

## 断线后空输入预览恢复（2026-10-23）

调查 Run `a325256f7747f9e9a6d1e93b3d970c32` / Attempt `757625f7548a705531ebfa6810dabadd`：原远端终端已为 done，最后消息 `READY`，prompt 为空；Runner 因断线丢失通知连续性，保持 `observation_unverified`。终端同时存在 `bd` 缺失的 Hook 错误，不能据此认定实施任务已完成。本次源码基线 `54019142685b3a1d6d54e818c954acc0dfa9f4fc`，未修改生产 EXE、数据库、原输入或原终端。

补充只读核验（独立原生查询及 SQLite `mode=ro`）：原 proof 的 `WorkingAt/RootWorkingAt=1792742346088`，当前元数据保留的 `turnStartedAt=1792743032957`，已是不同边界。provider 相同、无 session boundary/restored 标记也不能消除这一差异。因此本修复不承诺该原 Run 自动结算；标记变化的反例须继续拒收 `READY` 为原任务 final，不能以缺 prompt 为由忽略更强的边界证据。

修复在既有 `NativeProof.query` 中使用独立查询的原起点标记核验：必须有原持久可信 working 起点，且交叉核验后的 `turnStartedAt == RootWorkingAt`；原身份、时间、冲突、outcome 和正文门槛继续执行。initial/重连通知不获得连续性资格。协议层清空未经交叉核验的 marker，防止不同主记录或冲突标记被组合使用。无新增持久字段、codec、业务输入或查询循环，业务结果继续由原 workflow 判断。

回归先在未修复实现运行 `TestOrcaCompletionQueryRecoversEmptyPromptAfterDisconnect`，reconnect/restored/missing-body 三个子例均复现原 `Native original input preview is missing`。修复后，两受影响包的完整测试通过（orcatransport 46.785 秒，protocol/orca 6.666 秒），对应 `go vet` 通过。新增观察器测试核验诊断解除、原 command 的一次完成事件及持久 proof；反例覆盖缺接受/开始证明、缺失/变化标记、替换身份、历史冲突、中断/失败、过期记录和截断正文，原业务输入总数固定为 1。

正式 L1 使用 `GOFLAGS=-p=2`、`./scripts/test.ps1 -Level L1 -OutDir .evidence/orca-empty-prompt-reconnect-l1`：**33 包、614 项顶层 / 2171 项含子测试通过，0 失败**；16 项既定 opt-in skip 不计入通过。29 项终端扩展、native driver 编译及零匹配守卫通过，正式报告整体 passed。[报告](evidence/orca-last-message-20261023/reconnect-empty-prompt/summary.json)、[逐项汇总](evidence/orca-last-message-20261023/reconnect-empty-prompt/validation.json)、[原始 Go 事件压缩归档](evidence/orca-last-message-20261023/reconnect-empty-prompt/l1.jsonl.gz)和[源码/构建摘要](evidence/orca-last-message-20261023/reconnect-empty-prompt/source-index.json)保存于同一新增子目录。修复版 Windows/amd64 EXE 已构建并通过静态 `version --json` 检查，SHA256 为 `7C519C461F1EED43BAABFE67460245F6F93EFB6A3FB27CEA9C114DBC89DE3F19`。本轮是受控协议/adapter 和共享回归，不创建远端模型任务、PR 或新的真机资格；安装版和现场原 Run 未部署/恢复。

PR 前同步主线 `7634ee1`：保留主线手写 README，恢复说明随主线结构放入 CLI 指南，测试清单保留两项增量。六个运行/测试源码哈希仍与上述正式 L1 受测源码一致。重放两受影响包全包（56.061 / 9.713 秒）、application 观察诊断与 runview 导出定向检查（5.846 / 0.926 秒）、host 编译及对应 go vet 均通过；未将无匹配 host 用例计为测试通过。新主线检查单独登记在 [PR 同步证据](evidence/orca-last-message-20261023/reconnect-empty-prompt/pr-rebase-7634ee1/result.json)，不改写原 L1 构建记录。压缩、Hook 失败、回合标记变化与 READY 的完整因果调查及新增远端真机验证尚未完成，不属于本 PR 已完成资格；未部署生产。

## SessionStart 清空输入预览的 Runner 修复（2026-10-23）

调查用户指定 Run `65f8102fe2825ca1f49b1b3a8026313b` / Attempt `7f1136f6c0e42ae7dbf0dab771877f56`。原主 agent 仍为 working，持续修改 DragonDice；北京时间 18:26:07 起 Runner 因缺预览进入 observation_unverified。安装版 Orca 1.4.224 的相关实现与公开仓库 `stablyai/orca` 的 `d9dc87b2fb08ba52930378941ff470b69d23523d` 对照：Codex SessionStart 清空 cached prompt，输入边界分类同时含 SessionStart/UserPromptSubmit；主 agent 同为 working 时其状态起点不变，显示 turnStartedAt 却会重置。旧 Runner 把该显示标记当作只随业务输入变化的标记，故原修复的“相同标记”路径不足。没有修改 Orca 源码或安装文件。

只读原始状态证明：持久 RootWorkingAt/WorkingAt 为 `1792750467839`，BaseRootAt 为 `1792750428584`；原 provider、authority/incarnation 和 working 根仍相同，显示标记先为 `1792751898223`，后为 `1792752708891`，prompt 为空。现有 nativeChat.readSession 接口向前分页后，baseline 之后恰好只有一条真实用户输入；其 ID 为 `01a16841-3cef-7532-b9ae-8b4f6e934073`，时间为 `1792750468345`，完整正文与原冻结输入匹配。由此不能将显示标记重置解读为另一条真实用户输入；压缩触发的 SessionStart 与该机制相容，未把未保留的历史 hook 原报文冒充已取证事实。

Runner 增量仅在原查询预算内调用已有只读 RPC 补输入归属：核验 baseline 后唯一完整用户输入（新的同文输入也拒绝），再核验原资源/主记录/显示标记和 hook authority。working 缓存只在原 working 根、相同显示标记和可信通知下有效，边界、缺口或重附着使其失效；读取途中更晚的来源 revision 优先，不能用较早补证掩盖新输入。输入证明只在内存中流转且不接受 wire JSON 注入；不增加数据库字段、codec、远端文件部署、模型输入、发布调用或最终结果来源。原完成/outcome/正文完整性门槛保持。

现场 probe 用 SQLite `mode=ro` 复制原 Turn 到忽略的隔离证据目录，读取原公开会话并只在内存中执行新 NativeProof.query。2026-10-23 19:00 附近的实际结果为 `state=working / input_rows_after_baseline=1 / full_input_match=true`，诊断从原 missing preview 解除为为空。没有把该结果写回原数据库、重发任务或推进原 workflow；该验证也不代表原任务已完成。临时 probe 测试已删除，生产接口未增加现场参数。最小日志保留于 `.evidence/orca-active-state-binding/live-read-only.log`，原输入与 locator 不提交入仓。

新增六项确定性回归覆盖 working/adapter 重建/final、完整输入/唯一性、再次发送同文、缓存边界、分页完整性/游标/预算以及读取中身份/输入变化。两受影响包全包初验通过（orcatransport 46.759 秒，protocol/orca 7.311 秒）；最终源码全包复验通过（47.184 / 7.805 秒），对应 go vet 通过。完整 L1 使用 `GOFLAGS=-p=2`，624 项顶层 / 2235 项含子测试通过，0 失败；16 项既定 opt-in skip 不计通过，29 项终端扩展、native driver 编译和零匹配守卫通过，正式 summary 为 passed。L1 已含前五项新增入口；执行期间自审再补“较晚通知不能被先前查询覆盖”的第六项测试与来源水位守卫，随后对最终源码执行上述两包全包及静态复验。该增量未重复整套共享 L1，不移记为旧运行的冻结构建资格；也不继承以前的完整 Controlled/远端 PR 资格。

最终候选为 Windows/amd64 `0.2.0-dev.3-r2`，source `ca1a8a36bbaacaa83f4a14817766cf4f04a0f07f-dirty`，SHA256 `B6DA4193AA5893DF18768C832A549EBBB5F4CE7BE996B539A417A044D1977BE2`；静态 version 查询通过，resource digest / state codec 保持。最终七个运行/测试源文件摘要、正式 L1 报告和最终两包日志位于 `.evidence/orca-active-state-binding/source-index.json`、`l1/summary.json`、`final-packages.log`。19:13 的原服务只读复核仍为同一 PID/启动时间，原 Run 为 running / observation_unverified，符合尚未部署的事实。

生产保护：原 Runner PID `45284` 自北京时间 17:48:46 起保持；安装 EXE SHA256 为 `D6688642410E9E30FF5672896CFD9992BA81E5F348C6E5F29F160973DDB87474`。本轮候选只构建到当前 worktree 的 `.evidence/orca-active-state-binding/steerrunner.exe`，未替换安装 EXE、重启服务或改写原 Run。部署需在运行任务可安全换代时由后续操作执行；不把“代码已修复”表述为“生产现场已恢复”。

### 提交前质量审计

用户随后授权审计修复、创建 PR、直接合入主线及通知既有 L4 聊天。本轮为主会话自审，没有委派审计 agent；没有修改 Orca 或部署生产。上面的候选摘要属于审计前阶段，提交源码以本节的精确文件摘要为准。

| 复现的问题 | 修复及约束 |
| --- | --- |
| 独立主记录与 session 元数据不同，冲突 marker 清为零后仍携带元数据观察证明 | 只有 agent、完整 main（含 outcome）和标记相符才附带元数据证明；补证读前后均要求该证明，观察时间不早于主记录起点 |
| 找到输入/baseline 后提前返回，跳过游标检查 | 在检查正文前验证每页游标；负值、非前进游标即使携带匹配输入也拒收 |
| 过期通知撤销较新的有效缓存 | 旧于持久 proof 或输入查询水位的通知直接丢弃，不获得证明，也不撤销当前绑定 |
| 更换 feed 时沿用上一个 feed 的输入绑定 | 更换 feed 立即清除输入缓存，原已选公开消息的确认协议保持 |
| 新补查的身份错误/读前后确定冲突被一般读取失败吞掉 | 补证前后沿用原新鲜度与身份/归属/outcome 冲突规则；后置读取返回其实际新事实，已确认冲突持久保存，过期记录仍不能制造冲突 |

修复前先运行新增用例：首轮 9 个子例实际失败（两项分页、五项异源主记录、两项缓存），随后独立身份错误子例失败；原始失败日志完整保留，未改写为通过。修复后定向回归通过。新增 `TestOrcaNativeInputCannotCombineDifferentMainRecords`、`TestOrcaNativeInputCachedBindingHandlesFeedAndStaleSamples`、`TestOrcaNativeInputIdentityErrorRemainsDurable` 并扩展现有反例，覆盖补证前的 failure、读取后失败/边界、新鲜度与错误分类。

重新执行正式 L1：**627 项顶层 / 2253 项含子测试通过，0 失败**；16 项既定 opt-in skip 不计通过。29 项终端扩展、native driver 编译与零匹配守卫通过，正式报告 passed。L1 已包含八项原输入测试；执行期间最后补全确定冲突/观察时钟守卫、第九项用例及读前后 authority 围栏，随后最终源码两受影响包全量复验通过（protocol/orca 5.950 秒，orcatransport 44.164 秒），对应 go vet 通过。该最终增量未再次重复整个 application 长回归，不将前次测试二进制移记为最终冻结构建。

最小可审计材料归档于 [验证索引](evidence/orca-native-input-audit-20261023/validation.json)、[最终运行/测试源码摘要](evidence/orca-native-input-audit-20261023/audit-source-index.json)、[修复前失败](evidence/orca-native-input-audit-20261023/audit-before.log)、[身份冲突修复前失败](evidence/orca-native-input-audit-20261023/audit-identity-before.log)、[最终两包日志](evidence/orca-native-input-audit-20261023/audit-final-packages3.log) 和 [正式 L1 原始事件压缩归档](evidence/orca-native-input-audit-20261023/l1.jsonl.gz)。这些受控检查不新增模型输入或真实业务 PR；原 Run 的只读证明仍属于前节现场范围，后续独立 L4 交付资格由对应聊天继续执行。
