# Orca ProgressHost 结构化 profile 单次解码 Plan

2026-11-06，步骤 1–7 已实施、验收并完成交付前复审，进入已授权的 PR 交付，未合入或部署。依据 [Spec](orca-progress-host-decode.spec.md)，调查基线 `a12b24dc836218d65e1ae295fc958375c09773c1`，实际实施基线 `6dd4360a42314e65b427627508ffd2ee3e4287da`。生产范围仅 `progress.go` 的 ProgressHost，不新增抽象或缓存，不修改 Orca。结果及资格见[验收记录](../acceptance/orca-progress-host-decode.md)。

1. **固定基线（PH01–PH03）。** 实施前刷新 main，在隔离分支核对 ProgressHost → decodeLifecycleProfile → decodeAgentSessionProfile 的实际调用及 runtimeFile/progressSetup 调用方，保存受影响包基线。记录结构化本机一次、配对远端成功两次的调用事实；对照 ProgressHost 原错误顺序，不能借用 #123 构造器的 agent/ProcessHost 前置检查。确认原 fileSource 独立校验及旧 terminal 分支不在改动范围。
2. **最小实现（PH01–PH03）。** 在 ProgressHost 内按原协议分支。结构化只调用一次原完整解码器，成功后由同一结果返回本机 local/远端 PairingHostID；其余协议保留原 decodeLifecycleProfile、本机判断、旧远端 UsageHostID 路径和错误。保留签名，不改任何解码器、连接入口或调用方。审计不引入 frozenRecord/Lifecycle 构造依赖、额外配置副本、全局状态或生产计数 hook。
3. **等价对照（PH01–PH02）。** 复用已有 structured/旧 terminal 夹具补 ProgressHost 定向用例，分别独立设置 host 预期；覆盖原合法版本/agent 代表值、非法配置及可区分首错的多故障。必要时用同一测试文件 overlay 到固定主线原生产代码，比较返回字符串、错误类型/分类/完整诊断。结构化 setup.ProcessHost=nil、隔离且不可用的当前配对存储仍应按原冻结 profile 成功；原 legacy 远端正常/损坏/缺失配对则保持原返回或错误。不能为方便夹具把旧远端改成无 I/O。输入不变使用独立克隆和有效的字段/结构比较，处理原 Clone 对空集合的语义，不再忽略 json.Marshal(setup) 的函数字段错误。
4. **受影响回归与静态审计（PH03）。** Windows 运行两个受影响包完整测试，并定向复用应用层原进度/封存和单次执行门禁：

   ```powershell
   go test ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca -count=1 -timeout=6m -json
   go test ./internal/application -run '^(TestProgress|TestResultFile|TestSingleExecution)' -count=1 -timeout=10m -json
   ```

   上述应用命令是定向集合，不是 application 全包或完整 Controlled。核验 TestWorkspaceFilesUseTheVerifiedPairingSnapshot、TestResultFileRPCUsesOriginalSelectorAndIndependentPath、TestResultFileOriginalIPCBoundaries、TestFrozenReaderHasNoExecutionAndRejectsMutation、TestRetiredExecutionFactoryKeepsFrozenProof 的原断言仍执行。配对替换应保留不同 endpoint/公钥/token 且通告相同 runtime 的负例，原解析一次、同配置拨号、替代端点零连接/零文件请求不变；不新增通用连接模拟器。单次完整解码由 diff/调用图核验，不使用替换 decoder 或性能结论。
5. **同一候选双端文件联验（PH04）。** 测试稳定后固定源码摘要和写入 EXE，复用 #113 的现有默认 development/2 正式入口、已配对远端及新隔离目录/请求，不改真实配对。正文限定在本次工作区写进度与结果，内容区别于初始化字节；保留可观察活动窗口，公开 progress-file 查询前后均确认原 Run/turn 活动，result-file 保持 pending。进入 waiting_input 后仅按原 revision 受理一次 stop；停止 unknown 不补发。确认原 close、停止 scope、恢复/新输入撤销后，有界只读等待 result-file closed/version=1，并核对实际字节与重复快照稳定。等待不得延长生产封存预算、触发重采集或接受缺摘要；未知输入只核对原请求，不新建替代 Run 冒充恢复。Pi/audited 未真机时单列，禁止借默认双端声明通过。
6. **原目录排空（PH05）。** 原 writer 正常停服后，再用同源码、不同版本标识及 SHA 的 reader，与原 writer 一同冷读这两个实际原目录，调用现行 SteerLine 生产 VerifyUpgradeDrained。记录其实际提交与调用方式，不修改其门禁或源码；必须核对业务/资源/未决控制，不能只看服务退出。记录每端原 Run/input、原停止范围、两份 EXE 与 DB/shutdown 前后摘要。旧记录只用已有冻结记录/夹具，不启动旧执行，不清理安装现场。
7. **收尾审计与记录（PH06）。** 回填独立 `docs/acceptance/orca-progress-host-decode.md` 及对应证据目录，不强制更新中央索引。分别列出主线对照、候选包/定向应用测试、同候选双端与原目录排空的实际结果、计数、skip 和阻断；完整资格遵循 docs/testing.md。若后续进入 PR 交付，刷新 main 与实际文件合并状态；新主线影响已验收源码时按影响复验，不改写历史只为 Rebase 提示。再次审计严格校验/首错、local/原 host、旧 terminal/#121/#113 边界，修复后按影响复验。

本轮回填：生产仅 ProgressHost（+10/-7），复用结构化字段/首错矩阵并增加旧 terminal 路由与权限拒绝测试。主线同夹具对照 4 顶层/28 叶通过；候选 orcatransport 78 顶层/291 叶、protocol/orca 62 顶层/321 叶通过，后者保留 4 项显式 opt-in 跳过。应用定向 24 顶层/42 叶通过。相同候选 development/2 双端实际进度、原停止后结果文件与原目录双 EXE 排空通过；原输入一次、未知回执只读核实，停止额外资源清理未核实。无遗留阻断。测试及文档已审计，不复用历史资格；交付前复审未发现运行或测试缺陷，原始证据核对及原目录再次冷读通过；已进入用户授权的提交/推送/PR 交付，未合入或部署。
