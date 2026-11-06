# Plan B：Orca 历史读取与当前执行夹具分离

日期：2026-11-06。状态：P0–P4 已完成，见[独立验收](../acceptance/orca-reader-fixture-boundary.md)；以 [A](orca-stop-contract-boundary.plan.md) 独立通过点为实施基线。依据 [Spec B](orca-reader-fixture-boundary.spec.md)。用户追加授权后已按 A→B 实施；B 前整合 main `45818503`，上游和两批证据分开，不自动提交、提 PR 或部署。

## P0：A 通过后的消费者和保护基线

- [x] 固定 A 源码/测试/格式摘要，刷新主线后复核差异；上游变化与 B 修改分开记录。
- [x] 搜索候选方法的定义、非测试调用及所有夹具消费者，区分 Lifecycle 与 frozenRecord 同名方法；不能把 reader 的真实调用误认成旧执行转发消费者。
- [x] 保存既有顶层/子测试清单、关键断言、共享 fakeHost/request/bind/creation/IPC 工具及测试文件摘要。列允许修改的夹具和调用点，尤其是 structuredFixture、workspaceFixtureOn、CLI Golden 与原终端并发例外。
- [x] 分开登记局部 helper 单元测试、生产冻结入口测试及真实构造/队列/配对测试。逐一记录构造后对 profile/setup/probe/client 的修改，尤其是 access_mode_test、terminal_receipt_test 和 fixtureFixedControl；迁移后仍须到达原被测方法，不能被新增解码提前挡住。
- [x] 捕获全部相关生产持久声明/编解码和新旧代表编码，包括 Profile/State/NativeProof/receipt、结构化状态/RuntimeFence、操作/结果和 capabilities。编码错误不能忽略，含函数字段的 setup 用独立 Clone 与结构比较，不能用失败 Marshal 的空输出比较。
- [x] 审计设计：不扩大 frozen client 白名单，不制造仅供测试的第二生产 decoder，不把旧结构化版本测试改成全新版本后删除原未知分支，不删原终端并发或共享 creation 事实。

## P1：先分开装配，再删方法

1. 提取中性的 setup/host 测试数据构造；不赋予它执行能力。保留共享 host、请求/回执绑定和原生产数据来源。
2. 历史 codec/control/receipt/access/stop 测试改用 frozenRecord 测试夹具；局部单元测试继续使用相同已解码事实及原 profile/probe/client 修改，不盲目从未同步的 setup 重建 reader。生产冻结入口测试仍从原封套经真实 reader 构造，检查严格校验和只读白名单。旧 State envelope 编码辅助先移入 `_test.go`，按原版本/身份生成同字节封套，断言保持。
3. structuredFixture、workspaceFixtureOn 直接从中性数据装配合法结构化配置与当前对象；远端明确合成 PairingHostID，并核对 target/version/host/development/audited。经真实 NewLifecycle 构造后，在既有测试 seam 注入隔离 client/agent port，构造与普通单元调用不访问真实配对目录/IPC/宿主。队列/配对专属测试继续走其原生产路径，不用统一 fake 抹掉被测逻辑。保留版本覆盖、#123 clone/首错和完整派生字段；不得形成构造回归→structuredFixture→同一构造回归的递归依赖。
4. CLI/creation 测试使用用途明确的能力装配，保留原 argv/路由/LaunchCommand Golden、creation owner/原回执语义。不得把 terminal create 送进只读 client，再通过放宽白名单解决失败。
5. 原终端并发测试保留专用测试装配，继续测试原生产 RPC 及身份绑定；不再作为通用历史或当前结构化夹具基座。此时先跑迁移后的受影响测试，明确失败来自装配而非放松断言。

## P2：精确删除转发闭包

- [x] 逐一删除 Spec 列出的 17 个候选方法；保留 frozenRecord 同名生产读取实现。original 的 state 调用直接改接同一 reader，原 body 的其它身份/错误顺序保持。
- [x] 删除或收窄无消费者的通用 frozenFixtureLifecycle 装配，保留原终端/CLI 两类明确例外需要的测试帮助函数。未证实无消费者的 helper 不删除。
- [x] 核对 validate、inspectionGate、profileVersion、nativeAccess 和结构化读取投影、creation/队列、公共 InspectBoundTerminal 仍在；不按文件名整删 control_checkpoint.go、terminal_receipt.go、local_codex_stop.go。
- [x] 对每一处生产差异说明是转发删除、同一 reader 调用或测试 encoder 移出；实际发现其它生产调用时保留该项并修订清单，不用新增 facade 掩盖依赖。

## P3：格式、行为与公开入口验收

- [x] 比较 P0 与 B 的生产持久类型/codec/实际持久读写和解码；单独允许并登记无生产消费者的 envelope 方法删除与测试 encoder 搬迁。逐字节核对新旧代表 payload/envelope、CLI Golden 与摘要，使用独立 A 原字节；不能以同一新 encoder 的自我 roundtrip 充当基线证明。
- [x] 跑完整 orcatransport 包，包含历史 reader、codec、scope/stop、receipt/owner、原终端 RPC/并发、CLI/creation、当前 structured/development/workspace/usage/files 以及 #123 构造回归；Windows 专项在其合法平台运行。
- [x] 跑受影响 application 停止/结果/恢复和 `TestDevelopmentRunMessage*` 定向，以及 host/storage/contracts/usage 实际受影响组；核对 OSC-A 验收未回退。编译 tests/native 与 helpers，不把编译或 opt-in skip 记为真机通过。
- [x] 固定生产/测试源码后正式构建，以同一 EXE 经 scripts/test-foundation.ps1 明确 Pattern 执行 `TestSRResourceCreationRecovery`、`TestSRSingleExecutionColdStop`、`TestSRNativeUnknownAndStop`、`TestSRRestartPreservesAuthorization`。使用原公开受控入口/匹配保护，分开记录 A 与 B 的产物和实际范围。

## P4：审计与交付

- [x] 对照 P0 逐项复核既有测试名/子例与断言；改动仅限允许夹具和调用对象接线。核对没有删除生产 reader、共享 host/creation/IPC、独立诊断/Node 测试或 stability 工具。
- [x] 在 docs/acceptance/orca-reader-fixture-boundary.md 记录删除/保留的实际方法数、消费者迁移表、两个原终端例外、字节对照、命令、源码/产物身份、失败/skip 和未验范围。若只是换文件/改名而转发和旧装配仍在，不标记完成。
- [x] ORF-B01–B05 完成即结束；不续做 helper 资产退役、全量 fixture 重写或 stability 迁移。长期文档仅在后续实施验收后记录实际状态，不提前改写能力承诺。
- [x] 遵循 docs/testing.md 和 AGENTS.md 的资格/验收记录要求。本批完成实施及定向验证，首轮失败与同 EXE 复验分别记录；未提交、提 PR、合并 PR、部署或发布。

## P5：用户追加实现复审与 PR

2026-11-06 用户授权“审计后提 PR”。[实现复审](../acceptance/orca-stop-reader-reaudit.md)确认 17 项生产删除、原终端/共享夹具和格式保持；生产未再修改，完整 adapter 81、应用定向 76 及同最终 EXE 四公开场景通过。B 首轮 fixture 读取失败仍单列，不虚记修复；按追加授权提交、推送并创建 PR，不自动合并或部署。
