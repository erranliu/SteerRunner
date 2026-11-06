# StateFeed 聚合层退役：A 独立验收

2026-11-06，合并前定向验证。依据 [Spec](../spec/orca-state-feed-retirement.spec.md) / [Plan](../spec/orca-state-feed-retirement.plan.md)，调查及实施基线 main `dda9a827`；工作副本 HEAD `da788478` 的文件与该主线一致。A 全部验证和源码归档完成后才实施 B，不以 B 的通过倒追 A。

生产仅删除 StateFeed 的 11 个函数/方法、5 个瞬态类型和3个采样常量；`state_query_feed.go` 无保留声明后删除。保留的 2,258 个生产声明正文相同，无新增生产声明；原查询/订阅、解码、身份/完整性检查、socket/IPC/配对和共享缓冲均不变。[声明对照](evidence/orca-feed-helper-retirement-20261106/a-declaration-diff.json)及[文件与保护摘要](evidence/orca-feed-helper-retirement-20261106/a-file-diff.json)。

| 原测试语义 | 实施后的入口与处理 |
| --- | --- |
| sample-feed 的真实 snapshot/updated 传递 | 改接真实 SubscribeState/Next；保留原加密 WebSocket 和原帧/pane 身份检查 |
| 初始化 context 与订阅生命周期分离 | 初始化成功后取消其 context；独立 context 仍取得原 updated |
| 独立查询不干扰订阅 | 原订阅存活时 QueryState、QueryWorktreeAgents 均完成，再经屏障释放并取得更新 |
| 阻塞 Next 取消/断开 | 两个独立场景；测试 context 观察真实读取的取消注册后再取消/断开，核对原错误分类，无生产 hook 或伪造 Next |
| 其余八个 stream 场景 | subscription-query、wrong-runtime、wrong-request、cancelled 及四个 terminal-query 场景保持并实际通过 |
| 本机 pane/provider、完整正文、provider 替换拒绝 | TestOrcaLocalOriginalQuery → QueryState/DecodeOriginalState/QueryOriginalAgent；原 IPC fixture 保持 |
| 零后台订阅及阻塞查询取消 | 原订阅计数保持为零；在原 RPC 已进入 fixture 的屏障后取消调用 context，返回 context.Canceled |
| Initial/Drain/Kind/接收时钟/Feed.Close 及 Feed 自身生命周期 | 随无消费者聚合层退役，不建立测试 Feed 或兼容壳 |

`tests/helpers/stability/main.go` 的 probe-state/probe-subscribe、native networkPreflight、六组十四个旧稳定性场景、manifest/cases、pre-send/network 工具保持原字节；不迁移其旧终端协议或授予当前结构化资格。Node/JS、诊断 probe、其它共享夹具也未修改。

原 Profile/State/NativeProof、Record/Operation、结构化状态/RuntimeFence 的独立捕获为 15,528 字节，SHA256=`06034f8e8ccba2233eac1518a29ac706ae48f6273993da21cc6a69f233f16d62`，与基线完全相同。另捕获原 AgentState/StateResource/Observation（缺失 incarnation 与显式零）、owner/request/receipt、能力和资源摘要，共 158,875 字节，SHA256=`f5d69cd15df16ef2ba029b137e4fc6caea774aadf11565963dd0813254d78b62`，同样逐字节相同。state/v8、user_version=8、所有实际持久/wire 声明与 codec 未改；捕获错误均检查，未以当前 hash 补原证明。[格式捕获](evidence/orca-feed-helper-retirement-20261106/a-frozen.json.gz)及[原始查询/能力捕获](evidence/orca-feed-helper-retirement-20261106/a-wire.json.gz)。

| 验证范围 | 实际结果 |
| --- | --- |
| 完整 protocol/orca 包 | 62 个顶层通过，4 个原有显式真机/清理 opt-in 跳过，0 失败；全部 360 个执行节点通过 |
| 完整 orcatransport 包 | 81 个顶层通过，0 失败/skip |
| application 停止/结构化/消息/恢复/预算/封存定向 | 103 个顶层通过，0 失败/skip |
| host/storage/contracts/usage collect 完整四包 | 86 个顶层通过，3 个原有 usage 现场 opt-in 跳过，0 用例失败 |
| testkit/livetest 完整包 | 49 个顶层通过，0 失败/skip |
| live native driver、native/stability helpers | 分别编译通过；未执行真实模型 |
| 同正式 EXE 的公开受控 | SingleExecutionColdStop、NativeUnknownAndStop、RestartPreservesAuthorization 三项通过；重启四窗口全部通过，最终 result.json exit_code=0 |

组件使用 `go test <上述包> -count=1 -timeout=5m -json`；application 正则为 `^(Test.*(Stop|Scope|Structured|Message|Recover|Budget|ResultFile|ResultSnapshot|SingleExecution|Generation|ManualReconciliation|Observation|Frozen).*)$`，包预算15分钟。正式构建 `scripts/build.ps1 -OutFile .evidence/retirement-audit/a/steerrunner.exe`；公开测试经 `scripts/test-foundation.ps1 -RunnerExe <同文件> -Pattern '^(TestSRSingleExecutionColdStop|TestSRNativeUnknownAndStop|TestSRRestartPreservesAuthorization)$' -OutDir <全新目录>`。等待独立宿主最终报告后判定，启动脚本返回不计通过。

[实际结果与跳过清单](evidence/orca-feed-helper-retirement-20261106/a-summary.json)保留首轮命令路径错误：把 `internal/testkit/livetest` 写成不存在的 `internal/livetest`，该聚合命令退出1，其余五包实际通过；更正路径后独立完整 livetest 通过，不抹去原失败。首次沙箱内 WebSocket 定向运行出现 transport unavailable（包括原有八场景），在允许本机连接的环境复验完整协议包通过；迁移后又补强 Next 读取屏障，并在最终源码上完整复验通过，不修改生产传输来规避环境问题。

构建 source_digest=`2710a690c2a24ac64370281017a3fe6c40459f2afdae105dc8032180c7530ace`，EXE SHA256=`ad6d94d0ecdfc4e1e0600984358e42ff77d53816b5a41ed069cbaa941c6b0ea0`，source_dirty=true。A 源码/测试清单 SHA256=`b88d8339d951176fd9984a8a1379afb464c948073a995b4d994c597e690511b5`；可恢复归档在本工作副本 `.evidence/retirement-audit/a-final/source.zip`，其摘要见 summary。[公开结果](evidence/orca-feed-helper-retirement-20261106/a-controlled-result.json)和[日志](evidence/orca-feed-helper-retirement-20261106/a-controlled-integration.log)。

SFR-A01–A05 已完成。收益是退役无产品入口的采样聚合模型及其双实现；不宣称提速或扩大能力。原受理/提交、unknown 原操作只读对账、零重放、预算只阻新派发、同构建原资源恢复、原 scope 停止和结果封存的实现均保持。原证明缺失仍等待/未知，close 失败不变成功，不扩大为主机清场。未运行完整 L1/Controlled、L2/L3、真实模型或合并后回归；未提交、推送、提 PR、部署或发布。

后续用户追加授权审计后提 PR；[实现复审](orca-feed-helper-reaudit.md)未确认生产或测试迁移缺陷，源/测试字节保持并重跑完整协议、adapter 和 Node，原 A/B 资格与首轮失败不改写。上段未提交状态为本项初轮交付时点。
