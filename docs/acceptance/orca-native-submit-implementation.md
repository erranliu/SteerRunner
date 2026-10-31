# Orca 多行输入提交实施记录

2026-10-30 初始实施批次的历史记录。配套[Spec](../spec/orca-native-submit.spec.md)、[Plan](../spec/orca-native-submit.plan.md)。
当前原提交修正已取得[本机 3/3](orca-native-submit-local-final.md)、[远端 3/3](orca-native-submit-remote-final.md)
及[远端单次重跑通过](orca-native-submit-remote-rerun.md)。下文的 draft、0/3 与阻断均属于各历史批次，
不表示当前资格，也不把原失败改判为通过。2026-10-31 持续控制读取的复审修正与检查另行登记。

后续本机适配、隔离 agent 构建和真实准备诊断见[本机补验记录](orca-native-submit-local.md)。当时本机缺测试适配的问题已补齐，阻断为原控制回执默认读取发生屏幕折行。本机当时资格为 0/3，不将准备失败改为通过。

随后授权增量见[本机原始流补验](orca-native-submit-local-stream.md)：两处准备读取已修正，真实 control READY 校验通过；该批现场被既有认证 socket 的 Windows 1920 错误阻断，无业务提交、资格 0/3。前述状态按各批次保留，不覆盖原始证据。

最终本机补验为 **3/3 通过**；根因、环境恢复、测试目标门禁与全部原始凭据见[最终记录](orca-native-submit-local-final.md)。本机批次结束时远端尚未验证、PR 当时为 Draft；后续状态见页首。下文数字仍属于此前实施批次。

## 实现与保证

删除Runner原submitNativePaste分支，使普通远端Codex多行输入直接进入原terminal send的text+enter+原request路径。删除草稿全文/折叠标签/显示LF判断、双稳定、专用轮询期限、viewport fit/restore、专用fixture与probe手动分段模式。输入规范化、冻结wire/摘要和原runtime/terminal/incarnation/session绑定保留。原受影响输入NUL/ESC拒收及观察feed清理保留。

原生Codex受理回执补验provider=codex、observation=supported及阶段一致性；unsupported、旧host、缺字段、身份变化和未知不作可靠成功。沿原journal/Attempted拒绝重派，未增加request-show恢复、预算、固定sleep、状态机、codec或回退。受理不等于开始或完成。

可靠提交的职责交给Orca既有通用prompt能力；Runner交付既有编译后的完整正文，不再用显示投影控制Enter。这个接口没有独立的原子全文消费/任意人工写者排他证明，也可能内部raw/held-Enter；正式原文核对防止错误归属，不能撤销已经执行的错误文字。删除空草稿屏幕检查后，不把idle当空草稿证明。

生产变更仅lifecycle.go及删除native_paste.go、terminal_viewport.go；local_codex_history、remote_models、native_start/native_state、原输入RPC、prepare/auth/stop和codec生产实现不改。本机原model wire/history及32KiB结果能力保留；远端原NativeProof/原输入和新done补证保留。现有远端wait-submit20/CLI50秒上限保持，可能增加受影响多行的共享锁占用，不声称零影响。

## 测试入口与证据层次

正式受控入口：`scripts/test-native-input.ps1 -Mode controlled -Repeat 30 -OutDir <新目录>`。固定12个必需root、每叶例次数及零skip门禁；测试EXE从package目录运行，避免testdata读取假失败。JSONL保留原事件，summary逐叶列实际次数，result记录源码摘要及最终门禁。

| Spec需求 | 实际入口/断言 |
| --- | --- |
| NS-T01/T02 | TestNativePromptSubmission：两目标、6组单/多行/中文/空行/CRLF/长输入；捕获实际正文、原request和编译参数，提交后持久冻结数据及Turn事实；错误屏幕投影不被读取 |
| NS-T03 | 原TestOrcaNativeInputProofRejectsUnboundHistory、TestNativeCRLFPastePreservesOriginalInputAndCompletion和followup：正式原文改字节/多LF/重复、工具/旧结果拒收 |
| NS-T04/T06 | TestNativePromptReceiptFaults及ProcessingOrder：逐段字节（含UTF-8拆分）/消费ACK/处理与回执屏障；处理中取消及处理后丢回执；provider/observation/身份/阶段异常；实际调用和0/1效果、零重投 |
| NS-T05 | TestNativeUnstartedReceiptRecoversOnlyCompletedOriginalInput、CRLF/followup：仅accepted不提前started/completed，沿原新done/独立输入补证 |
| NS-T07/T08 | 原StateBinding、RecoveryRejectsProviderReplacement和ComposerGate；ReceiptFaults从仅持久数据重建adapter后重复submit/Reconcile仍零追加send |
| NS-T09 | adapter TestNativePromptSharedQueueWithStopAndCancellation：三个独立Run/resource身份、多行send持真实共享锁、stop与本机观察排队/取消；TestSROrcaCLIWaitRetry/three-runs-stop-queued另验真实服务三Run准备队列、deadline与撤销零派发 |
| NS-T10 | 既有model/history、original input/final、stop/reconcile随L1回归；未迁移模型或结果来源、未放松阈值 |

两项NS-T09证据各自有效，不能相加冒称同服务三个活动原生Run的多行发送/观察/停止组合已获完整证明。实际宿主正文处理延迟由真实会话验证，不以stub消费ACK代替。

## 实际结果

| 检查 | 实际结果 |
| --- | --- |
| 最终受控故障矩阵 | 116叶例×30＝3480，通过3480、失败0、跳过0；[逐例摘要](evidence/orca-native-submit-implementation-20261030/controlled-final-summary.json)、[最终源码门禁](evidence/orca-native-submit-implementation-20261030/controlled-final-result.json)、[原始事件gzip](evidence/orca-native-submit-implementation-20261030/controlled-final-tests.jsonl.gz) |
| 最终两个适配器整包 | 2包通过；184顶层通过，含子例963通过；0失败，6个明确opt-in探针skip，不算真实资格；[摘要](evidence/orca-native-submit-implementation-20261030/final-adapters-summary.json) |
| 既有真实服务受控回归 | TestSRStabilityLostSubmitResponse、TestSRRestartPreservesAuthorization、TestSRMultipleRunsIsolation、TestSROrcaCLIWaitRetry共4root通过；[结果](evidence/orca-native-submit-implementation-20261030/service-result.json) |
| 最终三Run队列补强 | three-runs-stop-queued子例通过，含服务退出后SQLite三行零CallsStarted/原deadline核验；[结果](evidence/orca-native-submit-implementation-20261030/service-queue-result.json)、[日志gzip](evidence/orca-native-submit-implementation-20261030/service-queue-final.log.gz) |
| 项目L1/框架/原生编译门禁 | 711顶层通过、0失败、17个既有opt-in/helper跳过；框架门禁、29项Node扩展、原生driver编译与零匹配门禁通过；[报告](evidence/orca-native-submit-implementation-20261030/l1-summary.json)、[完整事件gzip](evidence/orca-native-submit-implementation-20261030/l1-tests.jsonl.gz) |

L1运行期间补回了拒收时的feed释放及断言；该最终增量随后由两个适配器整包与最终3480次矩阵复核，其它L1源码未变。不把L1批次冒称含此增量的同一编译快照。

summary.source.passed是写出该摘要时的输入对象快照；最终执行门禁以独立result.json的passed=true、原始事件和顶层exit_code为准，不改写这份原始JSON。

失败批次单列保留：首轮91叶例87通过/4失败/0跳过（testdata工作目录、拒收效果计数及hostScope夹具）；次轮116叶例112通过/4失败/0跳过（旧多行两次send断言）；修正后116/0/0，随后两批30次均3480/0/0，只取最终批次作为当前源码验收。沙箱整包批次失败含IPC不可达，原日志保留，沙箱外最终两包通过；单独队列补强的退出服务后inspect失败为测试错误，改为SQLite核验后通过。[首轮摘要](evidence/orca-native-submit-implementation-20261030/controlled-first-failed-summary.json)、[次轮摘要](evidence/orca-native-submit-implementation-20261030/controlled-second-failed-summary.json)及相应压缩原日志可复核。

## 真实验证与未验边界

本机0/3、远端0/3，均未取得新方案真实会话资格。本次未创建真实模型Run/终端，未向历史Run重发或补Enter，不能提供六个新会话的受理/开始/完成/原文/清理证明，也不以受控通过代替。

远端既有environment两次只读status均返回remote_runtime_unavailable。本机已安装Orca1.4.242启动就绪，但未找到现成有效的固定本机agent/runtime配置；18份历史manifest中11份helper路径指向已删除worktree，其余使用历史转发器或无helper，不能套用。现有focused probe固定remote profile，本机需原ready/control/history真实测试适配与有效既有helper配置，不能删除Environment限制或将RepositoryURL改空来假通过。已向用户询问现成配置路径/远端在线条件；不安装、复制旧备份或修改生产链路绕过。

因此本轮完成的是代码与受控验证，完整真实验收仍未完成；历史原生根因与症状是否消失均未获证。未跑Unity/环境检测业务，不改SteerLine/业务材料，未部署安装或修改用户全局配置。

## 审计与清理

独立生产审查未发现P0–P2；测试审查发现逐段控制、漏选门禁、提交后状态断言和证据层次问题，已修正并复核。最后NUL/ESC拒收feed释放增量已随最终30次矩阵及两包整包复核，两个独立审计者均已解压原事件并核对源码摘要，无新增P0–P2。当前全仓代码/脚本/测试搜索草稿/viewport专用符号无残留；当前testing/test-catalog及focused目录已更新，历史acceptance/evidence保留。

证据[文件摘要清单](evidence/orca-native-submit-implementation-20261030/manifest.json)与[变更源码摘要](evidence/orca-native-submit-implementation-20261030/changed-source-sha256.json)用于独立复核。受控测试目录没有与真实模型目录混合；未创建本次真实Orca终端。只读进程清单确认本次隔离证据目录下的测试EXE均已退出。

2026-10-30 后续远端补验：用户指定现有 192.168.1.151 后，三个全新远端原生会话通过，连同本机已达双端各3/3。仅补测试身份/归档/正式停止及共享既有fixture总期限；生产流程未再改。原失败保持失败，范围及全部证据见[远端最终记录](orca-native-submit-remote-final.md)。
