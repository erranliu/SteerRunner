# Orca 状态与会话读取 B3 验收

日期：2026-11-02。状态：实现、定向及两包全集通过，待实施后独立审计。依据 [Spec](../spec/orca-state-session-capability.spec.md) / [Plan](../spec/orca-state-session-capability.plan.md)。本批不启用新组合，不平移 B0/B1 现场或授予 B5 资格。

## 1. 能力与策略所有权

新增 protocol/orca/session_read.go 返回原生 SessionMessage/Block/Page；OpenSessionReader 冻结明确目标、agent/provider/locator、页大小和页预算，管理一条原连接、40 秒上限、取消/关闭及 offset 推进。它不接收 baseline，不选择/去重用户，不解释正文或业务完成。相同正文的多用户以及 shortened/其他来源/空 blocks 可原样返回，原始页完整保留。

原唯一输入、thin/corroborated 资格、原 terminal/pane/provider/main 前后围栏和 originalInputQuery 已迁至 orcatransport/original_input.go 私有策略。生产 NewLifecycle、observer、原输入开始恢复和各原生探针消费同一策略；protocol 的 native_input.go/native_input_query.go 已删除，三份策略测试随所有者迁移并保留原 case/断言。

协议 AgentState 已去掉 json:"-" 的 NativeInput 和 CanQueryCompletionInput；执行侧 originalInputState 另带私有输入证明，NativeProof/观察算法通过显式参数消费，不新增持久字段、全局表或账本。nativeObservation 的既有输入缓存保持原边界，bindInput 返回当前 display stamp 证明而不改写原生状态。

state_feed/state_query_feed 及解码/查询使用自有 StateResource，原生状态文件不再导入 backend。执行侧 orca_state.go 映射 runtime/worktree/handle/incarnation，保留原执行 factory 窄端口和隔离构造；无消费者的原协议 StateFeedFactory 类型声明删除。瞬态输入证明仍比较完整 backend.ProcessIdentity，不能只凭四个原生字段复用。

原分页先验校验、100/16/40s、到基线停止、初始/查询/通知及接收时间、反证优先/先后 drain、正文/Started/完成、NativeProof/State/Profile/Target/operation/创建持久声明及停止 scope 不变。protocol 剩余 backend 依赖仅 client.go、readonly_close.go、stopped.go，留待 B4；本批不宣称整个包已独立。

## 2. 构建和验证

源码基线 `310f270` 加本批差异，最终 source_digest 为 `a62fa90ee9f7f62816d22da626f0899dee1d481bd5f578ead40205707c3b465d`。源码范围沿 Get-SRSourceIdentity；删除/移动文件先暂存，再计算最终索引清单，避免把旧路径的 missing 占位混入摘要。原始日志、实际命令及源码身份见[证据目录](evidence/orca-state-session-capability-20261102/README.md)。

| 验收 | 直接结果 |
| --- | --- |
| OSS-01 | 自有 StateResource、无执行导入的状态/reader 文件；原生 reader 返回 2 个同文 user 和 assistant，不拒绝 shortened/hook/空 blocks |
| OSS-02–03 | 迁出策略/真实构造器/缓存/反证/原输入/CRLF/followup 定向 16 根/181 子节点通过；thin/full 的原复制/篡改/换资源测试保持，并新增执行 executable 不同而原生四字段相同的拒绝断言 |
| OSS-04 | 最终 reader 1 根/13 叶例通过：分页/缺字段/负与不推进 offset/预算/错 runtime/取消；明确关闭和父上下文取消均由服务端观察到原 pipe 关闭，重复 Close/失败 Next 不重连或增加读取 |
| OSS-05 | 两包全集 223 根/1024 子节点、1100 叶例，0 fail、7 skip；workflow 依赖守卫 1 根通过；生产/格式白名单及 git diff --check 通过 |

这里子节点包含分组节点，叶例另计；各定向集与全集重叠，不相加。全集在补齐最后两个 reader 连接关闭叶例前执行；补验只改该测试及状态文件注释，最终 reader 全 13 叶例通过，未重跑无关全集。7 个 opt-in skip 为 TestOrcaLocalPipeNativeEvidence、TestOrcaLocalIPCNativeProbe、TestOrcaTerminalQueryLive、TestUsageLocalAcceptanceRepoCleanupLive、TestOrcaNativeStateLiveLifecycle、TestShortInputProbeInspect、TestOrcaWorkspaceStartupLive，不计真实资格。

初轮 reader 预算夹具第二页没有推进 offset，被正确拒绝；修正该 fixture 后补验通过。迁移中的两次测试编译失败和最终编译通过记录保留；修复类型/证明显式传参，没有放宽原语义断言。原 `TestOrcaNativeInputProofRejectsUnboundHistory` 的所有错误输入仍实际传入被检查的证明，避免用 nil 证明掩盖反例。

没有真实 Orca 资源或模型场次。本批原 RPC 方法/参数/页预算、输入 wire 及生命周期行为未变，原生字节、真实构造与全集直接支持结构等价；真实 Runner 开发/停止资格继续由 B5 验收。

## 3. 独立审计

实现提交后独立只读审计一次；确认问题独立 commit 修复并补受影响验证。完成前 B3 保持待审计。
