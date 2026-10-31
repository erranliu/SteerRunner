# PR #73 再审计与普通本机查询接线修正

2026-10-31。审计起点 `597d669d8f10b8ced46946d7b8f9a010f856f0f8`，[PR #73](https://github.com/erranliu/SteerRunner/pull/73)。配套[当前 Spec](../spec/orca-native-submit.spec.md)、[Plan](../spec/orca-native-submit.plan.md)。

本轮确认并修复两项P2：一项普通本机查询接线回归，一项模型凭据范围表述过宽。冻结源 `c21d915a6d5ce1d03e7b437b94048614781052312279076ac2cb678f2c3ad894`，正式稳定矩阵7170/0/0，原查询回归450/0/0。完整L1通过（722顶层/2735含子例、0失败，17明确opt-in/helper跳过与1无测试包另列；34包、24框架、29扩展、native编译与零匹配门禁通过），新源本机managed3/3、远端3/3已通过；最终独立补充复核通过，无未修P0–P2，按本报告边界放行，不平移[前次六场](orca-native-submit-integrity.md)为本次通过。

## 审计发现及最小修正

收紧原生Codex Started适用于所有普通原生回合，但生产NewLifecycle只在Environment非空时设置queryInput。既有development/2允许非GitHub、无managed helper的本机Codex；其WireFormat为空，观察进入observeNative而非observeRecorded。没有独立原输入查询，即使原生任务已完成也无法证明业务开始/完成。

以前部分完成/进度单测以已开始为前提，预置Started；另一些原输入回归手工接入queryInput。它们验证各自算法，无法替代生产构造器的接线覆盖。新增回归直接用NewLifecycle，既不注入Started，也不注入queryInput/queryNative，实际readSession计数揭示遗漏。

生产修正仅取消queryInput闭包的远端if：同一个QueryOriginalInput继续按Environment选择目标。本机managed回合的observeRecorded、远端原有查询、Pi/DSH/其它agent的分流与Codex补证条件均不变。没有新API、runtime、凭据、状态字段、重试、回退或完成算法，也未恢复send阶段授权。

复用已有thin输入IPC夹具，仅使terminal.show返回与原worktree id对应的路径，并返回实际原输入查询计数；未引入第二套夹具或测试模式。测试保持本机publicCandidate=nil的原契约，正式final通过TurnEvidence核验。

模型凭据表述也已纠正：正式turn_context的实际model/effort仅由本机managed核验；远端仅冻结启动偏好，没有独立核验实际model/effort，远端audited工作流本来就不支持。此前“六场模型/effort均核验”的泛称超出实际证据，不能沿用；不为填补文字承诺新增API、模型框架或旁路。

## 回归与旧失败

- 最终旧生产overlay红对照：四个叶例全部失败，原输入查询次数均为0，退出码1。原生产文件取自597d669，不修改Orca或安装源码。
- 新构造器thin/corroborated/duplicate/altered四例，每叶30次。包含中文、空行、CRLF及超过预览的长原文；查询1次、业务send1次，有效输入恢复Started及正式final，重复/改文不成功归属，0虚构streamed working/helper结果。
- 连同原thin/corroborated回归，3根15叶各30次＝450通过、0失败、0跳过。
- 原正式`test-native-input.ps1 controlled -Repeat 30`：25必需根、239叶、7170执行/7170通过/0失败/0跳过、缺失根0。未创建新执行框架或L3模式。
- 完整项目L1使用原入口、默认Go并行和原12分钟单包时限，正式passed、退出码0；源码冻结后验通过，不跳过失败。

开发期初版回归曾错误要求本机提供远端publicCandidate；当时正式Started/final已经成立，但展示层断言失败。查明原本机契约后纠正测试，不改生产展示行为。相关开发日志保留，不计资格通过。原历史输入损坏、单项候选失败、旧L1超时均保持原结论。

## 收敛与其他功能

逐项复核整个PR的生产diff，而非只看末次提交：屏幕草稿全文/标签匹配、显示换行推断、双稳定、专用paste/Enter和fit/restore路径及状态/测试已删除。生产搜索无相关符号或诊断探针残留；共享read、WaitTimeout、既有模型/原输入/final/认证/stop仍保留。

通用发送、统一LF→CR传输编译、逻辑wire/hash/预算、原请求及runtime/terminal/incarnation/session绑定不改。模型桥仍为128MiB上游/2MiB下游，原RPC结构化错误零转发，认证与单活动连接保持。控制读取仍区分严格READY与有限快照内持续分页，不增加持久cursor或恢复发送。

活动Spec/Plan已去除多轮历史增量叠加，集中描述当前单一契约、已删/保留项、五种凭据、NS-T01–T15和正式入口；历史阶段保留在原验收与原字节中，不改写历史失败。

## 新源真实验证与边界

本轮隔离构建现有Runner agent，L1后按原正式入口依次完成六场全新隔离现场：本机当前runtime `5bb5b0a8-9225-4553-b898-f751d0fd16d6`（Orca1.4.242）；192.168.1.151远端runtime `9832f197-1f7a-4c76-b3e7-074fd12b30f0`（Orca1.4.245）。启动前以status核实当前runtime，不套用过期模板；同一运行中的runtime可以承载新的独立会话，不向历史Run重投。

六场均有新工作区/终端/incarnation/session/Run/request，一次业务submit、0text-only/补Enter/业务屏幕读取；核验accepted、正式started/completed、完整原输入/final及原范围stop/完整原host空清单；本机另核验实际模型/effort，远端仅冻结启动偏好。异步rm不能当同步删除；后续not-found和本机路径缺席分别核验，远端物理路径未经独立核验不得宣称消失。

既有正式本机目标是GitHub/managed；它只能证明该路径的新源资格。普通无helper分支的生产构造器＋隔离IPC回归不等于该分支真实Orca资格，本轮未提供非GitHub的正式无helper目标，也不改变GitHub仓库URL、认证或业务材料绕过条件。

仍未证明全文原子消费或任意人工写者排他，事后完整原文核验不能阻止错误文本已经执行。历史dirty源码未恢复，不宣称完整复现历史故障。未部署安装、修改Orca/全局配置、运行Unity、创建业务PR、向旧Run重发或补Enter。

隔离agent SHA256为`E6F2891D8B2B9588ACD773812B15AC131F1B41D52D47B987CD641E1F78A90BE9`；六场driver均为`3085693FFED1F4672959B7CA21273F27C86DE6A2900E188B8E779EFC16C0391F`。构建时HEAD597d669、dirty=true，L1、矩阵和六场均为同一C21冻结源，不冒称在后续干净commit上构建。

六场原字节归档、源/产物摘要、完整测试事件和离线核验器见[证据目录](evidence/orca-native-submit-reaudit-20261031/README.md)；仅抽取本批本机三条原session历史，不上传无关历史、EXE或安装包私有源码。

| 目标 | Run | 原request | provider session | WireFormat/helper |
| --- | --- | --- | --- | --- |
| local | `2b402298625997216d17c9424e19d5d2` | `8926f407-36bb-4565-6a78-511a884cc70b` | `01a19004-e873-7150-a8cd-c5f4d5dc2f76` | native-model-command-v1 / true |
| local | `a1d8432cfe28049d1e0bf5aa01499a8a` | `43a5dad7-54fc-630f-9d0a-c46b9a7d1b48` | `01a19006-adad-79f3-b6c2-ad1c1e91720e` | native-model-command-v1 / true |
| local | `1404b7205966726652e21bb922e67462` | `5e0b73af-60c9-3d8f-4706-2f592c9676ed` | `01a19008-3447-7c83-81b7-d644768bbd13` | native-model-command-v1 / true |
| remote | `a934bc9eeaa25416ae5f2be9b828de09` | `500dabaa-b6dd-6999-c254-b80c4286ee2b` | `01a19009-c2c0-77d0-b4c0-89ba77eaa75c` | 普通native / false |
| remote | `61a80f88436dab538d19913ae381f6bd` | `f931c135-2f34-6af4-ca90-f1404cdbc574` | `01a1900c-4223-7423-9f34-3320371e5063` | 普通native / false |
| remote | `eaefc78516e0a7b7c4027c8d126acb21` | `7864f465-c464-1d48-cc40-4fffbe60c271` | `01a1900e-ba0c-78f2-b1bc-db2999574823` | 普通native / false |

## 独立复核

final_review已独立只读审计Spec/Plan、最小生产diff和红绿回归：无新增P0–P2，重计红对照4失败/query0、原查询450/0/0、正式7170/0/0，源码摘要与冻结值一致。最终L1、原字节归档和六场新真实现场已完成，最后补充复核通过，无未修P0–P2，按本报告边界放行。
