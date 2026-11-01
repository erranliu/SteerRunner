# 退出证明生产端与旧预算重建清尾：实施验收

日期：2026-11-01。初轮结论：EPBC-E/B 完成专项最小验收，当时未部署、提交或发布。后续[实现再审计](exit-proof-budget-reaudit.md)修正一处受理夹具并补验，最终构建/验证与 PR 状态以该报告为准；本文保留初轮事实。依据 [Spec](../spec/exit-proof-budget-cleanup.spec.md)、[Plan](../spec/exit-proof-budget-cleanup.plan.md)和[长期承诺](../spec/steerrunner.spec.md#长期产品承诺)。源基线为 `fca735ae30472b23584affd33e75d2e80b70ac34` 加本次 dirty 修改，基线文件树与 main `a06c280` 相同；具体文件 SHA 见[源码及产物清单](evidence/exit-proof-budget-20261101/source-identity.json)，不把 HEAD 本身当本次源码。

## 1. 实际变更和取舍

| 增量 | 生产新增/删除/净减少 | 实际删除 | 保留 |
| --- | --- | --- | --- |
| E | 7 / 110 / 103 行 | RemoteExit、专属 writer/verifier、helper 的预开/签名/Write/Sync/close、ReadStopArtifact/decodeStopArtifact；删除两份专属 IPC/解码测试 | Ed25519 同形公钥生成且立即清除私钥、原 readiness 编码与请求地址/完整匹配、Git exclude、上传和共享 IPC；实际 Job/reader 清理，直接 owner/checkpoint 签名及共享封装 |
| B | 25 / 108 / 83 行 | History message、内容标签/材料/schema、PromptBodies/反馈后缀三类反推及其补算保存 | 正常装配及既有预算验证、未渲染草稿、已保存旧来源标签、精确未外发副本复制、原生回答/parser/1000B 和传输保护 |

生产合计净减 186 行，计数限上述五个生产文件，不包含测试/文档删行、移动或历史归档。没有同条件性能比较，不声称任务提速；公钥生成、请求/readiness/configuration 文件和 Job 等待仍存在。

恢复承诺明确收窄：已冻结却缺原 StageInput.Budget 的新输入拒绝，不再保证凭旧正文补算，可能需本人处理。只有 PendingTurn 副本缺失时，原命令、步骤、版本、Attempt、正文、报告策略、模型和 effort 一致、原预算合法且无该输入持久外发意图，才复制原值并先保存。准备入口还检查“副本有预算、原 StageInput 缺预算”及两份预算冲突，避免绕过拒绝。检查限新输入准备，不加入加载、inspect、观察、原操作对账或 stop。

未新增格式或恢复权威；state/v8、CLI/Workflow/readiness 编码保持。原 intent 正文/摘要、接受/结果和停止事实不补写。直接 Codex、Pi/DSH、原生远端的停止保证未改。

## 2. 验证范围和结果

[原始日志和统计](evidence/exit-proof-budget-20261101/test-summary.json)含精确顶层/子用例、包通过、失败和 skip。下面顶层计数不含子用例，基线及重复执行不叠加为最终测试数。

| 验证 | 实际选择 | 结果 |
| --- | --- | --- |
| B 删除前入口基线 | application：`^(TestFrozenBudget.*\|TestBudgetCallerOwnershipAndFeedback\|TestContentConcurrentMessages\|TestRARecoveryBusinessInput\|TestRAAuditLifecycle)$` | 7 顶层、10 子用例通过，0 skip；基线执行时生产反推尚未删除。先断言生产 writer 保存非空 Budget，再调用校验，未靠反推掩盖缺失 |
| E 组件 | codex、orcatransport 整包；protocol/orca 在解码测试清理后单独整包重验；均 `-count=1` | 最终 290 顶层、1041 子用例通过；7 个原 opt-in/helper 测试 skip 单列，不能算真机通过 |
| B application | `^(TestFrozenBudget.*\|TestBudget.*\|TestContent.*\|TestSingleExecution.*\|TestRA.*\|TestSRManual.*\|TestSROperation.*\|TestSRStopSerializesWithNativeDispatch\|TestSRStopBindingCommitFailureDoesNotDispatch\|TestProjectPolicies.*)$`，`-count=1 -timeout 8m` | 63 顶层、72 子用例通过，0 skip；210.783 秒为整组累计，未延长生产期限 |
| 共享核心 | model、workflow、reliable/...、contracts、storage 整包，`-count=1 -timeout 4m` | 79 顶层、134 子用例通过，0 skip |
| 公开受控 CLI | foundation：`^(TestSRFrozenBudgetRecovery\|TestSRNativeUnknownAndStop\|TestSRServiceOwnership)$`，最终 Runner，8 分钟夹具上限 | 3/3 场景、20 子用例通过，0 skip；accepted/unknown 各自在 writer 停止后故障注入缺预算，重启后 inspect/events/原 request 查询与同 ID submit 回放可用，stop 成功，turn/start 始终 1，原 intent 正文/摘要未改 |
| 正式测试消费者 | `go test -c -tags live -o <本次产物> ./tests/native`、`./tests/integration` | 编译通过；最终 integration 编译在修正 CLI 查询夹具后再次执行，非运行 TestMain 零场景 |
| 真实 helper | `go test ./internal/host -run '^TestOrcaStoppedNativeProbe$' -count=1 -timeout 5m` | 新现场 1/1、0 skip，53.58 秒；真实 READY、公开 stop/普通停服及离线验证通过，业务输入 0 |

最终受影响组件共 432 个顶层通过，不是完整 L1 或完整 L2/L3。默认 skip 的名称在统计中完整保留，独立真实探针仅授予下述范围。

## 3. 最小验收逐项映射

| ID | 实际证据 |
| --- | --- |
| A00 | 长期/预算专项/本次 Spec/Plan、索引同步；新增链接及 diff 检查 |
| A01 | TestRemoteStopBindingNeedsNoExitArtifact：无 workspace 参数、无文件句柄的生成边界，阻塞旧导出根不影响生成；源码调用盘点为零。真实 probe 在 READY 和停止后均断言 exit.json 不存在。请求目录仍可能失败，不宣传 helper 零文件 I/O |
| A02 | TestRemoteStopRequiresCompleteOriginalReadiness 覆盖错正文/部分/超大及原请求；LocalCodexStopCooperationAndLostClose、ReconcileNeverClosesLiveResources、StoppedRootsAndInventory 等原计数/反例保留 |
| A03 | RemoteExitAccountsForOriginalOwnedTree、RemoteReaderFailedStartStillProvesNoNativeReader 及 history/turn/workspace 原测试；ExecutionOwnerExitRequiresSignatureAndCompleteOriginalBinding、ExecutionOwnerCheckpointPinsOriginalTerminalAndSignature 的正反例通过；settleRemoteTree 正文未改 |
| A04 | 五种 Workflow 的真实 queueInput/freezeContent；development/audited/execute 各 direct/Orca，共八种首次装配；Unicode/引号/CRLF、文件说明、SQLite/Clone、原已渲染恢复及 report/未渲染草稿。原业务反馈、并发 message、审计及人工恢复测试同时通过 |
| A05 | FrozenBudgetCopyRequiresOriginalUnsentBinding 的 18 个分支，以及 FirstRenderCommitFailureDoesNotPublish：来源/副本各自缺失、全部缺失、绑定/预算冲突、既有外发意图、事务失败；新 Execute/Reconcile/stop 为 0，失败不发布或补写；合法原预算可提交复制 |
| A06 | FrozenBudgetUnknownReconcileAndStopDoNotRepair 的 accepted/unknown 原查询和 stop；公开 CLI 重启/回放/停止、原操作提交失败和 stop 竞争、迟到 final/人工恢复回归；读取和结算没有预算全局门禁 |
| A07 | 已保存 legacy 来源标签读取、旧默认正文保真、model SHA/来源/计量/UTF-8 反例、原生回答及 parser 溢出后原一次 report 路径通过 |
| A08 | 当前模块内旧专属 writer/verifier/reader/解码符号为 0；历史取证 Go 工具由 evidence/go.mod 隔离，保留原字节、无兼容包装；native/integration 最终 live 编译通过 |
| A09 | 真实探针原 readiness/资源、公开回执、无文件核验与清理，见下节 |
| A10 | 生产分项增删、完整 SHA/选择器/skip/失败历史及证据摘要；不报未测提速 |

## 4. 真实现场与构建

Runner SHA256：`971e1134c12699fc039e22a53b2e4df3213f781ff5b166f0b2eee6f725faef32`；helper：`3ae4e9669617078f15ab6a766fe6afb2febdeaa146476ca5a75e16573422c21e`。Runner 由 scripts/build.ps1 生成并保存[构建记录](evidence/exit-proof-budget-20261101/runner-build.json)，helper 由 `go build -trimpath ./cmd/steerrunner-agent` 生成。Windows/amd64，Go 1.26.4；未替换安装版。

通过的 [native-confirmed/result.json](evidence/exit-proof-budget-20261101/native-confirmed/result.json)绑定原 Orca runtime/worktree，`real_codex=true`、`exit_artifact_absent=true`、`resources_cleaned=true`、`business_inputs=0`。Codex 0.159.2，`gpt-6.1-sol`，原已登录主机账号；使用 orca-cli skill 的已安装版本匹配指南和固定 `C:/Users/lmas1/AppData/Local/Programs/orca/resources/bin/orca.exe`，仅创建本次隔离 worktree/业务和控制终端。

原 READY 绑定 helper/app-server 根身份及 32 字节公钥；公开 inspect 的 scope 为 orca_local_codex_resources。活动态离线核验拒绝，公开 stop 到 stopped 后普通停服，再无 exit.json/无 inspection journal 核验成功；验证同时逐字比较 runner.db 和 shutdown.json，确认没有启动服务或改写回执。原终端、helper/app-server 根按现有范围结束，Job 清理仍由真实组件测试单独证明；不推广为所有脱离管理后代均退出。

[清理确认](evidence/exit-proof-budget-20261101/cleanup-confirmation.json)由测试清理回执与两次探针完成后的新 Orca worktree list 交叉核实，两个自有 worktree 均已不存在。未归档无关 worktree 清单、终端原文、运行库或凭据。

## 5. 首轮失败和未取得的资格

- 最早入口选择器漏匹配新增测试前缀，只运行四个旧用例，不算入口矩阵资格。修正选择器后审计工作流两用例因新夹具漏给原本必需的模型配置失败；补齐夹具后 7/7 入口基线通过，未修改生产 writer 掩盖漏预算。
- 首轮 adapter 命令的 codex/orcatransport 通过，但 protocol/orca 编译失败：另有专属 TestStopArtifactRequiresCompleteBoundedBytes 引用退役解码器。补查并删除该专属测试，协议包整包通过；原失败保存在 adapters-final.jsonl.gz，不改写为一次整体通过。
- foundation-final 的缺预算场景错误调用开发流程不支持的 history，按原契约返回 unsupported_capability，2/3 顶层通过。改用 events 和原 request inspect 后 foundation-confirmed 为 3/3；未增加产品 capability。
- native-final 在中间无证明核验成功后，最终读到原 PID 被不同进程复用，按现有精确身份规则返回未核实，整体失败；退出文件已不存在、资源清理完成。该轮当时与多进程测试并行，不据此断言并行是原因，也不将 PID 复用当 absent。后续不放宽判据、不延长期限，新隔离 native-confirmed 全流程通过。

历史 export-exit/verify-proof 工具不再能针对当前符号编译，应使用其固定旧源码；归档保持原字节。本次不跑所有模型、全部真机 Workflow、完整 PR/合并或 L2/L3；不排空、迁移或修改已有用户现场。缺预算新派发拒绝和精确身份不可核实时保留现场，是明确保留的限制。
