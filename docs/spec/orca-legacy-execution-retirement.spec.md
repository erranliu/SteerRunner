# Spec B：旧 Orca 执行支线退役

日期：2026-11-05。状态：设计审计后已实施，最终验证中。基线 main `6940c598`，在 [A](orca-frozen-reader.spec.md) 独立验证后实施；配套 [Plan](orca-legacy-execution-retirement.plan.md)。长期新 Run/旧记录边界见[Orca 设计](../orca-execution.md)及[已实施 helper 退役](orca-helper-retirement.spec.md)。

## 合法入口与删除范围

contracts 准入拒绝旧 Agent executable/args/helper；freeze 只生产结构化协议；host 启动拒绝旧协议库，application.port 再拒绝旧协议。旧写链没有当前 Runner 的正式生产执行入口。NewLifecycleWithStateFactory 及原生 probe 中仍有真实测试消费者，必须区分旧行为专属测试与当前/shared fixture，不能宣称零消费者或整包删除。

B 将当前 NewLifecycle 收窄为结构化执行；旧记录通过 A 的 reader 保留原严格解码/证明。删除 Lifecycle.PreparationFacts/Execute/ObserveLifecycle/Reconcile/StopLifecycle/ShutdownLifecycle/VerifyManualRecovery 的旧 terminal/helper 分支，以及只被这些分支使用的 launchAgent、prepareControl、prepareInput、TUI submit/readiness/observer、输入查询、helper 工作区审计/控制派发等函数。实施前生成逐函数消费者清单，确认没有 reader、usage、files、公共终端或结构化 workspace 的消费者才删除；未能确认的函数保留并登记原因，不借本批退役旧持久格式。

旧 prepareWorkspace 的公共创建/原 receipt 对账仍被 executeStructured 调用，必须保留；只可删结构化分流后专属的 Agent terminal 选择/启动尾段及其独占 helper。保留 creation owner、workspace creation/checkpoint 格式、默认辅助 shell 及最佳努力清理，不把辅助 shell 当 Agent owner。

## 保留项及共享夹具

- A 的全部旧状态/配置严格解码、原资源只读停止证明、原 scope 错误；旧库仍拒绝启动，离线 inspect/verify-stopped 不激活旧执行。
- 当前 Codex/Pi 原子首消息、原 session 续轮、完整结果/实际模型/审计工作区、原操作对账、close 一次及原回执复用；权限、预算、恢复及停止撤销保持。
- usage 原输入/旧 wire 绑定、progress/result worktree 读写、public terminal/client、直接 Codex owner/parser 和共享协议工具保持。
- fakeHost、fixture/fixtureOn、request/bind、structuredFixture/agentFixture、creation fixture 以及被 reader/结构化/usage/files 使用的类型/方法保持。必要时只调整夹具构造，使旧记录夹具不再通过已退役执行工厂；不引入生产 simulation 开关或保留仅供旧测试运行的第二执行器。

仅旧执行专属测试/opt-in 探针随相应能力退役，逐项登记；保留原共享夹具、历史证据及当前机制的反例断言。不能通过跳过当前 case、放松断言或伪造新 API 接口返回换取通过。混合文件按函数处理，当前/reader 测试保持。

入口审计补充：`scripts/test-native-input.ps1` 的 long-input/controlled/capability-baseline 都直接运行本批退役的旧 TUI writer/probe；从 focused native 目录注销 `orca-remote.native-input`，脚本保留原参数并在任何构建、资源或证据目录创建前明确拒绝，指向原构建。原生旧 writer 的 skip 白名单同步注销；其他共享模型桥测试、当前结构化场景及通用 Controlled 不变。原终端 `original`/`boundTerminal` 只读方法及并发回归保留，不为删除 writer 而删除这组读取能力。

## 格式、收益及最小验收

全部持久类型/字段/tag/版本、保存旧 action 的解码、原 Request/Receipt 和原历史资源绑定保持；不迁移、不回填、不按新 profile 解释旧 Run。允许删除无消费者的进程内字段及旧写函数，不能删除仍参与编码或读取的声明。能力声明不新增组合，PC-R01–R06/R08/R09 不变。

| 编号 | 验收 |
| --- | --- |
| B01 | 当前工厂与各执行入口不再可达旧 Agent launch/输入/观察/控制写链；旧协议确定拒绝，不存在错误后切旧链或测试专用生产开关 |
| B02 | 当前工作区创建/丢回执对账、原子输入/续轮/模型/完整结果、停止/重启/原预算回归；shared fixture 使用者完整 |
| B03 | 旧记录 scope/停止证明、usage/files/inspect 及错误与 A 基线一致；保存的 unknown 不重发/不重建，旧库启动仍拒绝 |
| B04 | 删除函数、退役 case、保留依赖和编码摘要逐项可审；正式构建及受影响公开受控，失败/skip 与资格分开 |

实际收益是当前执行不再与旧 TUI/helper 状态机共用派发分支和控制资源，不以删除行数替代边界验收。停止合同私有版本解释进一步收口（上轮建议 C）不在本批。
