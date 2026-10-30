# Orca 原生输入可靠提交 Spec

日期：2026-10-30。调查基线：`a086a9984dc0bd32b3f02b9698cd26c9ba313d90`。
状态：**重新核查后修正归因：Runner 多行路径绕开通用 prompt 提交；尚未证明 Orca 违反既有契约。** 原始强完整性目标尚未获证，不能冒充已实现。详见[重新核查](../acceptance/orca-native-submit-reassessment.md)。用户随后明确不修改 Orca、只允许查询和明确无输入副作用操作的有限重试；该 Runner 范围不以新增 Orca API 为前提，仍须单独实施/验收。
配套 [Plan](orca-native-submit.plan.md)、[调查与验收](../acceptance/orca-native-submit.md)。

## 1. 目标与范围

Runner 将原生 Codex 输入交给 Orca 的单一通用提交能力。本机与远端调用同一代码、请求协议、归属核验和失败规则，只改变目标连接；不能按是否多行选择提交算法。其它 agent 的 extension 契约不在此次重写范围。

当前本机 Codex 还可能采用 `native-command-v1/native-model-command-v1` 包装、`observeRecorded`/control helper 完成查询。不能只统一 `submit` 而留下它作为本机专用输入或完成旁路：新原生 Codex 回合的冻结正文编译、强提交、原全文查询和 final 核验必须统一为共同原生链路；清理相应本机 wire/observer 选择分支及仅供其使用的调用。现有本机执行主机准备、认证资格、模型参数传递和停止证明不得静默删除；若它们与本机 wire/reader 耦合，须在 P2 提供共用原生证据且保持这些保证后才删除。无法做到即继续阻断，不切换 structured runtime、不新建本机 helper，也不能以远端结果代替本机验证。

提交必须在任务可能执行前保证：冻结正文按约定完整进入原会话，提交不混入原有草稿或其它写者输入。执行后的原输入查询是第二道归属核验，不能倒推提交时已经安全。

不涉及环境检测业务、Unity、SteerLine、项目脚本或检测材料；不部署、安装、升级 Orca/Runner，不改全局配置。无新执行框架、第二套状态机、旧路径回退或实验模式。不能将 native terminal 换成 structured chat 来取得不同的执行/凭据契约。

## 2. 当前代码核实

| 位置 | 当前事实 | 限制 |
| --- | --- | --- |
| `orcatransport/lifecycle.go:submit` | 冻结 `Turn.Prompt/InputDigest`，生成 `Request`，记录 `Attempted`；远端普通 Codex 多行走 `submitNativePaste` | 本机/远端及单行/多行目前没有统一输入流程；多行原始 paste/send 不携带 `Turn.Request` |
| `native_paste.go` | 空草稿预读；一次 bracketed paste；轮询草稿、相同投影连续匹配两次；可能 fit/restore；查原主代理基线；一次 Enter | screen 是显示投影；空草稿检查与写入间存在竞争；字符数折叠标签不证明正文完整 |
| `lifecycle.go` 通用 send 分支 | `terminal send --text --enter --retry-request`，远端 v4 加 `--wait-submit 20`；检查 runtime/handle/incarnation/request/stages | `input_accepted` 与 `turn_started` 分离，但没有提交前完整正文消费/独占输入证明 |
| `native_start.go` | `tui-idle` 是准备；独立原输入及新 done 查询可恢复漏采的开始事实 | idle 不是空编辑缓冲、完整提交或完成证明 |
| `native_state.go`、原生查询协议 | `NativeProof` 保存 pane/provider/authority/revision/主回合时间、冲突和原结果摘要；正式查询核验唯一原输入及进程绑定 | 只能在执行后查到错误；不能阻止错误正文先执行 |
| `protocol/orca/terminal_viewport.go` | 临时 `resizeForClient` 与所有权绑定的恢复 | 仅为了读取正文；不提供输入消费确认 |

现有规范化保持：`nativePromptText` 只将 CRLF 转 LF，并去掉尾部 CR/LF；不能宽松折叠空格、内部空行或显示换行。保留冻结逻辑文本及其摘要，另核验规范化后的 wire UTF-8 长度/摘要。裸 CR、无效 UTF-8、NUL/ESC 等按现有输入预算/安全契约处理，不借新提交扩张可接受输入。测试必须明确预期规范化，不能宣称尾部换行逐字保留。

## 3. 五种事实与权威来源

| 事实 | 权威凭据 | 能证明 | 不能证明 |
| --- | --- | --- | --- |
| 传输写入成功 | 原 Orca PTY/transport 的写入 settlement，绑定原进程及请求 | 对应字节已交给指定传输层（按 settlement 实际语义） | TUI 消费完整正文、Enter 生效、任务受理/开始/完成；旧 `accepted/bytesWritten` 甚至可能是计算值 |
| 完整输入可靠提交 | **待 Orca 补齐**：原会话输入消费边界上的完整正文长度/摘要核验、排他输入与单次提交的权威回执 | 消费了确切正文后，向原会话提交恰好一次；没有混入草稿/并发输入 | 模型已经开始或完成；工作流成功 |
| 原任务受理 accepted | 原请求 durable receipt，绑定请求身份、原 runtime/terminal/incarnation/pane/provider 和正文摘要 | Orca 已受理此提交责任；若仅入队，仍须等待完整提交事实 | 原 provider 已消费/执行；不得将单纯入队或 PTY 接受当完整提交 |
| 原任务开始 started | 同请求的原 provider/native 主回合开始凭据；既有独立唯一输入查询的补证规则保留 | 原任务确已开始 | 普通 working/输出变化不能独自证明原正文归属；开始不等于完成 |
| 原任务完成与结果归属 completed | 既有 `NativeProof`、`QueryOriginalInput`/原主代理新 done、完整 final 与摘要、无边界/中断/冲突；随后 workflow 验证 | 原输入对应的原回合已经结束，原完整结果可归属 | 任意业务/发布已经成功；保留 `unreported/failed/cancelled/unknown` 的区别 |

新提交回执不得替代正式原输入和 final 核验。`accepted=true` 可以早于 committed，但不得因此设 `Started/Settled`。本文的 committed 是 Orca 回执事实，不是新增 Runner 调度状态。

## 4. 现有 Orca 通用入口审计

本机及配对远端当前只读 status 均为 Orca `1.4.242`，均公布 `terminal.prompt-delivery.v1`。CLI 的既有 `terminal.send` 携带 agentPrompt、mutation 请求身份；`--wait-submit` 观察同一请求，不因观察超时重发。优先复用该入口和 mutation journal，而非增加 Runner 的 paste/Enter 驱动器。

本机安装包 `orcad-template/orcad-server.js` 中核实：

1. `writeTerminalAgentPrompt` 写 bracketed payload 后等待 render gate，再写 CR；`createAgentPromptRenderGate` 可由静默/截止定时器完成，不是正文消费确认。
2. `resubmitHeldAgentPrompt` 可读取 screen draft，经 `Fas` 宽松匹配后再写 CR；`Fas` 可折叠空白、接受后缀和折叠字符数范围。因此直接复用仍有屏幕依赖和补 Enter。
3. `bytesWritten` 用 payload 长度加提交次数计算；`input_accepted`、`turn_started` 未附完整消费正文的长度/摘要。
4. `Uie` 以原 activity sequence/hook 观察开始，不核验正文。CLI 请求没有 provider-session/composer revision/正文消费条件；仅靠调用前后检查有 TOCTOU。
5. 原生 prompt lane 串行化提交，不足以证明所有人工/其它终端输入渠道共享排他边界。已有 `orca orchestration request-show --request <id> --json`，按同一 authenticated caller 查询 `terminal.send` 的 durable mutation；先前调查漏查，现已修正。本机/远端均实测此只读原请求入口可用。它返回保存的调用回执，不提供额外的消费全文证明；`completed` 不是模型任务完成，`absent` 也不证明无副作用。

CLI 只有同次调用的 `text + enter + !interrupt` 才启用 `agentPrompt`，触发能力预检、durable 请求与原生 prompt 提交。Runner 的多行 text-only paste 和 bare Enter 均绕开它。通用入口默认观察0秒，只有 `input_accepted` 属预期结果；`--wait-submit` 可以继续观察。内部有计时/held-Enter恢复，以及回执不包含正文消费摘要，说明实现/保证边界，不能单独定性为 Orca bug。

这些是当前安装包的静态证据和隔离函数行为，不是历史故障的完整原生复现，也不证明远端实际执行的 bundle 与本机模板逐字一致。远端目前只核实版本、runtime、capability；其服务端源码及强提交语义须另行提供证据。共享 capability **没有声明**上述强保证，不能仅凭同版本认定远端满足要求。

## 5. 原始强目标与既有契约的差距（不是已证实的历史缺陷）

涉及组件：**Orca 执行主机的 native terminal prompt delivery、原生 provider 输入接入及 mutation journal**。PTY write acknowledgment 不能升级为 provider 消费证明。如果未来仍要求原始强提交目标，须核实消费/排他证据；若无法证明则不得宣称强保证。这不表示 Orca 已违反其公开的 `input_accepted/turn_started` 契约，也不能据此认定历史根因。用户已明确不能修改 Orca；先复用既有能力处理 Runner 用法与有界恢复，不新增安装或上游接口依赖。

沿用 `terminal.send` 和既有 requestId，增加严格提交契约；以下是给 Orca 的接口要求，字段名待上游确认后才落实，不是 Runner 可以自行假造的协议：

```text
terminal.send
  existing: terminal, text, enter=true, agentPrompt=true, requestId (mutation envelope)
  strict precondition:
    expected runtime + terminal incarnation + pane + provider session
    expected native input boundary/revision
    wire UTF-8 byte length + SHA-256 + require empty/unowned composer
  receipt:
    same identity + input length/digest + accepted
    committed proof (authoritative consumed-input boundary, single submit ordinal)
    optional original turn-start marker (separate from committed)
  existing read-only query (reuse, not a new API):
    orchestration.requestShow / CLI orchestration request-show --request <id>
    same caller identity + requestId -> retained mutation receipt / pending / absent
```

要求：

- 能力版本必须明确承诺严格语义；客户端/host 均不得静默丢弃 strict 条件后使用旧 send。优先扩展既有 capability/receipt；必要时新增一个严格能力版本，不按 Orca 版本号猜测。
- 条件检查、排他所有权、正文消费校验及唯一提交在执行主机原输入边界完成。所有 UI/CLI/其它写者共享围栏；已有草稿拒绝；竞争者须在提交前被拒绝或使本次提交无执行地冲突。不得清空、覆盖或合并他人正文。提交完成后的新输入是新边界，不能重归属旧请求。
- 分段/延迟粘贴必须等待真实消费完成，不等待固定时长。长度/摘要不符时不执行；若不能确定有没有执行，返回 unknown 并保留原因，不尝试修复正文或补 Enter。
- 同一请求身份永久绑定冻结 payload 和原 incarnation/session。重连/服务重启只读查询，不能再投递；duplicate 返回原事实或 conflict，不执行第二次。journal 过期、丢失、进程重启或查询缺证保持 unknown，不能把 not-found 当首次发送授权。
- 持久 intent 必须先于任何输入副作用；提交写出后崩溃但无回执不能恢复性重写。查询须不触发写入，也不能自动完成尚未提交的请求。复用既有 mutation journal，不另造执行引擎。
- 开始与完成仍由原 provider/native 证据证明；强提交保证正文正确，并不让任意 activity 自动升级为本请求开始。停止清理仍限定原资源范围。

以上 strict 字段是原始强目标的候选要求，并非当前 API 参数；禁止直接发送后假设生效。回执恢复已有现成接口，不再要求 Orca 新增。既有去重依赖同一调用者、原进程绑定和保留记录；completed记录有30天及10000行容量清理规则，不能承诺永久去重。`absent`、身份改变或 `operation_unknown` 不授权重投。

## 6. Runner 实施与删除门槛

只有 §5 的原始强保证在两目标可验证，才可宣称完整正文提交/排他保证并按本节完整删除门禁。不能先删除门禁后宣称强目标已满足。但纠正 Runner 的通用入口用法、复用既有回执查询和有限无副作用重试，可在不修改 Orca 的范围另行审计/实施；这不自动等于本节强目标完成。

| 删除对象 | 连带清理 | 保留边界 |
| --- | --- | --- |
| `native_paste.go` 全部 | comparator、标签 regex、显示 LF 推断、两次 stable、250ms 轮询、40s paste 专用期限、composerViewport | 输入控制字符检查移至共同强提交前置，不能随文件丢失 |
| `Lifecycle.fitComposer` 和初始化分支 | `FitBoundComposer`、`ComposerViewport`、`ViewportRPC/Recorder`、fit/restore receipt、仅服务于此的协议测试 | 先做引用盘点；其它用途的 viewport 能力不误删 |
| 单独 paste/Enter 分支 | native single/multiline、local/remote 分流；仅为旧草稿门禁的 fixture 字段/断言 | existing terminal readiness、身份绑定、冻结/规范化、未知零重发和停止继续保留 |
| 本机 Codex 专用 wire/完成选择 | `prepareInput` 的 native-command/model 包装选择、`observeOnce` 的 `observeRecorded` 选择及仅为此使用的 reader/receipt 接线 | 不误删其它 agent extension；原有准备、认证、模型参数、停止证明须有等价共同接线；无法保留则阻断 |
| 草稿形态/重绘专用测试与文档 | `native_paste*_test.go`、`native_reflow_test.go`、viewport 测试、live probe 的草稿/resize门禁和 segmented-shape 专用模式 | 将输入内容、归属/故障反例迁入共同提交行为测试；原始历史 evidence 不改字节、不改判通过 |

本轮引用盘点未发现公开 profile 中专用 fit/stable 配置；不得因此删除仍用于其它等待的 `WaitTimeout`。实施时重新用全仓搜索核对配置、状态、脚本、测试目录和当前文档，删除生产新旧双路径；旧未知记录仅只读/精确停止，不补迁移为 committed。

复用现有 `Turn`、operation journal、`ProcessIdentity`、`NativeProof` 与 envelope；只增加保存权威提交回执所必需的字段及保守 codec 边界。缺强提交 proof 的旧记录不能自动升级；未知请求继续只观察/停止。所有动作均依既有输入授权，不能因能力补齐重派历史 Run。

## 7. 验收条件

Plan 的 NS-T01–T10 为待实施测试需求 ID，不是已注册 case。确定性夹具必须能控制 provider 消费、分段到达、排他竞争、回执丢失及重启窗口；按真实调用计数、request/identity、接收字节/摘要核验，禁止仅改屏幕字符串。

核心故障每例 30 次，两目标同表执行，报告叶子用例实际执行、失败、跳过数；总重复不能冒充覆盖。L3 优先扩展正式 focused native 入口与现有可靠性场景，登记真实 case 后才算已覆盖。

真实验收：全新隔离现场、无副作用最小任务；local/remote 各至少三个独立 native session。记录唯一原请求、实际业务 submit/底层写入次数、accepted/committed/started/completed 凭据、原全文/final/stop census；缺任一项为未验证或失败。无需 Unity、业务 PR 或完整业务交付。未满足强提交前置时不向真实模型投递来冒充修复验证。

用户给出的三轮历史活动仅说明该构建三轮已开始、负报告及清理正常，均有 TTY CreateProcessW 故障；不证明旧输入问题已修复。旧 dirty 构建源码未恢复、预期文本为重建，离线草稿拒绝不能称完整历史复现。
