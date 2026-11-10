# L3 显式复用完整前置证明

日期：2026-11-10。基线 `91507d7e78c4e89598a0f20a967062d888104a3f`，分支 `codex/l3-prerequisite-reuse`。修改位于独立 worktree `C:/Users/lmas1/.codex/visualizations/2026/11/10/01a1c455-f92f-7c60-9a1f-45ab317a09bf/SteerRunner-l3-prerequisites`。

## 行为与保留的核验

`scripts/test.ps1` 将 `-PrerequisitesFrom` 的单级入口从仅 L2 扩为 L2 或 L3。默认执行方式不变；没有隐式搜索最新证明或跨工作副本缓存。仍拒绝混合/重复级别、L4、Focused、ListAvailability 和自定义 catalog。

证明必须来自当前机器、账户和工作副本，具备匹配的源码/环境指纹、完整直接执行成功的 L1 与 Controlled、完整证据库存及文件哈希和匹配的 Runner 产物；不能复用不完整、失败或递归复用产生的证明。选择性 Controlled 结果不能替代完整前置。

每批重新冻结配置和 catalog，重新编译 native driver 并运行零匹配保护；派发前后仍核对证明、native 输入、driver 和冻结产物。复用模式的执行计数为 L1=0、Controlled=0、NativePreflight=1。只省去相同输入的重复 L1/Controlled，不省去各用例的交付、Git/PR、恢复、停止及清理断言。

选择性 L3 只记录实际用例，不能获得完整 L3 资格；全 catalog 与重复轮次仍须满足既有覆盖规则。L3 单独通过不能获得 L2 交付资格。编译、证明/driver 漂移或用例清理失败保持 failed，不能由完整 catalog 覆盖反向改写为通过。

## 独立审核

只读审核者 `/root/review_runner` 检查实际 diff、证明校验、分派与资格边界，未发现需修复的 P0–P2。确认本次不修改生产 Go、native 用例、worker 或资格函数。审核后的测试脚手架修正及最终验证另在 PR 验收记录中复核。

## 验证与证据

证据根 `C:/SteerlineDebug/wt-5d04-l3-reuse/`，各轮使用独立目录，未覆盖失败日志。

| 验证 | 结果 |
| --- | --- |
| `tests/framework/prerequisites-test.ps1` | 79 项通过，含真实入口对混合/重复 L3、自定义 catalog 和 ListAvailability 的拒绝；`targeted-3245438832df491ea0886e17d3d10a14/prerequisites/verified.json`。 |
| `tests/framework/focused-test.ps1` | 35 项通过，含单/全 catalog L3、重复并行、编译/零匹配失败、派发前后证明漂移、driver 漂移和清理失败；`focused-final-a377a3a579ac4bacb0062a663880c38f/focused/verified.json`。实际入口使用明确的隔离测试替身，不代表真实模型 L3 资格。 |
| `Invoke-SRNativePreflight -RequireStable` | 真实 native driver 编译成功、零匹配保护通过、前后输入指纹一致；`native-62168f38301240199b66d7456c538f7c/receipt.json`。只编译及运行零匹配，不调用模型。 |

前两轮新增分派测试分别暴露测试替身计数作用域、父进程缺少 catalog 导入问题，均修正测试脚手架后在新目录重验。失败保留于 `targeted-3245438832df491ea0886e17d3d10a14/focused.log` 和 `focused-fix-cf02bb00ab7840a6b113780f2648aa19/focused.log`，不计成功。最终提交的完整框架结果及独立复审记录在 PR 正文中登记，避免为补记测试结果改变已验证提交。

没有重跑真实模型/真实测试 PR，不声明完整 L1、Controlled 或真实 L3 资格。原受控回执失败现场 `C:/Users/lmas1/AppData/Local/Temp/sr-54729125` 及 `C:/SteerlineDebug/wt-5d04-receipt/original-readonly-evidence/` 保持原样；本次不启动、重放或清理原 Run。
