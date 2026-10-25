# Orca CLI 等待与重试：实施与独立验收

日期：2026-10-25。状态：实现与 L1/定向受控验证完成；完整验收被 Windows Defender 拦截，尚未取得独立交付资格。关联 [Spec](../spec/orca-cli-wait-retry.spec.md) / [Plan](../spec/orca-cli-wait-retry.plan.md)。从 `origin/codex/orca-cli-wait-retry-spec`（`d95bbc1ba207033e8fed7fb03d351d6e5190ed0d`）开始，工作分支 `codex/orca-cli-wait-retry-implementation`。不部署、不修改 SteerLine、不恢复生产 Run。

## 设计审计结论

Runner 有独立收益：直接使用其 CLI 的用户，同样可能因通道竞争耗尽准备读取预算；本仓独立修正等待/执行期限混用、有界读取重试和等待可见性，不需要 SteerLine 才产生价值。验收只依赖 Runner 服务/CLI、SQLite、隔离夹具与 Orca 目标，调用方 UI 联验不是门槛。

| 原问题 | 修订决定 |
| --- | --- |
| 三类加权 dispatcher、强制合并器与统一采样退避超出必要范围 | 只在 SerializeCLI 内增加同进程 FIFO，保留原命名互斥；不调整采样周期 |
| 为写操作加 120 秒窗口与核验子窗口，重叠原节点策略 | 仅为 remote repo list / local status 定义单层读取窗口；写操作、unknown 与停止保持 |
| disposition/effect/dispatch_state 等重复 operation/status/controls | 最小 execution_wait 仅描述准备查询；未知副作用继续用已有事实 |
| Runner 完成依赖 SteerLine 联验 | 本仓独立测试、原生资格与公开契约形成自身交付，调用方另验 |
| 等待事件推进 revision，可能被原 preparationQuery 判 stale 或被旧 clone 覆盖 | Plan 要求区分自有进度与真实控制/身份竞争、合并当前聚合；不另建 revision/store |
| 原调用链已有 90 秒 context，单独承诺 120 秒会暴露虚假等待期限 | 删除独立 120 秒预算，复用原 90 秒上限并取原准备/caller context 更早期限；不延长父操作/写操作预算 |

以上是静态设计与源码边界审计，不是实现/原生能力通过。未采用通用队列/重试框架、迁移兼容层或替代 owner。

## 自动验收计划

| 项目 | 最小条件 | 状态 |
| --- | --- | --- |
| OCW-A01 | >15 秒排队后有效窗口内自动继续一次 | 组件/定向受控通过；完整回归因 Defender 拦截未完成 |
| OCW-A02 | 两项准备读取的单层预算/分类/次数，空 JSON 错误修正 | 组件/定向受控通过；完整回归因 Defender 拦截未完成 |
| OCW-A03 | 同进程 FIFO/取消、原命名互斥、owner 无重入死锁 | 组件/定向受控通过；完整回归因 Defender 拦截未完成 |
| OCW-A04 | 原身份/预算跨重启、自有 revision 和旧 clone/停止竞争正确 | 组件/定向受控通过；完整回归因 Defender 拦截未完成 |
| OCW-A05 | capability/schema/同快照/事件及清除一致 | 组件/定向受控通过；完整回归因 Defender 拦截未完成 |
| OCW-A06 | 原写操作期限、unknown 和资源义务不变，stop 后零新输入 | 组件/定向受控通过；完整回归因 Defender 拦截未完成 |
| OCW-A07 | 独立 local / remote 竞争/恢复/取消和清理 | 未执行；local/remote 均未取得原生资格 |

按 Plan 执行，输出 `C:/SteerlineDebug/runner-cli-wait-retry/<唯一执行ID>/`。记录 source/dirty、EXE SHA/version/resource、原命令、选中 ID、Run/operation、结果和清理；失败不覆盖。禁止向 SteerLine 安装 home、共享 state-dir 或 Task 18 注入测试。不安排人工签收，本仓完成不依赖调用方。

## 文档验证记录（实施前设计轮次）

首轮原稿曾通过两仓本地链接及空白/示例一致性检查，仅适用于原稿。审计修订在提交前重新检查并登记，不继承原稿功能资格。全部功能项仍待验收。

审计修订静态检查：两仓 8 份新增/修改文档、225 个本地链接、代码围栏/UTF-8/尾随空白和最小公开字段/枚举/JSON 示例检查通过；SteerLine 全库链接检查 576 份 Markdown、3605 个本地链接，0 错误；两仓 diff 空白检查通过。修改仅限文档，未运行产品 L1/Controlled/原生测试。推送前另核对暂存文件仅为本 Spec/Plan、专属审计验收记录与文档索引。

## 本轮实施与执行记录

在 SerializeCLI 增加取消安全 FIFO；只给两项准备读取拆开排队和执行期限，统一 90 秒、5 次及 1/2/4/8 秒退避，旧缓存冲突循环已删除。等待预算在原 operation.Intent 中持久化，沿 ReadOnlyPreparation 释放/重取原 gate，同一 Run revision 与事件事务提交。成功只清除公开投影；原预算保持到操作结算。现有创建目录始终优先，缺计划保持原未知结果；重启继续读取还须原绑定和授权相同。

公开 capabilities/schema、inspect/observe/history 的完整 RunView 以及 execution_wait_changed 均已接通；原状态、controls、操作与资源事实继续权威。查询清除或迟到取消回执不能覆盖已受理 stop 的状态/诊断，stop 与清除同事务。

所有本轮输出位于 `C:/SteerlineDebug/runner-cli-wait-retry/`。`controlled-01` 的 TestSROrcaCLIWaitRetry 五变体通过（86.06 秒，scenario_count=1）：跨进程排队 >15 秒后恢复、正常停服重启、强杀重启、排队 stop、暂态 backoff。后续增加公开 observe 证据保存、契约预检及 stop 优先断言后，以最终完整批次复验结果为准。

初始组件全包失败仅为 TestWorkspaceCreationUsesConfiguredTimeout 的旧断言仍要求 status/repo list 使用 40 秒；已按 Spec 改为两项读取 15 秒、创建保持配置期限，并定向复验通过。`l1-01` 在框架阶段发现 native-result 资源核验访问可选 driver 属性触发 StrictMode；已用原可选字段读取函数修正，原结果保留。`availability-01` 因未显式提供构建好的 Runner/config helper 拒绝查询，没有运行原生场景；`availability-02` 使用新构建与本轮独立配置，四项选择均 available，仅表示静态资格。

配置来源为本仓 `docs/acceptance/evidence/runtime-progress-20261023/r16/frozen-config.json` 的非秘密归档副本；本轮另构建 agent helper 并更新绝对路径/SHA 到 `test-config.json`，没有部署。配置冻结 local-codex、orca-local-codex、orca-remote-codex 与独立 SteerlineTest fixture；原生 driver 沿框架新建 C:/srt 现场，使用独占测试 lease，故障 key 私有。

完整命令、最终构建摘要、L1/Controlled/L2/local/remote 结果与清理在下方最终登记中更新，不把静态 available、编译或历史受控通过记为原生通过。


### 补充复现、隔离修正与批次保留

[原分支隔离复现](evidence/orca-cli-wait-retry-20261025/baseline-reproduction.log)来自指定 origin 分支的源码归档，仅添加[测试驱动](evidence/orca-cli-wait-retry-20261025/baseline-fixture.go.txt)：实际 CLI 调用 0 次，在 15.0008838 秒结束，并附带 unexpected end of JSON input。仅持测试私有 cache key，不调用真实 Orca。

后续整组 L1 发现 TestLocalIPCQueryCapabilityBeforeCreation 的假 CLI 夹具首次接入共享门后未隔离 cache key，3 秒测试预算被其它用户 cache 调用占用；已为其分配独立 key，定向复验通过。旧 L1 批次保留失败，没有进入原生派发。原生转发器在本 fixture 的私有故障门之外仍取得原用户 CLI cache 的原互斥，防止测试绕过真实共享入口。

final-02 / final-03 的完整 L1 已通过，但各自顶层原生调度在最终边界修正后被替代。其原 summary 字节保持不变（running 不是通过）；独立 interruption.json 记录准确 coordinator/controlled PID、时点和未派发原生的证明。只结束旧调度器，已脱离的 Controlled 子测试继续运行并正常清理。final-04 的顶层调度同样在原生派发前被替代；最终冻结构建 final-05 从正式入口复验。其 L1 通过，Controlled 随后被系统安全拦截，原生资格没有取得。

最终边界还包含：stop 受理后的迟到读取回执不覆盖控制状态；resume 清除 exhausted 投影但保留预算，成功的新查询不复活历史等待；缺独立 owner/持久计划的旧直接传输即使有只读续行标记也不能重发未知创建。对应明确反例均已定向通过，完整回归以最终批次为准。


## 最终冻结构建与当前结论

最终测试构建的 Source 为 `d95bbc1ba207033e8fed7fb03d351d6e5190ed0d-dirty`（测试时实现尚未提交），Runner SHA256 为 `f83d475fb77c9848e9ed8aec9694bafbd54b30da1b407bec005e40050a7a442a5`，resource digest 为 `5909e7a962f5b50fd13ae4f4c93a445cad4d157b60d569a57fe7a0ff5ab4c166`。本轮本机 agent helper SHA256 为 `24088810d25abbaac9bc366c2228b863175e54de1e886a138a907f5e6a96c082`。[构建记录](evidence/orca-cli-wait-retry-20261025/final-build.json)、[755 份实现/脚本/测试文件清单](evidence/orca-cli-wait-retry-20261025/source-files.json)及[结束复核零变化](evidence/orca-cli-wait-retry-20261025/source-immutability.json)固定实际源码范围。

| 验证层 / 目标 | 实际结果 | 证据与限制 |
| --- | --- | --- |
| L1 正式入口 | 通过：673 个顶层测试、29 个终端扩展测试、22 个框架检查 | [最终批次](evidence/orca-cli-wait-retry-20261025/final-summary.json)、[包结果](evidence/orca-cli-wait-retry-20261025/l1-package-results.json)、[新增断言](evidence/orca-cli-wait-retry-20261025/l1-selected.jsonl)；原目录允许的 helper/现场 opt-in 跳过单列在 summary，无新增场景 skip |
| TestSROrcaCLIWaitRetry 定向 Controlled | 同一 Runner SHA 的 6 个变体全部通过，100.40 秒、scenario_count=1 | [结果](evidence/orca-cli-wait-retry-20261025/directed-result.json)、[原日志](evidence/orca-cli-wait-retry-20261025/directed.log)、[构建身份](evidence/orca-cli-wait-retry-20261025/directed-source.json)；包含服务启动前撤销排队授权及原预算/0 调用/零创建核验 |
| 完整 Controlled（24 个顶层） | 失败：执行中遭 Defender 阻断/隔离 EXE，不能声明整体通过 | [原结果](evidence/orca-cli-wait-retry-20261025/controlled-full-result.json)、[原日志](evidence/orca-cli-wait-retry-20261025/controlled-full.log)；不是将后续启动失败归类为产品通过 |
| L2 核心集合 | 未执行；两个 case 保持 queued | local-codex.smoke / local-codex.result-acquisition，完整前置未通过；不以旧构建资格替代 |
| Orca local | 未执行，未取得本机构建资格 | orca-local.cli-wait-retry/default，前置阻断，未生成替代现场 |
| Orca remote | 未执行，未取得远端构建资格 | orca-remote.cli-wait-retry/default，前置阻断；不以本机或组件测试替代 |

最终原命令（正式入口自动运行完整 L1 和 Controlled）：

```powershell
./scripts/test.ps1 -Level Controlled,L2,L3 -Scenario local-codex.smoke,local-codex.result-acquisition,orca-local.cli-wait-retry,orca-remote.cli-wait-retry -Config C:/SteerlineDebug/runner-cli-wait-retry/test-config.json -MaxParallel 1 -OutDir C:/SteerlineDebug/runner-cli-wait-retry/final-05
./scripts/test-foundation.ps1 -Pattern '^TestSROrcaCLIWaitRetry$' -OutDir C:/SteerlineDebug/runner-cli-wait-retry/controlled-normalized-final
```

### 已定位的恢复反例

完整早期 Controlled 发现 implementation 空集合在 SQLite/clone 往返后由 `[]` 变成 `null`，未经归一化的绑定摘要因此误拒绝原查询。现在比较两侧原快照的 canonical clone；真实 target 变化、已有 receipt、已知工作区或撤销授权继续保留原检查。查询无资源且没有 owner 声明目录时，只在原只读事实、原绑定、无回执与独立 owner 规则均成立的情况下继续；撤销后仅结算“尚未创建”，不查询、不创建。无独立 owner 的直接传输保持未知结果核验，不接受此续行证明。对应单位反例及六变体定向复验均已通过。

### Windows 安全阻断与收尾

2026-10-25 **08:45（Asia/Shanghai）**，Defender 将 `final-05/controlled/steerrunner.exe` 识别为 `Trojan:Win32/Bearfoos.A!ml`（ThreatID 2147731250），随后隔离该文件。测试原日志记录系统拒绝启动和文件随后不存在；[检测记录](evidence/orca-cli-wait-retry-20261025/defender-detections.json)与[威胁名称/状态](evidence/orca-cli-wait-retry-20261025/defender-threats.json)来自本机只读查询。该告警的真伪尚未由人工核查，不能自行断言误报。

没有关闭防护、添加排除项、恢复/更名受拦截 EXE 或改构建来绕过检测。已停止仍运行的、仅属于本任务输出目录的旧 Controlled harness，并核对该目录下剩余进程为 0；[收尾记录](evidence/orca-cli-wait-retry-20261025/containment-cleanup.json)保留逐进程路径/创建时间。旧失败夹具的数据库和未核实状态保持原样，[历史物理资源审计](evidence/orca-cli-wait-retry-20261025/historical-fixture-cleanup.json)与[原 owner/CLI 进程核验](evidence/orca-cli-wait-retry-20261025/historical-process-audit.json)另列，不把旧未知记录改写成 stopped。

本轮没有启动 L2 模型流程或两项原生 case，没有测试 PR，也没有部署或恢复生产 Run。须先由用户核查并处理安全告警，再用新的 OutDir 重跑正式完整入口。**OCW-A07、完整 Controlled 与 L2 资格仍未完成，当前不能宣称 Spec 全部验收通过。**

## 实现后单次审计（2026-10-25）

按用户后续要求完成一次实现审计，覆盖 FIFO/取消、两项读取的分类和预算、原操作恢复与未知创建边界、stop 撤销、自有 revision 与当前聚合合并，以及公开 schema/事件/快照和原生测试隔离。未发现需要新增修复的确定问题；本次审计未修改实现或测试代码。

逐文件复核冻结清单中 755 份输入，变化为 0。`go vet ./internal/... ./tests/helpers/stability` 和 `go vet -tags live ./tests/native` 均以退出码 0 完成，`git diff --check` 通过。审计结果见[机器记录](evidence/orca-cli-wait-retry-20261025/implementation-audit.json)。这些静态检查不替代尚缺的完整 Controlled、L2 和 local/remote 原生资格。

安全告警仍保留，未执行受拦截 Runner 或绕过防护。后续按用户明确指示创建并合并实现 PR、将主工作区 rebase 到最新；代码合并与完整功能验收分别登记，本记录继续保留上述未完成项。
