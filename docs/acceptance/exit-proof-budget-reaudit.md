# 退出证明与预算清尾：实现再审计

日期：2026-11-01。结论：未发现误删当前生产消费者、缺 Budget 阻断原结果/停止、或破坏保留可靠性承诺的生产缺陷；确认并修正一处验收夹具不符合生命周期合同的问题，补齐合法受理和原结果路径。生产逻辑与[初轮实施](exit-proof-budget-cleanup.md)完全相同，E/B 合计净减 186 行的统计保持。

本轮从最新 main `a06c2806c17056d76cb092b6a1045a0473765fb8` 的相同文件树建立独立 `codex/exit-proof-budget-cleanup` 分支。初轮基线 `fca735a` 与它文件树一致，但历史提交不同；PR 不重复携带已合并的 #80 提交。构建及验证绑定该基线加本次 dirty 源码；[逐文件/产物 SHA](evidence/exit-proof-budget-reaudit-20261101/source-identity.json)为实际身份，不以基线 SHA 代替改动。

## 1. 消费者与删除边界

重新搜索整个仓库的 Go 源码以及 cmd、scripts、tests、文档引用，退役 RemoteExit/newRemoteExitWriter/remoteExitWriter/VerifyRemoteExit/ReadStopArtifact/decodeStopArtifact 只剩两个历史取证源码消费者：export-exit、verify-proof。它们位于独立 evidence/go.mod 下；`go list ./...` 结果不包含该模块。没有保留兼容包装，也没有修改历史源码。见[消费者盘点](evidence/exit-proof-budget-reaudit-20261101/consumer-audit.json)和[模块包清单](evidence/exit-proof-budget-reaudit-20261101/packages.txt)。

逐项核对了保留用途：RemoteExitPath 仍是停止请求与故障注入的纯定位；ExitPublicKey 仍为 32 字节公钥，用于 readiness、history/reader 和原请求的完整绑定；UploadWorkspaceArtifact、共享 IPC、Git exclude 保留。signedRemoteExit 仍用于直接 Codex owner-stop/checkpoint 的真实签名及验签，未删除这些消费者。

将原版本 remote_exit.go 从 excludeRemoteExit 到文件末尾逐字比较：Git exclude、settleRemoteTree 的 Kill/Job accounting/原根核实/等待以及 awaitRemoteStop 的原请求匹配完全相同。RunRemoteAgent 的实际 TUI/app-server 结算次序、Close 和未结算错误仍在，删除范围只涉及专属证明产出。

## 2. 预算检查是否越界

重新追踪 Engine.New/存储读取、Handle 查询及控制、advance、resumeRun、reconcileManual、attachObservation/acceptEvent/finishWithGate、stopRun 与 prepareRun：

- 读取和加载没有新增 Budget 门禁，缺失保持原值；正常草稿仍在 queueInput 首次冻结。
- 未知原意图走只读 Reconcile，活动/已接受输入走原观察，stop 先于输入准备；原 payload/digest 不反填，不获取新的 Execute 权限。
- prepareRun 的检查只作用于待准备的新输入。已冻结缺预算拒绝；仅副本缺失从精确匹配且未提交该业务输入意图的原 StageInput 复制，再事务提交。提交失败不发布副本或派发动作。
- 查询、开始确认、历史、技术 resume 补收原结果、迟到 final、资源结算及原停止回执没有移入这个门禁；原失败、结果和 PR 事实不因缺预算被追改。

特别检查了“预算检查在 reliable.Next 之前，会不会阻止 cleanup”的假设：当前生产 PreparationFacts 没有生成 Cleanup 候选，现有清理/停止/观察及未知查询各有更早入口，未确认实际回归。不为这个假设新增通用 cleanup/fallback 调度层；未来引入该候选时须重新审计门禁位置。

## 3. 确认问题与修复

初轮 TestFrozenBudgetUnknownReconcileAndStopDoNotRepair 的 accepted 分支把 `ResourcePending=true` 放到 start_turn。现行 LifecycleReceipt.ValidateFor 明确只允许 accepted 的 create_resource 使用该标志，因此它不能证明一个合法 accepted 创建的恢复顺序，也不能把它当作 accepted 业务输入的观察用例。

本轮修复为真实类型的 create_resource 受理夹具，显式调用 ValidateFor；补上原资源迟到后只读取得并提交身份、再执行原 stop 的两个阶段。原业务输入的 accepted/等待开始状态由新增 TestFrozenBudgetMissingDoesNotGateOriginalResults 单独覆盖。

新增九个子用例：conversation、generate、execute 各自覆盖 accepted-waiting-start、paused-resume、stopped-late-final。通过实际 queueInput/既有生产夹具、SQLite 保存及 Engine.New 重启，验证 inspect/events/适用 history、原开始确认、原 resume/单次执行拒绝 resume、原 final 观察/保存与停止回执。断言没有新输入、没有补写 Budget、没有丢弃原结果或替换原停止命令。没有为了通过验证修改生产逻辑。

修正夹具后的首次重验仍假设 stop 会立即调用终止，因遗漏原创建回执核实阶段而失败；该次日志完整保留，后续补齐合法回执及有界推进后通过。初轮旧夹具的 accepted 分支不再作为合法资源受理证据，使用本轮结果；历史日志不改判。

## 4. 最终验证与构建

| 验证 | 实际结果 |
| --- | --- |
| application 定向 | 33 顶层、60 子用例通过，0 skip，60.199 秒；包括全部 FrozenBudget/Budget、原操作、人工恢复、原观察、停止竞争和迟到创建等相关用例 |
| codex / protocol-orca / orcatransport 整包 | 290 顶层、1041 子用例通过，7 个原 opt-in/helper skip 单列；直接 owner/checkpoint 签名、真实 Job/reader、IPC 上传、原请求和停止零重复动作保持 |
| 公开 CLI 最终构建 | TestSRFrozenBudgetRecovery、TestSRNativeUnknownAndStop、TestSRServiceOwnership，3/3、20 子用例通过，0 skip；缺预算 accepted/unknown 记录重启后仍可查询、同 ID 回放和停止，无业务重发 |
| 正式 live 测试消费者 | native、integration 各自 `go test -c -tags live` 编译通过；没有拿 TestMain 零用例运行当验收 |
| 真实 Orca/Codex | TestOrcaStoppedNativeProbe 1/1、0 skip，49.66 秒；真实 READY→公开 stop→停服→无 exit.json/inspection journal 离线核验，DB/回执字节不变，业务输入 0 |

本轮组件为 323 顶层通过，默认 skip 完整登记在[原始统计](evidence/exit-proof-budget-reaudit-20261101/test-summary.json)。精确命令、选择器、退出结果和编译身份见[命令记录](evidence/exit-proof-budget-reaudit-20261101/commands.json)。初轮未修改的共享核心等验证仍按其原构建阅读，不重复累加为本轮测试数，不授予完整 L1/L2/L3。

最终 Runner SHA256：`b33f3a4049fd2c5036be1f56b21a6a78cc497bfe7057066c525c2506368c44bf`；helper：`3ae4e9669617078f15ab6a766fe6afb2febdeaa146476ca5a75e16573422c21e`。helper 字节与初轮完全相同；Runner 因基线提交身份改变而重新按正式脚本构建并绑定本轮验证，没有替换安装版。

真实 probe 的[结果](evidence/exit-proof-budget-reaudit-20261101/native-final/result.json)与[新查询清理确认](evidence/exit-proof-budget-reaudit-20261101/cleanup-confirmation.json)均通过；仅使用本次独立资源，原 worktree 已不存在。固定同一 Orca CLI，沿 orca-cli skill 的版本匹配指南操作；未归档无关资源清单、终端原文、运行数据库或凭据。

## 5. 保留的限制

缺失原 Budget 的冻结输入不能再由旧正文补算，可能转人工；新输入装配、已有预算和 legacy 标签验证保持。原身份核实失败仍返回未知，不把 PID 复用解释为退出；本机停止成功只证明冻结登记范围，不承诺全部脱离管理后代结束。没有新增迁移、资源替换、发布或宿主权限框架，也没有同条件耗时加速结论。

本轮只创建代码审查 PR，不部署、排空旧现场或合并。初轮所有失败与本轮夹具失败分别保留。
