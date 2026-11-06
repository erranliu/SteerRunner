# Spec A：Orca 停止合同的私有解释收口

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-06。状态：设计审计后已实施，A 独立最小验收通过，见[验收](../acceptance/orca-stop-contract-boundary.md)。调查基线为 main `6dd4360a42314e65b427627508ffd2ee3e4287da`，已包含 #122–#125；设计时工作副本在 `e2e4ee69`，仅通过主线源码差异核对；实施前已从上述主线建立独立分支。依据[长期产品契约](../../../../spec/steerrunner.spec.md#长期产品承诺)、[Orca 长期设计](../../../../orca-execution.md)，配套 [Plan](orca-stop-contract-boundary.plan.md)。A 验收后再进入 [B](orca-reader-fixture-boundary.spec.md)，两项分别固定源码与验收记录。

## 目标与实际消费者

目标是减少业务协调层对 Orca 私有 profile key、codec 及停止诊断的解释。以后调整后端停止合同，由 adapter 修改其私有分类；application 继续决定业务授权、Workflow 和事务，不复制后端版本矩阵。不是减少停止校验，也不以净删行数或未测量提速验收。

| 现行位置 | 实际调用根和合法用途 | 本批处理 |
| --- | --- | --- |
| application/control.go 的 structuredDevelopmentClose | Engine.Handle 的 stopped stop/resume 准入；settleRunStop 的恢复资格，后者由两处 stopRun 结算路径调用 | 私有协议/profile 标记判断交回 adapter；Workflow ID/version 条件仍在 application |
| application/stop_scope.go 的 requiresFrozenOrcaScope | application.port 构造成功后的冻结 scope 比较；离线 VerifyStopped 在 scope 读取成功后的比较 | 后端版本/计算 scope 的解释交回 adapter；已存 scope 非空及两值相等的通用约束留在 application |
| application/control.go 的 stopped 诊断分支 | 原 stop 事务提交前，且原端口为 *orca.Lifecycle、端口 scope 非空时覆盖诊断 | scope 对应诊断文本选择交回 adapter，保持原适用条件 |
| orcatransport/frozen_record.go、agent_session.go | 原 scope 分类、close 适用性、严格状态与原回执核验 | 继续由 adapter 负责；合同分类不代替证明 |
| application/freeze.go | 合法受理时选择 Workflow、模型、目标并写冻结 profile | 保留。这是集中生产接线，不属于本批删除的读取解释 |

不能因“只读”构造执行端口，或引入新的状态机、通用策略接口、注册表和跨调用缓存。使用窄纯查询及现有 adapter 方法即可，不给 adapter 传 Record、Engine、Store 或整个 Workflow。

## 合同分类与证明的边界

拟以两个窄查询承接原谓词中的后端部分，函数名在实施时确定：一项接收 protocol/profile envelopes，报告原开发关闭合同标记；一项接收 profile envelopes/本次计算的 scope，报告原后端是否要求冻结 scope。停止成功诊断使用现有 Orca 端口的窄方法或同等静态函数，不扩展通用 LifecyclePort。

必须保持以下原判断，不能借搬迁改变输入集合：

1. 开发关闭合同分类：原 protocol 为 orca_agent_session_v1，且任一 envelope 的 key 为 AgentSessionProfileKey、codec 为 AgentSessionCloseProfileVersion 时，后端部分为真。application 再按原 development/structuredAudited 及 Workflow version=2 条件组合。execute/1 不因此获得开发停止后的准入行为。
2. 冻结 scope 要求：已保存 StopEvidenceScope 非空，或本次计算 scope 为 LocalCodexStopScope，或任一 envelope 为 ProfileKey/NativeProfileVersion，仍要求已存 scope 与本次计算 scope 相等。非本批引入的其它兼容分支保持。
3. 诊断仅在原 stopped 分支、原具体 Orca 端口及端口 scope 非空时选择；仍以记录已存 scope 区分 local Codex、agent session 和默认 native 诊断。不能顺便改用另一来源、覆盖失败诊断，或给测试替身/非 Orca 端口新增覆盖行为。

上述是既有标记分类，不是新的严格配置验证，更不是 stopped 证明。原谓词不解析 payload、不验证封套摘要，也不因重复标记新增错误；搬迁后保持这一事实。缺标记、其它 key/codec、混合或不完整 envelope 的分类与基线一致。不得在 Engine.Handle/settleRunStop 中调用完整 decoder、查询主机或读取文件，从而新增 stop 的失败路径或持锁 I/O。

严格 decoder、scope 读取和原停止证明继续在原合法入口执行。application.port 保持构造错误优先于 scope 比较；离线 VerifyStopped 只有原 scope 查询成功才比较，原错误不得降级成空 scope/false 后继续成功。不能用相同 scope 字符串把 profile v1/v2 统一为相同关闭资格，也不能将“没有要求”当成“已有停止证明”。

## 删除、保留与提交边界

删除 application 读取侧对上述私有 key/codec 的扫描以及 scope→成功诊断文本的重复解释。业务组合函数可以保留，但只消费 adapter 事实；不以函数改名或 import 数量代替职责变化。

保留停止受理事务、原 request_id 回执与发布时机、revision/CAS、派发锁、外发前意图、原操作/结果合并和所有停止提交点。保留 control.go 的原预算、原 Snapshot.Operations 补入条件及其它 scope 分支，本批不把它们扩张成全量停止策略重构。保留在线/离线原资源核验、原 receipt 严格绑定、ConfirmedTerminalStopReceipt、unknown/unsupported 和原诊断/错误顺序。

共享测试 result_snapshot_test.go 使用合法封套与最小 `{}` payload 验证确认停止后撤销 resume；它不是完整 adapter 配置测试，不能为新解码要求而删掉或改弱。orca_structured_test.go、shared_native_stop_test.go 以及同构建重启、结果封存、先 stop 后迟到结果测试保留原断言，可新增边界用例。

#125 的开发 pr_ready 后 message/history、原会话问答、消息身份/RuntimeFence、原工作流及 PR 保持；stop 在原受理事务撤销尚未发送输入，已提交的 unknown 原输入继续只读对账。A 不把“开发阶段完成”改成“会话永久关闭”，也不恢复已停止会话的输入资格。直接 Codex、公共 terminal、parser 不变；旧 Orca 执行仍由原构建排空。

## 持久格式和可靠性

相对调查基线，state/v8、user_version=8、Request/Record/Operation/Receipt、Profile/State/Turn/NativeProof、JSON tags/字段顺序、codec、CLI/能力/Workflow/结果文件合同不变。包括 #125 已有 RuntimeFence 等字段，不回退至旧工作副本格式。不新增持久合同描述符、回填、迁移或重写历史；相同原输入/事实的代表编码和摘要保持。瞬态查询返回值不进入持久模型。

PC-R01 原受理/提交、PC-R02 原身份及未知零重发、PC-R03 原归属、PC-R04–05 同构建原资源恢复/有界对账、PC-R06 撤销与冻结 scope、PC-R08 观测容错、PC-R09 精确资源、PC-R10 原构建排空均保持。原 close 成功或原创建未派发才能结算会话；关闭丢响应仍未知、不重发 close，不宣称 WritersAbsent 或主机级清场。缺预算仍只阻止新派发，不阻止原结果、只读对账或停止。

## 最小验收及范围停止条件

| ID | 必须取得的证据 |
| --- | --- |
| OSC-A01 | 全仓调用/夹具清单和基线固定；application 读取侧不再解释上述私有 key/codec，Workflow/事务仍在原层；分类与诊断无 I/O、无执行构造/缓存 |
| OSC-A02 | 对基线原谓词作独立对照：本机/远端、Codex/Pi、execute/development/audited、v1/v2/旧 native、空/缺失/重复/混合/坏 payload/其它版本、已存 scope 空/相等/不一致；分类、首错及原诊断适用条件一致 |
| OSC-A03 | 实际 Engine.Handle 与两条停止结算路径：已停止开发 stop/resume 仍按原顺序拒绝、确认关闭撤销 resume、未知关闭不伪造 stopped；最小 `{}` 夹具不被新增 decoder 门槛阻断；原提交和新输入撤销不变 |
| OSC-A04 | 原 scope 缺失/不一致、坏 profile/回执、旧 v1/v2 区别、未创建、原成功回执重启/离线复用、unknown 零重放；结果封存及 #125 消息/答案停止窗口回归保持 |
| OSC-A05 | 原持久声明/编解码及代表编码相同；受影响组件、正式 EXE 的定向公开受控执行/恢复/停止通过，记录失败、skip、实际产物和测试范围 |

满足上述证据即可结束 A，不以全面消除 Orca import、实现 B、迁移 stability driver 或真机全组合资格为前提。若需新增校验门槛、改变诊断/提交顺序、持久格式或停止承诺，超出本项，应先修订范围。实现结果以后存入 docs/acceptance/ 独立文件；此条为设计阶段的资格边界；本轮实际实施与验证见末节及独立验收。

## 设计审计修订

2026-11-06，按 main `6dd4360a` 复核生产谓词、两处结算、离线错误顺序和共享 `{}` 夹具。未发现需要改变 A 行为边界的问题；确认标记分类不能改成完整 decode，closeSettlesAgentSession 的实际证明适用性不能代替开发 Workflow 分类。

修订一项实施前置：当前源码不含 #123/#125，仅通过 git show 审阅主线不能成为实施基线。实施前必须让实际源树包含本调查基线或已复核的更新主线，再固定源码/编码/测试；不得在旧树实施后，把 #125 的字段或能力差异算作 A 格式变化，也不得因回填新行为而扩大 A。设计审计只做源码/文档静态核对，未执行产品测试；P0 捕获和 A01–A05 仍待实际实施。

## 实施状态

2026-11-06 用户追加授权按 A→B 分开实施。A 的分类/诊断收口、编码对照、受影响回归和同最终 EXE 三公开受控已完成；原设计审计段落记录当时状态。后续 B 结果单独保存，不作为 A 初次通过依据；未提交、部署或授予完整级别资格。
