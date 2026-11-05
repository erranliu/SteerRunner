# 内容结果锁与离线 inspect 职责边界

日期：2026-11-05。基线 `f6a389cfd4ae7827d7c722555a85e94f1e1ba708`；分支 `codex/content-inspection-boundaries`。先完成四份文档，再设计审计，之后按 A→验证→B 的顺序实施。本文不代表发布资格。

## 设计审计（生产代码修改前）

- A：核对 worker.invoke、observation、resumeRun，三个生产入口传原 worker.gate；finish 的 nil gate 仅用于测试。Engine.Handle 控制先 gate 后 mu，commitLocked 先 Store.Commit 再发布内存；每次提交增加 revision。因此材料读取只释放 mu，重获后复核 revision 足以保护原持久输入/来源/控制，不需要新增授权缓存。原生 final 判定留在采样前原临界区，避免提前 I/O 改变错误优先级；不添加 Allowed 门，保留 stop 后晚到原结果。
- B：核对 CLI offline、host lease/bootstrap/OpenReader、Engine.New 的全量 Runs→进程身份→workers 及 Handle 的 Command/RunView。保留全部 Run 的严格解码及错误先后；Command SQL 错误由 application fault 处理，不能误改为 host service_unavailable。移除进程身份构造失败这一额外前置是明确收益。驻留 command/Run 投影仍同锁。当前主线已删除 RefreshPR，本批不恢复旧字段。
- 新主线已有 verifyContentInputs/verifyFrozenDocuments 去重与新的 final 原生来源形状；本批保留，不按旧审计基线复写。生成材料 finish 门仍有文件 I/O，因此 A 未被上游替代。离线构造仍在，因此 B 未被上游替代。
- 保留 contentEngine、repositoryFixture、auditEngine、R2/运行时文件及原生共享夹具；保留 Engine.New 驻留及 nil launcher 原生 owner 消费者。本批不改持久声明、Store 读写、原预算/恢复/停止。源码摘要及代表编码在实施前采集到 `.evidence/content-inspection-20261105/`。

结论：两份 Spec/Plan 可实施；A 的收益为解除跨 Run 文件读取与提交锁耦合，B 的收益为读取不再依赖执行构造。均不承诺少做校验或测得提速，不新增框架。测试结果待实施后补记。

## A 独立实施及验收（B 生产代码修改前）

仅修改 application/results.go、content.go，新增 content_result_lock_test.go。生产净增20行；finishContent不再做材料I/O，原采样位于mu之外且保持gate；重锁比较原revision后，沿原result_committed事务提交。新增私有函数参数用于阻塞原校验器，无Engine配置/新持久状态。

- `go test ./internal/application -run '^TestContentMaterial(ReadReleases|SampleRejects|ResultCommitFailure|ResultErrorPriority)' -count=1 -timeout=3m -json`：4顶层通过，0失败/跳过，1.525秒。
- `go test ./internal/application -run '^Test(Content|RecoveryFrozenMaterials|LocalCodex|NativeCompletionProof|SingleExecutionLateFinal|R2|SRStopSerializes|FrozenBudgetUnknown|SRExecutionWaitRecovery)' -count=1 -timeout=8m -json`：40顶层通过，0失败/跳过，103.132秒。
- `go test ./internal/storage ./internal/host -count=1 -timeout=5m -json`：35顶层通过，0失败；TestOrcaStoppedNativeProbe原生opt-in跳过，未运行真机探针。
- 351个既有internal测试文件原字节保持；代表Record/Snapshot/Request/Receipt、持久类型、capabilities共148565字节，前后SHA256均为`e11066197caa8eed82b90b01618e241a0a083715ffa1f1be571446e51b4dce3a`。代表编码不替代全部格式证明，Store/声明/原编解码无改动另作静态核对。

初次沙箱工具反馈延迟，手动终止会话；其JSON文件已有4顶层通过，但会话返回1，另一个诊断编译会话也主动中止，list随后完成。正式资格采用宿主exit0结果，不以中止会话替代。A上述验证完成后才开始B。

## B 实施中的边界修正

新增测试最初要求所有文件清单不变，首轮失败；响应/错误断言均通过。用main原InspectOffline覆盖复现相同文件副作用失败，确认原storage.OpenReader的SQLite mode=ro仍会创建临时WAL/SHM协调文件。修订B03为保持DB原字节及持久/执行文件、原SQLite副作用语义；新夹具仅排除这两个精确临时文件名，仍断言WAL无数据、其它文件/endpoint无增改。生产OpenReader不变，不为新断言修改原读取协议。原失败与overlay复现日志保留。

## 联合受控验证的主线夹具修复

主线#114改变workflow.ImplementPrompt，resources/prompts.json及StagedBusiness仍识别旧正文。前两轮公开受控的服务归属/未知停止通过，重启等待原turn时因fixture unknown stage受阻；这发生在任何final材料采样前。新增使用真实workflow.RenderPrompt、真实Git的最小测试，在未修改StagedBusiness时exit1复现。仅向原implementation分支追加新前缀，保留全部旧前缀、真实Git动作和既有测试。测试支撑适配不改变Runner生产业务、持久格式或提示词，也不以放松原断言换取通过。

`go test ./internal/testkit/protocol ./tests/helpers/native -count=1 -timeout=2m -json`：4顶层通过、0失败/跳过。首轮旧受控在后续case耗尽聚合期限，次轮在同一原因下主动中止；两轮不计完整通过。对两轮最后原Run分别公开stop、停服并service verify-stopped，均resources_settled=true；仅在核实任务自有PID/创建时间/完整命令行后终止仍活着的过期harness。原失败fixture保留，未删除现场；已结束case沿原cleanup处理。

## B 独立实现及组合验证

新增application/inspect.go，只有Store/Runs/Command与RunView；resident原锁范围保持，host保留完整入口护栏。删除离线Engine.New/Shutdown、discard logger与其身份/epoch/workers构造依赖；原Go内存值形状及JSON null/对象形状也保持。B生产净增42行，A生产净增20行，收益不是净删。没有新增通用查询服务、后台线程、持久字段或校验缓存。

| 验证命令/范围 | 实际结果 |
| --- | --- |
| `go test ./internal/host -run '^TestOfflineInspection' -count=1 -timeout=3m -json` | 修订SQLite断言后3顶层通过；原InspectOffline overlay也3顶层通过，证明既有响应/错误边界一致 |
| `go test ./internal/host ./internal/cli ./internal/storage ./internal/contracts -count=1 -timeout=5m -json` | 最终70顶层通过、0失败，4个既有现场opt-in跳过：OrcaStoppedNativeProbe、RunUsageLocalCodexPublicLive、RunUsageLocalPiPublicLive、RunUsageRemotePublicLive（均有Test前缀） |
| `go test ./internal/application -run '^Test(ContentMaterial\|ContentConcurrent\|SingleExecution\|NativeCompletionProof\|SRStopSerializes)' -count=1 -timeout=4m -json` | 16顶层通过、0失败/跳过；表中反斜杠仅为Markdown竖线转义，原命令为正则alternation |
| 原源码及编码核对 | 仅4个原生产文件改变，加新inspect.go；另有1行共享测试支撑追加。351个既有internal测试文件未改，storage/runstate/contracts/backend/workflow/adapters原源码未改；148565字节代表编码相同 |

## 最终公开受控及构建身份

正式入口：`scripts/build.ps1 -OutFile .evidence/content-inspection-20261105/build-qualified/steerrunner.exe`，随后 `scripts/test-foundation.ps1 -RunnerExe <该EXE> -Pattern '^(TestSRServiceOwnership|TestSRNativeUnknownAndStop|TestSRRestartPreservesAuthorization)$' -OutDir .evidence/content-inspection-20261105/controlled-qualified -TimeoutMinutes 10`。该入口使用修复后的原共享fixture，不改变场景、轮数、故障注入或断言。

最终result.json：exit_code=0、scenario_count=3；ServiceOwnership84.35秒、NativeUnknownAndStop25.84秒、RestartPreservesAuthorization177.18秒。最后一组的graceful/kill/creation_receipt/pending_stop四case均通过，分别52.39/57.25/38.94/28.60秒。

- 源：`f6a389cfd4ae7827d7c722555a85e94f1e1ba708` dirty；source_digest `7407e052e0b03a8771d14f6d1c3b8e7656837d0fa671496cf779a4878fb5e1ee`。
- 最终Runner SHA256：`e40b074f24b9b736cd3857b9d15be2b6457ac72833a6e31163696a31e86e1a7d`。修复仅影响独立fixture，Runner二进制与前一构建相同，但source_digest/fixture身份使用本轮新记录，不冒用旧资格。
- 代表编码SHA256：`e11066197caa8eed82b90b01618e241a0a083715ffa1f1be571446e51b4dce3a`。声明/原编解码无变及现存储回归与该抽样相互补充，不声称遍历全部可能记录。

A01–05、B01–04按专项范围完成。没有修改产品承诺、执行/恢复/停止机制、持久格式或旧解码；仅取消离线读取对执行身份探测的额外依赖。未取得完整L1/L2/L3、真模型或发布资格；改动位于`codex/content-inspection-boundaries`工作分支，未提交/推送/提PR或部署。

原始日志、基线/最终摘要、编码捕获、构建身份、失败与清理证据归档见[证据说明](evidence/content-inspection-boundaries-20261105/README.md)。

以上为首轮资格和交付状态。随后用户授权的[实现再审计](content-inspection-reaudit.md)保持生产实现，新增真实stop入口并发回归；按追加授权提交、推送并提PR，当前提交范围及验证边界以再审计记录为准。
