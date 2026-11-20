# Runner #172 L1 / Controlled 失败闭环

2026-11-20，按用户要求继续修复 [PR #172](https://github.com/erranliu/SteerRunner/pull/172) 的剩余验证失败，通过完整验证后再取消 draft。起始提交 `9b9a1f18fb0332ee1c84863c93c31cd59c4ddaba`；原失败及原始证据保留在[启动索引专项](codex-startup-index-reuse.md)，不覆盖旧报告。未部署、未合并，不操作原任务。

## 定位与修复

- `TestSRStandalonePackage` 的旧断言要求提交准备失败后 inspect 返回 request_not_found。原 caller-commands.jsonl 实际显示命令已被正确保存为 receipt_status=rejected、action_status=rejected、run_id=null，并保留 environment_unavailable 错误；符合现行[提交持久化合同](../cli-guide.md)和[原验收](durable-submit-admission.md)。更新受控集成断言，核对原请求/命令、拒绝状态及原错误、无 Run，并再次用同一请求重放后按原 command_id 核对拒绝身份和无 Run；不修改生产受理逻辑。
- `TestSREndpointPublicationWithDiscoveryReader/persistent-reader` 原实现最多 100 次、每次睡 5ms，但 Windows 定时粒度与负载可能使一次短睡眠显著超时，实际 2.43 秒才失败。生产替换现在按单调时钟约束原预期 500ms 重试窗口，睡眠后若预算已尽立即返回原共享冲突；仍只重试 Windows sharing/access 错误、保持原子替换。既有真实文件句柄测试的两秒上限及原/新文件内容断言保持。
- `TestCLITimeoutStartsAfterAdmission` 的一秒 attempt 包括版本查询和 OS 子进程启动，整批负载下未到 started-2 屏障就已合法超时；这不表示排队期间启动了期限。测试改用生产最小合法值 10 秒，保留排队时未读 deadline、准入后已读 deadline、真实 CLI 到达屏障、DeadlineExceeded、零重试等断言；另外取消失败路径的排队调用，避免测试退出后留下等待进程。生产配置、attempt 期限实现及并发准入不变。

本轮不修改提示词、模型策略、注册测试范围、正式资格判定或生产超时配置。原 L1 的输入身份变化是上轮测试过程中修改了参与指纹的源码；本轮先固定代码，再串行执行完整 L1/Controlled，执行中不改输入。

## 定向验证

证据根 `C:/SteerlineDebug/wt-9ed5-runner172-qualification/`，受测版本为上述提交加本节生产/回归差异。

- `go test ./internal/host ./internal/textanalysis -run '^(TestSREndpointPublicationWithDiscoveryReader|TestCLITimeoutStartsAfterAdmission)$' -count=3 -json`：两项原失败各三次通过，端点两种实际 reader 子例均通过；`diagnostics/timing-01/tests.jsonl`、`stderr.txt`、`result.json`。
- `./scripts/test-foundation.ps1 -Pattern '^TestSRStandalonePackage$' -OutDir C:/SteerlineDebug/wt-9ed5-runner172-qualification/diagnostics/standalone-01 -TimeoutMinutes 5`：真实 EXE/IPC/SQLite/Git 独立包回归通过，1 项、零 skip、exit_code=0；`integration.log`、`result.json`、`source-identity.json` 与 caller journal 保留。其后只加强解码前清零以防缺字段复用前值，由下述完整批次覆盖。

## 完整验证与交付

在源码固定后使用正式入口 `./scripts/test.ps1 -Level L1,Controlled`，实际命令在最终结果中登记。必须完整 L1、原生 driver/零匹配守卫、完整 Controlled 的直接证据通过、输入指纹不变及资源结算后，才取消 draft；定向通过不替代完整资格。

第一轮正式完整命令：

```powershell
$env:GOFLAGS = ''
./scripts/test.ps1 -Level L1,Controlled -OutDir C:/SteerlineDebug/wt-9ed5-runner172-qualification/test-results/full-01
```

受测 `baa0a1250355a49c5211f016d417e26da8d3b7f7` 干净源码，批次 3,677.79 秒，整体 failed。L1 直接封存 passed：38 包、813 个顶层测试通过，零失败；原规则允许的 12 个 helper/live opt-in skip 原样列出，21 项扩展及 native driver/零匹配守卫通过；L1 耗时 1,502.33 秒，前后输入指纹相同。Controlled 全部 30 个注册父场景实际执行，27 通过、3 失败、零 skip；原 StandalonePackage 已通过。

完整批次又暴露了资源身份读取、进度及结果文件三个父场景的问题，保留失败现场及 `controlled/caller-commands.jsonl`，不将其改报通过：

- wire-loss 的证据读取忽略了 ReadFile 错误。原文件通过原子写发布，事后 1,054 字节完整可解析；但原错误未记录，不能断言这一次必然是 Windows sharing violation。新增读取屏障仅对不存在/Windows sharing/access 错误在原 60 秒窗口内等待，其他 I/O 或坏 JSON 明确失败；仍须通过全部原身份断言。新增同父场景的真实 Windows 独占句柄子例，明确复现“文件存在但不可读”，要求释放后取得原 JSON，不用旧结果代替新证据。
- progress 用例在只有 workspace、尚未完成 session/首次输入准备时就开始 20 秒采集窗口。生产可选文件在首次输入前准备，故等待 TurnID 后才检查初始版本；保留 20 秒采集期限、种子字节、两个 Run 的隔离、版本与停止断言。
- 两次 result-file 提交的原 CLI 在有限响应窗口后返回 request_outcome_unknown，服务日志分别在 37.67/54.79 秒记录原提交成功。夹具现在只对正向 submit 的这一明确错误，用相同 request_id 和完全相同字节进行一次文档允许的幂等对账；保留两次物理 CLI 日志，不产生替代请求或伪造成功响应，原业务输入计数继续约束零额外输入。
- final-seal/enabled-2 在首次输入已发出时没有种子文件，保留元数据明确显示可选准备已尝试、写入已结算、Initialized=false、最终 read_failed。可选文件失败不阻断业务是现行合同；本次没有延长四秒准备窗口，也没有移除正常种子内容断言。此失败由新现场再次验证，不能拿其他变体通过覆盖。

原有来源、收尾与资格判定保持。`diagnostics/full01-resource-check.json` 只读核对 full-01 冻结程序及五个失败现场的精确路径，进程匹配为空；未把 wire-loss 的 unknown 创建及两个 blocked 提交的原数据库记录改写为 stopped。

后续完整父场景补验：

```powershell
./scripts/test-foundation.ps1 -Pattern '^(TestSRResourceCreationRecovery|TestSRProgressFile|TestSRResultFile|TestSRStabilityLostSubmitResponse)$' -OutDir C:/SteerlineDebug/wt-9ed5-runner172-qualification/diagnostics/controlled-fixes-01 -TimeoutMinutes 15
```

4 项通过、零失败、零 skip、exit_code=0；新增真实独占句柄子例与原 wire-loss 分支通过，ProgressFile 原版本/隔离/停止断言通过，ResultFile 的全部启用组合及种子/最终内容断言通过，同 ID 丢回执对账及零额外业务输入通过。原始日志、所有物理 CLI 响应、测试程序与构建身份保留在该目录；这仍是定向诊断，不充当完整 Controlled。

夹具修改使完整前置指纹发生变化，因此不将 full-01 的 L1 复制或拼装成新资格。后续先冻结新提交，再执行 `./scripts/test.ps1 -Level L1,Controlled -OutDir C:/SteerlineDebug/wt-9ed5-runner172-qualification/test-results/full-02`；执行期间不修改受测输入。结果待封存后补齐，当前 PR 仍为 draft。
