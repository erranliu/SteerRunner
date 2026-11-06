# Orca 停止合同解释收口：A 独立验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-06，合并前定向验证。依据 [Spec](../spec/orca-stop-contract-boundary.spec.md)/[Plan](../spec/orca-stop-contract-boundary.plan.md)，实施源树已从 main `6dd4360a42314e65b427627508ffd2ee3e4287da` 建立，包含 #123/#125。A 验收及源码固定后才进入 B；本记录不使用 B 的结果追认 A。

## 变化与保留

新增 adapter 三个纯查询，承接原 protocol/profile 标记、后端冻结 scope 要求和成功 scope 诊断。application 保留 Workflow 组合、已存 scope 非空/相等约束，以及 stopped/具体 Orca 端口/端口 scope 非空的原诊断 guard。未新增 decoder、执行构造、I/O、缓存或持久字段。freeze 合法写入、Engine.Handle 顺序、两处 stop 结算、意图/结果合并和事务边界保持。

全生产 AST 对照基线：新增 3 个函数，修改 stopRun（仅成功诊断选择）、structuredDevelopmentClose、requiresFrozenOrcaScope，原 2,288 个声明相同，无类型删除或修改；其余原编码/证明/停止实现保持。[声明差异](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/a-declaration-diff.json)。这不是全面删除 scope 分支或具体 adapter 接线，收益为三个私有解释的所有权归回 adapter，不宣称运行提速。

## 独立编码与验证

原代码先经临时 Go overlay 捕获合成旧 Profile/State/NativeProof、Record、操作封套及结构化状态/RuntimeFence 的实际编码；A 用相同捕获比较。15,528 字节完全一致，SHA256=`06034f8e8ccba2233eac1518a29ac706ae48f6273993da21cc6a69f233f16d62`。不是仅新 encoder 自我 roundtrip；[原字节](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/baseline-format.json)和[捕获入口](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/capture_test.go.txt)保留。全源码、既有测试/工具基线及 A 源码归档保存在本工作副本 `.evidence/orca-stop-reader/`。

| 范围 | 实际结果 |
| --- | --- |
| 完整 orcatransport 包 | 78 个顶层通过，0 失败/skip |
| application 定向 Stop/Stopped/Scope/ResultSnapshot/Structured/DevelopmentRunMessage/Restart/Budget/SingleExecution/Seal | 75 个顶层通过，0 失败/skip |
| host/storage/contracts/usage 完整四包 | 69 个顶层通过，0 失败/skip |
| live native driver 与共享 helpers | 编译通过，没有运行真实模型 |
| 同最终 EXE 的公开受控 | SingleExecutionColdStop、NativeUnknownAndStop、RestartPreservesAuthorization 三项通过；重启含 graceful/kill/creation_receipt/pending_stop |

组件命令：`go test ./internal/execution/adapters/orcatransport -count=1 -timeout=5m -json`；application 使用上述明确正则及 8 分钟包时限；四组件包使用 `-count=1 -timeout=5m -json`。正式入口为 `scripts/build.ps1 -OutFile .evidence/orca-stop-reader/a/steerrunner.exe`，再经 `scripts/test-foundation.ps1 -RunnerExe <同文件> -Pattern '^(TestSRSingleExecutionColdStop|TestSRNativeUnknownAndStop|TestSRRestartPreservesAuthorization)$' -OutDir .evidence/orca-stop-reader/a-controlled`。

新增反例覆盖最小/坏 payload、重复/混合标记、旧版本/协议及 scope 分类，保持分类与原严格证明入口的区别。实际 Engine 的原资源尚未创建分支确认停止后撤销 resume，stop/resume 在 stale revision 之前仍返回原 invalid_state，且无新增 stop 意图。既有最小 `{}` 的确认停止测试、旧 close 未知/成功冷读、scope/停止及 #125 消息/答案窗口回归保留。新预创建测试首跑漏填持久封套身份失败；仅补齐新测试封套，后续定向和最终应用组通过，未放宽产品校验。

[测试清单/格式结果](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/a-summary.json)，[正式构建](../../../../acceptance/evidence/orca-stop-reader-boundaries-20261106/a-build.json)：source_digest=`6cc58945df5e7a49d0743cf089ac1002bf448aac6fe3f18ba1b0fe28b1832884`，EXE SHA256=`8693d6a7497093539f3dcdfee9048aa0c456a05b3348184fc819e0e287d8fe9b`，source_dirty=true。进入 B 前的 846 个源码/测试/工具文件摘要清单 SHA256=`780bd150b94c0e5bb299c134ceefeb199b7d2f589529d9338c611cdc0eb95bb0`。

## 资格与交付边界

OSC-A01–A05 的最小验收通过，A 实施点已独立保存。持久格式及执行、恢复、停止承诺不变。公开受控主要使用协议替身，不代表 Orca 真机关闭资格；本批未运行完整 L1/Controlled、完整 L2/L3、真实模型或合并后回归，未提交、推送、提 PR、合并或部署。稍早进度消息中的应用数量为中间统计，最终日志为上表 75 项。

同日用户追加审计后提 PR；[实现复审](orca-stop-reader-reaudit.md)未确认生产缺陷，修复输入不变断言的 Payload 浅拷贝，补实际在线/离线 scope 与错误优先级回归。初轮 A 数据保持，最终复验和提交资格以复审记录为准。
