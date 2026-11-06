# 旧 helper 生产端与测试资产分离：B 独立验收

2026-11-06，合并前定向验证。依据 [Spec](../spec/orca-helper-producer-retirement.spec.md) / [Plan](../spec/orca-helper-producer-retirement.plan.md)，先完成 [A 独立验收](orca-state-feed-retirement.md)并保存可恢复源码归档，再实施 B。进入 B 前刷新 main，仍为 `dda9a827`，无新增消费者或上游差异。

## 精确删除与保留

生产删除无调用入口的 TerminalRuntimeMetadata、TerminalMetadata、TerminalConfig.Command；terminalAssets/TerminalExtensionDigest 从生产移出。在 `_test.go` 中以私有 historicalTerminalAssets/historicalTerminalAsset/historicalTerminalBundleDigest 提供原资产读取和摘要，读取失败明确使测试失败。无新生产接口、兼容路由、执行器或资产服务。[声明对照](evidence/orca-feed-helper-retirement-20261106/b-declaration-diff.json)证明仅这5个声明删除，2,253 个其余生产声明正文相同，无生产声明新增/修改。

TerminalConfig、严格 UnmarshalJSON、TerminalOwner 原字段顺序/tag/正文保持；Profile.Terminal、frozenRecord 的原 owner/receipt/退出/Job/hash 校验、usage_binding 和 usage collector 消费者保持。只改 access_mode_test/terminal_receipt_test 的资产装配调用，原正反断言不删。缺 bundle 反例仍先核对合法原 bundle，再清空冻结 bundle、提供非空当前测试资产摘要并拒绝，不以空常量代替真实反例。

五个 JS、两个原脚本、所有相对 import 路径原字节保持。Go list 确认原生产包没有 embed patterns，四资产仅列于 TestEmbedPatterns。直接调用删除前的生产 TerminalExtensionDigest 捕获原 bundle，再与新测试 helper 实际输出比较，均为 `2e59863050d0172b44fd19555c73e8e2e8d7547737ee42c7b9f6b0305cfb7a0f`；evidence SHA 均为 `3f5076c674f0811dec0768c8a9e5eaadc39bb3345a6ac49ead4659066ea4a95f`。文件名/顺序/字节串联算法不变，不新增分隔符。[保留与入口核对](evidence/orca-feed-helper-retirement-20261106/b-retention.json)及[保护摘要](evidence/orca-feed-helper-retirement-20261106/b-file-diff.json)。

保留 probe-orca-pi.ps1 的原 Node 前置、私有 evidence 复制路径、临时独立无 remote 仓库及原版本/路由限制；本轮只静态核对，没有运行模型 probe 或安装 SDK/Node/Pi。它仍是旧 TUI 诊断，不是当前结构化 Runner 的资格入口。InputContract 和资源摘要、共享 codex/process/terminaltools、#127 frozen/current/capability、fakeHost/request/bind、创建 owner、IPC、终端并发与旧快照 ShutdownLifecycle 拒绝夹具保持。

A 的69个协议源码/测试文件全部与 A 通过点逐字节相同，明确复用 A 完整 protocol/orca 结果，未虚记 B 重跑；原始查询/订阅及其迁移后的通知/取消/身份负例保持。稳定性 helper、networkPreflight、六组十四场景和 manifest/cases 均不变，不顺带迁移或删除其旧接线。

## 格式与验证

原持久/wire 声明及 codec、state/v8、user_version=8 均不变。15,528 字节 Profile/State/NativeProof/Record/Operation/结构化状态及 RuntimeFence 捕获与 A、原基线相同，SHA256=`06034f8e8ccba2233eac1518a29ac706ae48f6273993da21cc6a69f233f16d62`。158,875 字节原始状态/Observation 缺失与显式零/owner/request/receipt/能力及资源摘要捕获也与原基线相同，SHA256=`f5d69cd15df16ef2ba029b137e4fc6caea774aadf11565963dd0813254d78b62`。TerminalMetadata 虽带 JSON tag，但无持久字段/decoder 消费者；其删除单列，不借此删除实际冻结字段。[独立原字节比较](evidence/orca-feed-helper-retirement-20261106/format-comparison.json)。仅 source_digest 和新 EXE 哈希正常变化，不回填历史 runtime/bundle/evidence 摘要。

| 验证范围 | 实际结果 |
| --- | --- |
| 完整 orcatransport 包 | 81 个顶层通过，0 失败/skip；351 个执行节点通过 |
| application 同 A 定向组 | 103 个顶层通过，0 失败/skip |
| host/storage/contracts/usage collect/testkit livetest 五包 | 135 个顶层通过，3 个原有 usage 现场 opt-in 跳过，0 失败 |
| 原 Node L1 TAP 命令 | exit=0，tests=21、pass=21、fail=0、cancelled=0、skipped=0，摘要与实际 ok 计数一致 |
| live native driver、native/stability helpers | 独立编译通过；未执行真实模型 |
| 同正式 EXE 的公开受控 | SingleExecutionColdStop、NativeUnknownAndStop、RestartPreservesAuthorization 三项通过；重启四窗口全部通过，最终 exit_code=0 |

组件与应用命令/正则、包时限同 A，B 独立重跑；原 Node 命令为 `node --test --test-reporter=tap internal/execution/adapters/orcatransport/extensions/pi-evidence.test.mjs`。统一前置脚本的 TAP/结果规则未改，局部 TAP 通过不构造完整 L1 报告。普通 `node --test <原路径>` 和诊断入口保持，未额外重复执行普通 Node 或模型 probe。

正式构建 `scripts/build.ps1 -OutFile .evidence/retirement-audit/b/steerrunner.exe`；同 EXE 经 test-foundation.ps1 的三项明确 Pattern 独立运行，等待最终 result.json/integration.log，未将启动返回计为成功。[最终明细](evidence/orca-feed-helper-retirement-20261106/b-summary.json)、[公开结果](evidence/orca-feed-helper-retirement-20261106/b-controlled-result.json)和[原 TAP](evidence/orca-feed-helper-retirement-20261106/b-node-final.tap.gz)。

首轮编译因删除 Command 后遗留未用 encoding/json import 失败；移除该独占 import 后，摘要/格式捕获、完整 adapter、应用组、正式构建和公开验收均在修正后的稳定源码通过。首轮沙箱内 Node 失败为临时 fixture 原子重命名 EPERM；在允许该原操作的环境运行原命令21项通过，没有改 JS、脚本、断言或权限语义。两项原失败[编译](evidence/orca-feed-helper-retirement-20261106/b-first.jsonl.gz)/[Node](evidence/orca-feed-helper-retirement-20261106/b-node.tap.gz)保持，不改记通过。

构建 source_digest=`bc865f625d72a5c61aa9d898c9bc5b09ec1e63472e4b22805042221839efb378`，EXE SHA256=`ba4b32d2048b65140f8a3004c3b45bc3c9de9b4358dd5c8cbe898d52008d85f3`，source_dirty=true。B 源码/测试清单 SHA256=`b1f21951b3117386d254a49e0eae7fe6c4426a21f0619d26be1ca0fd18807474`；独立可恢复归档在本工作副本 `.evidence/retirement-audit/b-final/source.zip`，摘要见 summary。A 的源码归档、构建与结果不回写。

HPR-B01–B05 已完成。收益是生产 adapter 不再提供旧 helper 构造知识，历史读取仍只解释原证据，资产装配归测试所有。PC-R01–R10、原提交边界、执行/恢复/停止、预算只阻新派发及未知不重放的实现保持；原证明缺失仍等待/未知，原 close scope 不扩大，不用当前构建摘要补证。不宣称性能或包体积收益。未运行完整 L1/Controlled、L2/L3、真实模型或合并后回归；未提交、推送、提 PR、合并、部署或发布。

后续用户追加授权审计后提 PR；[实现复审](orca-feed-helper-reaudit.md)未确认生产或测试迁移缺陷，848个源/测试输入与本项一致。本次重跑完整协议、adapter 和 Node，明确复用本项应用、共享组件及原同 EXE 公开受控，不改写初轮 source_commit、失败或资格。上段未提交状态为本项初轮交付时点。
