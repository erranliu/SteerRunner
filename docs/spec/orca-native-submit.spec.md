# Orca 原生输入通用提交 Spec

日期：2026-10-30。调查生产源码基线：`a086a9984dc0bd32b3f02b9698cd26c9ba313d90`。
状态：按用户确认的简化方案修订，**尚未实施**。本版取代此前的新增 Orca 强提交接口、committed 状态及 Runner 重试预算方案。
配套 [Plan](orca-native-submit.plan.md)、[调查与验收](../acceptance/orca-native-submit.md)、[归因修正](../acceptance/orca-native-submit-reassessment.md)。

上一版“无P0–P2”审计结论已被后续深审撤回。本版修正文档中的本机能力迁移、宿主内部回退、持锁等待和查询触发范围。2026-10-30，design_audit与final_review独立只读复审均无新增P0–P2文档发现，并均明确M1仍未通过、完整实施设计尚未具备执行条件。生产实施尚未开始，且必须先通过§5的M1能力等价门槛，不能把文档修订当成M1已通过。

## 1. 目标与范围

Runner 原生 Codex 输入使用 Orca 已有通用 prompt 提交流程。本机、远端以及单行、多行共用同一实现和判定，仅目标连接不同。其它 agent 的正式 extension 契约不在此次重写范围。

一次提交调用同时携带冻结并规范化的正文与 Enter，Orca 负责正文 framing、粘贴处理、提交和开始观察。Runner 删除自己对屏幕草稿正文、显示换行和视口布局的提交门禁，不再驱动分开的 paste/Enter。

不修改、安装或升级 Orca，不新增上游 API、能力版本或消费正文摘要字段。不新增 Runner 重试次数、重试预算、配置、后台重试器、committed 阶段、第二套状态机或新旧路径回退。既有准备查询恢复和既有有界等待保留，不扩张为输入重投机制。

不涉及环境检测业务、Unity、SteerLine、项目脚本或检测材料，不部署安装环境、不改用户全局设置，不改写历史失败。

## 2. 已核实的契约与证据边界

Runner 当前远端普通 Codex 多行路径进入 submitNativePaste，先 text-only bracketed paste，后 bare Enter。CLI 只有同次 text + enter + !interrupt 才启用 agentPrompt；当前两次 raw 调用都绕开通用 prompt 的 durable 请求绑定和开始回执。这是确定的调用路径差异，不能据此倒推历史故障唯一根因。

已核实的 Orca 入口：

```text
terminal send --terminal <原终端> --text <规范化正文> --enter
              --retry-request <原请求ID> --json
orchestration request-show --request <原请求ID> --json
```

`--retry-request` 在此用于绑定既有 durable 请求身份，**不表示 Runner 新增自动重试**。正文不带 Runner 手动 ESC 粘贴框架，否则 Orca 会将嵌入 ESC 净化成字面 `<ESC>` 再包装，改变输入。两目标均不传`--wait-submit`，不在共享CLI锁内追加20秒开始观察；沿用现有50秒CLI调用上限及调用方更短期限，不新增预算或循环。Orca自身处理正文仍可能耗时，不能宣称锁占用为零或首次受理必然立即返回。

默认 send 观察0秒，只有 input_accepted 是允许的结果。--wait-submit 只观察原请求，超时不保证任务没有执行。现成 request-show 查询同一调用者身份下的 mutation 记录，不发送、重试或结算原请求；本机、远端都已用全新不存在ID核实入口可用。

正常取得受理后由原native observer取得开始/完成证据，不用send重放或request-show补“开始等待”。现有NativeProof的working分支依赖已核实的Turn.Started；不能因省掉20秒观察就假设working通知自动证明原请求开始。未取得即时turn_started时保留accepted等待开始状态，沿现有独立唯一原输入/新done补证恢复；缺证时不放宽归属条件。实现回归必须覆盖首次只有accepted且随后正常完成，不能改回持锁等待、补Enter或新增开始轮询器掩盖问题。

Orca 内部的渲染等待和 held-Enter 恢复属于其通用提交实现；Runner 不再复制这些机制。**删除的是 Runner 屏幕依赖，不是宣称 Orca 内部不读屏或绝无补 CR。** 当前证据未证明 Orca 违反公开契约，也未证明历史问题已修复。

CLI能力预检只证明宿主支持prompt-delivery。宿主若未识别到可用的前台agent，terminal.send内部仍可能调用raw send后返回`provider=unsupported`，不能由Runner参数保证消除此分支。本方案的“无raw回退”仅指Runner不主动选择或再调用raw路径；不承诺所有失败都在输入副作用前发生。只有明确的调用前拒绝才能记本次未发送；不支持/身份不确定的返回必须按未知保留，禁止再投或补Enter。

本次生产能力白名单为已发布CLI的terminal send及request-show、现有terminal.prompt-delivery.v1，以及Runner已经使用的原native查询RPC。安装包私有函数只用于调查，不导入、复制为运行时依赖或用其符号名判断兼容性；不新增experimental开关、私有RPC、模型控制面或宿主安装补丁。公开能力也须按实际受测版本和两个真实目标验收，不能由未见experimental标记推导任意版本稳定。

## 3. 五种事实分别判断

| 事实 | 权威来源与能证明的范围 | 不能据此证明 |
| --- | --- | --- |
| 传输写入成功 | Orca 原传输写入回执，按实际 settlement 语义解释 | TUI已消费完整正文、任务开始或完成；bytesWritten不能充当输入全文证明 |
| 输入提交 | Orca 通用 prompt 流程承担正文处理、提交顺序和开始观察；Runner证明自己只交付一次确切冻结正文 | 没有独立的原子消费全文/所有写者排他证明；不增加或伪造committed回执 |
| 原请求受理 accepted | 绑定原runtime、terminal、incarnation、request，且原生Codex provider/observation可验证的send.prompt回执 | 不是完整正文证明，不是模型已开始/已完成；unsupported/old-host不升级为可靠受理 |
| 原任务开始 started | 同请求turn_started回执及既有原native主回合/唯一原输入补证规则 | 不能仅以accepted、idle、普通working或输出变化认领原任务 |
| 原任务完成及结果归属 | 既有NativeProof、唯一原输入全文查询、原主代理新done、完整final/摘要及冲突排除，随后workflow验证 | mutation记录completed不等于模型任务完成，更不等于业务/发布成功 |

冻结 Turn.Prompt/InputDigest、wire编译、原请求ID、operation intent先于副作用的顺序保持。nativePromptText 继续只将CRLF转LF并去掉尾部CR/LF；不能折叠内部空白、空行或接受显示换行差异。保留预算及现有控制字符检查，不扩大允许输入范围。

提交阶段的正确性依据是：Runner在原资源/原会话准备边界核验身份和资格，将确切正文交给 Orca 的专用 prompt 处理，不再自行判断屏幕投影已经粘贴完整或选择按 Enter 的时机。正式原输入/final查询继续防止错误归属，但不声称事后核验能够阻止已经执行的错误文本。

不再承诺上一版要求的provider原子全文消费/全输入渠道排他事务；这不是现有Orca API已证明的能力，也不作为本版必须先扩展Orca的门槛。保留现有Runner操作串行化、原native会话/主回合基线和权限/准备检查；发现会话、身份、输入边界冲突或明确外来输入时立即阻断，不能清空/合并他人草稿。**不能把idle或通用send冒充原子防混保证。** 任意人工输入竞态及已有草稿能否在执行前完全排除，须如实记录公开证据边界，不能用删掉的屏幕比较补造保证。

## 4. 未知回执与原请求对账

仅当原输入已尝试、send回执因传输丢失/不完整而缺失，且尚无可验证的accepted/rejected/started/final、也无明确身份/输入冲突时，沿既有Reconcile/observer流程只读补查原请求。不按诊断文案推导此条件，使用现有Turn事实、原operation和保存回执。已知accepted但尚未started、缺final、普通观察间隙都不触发request-show；已收到的unsupported/身份冲突回执也不是“缺回执”，不能据此循环查同一不支持结果。

查询复用原身份、已有节奏/期限与错误返回，不增加三次重试或其它预算设计，不重新调用业务send、补Enter、换ID、建替代资源或重派历史Run。它只补回缺失的send事实，不成为正常开始或完成的第二个必调门禁。

request-show 必须保持同一runtime和authenticated caller；核验requestId、method及保留receipt中的send.handle、processIncarnation，再复用同一个send.prompt解析器。原生Codex只将`provider=codex`且`observation=supported`、身份完整且无调用不确定性的回执作为可靠受理/开始来源；`unsupported`、`old-host`、缺provider/observation、`incarnation_replaced`或未知值不按成功处理。`permission`保留原阻断语义，不视为已开始。它们可能已有输入副作用，不能报成未发送或触发重投。查询响应没有独立payload全文摘要，不能凭state字段跳过冻结输入与后续原文核验。

已验证事实只单调合并：补查不能清除既有accepted/started/final，不能覆盖已绑定的完整final，也不能抹掉持久冲突。查询前后若native观察已经提供更强事实，弱回执不得降级它；取得可验证accepted后停止该回执补查，只继续原native观察。query失败/absent/pending不阻断已具备完整原生证明的正常完成。

| 查询结果 | Runner处理 |
| --- | --- |
| completed | 读取保存的调用回执，可能只有input_accepted；按原规则判断accepted/started，继续原native完成观察 |
| pending | 原调用尚未结算或宿主在结算前重启；保持未知并只观察，不重发 |
| absent | 可能未到达、未记录或回执被清理；不是未执行证明，不授权send |
| 身份改变、request_mismatch、operation_unknown、查询不可用/超时 | 沿既有阻断/错误规则保留原因和原资源；不报成确定未执行、未开始、已停止或已完成 |

completed记录有30天及10000行容量清理规则；不能把同ID理解为永久安全重放。本版不依赖重放send取得回执，优先使用现成只读查询。

## 5. 身份、执行与本机/远端一致性

继续以 ProcessIdentity、SessionID、Turn和NativeProof绑定原runtime、worktree、terminal、incarnation、pane、provider session、command/request、authority/revision及回合基线。accepted/started/settled分别保存，原输入唯一性、完整final、工具/子代理排除、旧done排除、中断和会话边界检查不放松。保留原生outcome与workflow结果区别。

本机native-command/model与observeRecorded并非屏幕门禁的附属物：remote_models.go解析模型wire后才设置原turn/start的model/effort及clientUserMessageId；去掉wire会使现有桥接报remote_model_input_missing。local_codex_history.go还核验实际模型，并允许32KiB完整final；NativeProof当前会拒收≥7999 UTF-16的hook正文。只把普通正文交给Orca并改用NativeProof并不等价。

**M1：先闭合能力等价设计，后启动生产实施。** 最终共同正文/提交/原输入/final流程目标不变，也不能把保留旧本机执行分支当成最终完成。但本次不得顺带重写Codex模型桥接或结果获取栈、临时搭建新控制面；在下表给出具体的现有非实验通路、代码接线和可复核证据前，不执行共同路径切换，也不删除本机wire/reader。不能以“保留保证”“先解决耦合”或拟定的新接口代替证明。

| M1必需证明 | 最小交付与判据 | 当前状态 |
| --- | --- | --- |
| 每阶段模型/effort | 明确冻结step/command如何进入原turn/start；覆盖执行模型→审计模型→执行模型及真实历史中的实际model/effort/请求绑定，不能仅靠启动参数或提示词建议 | 未闭合；公开terminal send没有这些参数 |
| 长final无收缩 | 明确同一原回合完整assistant结果的现有权威来源、分页/截断判断及身份围栏；覆盖>7999 UTF-16且≤32KiB样例、32KiB边界、超限和工具/旧结果拒收；不得放宽hook截断检查 | 未闭合；当前NativeProof单独不能等价替代本机history |
| 准备/认证/停止 | 逐项列出原资源所有者、准备/认证资格、停止/清理证明的保留接线和测试；不能因删除reader而顺手撤销原保证 | 接线与证据待实施前核实 |

M1尚未通过是具体Runner能力迁移的阻断项，不是要求修改Orca。当前文档只把原本隐藏的工作显式化，没有声称模型/结果迁移已获证；未闭合时保持生产代码不变，不能发布“已统一两目标”或“输入修复完成”，也不扩张本次范围造框架。

本次固定复用现有目标选择/连接层、Turn.Request/Accepted/Rejected/Started/Settled、operation receipt、NativeProof及状态envelope，不新增字段或改codec。若M1证明需要独立模型/结果迁移或新持久化事实，先作为范围问题修订设计，不留给实现阶段自由扩张。本次不引入第二套状态机；旧未知记录只读/精确停止，不自动迁移为已受理或重投。

## 6. 删除范围

| 删除对象 | 连带清理及保留边界 |
| --- | --- |
| native_paste.go | 屏幕全文比较、显示LF推断、折叠标签regex、连续两次相同投影门禁、250ms轮询和paste专用40秒期限；输入安全检查移到共同前置 |
| Lifecycle.fitComposer及初始化分支 | composerViewport、FitBoundComposer、ComposerViewport、ViewportRPC/Recorder、fit/restore回执；仅供正文匹配的terminal_viewport.go及协议测试 |
| 分开的paste/Enter提交分支 | 手工bracketed框架、按是否多行/本机远端选择提交算法的分流；保留共同准备和身份检查 |
| 本机Codex专用wire/完成选择 | 仅在M1已通过、具体等价接线已审计后清理prepareInput的native-command/model、observeOnce的observeRecorded及专用reader/receipt；未闭合前不得作为屏幕旧逻辑直接删除，也不能保留为最终双路径 |
| 旧测试、脚本、配置/fixture与当前文档 | native_paste*、native_reflow、viewport专用断言，live probe草稿/resize门禁及segmented-shape专用模式；将内容/归属/故障反例迁到共同提交行为测试 |

实施时全仓搜索引用，删除仅服务以上逻辑的状态和配置。当前未发现公开fit/stable专用配置，仍需复核；不能误删共享WaitTimeout。历史原始证据不改字节、不改判通过。最终只留共同通用提交路径，无legacy回退、实验模式或隐藏旧分支。

## 7. 验收与交付

Plan的NS-T01–T10映射正式入口、条件与证据；实现后登记真实测试/case ID。核心确定性故障各目标每叶例30次，报告执行/失败/跳过；重复次数不能代替覆盖。夹具控制回执顺序、延迟、丢失和身份变化，按实际调用、冻结正文/摘要、请求身份和原结果验证，不靠时间碰运气。

真实验收在全新隔离现场，用无副作用最小任务，本机/远端各至少三个独立native session。记录唯一请求、Runner通用提交次数、原accepted/started/completed证据、原输入/final和精确停止清理。明确计数层次：一次Runner RPC不等于一次底层按键；Orca内部不可观测的次数记为不可观测，不反推或伪造。最终原生历史须证明原输入唯一。

不要求Unity、完整业务交付或创建业务PR。缺真实路径条件记未验证，不用受控通过代替；历史三轮活动和旧草稿重放不改写为历史原生问题已修复。完成项目检查、稳定回归、真实验证及独立复审后，更新单独PR；本轮只修订Spec/Plan，生产实施与删除数仍为0。
