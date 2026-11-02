# Plan：Orca 状态与会话读取 B3

日期：2026-11-02。状态：P0–P5 完成，独立审计未发现确认的 P0/P1/P2；见[验收](../acceptance/orca-state-session-capability.md)。依据 [Spec](orca-state-session-capability.spec.md) 和[长期设计](orca-capability-layer.spec.md)。基线 `d30cd2b`，B0/B1/B2 已通过独立审计。

## P0 固定语义、类型和调用清单

记录 native_input.go/native_input_query.go、state_feed.go/state_query_feed.go 的生产职责与全部消费者；核对 native_state/native_observation/native_start 的 proof 调用、queryInput 和真实探针。原 NativeProof/State/Profile/Target/operation 及创建声明保持，保留 JSON/摘要 oracle。

白名单为这些协议/执行职责、必要的边界值映射和测试迁移。不能修改完成判据、范围/权限、停止或用户材料来降低迁移难度。协议包剩余 client/close/文件等 backend 依赖待 B4，不用本批声明整包已独立。

## P1 原生 reader

1. 新增自有 SessionReadOptions、SessionMessage/Block/Page 和 OpenSessionReader。冻结原目标、agent/provider/locator/page 约束，单连接读取，reader Close 负责回收；不接收基线或业务类型。
2. 每页先解析完整 messages/hasMore/beforeOffset/error，再校验原 offset 规则；保留所有用户、assistant、其他来源和 shortened/blocks 原事实，不执行唯一性或正文判断。
3. 保留原 40 秒总上限和短父期限，显式 100/16 从执行侧传入；原基线到达即停止可关闭，页预算耗尽不重连、不重置期限。保存原始页用于现有诊断，禁止远端路径本机打开。
4. 新能力测试先证明多用户/同文用户可完整返回、调用和分页/连接关闭有界，不依赖 Run 或 Snapshot。

## P2 原输入策略迁出

1. 将 NativeInput、originalInputQuery、唯一用户算法、QueryOriginalInput 及 thin/corroborated 资格与围栏组合移到 orcatransport 私有实现。保留原判断次序、错误阶段、深拷贝快照和完整执行进程绑定；RPC 操作消费原生 reader、B1 原终端和原生状态查询。
2. 定义执行侧原输入查询结果，嵌入 native.AgentState 并另带私有 NativeInput。queryInput 及真实/受控消费者改用该返回值；协议 AgentState 移除 NativeInput，CanQueryCompletionInput 改为执行侧私有函数。
3. NativeProof 的 attribution/recordedInputMatches/incomplete/signal/end/query 和 nativeObservation 中需要输入证明的位置显式传入该证明；不新增持久或全局证明槽。缓存仍归原 nativeObservation，bindInput 仅返回当前 display stamp 的原匹配证明，不回写原生状态。
4. 修改 queryCompletedOriginalStart 与 observer 查询流程，把状态和输入分别传递，保留 query 前后 drain、反证优先、原 process/query 匹配和 LastQuery/原预算。原全量匹配不改为四字段身份比较。
5. 更新 B0/原 live/只读诊断探针和原生产 NewLifecycle，使用同一迁出的策略；不保留协议旧策略 wrapper/复制算法或注入新的 production callback。

## P3 状态 API 的值边界

1. 定义 Orca StateResource，替换 state_feed/state_query_feed 及状态解码/查询/factory 的 backend 参数；只携带 runtime/worktree/handle/incarnation。
2. 原生 AgentState/StateSample、Kind/ReceivedAt、authority/revision、局部元数据缺失及交叉核对保持；只移除认领字段/方法，不把 initial/query 变成 notification。
3. 执行侧 factory 和 native 查询闭包映射完整执行身份至 StateResource，隔离 fixture 构造仍在执行边界；生产 RPC 失败不回退。停止等尚未迁操作继续用原执行合同。

## P4 测试及验证闭环

新增 TestOrcaSessionReadCapability 验证自有 reader，尤其多用户/同文用户原样返回、分页协议反例、取消/短期限/单连接回收/零 mutation。原协议唯一用户及 thin/full 原查询策略 tests 随所有者迁至 orcatransport，保留原 case 名和断言；必要 IPC 支撑只放 *_test.go，不建生产注入接口。

迁移 fixture 的变化仅为值/证明传参和所有权，原重复、altered、换 process/provider/pane/authority、负 offset/不推进页、部分元数据、wire/复制证明等反例不得删除。原 input query/controller、NativeProof、零 incarnation、原生产 constructor、CRLF/followup、缓存/断线/旧通知/新反证、public sample、helper/control 及停止消费方的语义断言保持。

先运行原生 reader和迁出策略/身份 API 定向，再使用 go test -json 运行两个受影响包全集、workflow 的 TestSRExecutionDependencyBoundary。IPC/WS/子进程夹具使用匹配权限；记录真实 roots/子例/fail/skip/失败修复，不把 opt-in skip 当资格。无新增输入 wire/生命周期行为时不重复全模型矩阵，受控不能排除的差异另补现场。

## P5 删除、验收和提交

- 检查协议不再含 NativeInput/QueryNativeInput/QueryOriginalInput/CanQueryCompletionInput 或 backend 状态入参；原生多用户读取成功与执行侧错误归属拒绝同时成立。
- 检查原持久声明、编码/target/正文与 stop scope 不变，状态文件移除的 NativeInput 原为 json:"-"；证明载体不进入 wire 或持久 codec。
- 按 OSS-01–05 写 docs/acceptance/orca-state-session-capability.md，列实际职责/消费者/删除清单、剩余依赖、证据与未验边界，同步路线及索引。
- 提交实施后独立只读审计一次，确认修复独立 commit 并补相应验证；不提阶段 PR。完成边界、直接验证和审计均闭环才标 B3 完成。
