# Spec B：退役旧 helper 生产端并隔离测试资产

日期：2026-11-06。状态：已实施并完成合并前定向验收。调查基线 main `dda9a82727b6e7ac88e687a250750e8bbae236dc`；实施基线为 [A](orca-state-feed-retirement.spec.md)/[Plan](orca-state-feed-retirement.plan.md) 的独立通过点。配套 [B Plan](orca-helper-producer-retirement.plan.md)，依据[长期产品契约](steerrunner.spec.md#长期产品承诺)、[Orca 长期设计](../orca-execution.md)及[已实施 helper 退役](orca-helper-retirement.spec.md)。本项不是再次删除旧执行链，也不改变历史读取合同。

## 目标与消费者审计

新 Run 已拒绝旧 Agent executable/args/helper 配置，当前执行只使用结构化会话。terminal_runtime.go 仍声明当前 helper 元数据生成、旧命令拼装与 Go 嵌入资产，给后续开发留下仍须维护旧构造路径的假象。本项删除无调用根的生产能力，将真正只供测试使用的资产读取隔离到测试侧；保留原冻结配置、owner/receipt 和摘要核验。

| 对象 | 已核实生产/测试入口 | 删除或保留 |
| --- | --- | --- |
| TerminalRuntimeMetadata、TerminalMetadata | 只在 terminal_runtime.go 的生产端互相引用；无当前 CLI、受理、构造、历史 decoder 或测试消费者 | 删除函数和仅服务该输出的类型；不保留兼容壳 |
| TerminalConfig.Command | 仅方法声明；当前没有 --terminal-configuration/--terminal-metadata 路由或调用者 | 删除旧命令生成，不新增假成功/unsupported 专用入口 |
| terminalAssets、TerminalExtensionDigest | 被上述无入口生产端以及 access_mode_test.go、terminal_receipt_test.go 使用 | 移出生产，测试 helper 保持原资产字节与摘要算法 |
| TerminalConfig、TerminalOwner 及 TerminalConfig.UnmarshalJSON | Profile.Terminal、冻结解码、原 owner/receipt 证明、usage_binding 与 usage collector 历史夹具 | 原声明/tag/字段顺序/严格解码保留，不能整删 terminal_runtime.go |
| frozenRecord.validateTerminalReceipt/terminalOwnerBinding/verifyStopped、readTerminalOwner | 比较记录中的原 EvidenceSHA256/BundleSHA256/RuntimeSHA256 和原 owner/Job/退出事实 | 原实现和缺失/错身份/篡改拒绝保持；不读取当前 asset 来补原证明 |
| extensions/pi-evidence.mjs、pi-managed.mjs、terminal-tools.mjs、receipt.mjs、pi-evidence.test.mjs | Go 原资产摘要夹具；Node 契约测试；probe-orca-pi.ps1 的 Pi 诊断及其前置测试 | 五个文件的内容和路径保持，不整目录删除 |

TerminalMetadata 虽有 JSON 标签，但已不是任何当前持久 Record/Profile/Operation 字段或历史 decoder 的对象。允许删除的只是这一无消费者生产端类型；实际冻结配置中的原摘要字符串不能随类型删除。实施前再次以全仓引用、结构字段和合法入口确认，不仅按类型名或旧注释判断。

## 精确删除与测试侧装配

删除 terminal_runtime.go 的 TerminalRuntimeMetadata、TerminalMetadata、TerminalConfig.Command；生产不再声明 terminalAssets embed.FS 或 TerminalExtensionDigest。只清理由这些项独占的 import/注释，不删除其它实际 format/reader 声明。

在 `_test.go` 中提供用途明确的历史资产 helper，可沿原相对 extensions/ 路径使用 Go embed。摘要必须仍按 pi-evidence.mjs、pi-managed.mjs、terminal-tools.mjs、receipt.mjs 的原顺序，将每个文件名和原文件字节依次写入 SHA256，不增分隔符、改编码、换序或静默忽略缺失。capture/read 错误应在测试装配时明确失败，不以空文件生成“可用原摘要”；这不是给生产读取新增校验。

更新原 access_mode_test/terminal_receipt_test 的调用对象或测试 helper 名称，保留所有有效断言。TestOriginalTerminalOwnerNeverInfersMissingBundleFromCurrentBuild 仍要先验证合法原 bundle，再清空冻结 bundle、提供当前测试资产摘要并拒绝；不能把反例删掉，或用固定空串让其无条件失败。receipt 测试仍使用与原资产一致的 source/bundle hash、原 instance/model/effort/权限与 turn 绑定。

不新增通用资产注册表、生产资产提供接口、build-tag 回退、启动时扫描源码目录或跨构建摘要缓存。不复活旧 helper执行，也不把当前 JS 打包算法变成历史证据验证依赖。旧 hash 未知仍未知/拒绝，不从当前构建推断。

## Node、诊断脚本和其它共享消费者

五个 JS 文件保持原字节和相对 import，包括 pi-evidence.test.mjs 对 terminal-tools/pi-evidence、pi-managed 对 SDK builder/pi-evidence/terminal-tools、其它文件对 receipt 的引用。即使 pi-managed 没有当前生产启动入口，它仍是原 bundle 摘要组成，不能在搬 embed 时连带删除或改写。

保留统一 L1 的 Node 命令和 TAP/结果检查：`scripts/test-prerequisite-contract.ps1` 仍调用 `node --test --test-reporter=tap internal/execution/adapters/orcatransport/extensions/pi-evidence.test.mjs`，不改报告、跳过或前置资格规则。保留可单独执行的 `node --test internal/execution/adapters/orcatransport/extensions/pi-evidence.test.mjs`。

最小验收实际执行上述带 test-reporter=tap 的命令，保存真实退出码和原 TAP 输出，按既有入口条件核对 tests>0、fail=0、skipped=0 及计数一致。不能仅凭普通 Node 命令成功或脚本字节未变推定统一 L1 的 TAP 输入有效；该局部验证不构造完整 L1 报告，也不虚记完整前置通过。独立普通 Node/诊断入口的路径与语义仍保持。

保留 scripts/probe-orca-pi.ps1 的所有脚本内容和原路径：先运行 Node 测试，再复制 pi-evidence.mjs/receipt.mjs 到明确的新 evidence 目录。既有临时独立无 remote 仓库、版本限定、路由约束及有限原 TUI 诊断语义保持；它不是当前 Runner 结构化能力的资格入口。本项默认只静态核对脚本/路径，不执行会创建会话或消耗模型的 probe，也不联网安装 SDK/Node/Pi。保留 docs/testing.md 的上述入口及历史资格，不追认新真机结果。

保留 #127 的 frozen/current/capability 夹具与 fakeHost/request/bind、creation owner、IPC、原终端并发、只读白名单及旧快照 ShutdownLifecycle 拒绝断言；保留 usage_binding_test 和 usage/collect/collector_test 的 TerminalConfig 消费者。共享 codex/process/terminaltools 包不在闭包，不因少一个 import 顺带退役。InputContract 仍参与 resources.ModelInputContract/资源摘要，本项不修改它。原始查询/订阅及六组十四个 execution-stability 场景也不变。

## 持久格式、执行与恢复停止承诺

相对 A 通过点，state/v8、user_version=8、Request/Record/Operation/Receipt、Profile.Terminal/TerminalConfig/TerminalOwner/receipt、State/Turn/NativeProof、结构化状态/RuntimeFence、所有实际持久读写/decoder、wire、CLI/Workflow/capabilities/resource_digest 不变。不删除旧字段、不重写/回填/迁移原记录，不改变新写入配置或冻结输入字节。只允许源码/构建 source_digest 和新产物 EXE 哈希随源码清理正常变化；五个 JS 原字节、四资产原 bundle/evidence SHA 及已存 RuntimeSHA256/BundleSHA256/EvidenceSHA256 不变，不要求新旧 EXE 哈希相同，也不使用新 EXE 哈希替换原 RuntimeSHA256。

允许的源码差异包括无消费者的 TerminalMetadata 类型及两个生产函数/方法删除，原 embed/摘要计算转为测试私有。不能用“全部生产类型/函数必须不变”的笼统断言阻止已批准的删除，也不能因此放松其余持久声明和编码检查。旧/新代表 Profile/owner/receipt/State/Record/Operation 的独立原字节及四资产 bundle/evidence SHA 必须相同。

PC-R01–R10 与 A 的边界保持：原事务/确认、原结果归属、预算、unknown 原操作对账和零重放、同构建原资源恢复、停止撤销与原 scope、结果收取/封存、观测容错及升级排空不变。原 close/退出/owner 证明不足仍等待或未知，不能从删除的生成函数推导成功或用当前 hash 填空。公共终端、直接 Codex、独立 parser 及当前 Codex/Pi/本机/配对远端支持范围不变。

## 实际收益、最小验收与结束范围

收益是生产 adapter 只保留旧证据解释，不再装配无人使用的 helper 元数据/命令生产端；测试资产有明确所有者，减少将旧构造误作当前入口的搜索分支。不宣称 EXE 一定变小、任务提速或全局取消 Orca→Codex 依赖。

| ID | 必须取得的结果 |
| --- | --- |
| HPR-B01 | 全仓消费者/合法入口核对：生产无旧 metadata/命令生成及 asset embed/摘要提供，TerminalConfig/owner/receipt/历史解码真实消费者保留；无替代生产壳 |
| HPR-B02 | 四资产原字节、文件名/顺序和 bundle/evidence SHA 相同；五个 JS、probe 脚本及统一 L1 Node 入口/检查原字节不变，所有相对路径仍成立 |
| HPR-B03 | 原缺摘要不能用当前 bundle 补足，owner/receipt/权限/model/effort/原输入等反例保持；共享 fixture、usage/历史读取及 Node 原 TAP 命令有效执行，真实退出码和 tests/fail/skipped/计数满足原条件，不冒充完整 L1 |
| HPR-B04 | 明确区分可删 TerminalMetadata/producer 与实际持久类型；原持久/wire/capability 声明及代表编码相同，A 的原始查询/订阅边界未回退 |
| HPR-B05 | 完整受影响 adapter、相关应用/host/usage 定向、Node 原命令、live driver/helpers 编译及正式 EXE 的定向公开执行/恢复/停止通过；A 协议包结果按相同声明核对后明确复用或实际重跑，独立记录源码、产物、失败/skip 和未验资格 |

HPR-B01–HPR-B05 完成即结束，不删除 JS/Node 测试、诊断 probe、InputContract、历史证明或稳定性 driver。发现实际生产调用、需要修改上述持久/wire/停止合同或新增兼容执行器时先修订范围。实际实施结果见[独立验收](../acceptance/orca-helper-producer-retirement.md)；未部署，不扩大完整级别或真机资格。

## 设计审计修订

2026-11-06，按上述主线静态核对 producer/metadata 引用、TerminalConfig/owner/receipt/usage 历史消费者、五个 JS 的 import、统一 L1 的 TAP 计数/零 fail/零 skip 检查，以及诊断脚本的实际复制路径。删除闭包没有新增实际调用根；保留项不缩减。

确认并修订两处文档缺口：原“资产与源构建摘要正常变化”与四资产摘要不变冲突，现只允许 source_digest/EXE 哈希变化，原 JS/资产及冻结摘要不变；原最小步骤只明确普通 Node 命令，现要求实际执行原 TAP 命令并核对其原摘要条件，不以静态保留脚本代替。没有修改或运行 Go/JS/脚本，持久格式及执行、恢复、停止验收仍待实施。

## 实施状态

2026-11-06，按修订后的设计先 A 后 B 分别实施和验证，原设计审计段落保留当时文档阶段事实。实际删除闭包、测试迁移/保留、原字节、构建和公开受控结果以[本项验收](../acceptance/orca-helper-producer-retirement.md)为准；完整 L1/Controlled、L2/L3 和真实模型未运行，不以定向通过替代。
