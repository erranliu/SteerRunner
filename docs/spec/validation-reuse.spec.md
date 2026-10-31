# Spec：验证链路解耦与前置结果复用

日期：2026-10-31。状态：已实施，完整默认前置及正式复用 L2 通过，最新资格与实现审计见[审计验收](../acceptance/validation-reuse-audit.md)，[首轮历史](../acceptance/validation-reuse.md)保留；文档审计见 [Plan §9](validation-reuse.plan.md#9-文档审计与修正2026-10-31)。资格仅按实际验证范围授予。

调查基线：`5c5c1653961ad08a836e82fc84f0898667864a5c`。前序动机见[宿主认证审计与 L2](../acceptance/host-native-auth-audit-l2.md)：完整前置通过后，仅修改 `tests/native` 的审计消费者，仍需手工拼装重跑 job 和资格来源。本轮将这一条反馈链路正规化。

## 1. 目标与范围

使完整 L1、完整 Controlled 可以分别执行并产生可验证的阶段结果；L2 原生 driver 修改后，由正式入口显式复用未受影响的成功前置，重新编译、检查 driver，并在新 fixture 中执行真实 case。完整资格仍要求全部既有前置和所请求级别的完整 case 集合。

首轮只处理 **`-Level L2` 单级别、同一工作副本、同一主机和执行账号、显式指定一个来源目录**的复用。L3、L4 和混合级别请求不开放复用；它们继续使用既有默认完整链路。采用保守的文件摘要范围，不开发自动依赖分析器。优先缩短 `tests/native/**` 单独修改后的反馈时间；其他改动可以保守地要求重新跑前置。

纳入：测试编排中的阶段边界、阶段证明、正式入口复用、固定产物传递、报告来源和阶段耗时。以下不纳入：

- 不改生产执行链路、配置/状态格式、权限、发布或停止语义。
- 不拆 Controlled 顶层测试或内部 case，不删断言、缩减循环、改等待预算、扩大 skip allowlist；不改变 catalog/registry 的场景归属。
- 不改并行上限、lease、资源互斥、管理员排除、故障注入和清理规则。
- 不缓存真实 L2/L3/L4 成功结果，不复用 fixture、Run、session、PR 或未结算外部操作。
- 不做跨机缓存、自动寻找最近成功批次、任意阶段拼装、历史报告迁移、签名服务或通用 CI 缓存平台；交付包资格仍使用原入口。

## 2. 基线事实与需切开的依赖

| 当前位置 | 耦合或限制 | 本轮结果 |
| --- | --- | --- |
| `scripts/test.ps1` | L2/L3/L4 前无条件执行 L1，再执行完整 Controlled | 默认行为保留；仅显式复用且核验通过时用阶段证明满足前置 |
| 同脚本 L1 段 | framework、Go、Node 与 native driver 编译/零匹配 guard 混在一个结果中 | 内部拆成 `l1-core` 和 `native-preflight`；对外完整 L1 仍要求两者 |
| `test-foundation.ps1` | 已能独立执行指定 Pattern；PreparedArtifactsDirectory 只证明至少一个场景通过及产物未变 | 保持原入口；新增的“完整 Controlled 结果复用”必须另外证明完整测试名集合，不能借此参数认定完整前置 |
| 正式 live 批次 | live Runner 与 Controlled 默认分别构建，没有强制为同一个文件 | 默认构建方式保留；证明绑定实际受测的 Controlled Runner，复用 L2 必须使用该 Runner 的相同字节 |
| `test-native.ps1` | 每 fixture 再编译 driver；前面编译并检查的 driver 未直接传给实际执行 | 仅新增复用 L2 路线传入已检查 driver 的路径和摘要，实际执行前再次核对；默认编译路线保留 |
| `run-test-case.ps1` 的 L4 分支 | 固定从本批输出的 `controlled` 目录取 PreparedArtifactsDirectory | 首轮不开放 L4 复用，不改其目录约定、产物来源与专属链路 |
| scheduler / case worker | 已分开汇总写入者与单 case 写入者 | 继续复用；不把复用判定放进每个 worker，也不新增另一套调度器 |

当前默认 Controlled 为 catalog 中 25 个顶层测试，`TestSRResultAcquisition` 的 11 个内部 case 保持全部执行。L2 为 `local-codex.smoke/default`；其他级别按唯一 registry 展开。数字是调查基线，资格以当前正式集合为准，不能把 25 或某次 L1 测试数量写成永远成立的替代断言。

## 3. 行为契约

### VLR-01：入口与默认兼容

1. `scripts/test.ps1` 的既有参数、默认 `-Level L1`、可用性查询及退出码语义保留。未指定复用来源时仍执行原有全部必需阶段；不能默认为寻找缓存。
2. 新增 `scripts/test-prerequisites.ps1 -Stage L1|Controlled|All -OutDir <新目录>`，`Stage` 默认 All。L1 执行完整 L1（含 native-preflight）；Controlled 只执行完整 Controlled；All 执行两者。此入口不准备真实目标、不消费模型、不创建远端 PR。其成功只表示请求阶段完成，不声明 L2–L4 资格。
3. 新增 `test.ps1 -PrerequisitesFrom <已有批次目录>`。首轮要求一个来源内同时有完整成功 `l1-core`、完整成功 Controlled 及相应产物证明；不支持多个来源合并、部分命中或缺失阶段自动补跑。不合格时明确拒绝并提示重建前置。
4. 复用参数只适用于恰为 `-Level L2` 的正式执行；与 ListAvailability、L1/Controlled、L3/L4 或混合级别请求组合时，在准备 fixture 前拒绝。既有 `-Scenario`、`-Repeat`、`-SkipAdminRequired` 的选择和范围语义保留。首轮复用只接受默认 catalog，不接受显式自定义或外部冻结 catalog；旧自定义 catalog 继续使用不带复用的原入口。来源证明也必须记录其直接执行使用了当前默认完整前置定义。
5. 默认完整执行及新前置入口调用同一组阶段函数，不能复制两套 L1/Controlled 命令与判定。`test-foundation.ps1`、`test-native.ps1` 的既有直接调用仍可用；它们单独成功不等于正式批次通过。

### VLR-02：阶段边界不减少义务

| 阶段 | 保留的义务 | 可否复用 |
| --- | --- | --- |
| `l1-core` | live opt-in 环境拒绝、全部 framework guards、catalog 的全部 L1 包、Go 原有 skip 规则、两个正式 Node 扩展零失败零跳过 | 仅完整通过且 VLR-03/04 成立 |
| `controlled` | 原 foundation 单元前置、当前默认全部 Controlled、原 harness 与内部 case、退出码/数量/零 skip、Runner 与 fixture/helper 身份 | 仅完整通过且 VLR-03/04 成立 |
| `native-preflight` | 当前 live driver 编译；零匹配运行必须退出 1，result.exit_code=1、scenario_count=0 | 每个正式 live 批次重新执行，不能沿用来源中的旧 guard |
| 当前 live case | 原有无模型 Git 边界检查、配置/能力/需求检查、fixture 准备、真实执行、发布/结果/权限审计、停止与资源结算 | 不复用；每次新现场完整执行 |

完整 L1 仍等于 `l1-core + native-preflight`。复用报告不能把旧 L1 中的 driver 摘要冒充新 driver；来源 `l1-core` 与本轮 preflight 分别列明。复用 L2 用同一个已通过 guard 的 driver 执行正式 case，禁止静默回退为未经本轮检查的另一次编译。未指定复用的原入口仍保留原编译和运行方式。

真实副作用开始前完成来源核验、当前配置可用性判定与 native-preflight。继承当前“无可运行目标则 incomplete，且不跑前置/不准备 fixture”的边界；复用不能把 unavailable 改为 available。即使复用 L1，当前进程的 live opt-in 环境拒绝检查仍执行。

### VLR-03：简单、保守的输入身份

不以 HEAD 相同、工作区 clean、Runner SHA 相同或文件时间单独作为复用依据。源码身份记录提交与 dirty 供溯源；适用性依据完整输入清单、实际字节摘要及固定产物。

首轮固定范围如下，范围定义自身也进入身份：

| 身份 | 固定输入范围及约束 |
| --- | --- |
| 前置源码范围 | `cmd/**`、`internal/**`、`scripts/**`、`tests/**`，仅排除 `tests/native/**`；加 `go.mod`、`go.sum`、`.gitignore`、`.gitattributes`、`docs/workflows/generate-repository.example.json`。包括全部 framework、integration、helper、fixtures、catalog、case registry、Node 扩展及其测试，以及 L1 实际读取的仓库外部样例。该范围对 L1 与 Controlled 共用，允许多失效，不允许猜测无关 |
| native-preflight 范围 | 上述范围加 `tests/native/**`；记录实际编译命令、driver SHA 与零匹配证据 |
| Runner | 原 build.json 与文件 SHA，原构建源码摘要须与当前 build.ps1 的同一范围一致；Controlled 与 live 使用同一 Runner 字节 |
| 工具与执行环境 | 同一规范仓库路径、主机、执行账号 SID、Windows/架构；实际解析到的 Go、Node、Git、宿主 PowerShell 和 Windows PowerShell 路径及版本；Go 的目标、GOAMD64、GODEBUG、CGO、GOFLAGS/GOWORK/GOTOOLCHAIN 等设置。非空 NODE_OPTIONS 不支持复用；记录阶段实际命令和选中的包/测试名，不枚举完整 PATH、用户环境或登录配置 |

采用固定 ordinal 排序的相对路径/文件 SHA 清单；新增、删除、修改都导致身份变化，未跟踪的输入不能漏掉。原始清单保留以说明失效原因。不把仅有 commit/dirty 的旧 source-identity 当完整输入清单。沿用现有摘要工具时，接受其保守失效，不为跨提交复用改写构建回执。若发现被忽略文件、外部 workspace/local replace、vendor、CGO 编译器输入、overlay、额外构建参数等不能被范围描述的实际输入，首轮拒绝复用，提示使用默认入口；不得标为命中。当前复用限定 CGO_ENABLED=0，记录 GOROOT；默认入口仍可按原设置执行。

只改 `tests/native/**`，或只改未列入输入范围、未参与构建/测试的说明文档，可以复用；`docs/` 不是一概无关，例如上表 JSON 样例被 `TestContentRepositoryPublishedSchema` 直接读取，修改必须失效。修改 `scripts/test-native.ps1` 等脚本在首轮仍会使前置失效。这是有意限定，不承诺所有“测试专属改动”都可复用。共享 helper、内嵌资源、扩展、integration、framework、catalog/registry 或 Go 依赖改变均要求重跑完整前置。

不需要冻结整个用户环境或读取登录凭据。上述身份不代表复用原机的模型/Git/Orca 可用性；真实目标、当前配置和原生工具资格继续在本轮按既有规则验证。首轮不增加跨主机复用、TTL、账号权限证明或凭据缓存。

生产构建回执、阶段来源及本轮 source_commit/source_dirty 分开记录。文档提交可以改变 HEAD 而不改变相关输入，不能将旧二进制的 SourceCommit 写成本轮 HEAD；dirty 工作区在清单完整、输入稳定时可产生证明，但不能被报告成 clean。

### VLR-04：成功阶段证明与消费

新增测试私有 `prerequisites.json`（`steerrunner-test-prerequisites/v1`），只存阶段结果与必要来源，不改变生产格式。最小内容：范围定义版本、每个阶段各自的开始/完成输入身份、环境身份、实际阶段集合、各阶段完成状态、预期与实际包/测试名、skip、退出码、必要证据文件 SHA、被测/测试产物及构建回执 SHA、原始来源目录、开始/结束时间。两个前置的输入身份必须分别匹配当前状态；不能用最后一个阶段的身份覆盖前一阶段身份。

1. 阶段执行前采集源码/环境；产物构建后、实际执行前记录产物身份，执行后重新核对。所有原判定通过且已确认 harness 退出、原始日志/结果完成后，才原子写入该阶段成功证明；仅出现 result.json 不能证明子进程完成。沿原期限等待，不更换 CIM 启动机制或引入进程监管器；不能确认完成则不产生可复用成功证明。不存在、进行中、失败、未知或证据缺失不能作为成功。
2. L1 从原 Go JSON、Node TAP 和 framework 结果核实全部选定包已完成、非零测试、无意外 skip；不能仅凭一个 test_count。Controlled 从原始日志核实精确的顶层测试名集合、全部 PASS、无重复/漏项/额外匹配/skip，并继续检查 result.json 的退出码和数量。子测试不误计成顶层测试；原内部断言不删。
3. 消费者重算当前输入和来源实际证据/产物摘要，核实预期集合与当前默认定义一致。L1 的包通配符须在生产证明时展开成实际包集合并记录，消费时与当前展开集合一致，不能只保存 `./internal/...` 字符串。Controlled 的默认名单来自当前仓库定义，不信任来源自称的 expected 名单。只接受直接执行生成的阶段证明；本轮引用不能递归转成新的“原始执行证明”。
4. 来源总批次可以因其后的 native case 失败；仅在该来源两个前置各自已有完整成功证明时允许引用。来源 summary 的 failed 状态、错误和原文件保持，不追认原批次通过。失败的前置绝不允许引用。
5. 新报告写入来源 manifest 和各证据摘要、阶段实际完成时间以及本轮校验结果；复制或引用必须保持原始字节和可解析路径。缺文件/摘要变化则拒绝，不能只信 copied status=passed。
6. 消费前、启动 worker 前及结束汇总前复核相关输入/产物。中途变化使本批资格失败，仍保存实际 case 结果并按原逻辑收尾；不通过重跑外部动作补证明。这里检测意外变化，不新增恶意本机对抗机制。
7. 未包含新证明的历史报告仍可阅读，首轮不能用来复用，也不扫描旧日志补签。首次使用需要新入口或实施后的默认入口生成一轮前置。
8. 生成可复用证明的严格身份要求只控制复用资格。不带复用的现有入口仍按原规则执行和报告；显式 Runner 没有 build.json、存在不支持复用的构建输入或自定义 catalog 时，不因此拒绝原本可执行的测试，不补造回执，只标明该批次不能作为复用来源。

证明只绑定已完成阶段的固定证据清单：L1 的 Go JSON/诊断、Node TAP/诊断、framework 结果及其完成证据；Controlled 的 unit-foundation.log、integration.log、result.json、source-identity 和固定产物。不能把仍会随 native case 更新的批次 summary、case 记录或整个输出目录摘要纳入前置成功证明；否则后续 native 失败会使原成功前置无故失效。两个前置封存后，后续 native 只更新原批次结果，不改写这些前置证据。

Controlled 产物包括 Runner、integration driver、native-fixture 及原始证据。live 的 test-config helper 与 native-fixture 即使来自同一 Go 包，也不是同一份产物证明；前者在本轮构建或核实显式提供的文件，并沿原能力/requirements 检查绑定，不用 fixture helper 的 SHA 代替。

### VLR-05：固定产物与原生重跑

默认入口保留既有 Runner 选择、构建顺序和显式 Runner 支持，证明记录 Controlled **实际测试**的 Runner；不能拿 live 条目的 runner_sha256 代替。新前置 All/Controlled 沿原 build.ps1 生成 Runner。仅复用 L2 默认选来源中已通过 Controlled 的 Runner，不重新 build 替换。若用户仍显式提供 Runner，必须同一 SHA 与有效构建回执，否则在 fixture 副作用前拒绝；不能为了命中忽略用户指定产物。来源两个前置不能用不同输入状态拼成一个成功来源。

复用执行顺序固定为：校验请求和来源身份 → 选择来源 Controlled Runner（核对显式覆盖）→ 构建或核实现有配置 helper → 当前 `Get-SRTestConfiguration` 评估和选择展开 → 无可运行目标则原 incomplete 早停 → 冻结当前配置/catalog → 当前环境拒绝检查和新 preflight → 原 scheduler/worker。身份核验和离线评估允许在早停前执行，但不允许提前启动 L1/Controlled、模型或 fixture；不能先按默认路径重建 Runner 再尝试复用。

catalog/requirements 核对当前定义，不继承旧 available 或 l2_verified。worker 继续核对冻结配置、Runner、helper、requirements；仅复用 L2 增加已检查 driver 身份的传递与执行前核对。路径/摘要两个新字段都缺省时，旧 job 和不带复用的调用继续走既有路径；任一字段出现则必须成对齐全且匹配，否则拒绝，不能降级回自行编译。固定 Runner 不等于冻结旧账号、旧模型配置或旧目标。

每次重跑使用新输出目录、新 fixture、新 case/repeat 记录，经原 scheduler/lease 执行。前一轮有 unknown/未停止资源时，保留原现场并遵守现有冲突和收尾限制；复用不豁免这些限制，也不新增全局阻塞规则或自动清理未知现场。原 L2 仍执行真实提交、push、PR 核验、专用测试 base 合并、合并后检查及停止；真实权限/命令审计、反馈拒绝零新输入等不改。

### VLR-06：报告和完整资格

保留 `steerrunner-test-report/v1` 既有结果和 coverage 集合字段，新增可选阶段来源字段；不删除旧字段、不把 unavailable/excluded/queued 变成 passed。旧消费者所需 L1 字段仍报告，但 fresh/reused、原完成时间、本轮 preflight 摘要必须另行说明。改造时盘点全部消费者，不能仅保证新 writer 自己能读。

正式报告 passed 必须同时满足：本轮实际通过或有效引用的 `l1-core`、当前 `native-preflight`、本轮实际通过或有效引用的完整 Controlled（请求需要时）及原 outcome 规则。仅执行 Controlled 阶段也必须满足 Controlled 判定，不因没有 live case 而漏验。

完整级别资格还要求当前 registry 下请求级别的全部 `(scenario, case, repeat)`。保留原 registered/requested/executed/passed/missing 等集合，不用集合去重掩盖缺失 repeat。最终资格闸门在输入/产物复核之后执行：复核失败，即使所有 case 已通过，也必须 status=failed、相关 `complete_level_passed=false`、`baseline.l2_verified=false`；保留实际 passed case 集合供诊断。禁止 finally 中再按 case coverage 将这些布尔值覆写为 true。正常完整批次、原排除/不可用规则保持；这里明确收紧失败批次的资格布尔值，不能声称完全不变。

`-Scenario` 选择与管理员排除继续如实记录其范围，不能用复用补齐本轮未执行的 case 或扩大为完整目录；原重复运行计数、first_pass_rate 的既有批次口径保持。历史失败通过来源引用可见，本轮通过不称为“首次即通过”。

报告增加各阶段 `executed|reused`、原完成时间、本轮验证耗时、实际执行耗时和总耗时。复用耗时不能写成来源执行耗时；preflight、准备、native 和收尾按实际记录。性能目标以无需重新启动 L1/Controlled 进程为准，不通过缩减 case 或硬定模型耗时达成。

现有 native harness 自行完成收尾，本轮不为分拆计时改写 driver：case 的 `native_and_cleanup_seconds` 如实包含执行及原清理；未取得的独立清理时长不伪写为 0。fixture 准备、Git 边界及总耗时分别记录。

## 4. 使用方式

```powershell
# 独立生成完整前置；不需要真实模型目标
.\scripts\test-prerequisites.ps1 -Stage All -OutDir C:\srt-evidence\prereq-a

# 仅修改 tests/native 后，显式复用前置，仍完整执行当前 L2
.\scripts\test.ps1 -Level L2 -Config C:\srt-config\local.json `
  -PrerequisitesFrom C:\srt-evidence\prereq-a -OutDir C:\srt-evidence\l2-b

# 原正式入口保持：不指定来源则完整重跑前置
.\scripts\test.ps1 -Level L2 -Config C:\srt-config\local.json -OutDir C:\srt-evidence\l2-full
```

单独 `-Stage Controlled` 的成功不能直接供上述复用，因为缺少同来源的 l1-core。既有 focused `test-foundation.ps1 -Pattern ...` 仍可做开发反馈，不升级为完整 Controlled。

## 5. 验收矩阵

| 编号 | 必须取得的证据 |
| --- | --- |
| A1 默认等价 | 不带复用的 L1、Controlled、L2 路径仍要求原全部阶段；25 个当前 Controlled 顶层及全部内部 case、Node/Go/framework/zero-match 判定不减；注册集合和超时不改 |
| A2 独立阶段 | 前置 All/L1/Controlled 各执行准确阶段；无模型或远端 PR 副作用；单阶段不声称完整 live 资格 |
| A3 正向复用 | 隔离合同验证 native-only 修改保留前置命中、重新编译并检查新 driver；最终代码通过正式复用入口在新现场取得真实 L2，未启动 L1/Controlled，实际 driver 与本轮 guard 一致。若有必要的 native driver 修改，合并为同一次真实验证；不为改变 SHA 添加无用代码 |
| A4 保守失效 | 分别改变生产源码、共享 helper、integration、framework/script、扩展、catalog/registry、参与测试的 docs JSON、Go/Git 等工具身份或构建参数、主机身份，均在 fixture 副作用前拒绝复用；无关说明文档可复用 |
| A5 结果可信 | 缺失/变化的产物或日志、旧 schema、零测试、Controlled/Node skip、L1 意外 skip、同数量但不同测试名、子集冒充完整、失败/运行中前置都拒绝；L1 原允许的 opt-in skip 不阻止复用，也不计真机通过；原失败批次中独立成功前置可用且原失败保持 |
| A6 driver 与稳定性 | 复用 L2 实际 driver SHA 等于本轮 guard 的 SHA；编译/guard 失败不建现场；执行前或结束时输入/产物变化使资格失败、资格布尔值为 false，实际收尾不省略；结果文件先出现但 harness 未结束不能产生成功证明 |
| A7 资格不扩大 | 部分场景、管理员排除、unavailable、缺 repeat、case 失败或清理未结算保持原 outcome；不能借旧 native 结果补齐；复用 L3/L4/混合级别明确拒绝，原不带复用的链路和可用性查询不变 |
| A8 兼容和追溯 | 现有 framework、配置/目录、scheduler、PreparedArtifactsDirectory、delivery guards 均通过；原脚本参数和 summary 消费者可用；阶段耗时与来源可核查 |

## 6. 完成边界

完成指 A1–A8 有对应证据，默认无复用路线与显式 L2 复用路线都可执行；同一固定 Runner 上取得一次完整前置和最终代码下的一次真实复用 L2，并由正式报告自动表达组合资格，另以隔离合同验证 native-only 变更命中。合成 framework 反例不授予真实 L2/L3；本轮不开放 L3/L4 复用，未执行的真实目标继续列未验。

完整 Controlled 分组/内部 case 拆分留到下一轮，需先用本轮阶段计时确认瓶颈，再独立评估共享状态和覆盖等价。本轮不以任何测试义务的删除换取提速。
