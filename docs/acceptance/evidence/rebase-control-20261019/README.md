# 最新主线、固定控制与原生退出证明组合验收

2026-10-19。已 rebase 到 `2aa17cce7da6fd75e59db76e18f2521d9e9777db`，解决冲突并审计；最终干净验证源码 `bb84ea80cab2e37c2c7d184fcd82765dde2b1d32`。产品验证源码之后，仅修正测试证据与停止探针接线（`51792ee5cc3882d2a5127606e411fed1b801fc4d`），生产/集成/脚本源码完全相同，见 [代码等价](checks/code-equivalence.json) 与 [source-files](source-files.json)；产品批次与测试修正提交分别登记。确认的执行期恢复边界内适合合入；本轮不部署或合并，不修改 SteerLine 或 Orca。

## 修复与审计

- 保留主线原 helper owner/Ed25519 公钥、完整 Job/TUI/认证签名退出、原生 reader Job 结算、原 host artifact 与完整清单。四个证明文件与主线逐字节相同，history 只增加 NativeHome 访问器，见[源码审计](checks/source-audit.json)。
- 固定控制仅有 original-turn、authentication-stop。失败查询可保留资源事实，但不能算业务成功；替换传输回执前必须验证原输入/readiness/reader 已结算。未知 reader 不被后继响应覆盖。
- 停止先对账原控制撤销，再请求并核验原 helper 签名退出，随后关闭业务 PTY。认证未核实仍停止业务原树并保留原控制；缺原签名不销毁唯一证明路径。被动停服和离线核验保持原边界。
- 接入主线独立配置：保留 validate/capability、配置/catalog/build/requirements 冻结；测试使用 workflow/2，补齐获取 case 需求，移除废弃反馈资格；修复子进程 OEM 编码导致中文能力 JSON 失败的问题。UTF-8 只在捕获边界设置并恢复。
- Record/Intents/Commit/CAS 为唯一业务状态权威；控制检查点只保存绑定的传输事实。未新增队列、第三方法、通用 RPC、DSL 或第二套恢复权威。内部 codec6/profile3、原生 readiness v3；CLI v1、runstate/v3、workflow/2 沿现约定。

## 实际验证

Windows/amd64、Go 1.26.4、Codex 0.159.2、Orca 1.4.223，现有本机/已配对远端与原生认证。helper 从最终源码重建 SHA `4de01c93e145da8c2d5c5173eff5b445677fa0734333ea7465a843ad79d87803`；与此前已传输不可变产物相同，重新核实远端 SHA/metadata 并关闭专属验证终端，见 [helper](checks/helper-build-r3.json)、[核验](checks/stage-reuse-r3.log)。不复制凭据，不新增登录。

```powershell
./scripts/test.ps1 -Level L3 -Scenario @('orca-remote.service-restart','orca-remote.stop-revocation') -SkipAdminRequired -Config .evidence/rebase-control-20261019/test-config-r3.json -OutDir .evidence/rebase-control-20261019/final-latest -MaxParallel 2
```

正式入口整体通过：完整 L1 **385 Go / 29 Node**，8 项已登记 helper/native opt-in 跳过，框架 **16/16**；完整 Controlled **16/16**，含结果获取8条子路径、零真实模型；所选原生 workflow **3/3**。L1/Controlled 整套预算12/25分钟，单场景/产品期限、断言和计数门槛不变。见[最终协调报告](prerequisites/summary.json)、[Go 原始输出](prerequisites/l1.jsonl)、[Controlled](prerequisites/controlled/result.json)、[日志](prerequisites/controlled/integration.log)。配置、catalog 和构建随报告冻结；没有拼接旧源通过替代最终入口。

| 原生 case | 结果 | 可核验证据 |
| --- | --- | --- |
| orca-remote.service-restart/graceful-active | 通过 | [原签名退出](native/orca-remote.service-restart/graceful-active/6231a2f36445/native-signature-proof.json) · [固定控制/reader](native/orca-remote.service-restart/graceful-active/6231a2f36445/fixed-control-native-proof.json) · [服务结算](native/orca-remote.service-restart/graceful-active/6231a2f36445/service-status-final.json) |
| orca-remote.service-restart/kill-active | 通过 | [原签名退出](native/orca-remote.service-restart/kill-active/5dabbee3c1d6/native-signature-proof.json) · [固定控制/reader](native/orca-remote.service-restart/kill-active/5dabbee3c1d6/fixed-control-native-proof.json) · [服务结算](native/orca-remote.service-restart/kill-active/5dabbee3c1d6/service-status-final.json) |
| orca-remote.stop-revocation/active | 通过 | [原签名退出](native/orca-remote.stop-revocation/active/b489f4de9727/native-signature-proof.json) · [固定控制/reader](native/orca-remote.stop-revocation/active/b489f4de9727/fixed-control-native-proof.json) · [服务结算](native/orca-remote.stop-revocation/active/b489f4de9727/service-status-final.json) |

本机/远端既有停止探针各一次 `go test ./internal/host -run '^TestOrcaStoppedNativeProbe$' -count=1 -timeout=8m -v`，显式参数、EXE 和新隔离目录见 [probes](checks/probes.json) 与 [脚本](checks/probes.ps1)，两项通过。探针 Go 测试使用测试修正提交，复用 bb84ea8 的冻结 Runner/helper；实际两种源码分别记录。公开 verify-stopped 活动时拒绝，原 helper 签名退出后通过；原 DB/失败停服回执不变，不启动服务。每项 helper 资格准备含真实 READY 模型调用，业务输入为0；不能宣称整场零模型。原 reader 的真实查询结算由上述 workflow3覆盖。见[本机签名](probes/local/native-signature-proof.json)、[远端签名](probes/remote/native-signature-proof.json)、[本机原控制](probes/local/native-control-proof.json)、[远端原控制](probes/remote/native-control-proof.json)。

五份退出正文由[生产验签函数复核工具](checks/verify-proof/main.go)逐份验签，控制 ledger 则由[独立复核脚本](checks/verify-control.py)检查首业务输入前就绪、同 handle/incarnation、无逐次 reader/revoker 创建和最终关闭。[复核日志](checks/post-proofs.log)、[汇总](verification.json)、[资源/服务结算](cleanup-summary.json)。原始公共退出字节均另存为 native-exit.json。workflow 三项从相同原 runtime/worktree 的 artifact 通道重读，来源和 SHA 见各自 native-exit-export.json；原报告内 signed_native_exit 被外层缩进改写，只作诊断，不用于验签。测试已改为 signed_native_exit_bytes 的 base64 原字节保存，并有归档后验签/篡改拒绝回归（[修正验证](checks/capture-validation.json)）。没有重签、重构原文或替换原执行 final。

## 失败历史与边界

早期组件夹具/编译失败及 application 整包5分钟超时、首次 L1 的8分钟超时与遗漏 opt-in 跳过登记保留为失败；同源 application 12分钟诊断重跑通过，随后正式入口修复总预算并完整重跑。`5861ded` 的完整 L1通过，但 Controlled 因主线推进主动中断，未启动 native；中断后发现另一自有 Controlled 服务，首次缺写信封被拒绝，随后以原 instance 的写信封公共 stop 清理，最终 not_running/resources_settled=true，旧现场可执行进程清单为空（historical/interrupted-service-residue-after-bound.json、interrupted-processes-final.json）；原 running 摘要和[中断事实](historical/interrupted-before-latest-main.json)保留，不算最终通过。接入新配置的组件首轮版本失败、两次 OEM JSON 框架失败也保留；修复后框架与最终整批重新通过。停止探针首轮误用内部 ID 作为 Orca UUID，在 helper 启动前被拒绝，原失败和清理结果保留；修正首次发送后，两项新现场重跑通过。签名归档问题及原字节回收、测试修正见[首次验签失败](checks/signature-archive-initial.log)、[原字节验签](checks/signature-exact-bytes.log)、[回归](checks/capture-probe-regression-final.log)，只改测试/证据，未重跑无变化的业务模型。helper 第二次冷上传遇到已知 SHA 目录 EEXIST，未创建终端或模型；之后只复用已知同字节产物并重新核验。详见 [historical](historical/)、[组件首轮](checks/newmain-components.log)、[组件修复后](checks/newmain-components-r2.log)、[框架失败](checks/newmain-framework-r2.log)、[框架修复后](checks/newmain-framework-r3.log)。

初始化 terminal create 身份未知继续阻塞，不按标题/空清单重建。四项管理员网络/阶段边界变体明确排除；未重跑无关完整 L2/L3 或 Pi/DSH 矩阵。本次结论限于确认的 Windows/Codex 与执行期强杀窗口，不宣称初始化任意强杀可恢复。[测试 PR](created-test-prs.json)属于隔离 SteerlineTest；产品 PR 不在本轮合并。

原始公共文件来源与摘要见 [raw-origins](raw-origins.json)，文档/JSON/链接及敏感模式检查见 [doc-check](doc-check.json)，归档字节摘要见 [hashes](hashes.json)。不归档 EXE、数据库、私钥或认证内容；`.gitattributes -text` 保持原始证据字节。
