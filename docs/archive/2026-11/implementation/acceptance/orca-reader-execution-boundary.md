# Orca 冻结证明与旧执行退役

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-05。基线 `6940c59872d2b857a97b97b0f98090493379a219`；分支 `codex/orca-reader-execution-boundary`。先修正长期组合，再写 A/B Spec+Plan，设计审计后按 A→B 实施。未部署或发布。

## 设计审计（生产代码修改前）

A 的只读根为 StopEvidenceScope/VerifyStopped，依赖原 profile、resourceState/state、control/terminal receipt 身份、结构化 agentState 和各 scope 的只读停止核验。共同严格校验应只有一份；将这些方法迁入非持久 frozenRecord，Lifecycle 通过内部视图复用，避免复制规则或为现有共享 fixture 改大批字段。冻结 reader 的 CLI 只允许原 terminal list/show 查询，保留原 FIFO/期限/配对身份，不承载 feed/observer/输入/关闭上传能力。原 live 工厂的 nil factory 错误顺序保持，A 期间旧执行仍保留以验证纯职责迁移。

B 的生产根已在 contracts/freeze、host.checkOrcaStartupState 和 application.port 三处关闭旧执行。不能因此删除旧 Profile/State/Turn/codec：scope、离线停止、usage、files 仍读取；executeStructured 直接调用 prepareWorkspace，creation owner/原 receipt/辅助 shell 继续共享。fakeHost、fixtureOn、structuredFixture、creation fixture 真实共享。旧专项执行测试可随旧分支退役，但共享支持和保留读语义必须逐项核验，不能整体删除混合文件。逐函数清单在 A 完成后、B 编辑前固定。

长期组合原表的旧 executable/helper、audited、Pi 行已按已实施退役改正；历史证据不改写，旧 scope 仍只读保留。新内容只解释现行行为，没有新增支持组合、迁移或跨构建恢复保证。

审计结论：按 A 的独立 reader 边界实施，完成其验证后再按 B 的实际可删清单推进；不先删旧解码或共享 workspace。源码/测试/代表编码基线和只读 AST 清单位于 `.evidence/orca-reader-boundary/`。

## A 独立实施（B代码修改前）

23个原只读方法的正文迁入frozenRecord，Lifecycle以非持久视图复用；只读scope/VerifyStopped直接构造reader，usage及文件状态读取也不再临时构造Lifecycle。CLI原组装/FIFO/期限/host绑定共用，proof额外限制为terminal list/show。没有feed、observer、Job、upload或Execute方法。

- `go test ./internal/execution/adapters/orcatransport -count=1 -timeout=8m -json`：213顶层通过，0失败，112.748秒。原生opt-in 3个跳过：TestOrcaNativeStateLiveLifecycle、TestShortInputProbeInspect、TestOrcaWorkspaceStartupLive。
- `go test ./internal/host ./internal/storage ./internal/usage/... -count=1 -timeout=5m -json`：85顶层通过，0失败；原4个现场opt-in跳过，详见统计。
- 所有原既有测试字节保持，新增2个reader边界测试；原固定记录/类型/能力代表编码一致。没有把编译或skip算作真机通过。

A代码与原函数迁移清单已固定后再开始B；A阶段没有删除旧执行或原测试。

## B 删除前清单审计

根据A固定调用图，当前工厂移除WithStateFactory及旧协议装配；8个公开执行/策略/恢复方法只转入结构化实现，工作区公共创建保留、旧Agent终端准备/选择尾段删除，工作区审计只保留当前结构化原源。保守名称调用图生成约80个独占函数提案，分布为旧launch/control/checkpoint、TUI input/readiness/observation、旧stop mutation和terminal helper运行函数。提案完整JSON保存在证据中；shared/exported reader函数及持久类型未作为死代码删除。编译核对若发现实际reader/shared消费者，保留该纯读函数并登记原因，不按提案强删。

旧测试分类：仅依赖退役writer的行为case/原生执行probe退役；reader、profile/codec、usage、文件及当前结构化case保留。fakeHost/fixtureOn、request/bind、structuredFixture/agentFixture、creation fixture等按实际引用保留；reader测试中原先用旧Execute准备数据的前置改为冻结原事实构造，保留原断言，不能增加第二旧执行器。application中旧终端实际派发fixture和host旧helper执行probe逐项区分，不能删除同文件的原停止授权/只读核验反例。

## B 实现复核

当前工厂拒绝旧协议，执行、准备、观察、恢复、停止、停服及手工恢复根均只转结构化实现。旧 launch/control/TUI submit/observer/stop mutation 及独占 readiness/feed 管理已移除；没有保留旧执行器供测试调用。持久 State/Turn/Profile/NativeProof、control/initial-terminal/stop checkpoint 和 terminal receipt 声明仍保留，原证明缺失或身份不完整仍不能确认停止。

实际共享消费者核对后的取舍：

- `original`/`boundTerminal` 及原终端并发读取测试保留。自动审查曾拒绝进一步删除这组读取，已采用保留读取、改用冻结夹具的方案；没有待用户批准的删除操作。
- 当前 workspace 创建、原 receipt 恢复、命名、仓库发现/重试/撤销、owner 未知/损坏/期限及未重复派发断言保留。恢复旧记录的夹具直接保存原身份/状态，不再通过旧 Execute 生成前置。
- Engine 的首个停止授权和业务反馈准入断言保留，改用已有 port stub 隔离业务规则；其旧实际终端执行专项退役。真实 Orca 构造与工厂接线仍由 `orca_structured_test.go` 及适配器结构化用例覆盖。
- 公开 creation wire/IPC fixture 还服务 CLI 排队测试，保留该共享夹具及创建恢复的五个故障窗口。只退役旧 initial-terminal 计划两个子例；receipt-before-commit 门移动到当前真实读取的 worktree.show。恢复尚未创建 Agent 的工作区核对原 runtime/worktree、`session_not_created`、无原生 close 回执及零 Agent create/send/close RPC；`CloseAttempted` 仍是原 stop-operation 标记，未改变其持久含义。
- CLI 排队七个变体保持；scope 断言改为当前冻结的 `orca_agent_session`。共享 fixture 收紧为拒绝所有 terminal send，不再放行旧 bootstrap。
- 注销只调用旧 writer 的 `orca-remote.native-input` 专项及对应四个 opt-in skip；脚本保留参数并在任何创建前明确拒绝。通用 Controlled 注册、独立模型桥、当前结构化专项和原公开场景名称保持。

## 编码及验证边界

全部 35 个原 Orca 类型声明（仅排除非持久 Lifecycle）与基线规范化 AST 一致；新增的 frozenRecord 非持久。Request/Receipt/Record/能力等代表编码在 before/A/B 三点 SHA256 都为 `e11066197caa8eed82b90b01618e241a0a083715ffa1f1be571446e51b4dce3a`。原 codec、schema/state v8 和历史 payload 未迁移或回填。

完整 adapter 验证及共享公开回归的首次失败均保留：编译期间发现的旧夹具依赖逐项处理；公开创建/排队用例原先仍要求旧 terminal session/scope，已按实际结构化冻结事实修订，未改产品执行或停止规则。新增核验曾错误要求未创建 Agent 的 `CloseAttempted=false`，现按原 stop-operation 语义改为 true，另用实际 RPC 计数证明零派发，不能把该标记当原生 close。

扩展组件回归的 workflow `TestRADefinitionRename` 失败已在独立 HEAD archive（仅原 workflow 及依赖、原 go.mod/go.sum）复现；整合后资源包 `TestSRWorkflowSelection` 的提示一致性失败也在独立目标 main 复现。这两处提示代码/资产本轮未改，不以本批改动或放宽断言处理；因此不宣称整组/完整 L1 全绿。零匹配运行 integration 会由原守卫拒绝，本批保留此守卫并用 `go test -c` 和正式 foundation 执行验证构建。

## 主线整合与最终验证

实施点 `0a374215` 保存 A→B 的独立验证成果。收尾 fetch 发现 main 已新增 #117/#118；按仓库规则在独立工作树解决 docs/testing、公开创建恢复及 CLI 排队三处真实内容冲突，以合并提交 `68c3057a` 保留两侧历史。未为制造线性历史重放或 rebase 已验证提交。

保留上游新增目录能力、`worktree-receipt-without-terminal`/`missing-creation-plan` 两个当前工作区反例、原 shell 身份及完整 RPC 审计；最终创建恢复七个窗口，CLI 排队七个变体。删除的是旧 initial-terminal 计划子例，没有删上游这两个替代反例。原子首消息、当前 create/send/history/close 的实现正文未改；共享创建只删旧 Agent 终端尾段。

| 范围 | 实际结果 |
| --- | --- |
| A 独立点 | adapter 213 通过/3 原 opt-in 跳过；host/storage/usage 85 通过/4 原 opt-in 跳过；既有测试原字节不变 |
| B 完整 adapter | 72 顶层通过，0 失败、0 跳过；整合后同包再次通过 |
| B 扩展组件（整合前） | application/host/storage/contracts/usage/execution 共 619 顶层通过，1 个已复现 workflow 基线失败，10 个原 helper/现场 skip；原 host probe 随后退役，最终 host 单独 19 通过、0 skip |
| 整合后九包 | Orca adapter/protocol、host、contracts、cli、resources、nativecatalog、metadataprocess、textanalysis：218 顶层通过，1 个已复现 resources 基线失败，7 个既有 helper/现场 skip |
| 最终公开 Controlled 定向 | 正式 foundation 构建 Runner/driver/native fixture；CLIWaitRetry、SingleExecutionColdStop、ResourceCreationRecovery、NativeCatalogPublicCLI、NativeUnknownAndStop、RestartPreservesAuthorization 六个顶层全部通过，0 失败/skip |
| 目录与前置守卫 | 整合后 catalog 冻结/旧目录兼容/冲突拒绝通过；prerequisites 75 项通过；旧 native-input 入口明确拒绝，未创建输出目录 |
| 编译及静态核对 | 普通与 live 驱动编译；未运行真实 opt-in。无旧执行工厂/RunTerminalAgent 的残留 Go 消费者，diff whitespace 检查通过 |

最终公开构建的 Runner SHA256 为 `a1a0b03257558037456138d3969d9f37e1e17b15e8e03e8dd568a8be4a17b498`；当前最终产品源码摘要与该构建一致。整合后代表编码与独立 main `a78d87da` 均为 `9af1c6d224c5c1c9bced002525ad75d3541554d41277caaa14d81fb941f0c4f7`。与早期 E110 编码点之间的差异来自上游新增目录资源摘要，不是 A/B 持久字段/codec 变化。

从 A 验证点到 B 删除 86 个旧生产函数，退役 145 个旧 adapter 顶层测试、3 个旧实际终端 Engine 执行专项、1 个旧 host 创建 probe；逐函数/逐测试清单及调用边保存在证据中。保留或改写前置的共享案例没有作为旧 writer 整文件删除。收益是当前执行派发只需理解结构化链，历史证明不再随执行构造扩展；不宣称端到端性能提升。

可复核证据归档：[records.json.gz](../../../../acceptance/evidence/orca-reader-execution-boundary/records.json.gz)。包含 A/B 来源摘要、原/最终调用图、实际删除清单、全部类型比较、代表编码、原失败与最终日志、公开构建回执和目标 main 基线复现。每条记录含 `path`、`bytes`、`sha256`、`utf8`，解压后按 UTF-8 重编码可核对原字节摘要；不含 EXE、数据库或真实认证材料。

这是受影响组件与定向公开受控资格，不能替代完整 L1/Controlled 或真实本机/配对主机/模型资格。没有部署、发布或提 PR。

## 2026-11-06 PR 前目标刷新

目标 main 刷新至 `efb024e1`（#119/#120）。在独立 worktree 解决 `workspace.go` 一处真实内容冲突：保留 #119 的共享 `workspaceCreationFacts` 及结构化调用，继续删除旧 Agent 终端准备尾段。新上游 timeout 单测的终端身份断言按本分支当前结构化夹具适配，同时保留工作区全部事实并加强 runtime/worktree、无 terminal/incarnation/PID/session 的核对；原操作对账与 owner 用例保留。没有 rebase 或重放已验证历史。

复验结果：adapter 全包 73 顶层通过；其余 host/storage/contracts/cli/usage 消费者与最终 adapter 合计 192 顶层通过、0 失败，6 个既有 helper/现场 opt-in 跳过。合并初次复验中上述旧身份断言失败的日志一并保留，未计为通过。正式 foundation 重建 Runner/driver，`TestSRResourceCreationRecovery` 七个窗口及 `TestSRSingleExecutionColdStop` 全部通过，0 跳过。本次为目标更新影响范围复验，没有重跑完整 L1/Controlled；上节两项提示基线失败的结论和资格边界保持。

单测修正后的最终构建与这次公开验收的 Runner 原字节相同（SHA256 `fe8eeb48cce9145fddccc153bac85fe02fb11463fd22d6dff562fb86a89672d2`），产品执行逻辑未为通过测试改变。追加原日志、构建回执和冲突解决差异见 [PR 前复验证据](../../../../acceptance/evidence/orca-reader-execution-boundary/pr-20261106.json.gz)。
