# Plan B：旧 helper 生产端与测试资产分离

日期：2026-11-06。状态：已实施并完成合并前定向验收。依据 [Spec B](orca-helper-producer-retirement.spec.md)，以 [A](orca-state-feed-retirement.plan.md) 独立通过点为实施基线；调查 main `dda9a82727b6e7ac88e687a250750e8bbae236dc`。以下为已完成步骤，实际范围见[验收](../acceptance/orca-helper-producer-retirement.md)，未提交、提 PR、合并或部署。

## P0：A 通过点、真实消费者和原字节

- [x] 固定 A 源码/测试/格式/产物点，再刷新 main；相关上游差异单独审计、捕获，不倒记为 A 或 B 的变更。复核 Metadata/Command 是否出现新调用、Node/probe 是否新增消费者。
- [x] 核对 terminal_runtime.go 全部声明、Profile.Terminal、frozenRecord 原 hash/owner/receipt、access_mode/terminal_receipt/usage_binding/collector 测试、cmd/脚本路由，形成逐声明删除/保留表。不能因 TerminalMetadata 有 JSON tags 就误作持久字段，也不能因它可删就删 TerminalConfig/原摘要。
- [x] 捕获五个 JS、probe-orca-pi.ps1、test-prerequisite-contract.ps1 的原字节和 SHA，四资产的文件名/顺序、bundle/evidence SHA、Node/脚本相对路径与入口；先保存原生产算法实际计算结果，不能只让新 helper 自算自证。
- [x] 捕获原持久/wire/资源摘要声明、编解码和代表 Profile/owner/receipt/State/NativeProof/结构化状态/Record/Operation 字节；保存共享测试和 A 保留原始 RPC 的保护摘要。序列化错误不忽略。
- [x] 审计设计：无旧生产入口；生成端移除不影响原证明；JS/Node/probe 不改；不会创建生产 asset service、当前摘要补原 hash 或测试用执行兼容器。

## P1：测试侧接回真实资产

1. 在 `_test.go` 以不同名的私有 helper 接回原四文件 embed/读取，顺序和摘要算法完全保持，测试装配读取失败明确失败。P1 可临时对照原生产函数，避免同名声明冲突；P2 删去原生产版本后只留测试 helper。使用静态原路径，不扫描安装版、业务工作区或网络。
2. 将 access_mode_test、terminal_receipt_test 的读取/摘要调用接至该 helper；保留所有正反断言，特别是缺原 bundle 而提供当前 bundle 仍拒绝。测试 encoder/历史 reader 不变，不要求给最小 unit fixture 新增完整生产配置验证。
3. 对照 P0 原结果验证文件名/字节/bundle/evidence hash；五个 JS 和两个脚本按原字节保持，不因 Go import 清理去删源码资产。保持普通 Node/诊断入口及相对 import，实际 Node/TAP 资格按 P3 核对。

## P2：删除精确生产闭包

- [x] 删除 TerminalRuntimeMetadata、仅服务它的 TerminalMetadata、TerminalConfig.Command；生产侧移除 terminalAssets/TerminalExtensionDigest 和独占 imports。terminal_runtime.go 仍保留原 TerminalConfig、UnmarshalJSON、TerminalOwner。
- [x] 核对无生产 asset embed/运行时摘要生成入口，无新兼容 CLI route、stub、接口、注册表或 build-tag 回退。实际出现消费者时保留并修订 Spec，不删除调用来制造零引用。
- [x] 再核对 frozenRecord、Profile/State/receipt/owner、usage、InputContract、公共 terminal、Codex/parser、Node/probe 及 A raw 查询/订阅实现；除批准闭包外正文/字段/tag/错误保持。

## P3：格式、共享测试和公开入口

- [x] 生产声明差异只允许原 producer/metadata/embed/摘要的删除或测试侧搬迁；保留持久类型/tag/codec 和实际读写/decoder，原代表编码与 bundle/evidence hash 独立对照相同。JS/脚本字节和路径检查通过。只把 source_digest/新 EXE 哈希变化记为正常构建变化，不容许资产或已存 runtime/bundle/evidence 摘要变化，不以新产物回填原证据。
- [x] 运行完整 `go test ./internal/execution/adapters/orcatransport -count=1`，保留原 owner/receipt/缺 hash 反例、历史/current/capability 夹具及原终端并发。运行相关 usage/collect、host/storage/contracts 和应用停止/恢复/预算/结果/消息定向；按实际 diff 选择范围并保存命令/JSONL。协议包如与 A 的源和测试均相同，可明确复用 A 的原结果；发现变化或影响则实际重跑，不能虚记本次执行。
- [x] 实际运行原 L1 使用的 `node --test --test-reporter=tap internal/execution/adapters/orcatransport/extensions/pi-evidence.test.mjs`，保存真实退出码/TAP 并核对 tests>0、fail=0、skipped=0 与计数一致；保留独立普通 Node 命令的路径/语义。不伪造 l1.jsonl/前置报告或计为完整 L1。Node 缺失或版本不符不能记通过，也不擅自联网安装；probe 仅静态核对，不执行模型诊断或登记真机资格。
- [x] 编译 live native driver 和现存 helpers，尤其稳定性共享工具；不修改其旧终端接线或注册场景。确认 A 的原始订阅/查询及其共享负例仍存在并通过。
- [x] 冻结生产/测试后正式构建；以同一最终 EXE 经 scripts/test-foundation.ps1 明确 Pattern 运行 `TestSRSingleExecutionColdStop`、`TestSRNativeUnknownAndStop`、`TestSRRestartPreservesAuthorization`。实际修改若影响创建接线，再补 ResourceCreationRecovery，不以当前“无消费者”的推断替代发现的新影响。
- [x] 等待该独立宿主的最终 result.json/integration.log，核对退出码、匹配数量和子例；启动脚本返回不是完成。失败/skip 保留，不能改断言、目录登记或历史证据换取通过。

## P4：独立验收与结束

- [x] 在 docs/acceptance/orca-helper-producer-retirement.md 记录删除生产端/保留历史格式及共享消费者、测试资产归属、字节/hash 对照、实际收益、命令/测试范围、源码/EXE 身份及失败/skip；A/B 独立固定，不回填资格。
- [x] HPR-B01–B05 完成即结束；不继续删除 JS、probe、Node 测试、InputContract 或旧稳定性场景，不宣称包体积/性能改善或全后端解耦。
- [x] 按 docs/testing.md 区分定向与完整、合并前后、组件与真实模型。实际验收后才更新长期文档状态；无完整 L1/Controlled、L2/L3 或模型执行就不申报相应资格。中央 README 无导航需要则不改。

## 完成记录

A 完整验收和源码归档后再实施 B；各项命令、实际跳过与首轮失败保留于[独立验收](../acceptance/orca-helper-producer-retirement.md)。源码、产物及三项公开受控分别冻结，不回填资格。B 协议源码/测试与 A 相同，明确复用 A 完整协议结果；其余受影响组件、应用定向组与公开受控分别执行。
