# Orca 原生输入通用提交 Spec

日期：2026-10-30。调查生产源码基线：`a086a9984dc0bd32b3f02b9698cd26c9ba313d90`。
状态：按用户确认的简化方案修订，**尚未实施**。本版取代此前的新增 Orca 强提交接口、committed 状态及 Runner 重试预算方案。
配套 [Plan](orca-native-submit.plan.md)、[调查与验收](../acceptance/orca-native-submit.md)、[归因修正](../acceptance/orca-native-submit-reassessment.md)。

本版已通过独立只读文档审计（reassessment_review，无P0–P2）；不代表生产实施或真实验收完成。

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
              --retry-request <原请求ID> --wait-submit 20 --json
orchestration request-show --request <原请求ID> --json
```

`--retry-request` 在此用于绑定既有 durable 请求身份，**不表示 Runner 新增自动重试**。正文不带 Runner 手动 ESC 粘贴框架，否则 Orca 会将嵌入 ESC 净化成字面 `<ESC>` 再包装，改变输入。沿用当前远端通用分支的20秒开始观察及50秒CLI调用上限，使本机/远端一致；调用方更短的原期限仍优先，不新增预算或循环。

默认 send 观察0秒，只有 input_accepted 是允许的结果。--wait-submit 只观察原请求，超时不保证任务没有执行。现成 request-show 查询同一调用者身份下的 mutation 记录，不发送、重试或结算原请求；本机、远端都已用全新不存在ID核实入口可用。

Orca 内部的渲染等待和 held-Enter 恢复属于其通用提交实现；Runner 不再复制这些机制。**删除的是 Runner 屏幕依赖，不是宣称 Orca 内部不读屏或绝无补 CR。** 当前证据未证明 Orca 违反公开契约，也未证明历史问题已修复。

## 3. 五种事实分别判断

| 事实 | 权威来源与能证明的范围 | 不能据此证明 |
| --- | --- | --- |
| 传输写入成功 | Orca 原传输写入回执，按实际 settlement 语义解释 | TUI已消费完整正文、任务开始或完成；bytesWritten不能充当输入全文证明 |
| 输入提交 | Orca 通用 prompt 流程承担正文处理、提交顺序和开始观察；Runner证明自己只交付一次确切冻结正文 | 没有独立的原子消费全文/所有写者排他证明；不增加或伪造committed回执 |
| 原请求受理 accepted | 绑定原runtime、terminal、incarnation、request的send.prompt回执 | 不是完整正文证明，不是模型已开始/已完成 |
| 原任务开始 started | 同请求turn_started回执及既有原native主回合/唯一原输入补证规则 | 不能仅以accepted、idle、普通working或输出变化认领原任务 |
| 原任务完成及结果归属 | 既有NativeProof、唯一原输入全文查询、原主代理新done、完整final/摘要及冲突排除，随后workflow验证 | mutation记录completed不等于模型任务完成，更不等于业务/发布成功 |

冻结 Turn.Prompt/InputDigest、wire编译、原请求ID、operation intent先于副作用的顺序保持。nativePromptText 继续只将CRLF转LF并去掉尾部CR/LF；不能折叠内部空白、空行或接受显示换行差异。保留预算及现有控制字符检查，不扩大允许输入范围。

提交阶段的正确性依据是：Runner在原资源/原会话准备边界核验身份和资格，将确切正文交给 Orca 的专用 prompt 处理，不再自行判断屏幕投影已经粘贴完整或选择按 Enter 的时机。正式原输入/final查询继续防止错误归属，但不声称事后核验能够阻止已经执行的错误文本。

不再承诺上一版要求的provider原子全文消费/全输入渠道排他事务；这不是现有Orca API已证明的能力，也不作为本版必须先扩展Orca的门槛。保留现有Runner操作串行化、原native会话/主回合基线和权限/准备检查；发现会话、身份、输入边界冲突或明确外来输入时立即阻断，不能清空/合并他人草稿。**不能把idle或通用send冒充原子防混保证。** 任意人工输入竞态及已有草稿能否在执行前完全排除，须如实记录公开证据边界，不能用删掉的屏幕比较补造保证。

## 4. 未知回执与原请求对账

回执丢失、超时或观察尚未证明开始时，沿既有Reconcile/observer流程只读对账原请求。复用原身份、已有节奏/期限与错误返回；不增加三次重试或其它预算设计，不重新调用业务send、补Enter、换ID、建替代资源或重派历史Run。

request-show 必须保持同一runtime和authenticated caller；核验requestId、method及保留receipt中的send.handle、processIncarnation，再按同一send.prompt规则接收事实。查询响应没有独立payload全文摘要，不能凭state字段跳过冻结输入与后续原文核验。

| 查询结果 | Runner处理 |
| --- | --- |
| completed | 读取保存的调用回执，可能只有input_accepted；按原规则判断accepted/started，继续原native完成观察 |
| pending | 原调用尚未结算或宿主在结算前重启；保持未知并只观察，不重发 |
| absent | 可能未到达、未记录或回执被清理；不是未执行证明，不授权send |
| 身份改变、request_mismatch、operation_unknown、查询不可用/超时 | 沿既有阻断/错误规则保留原因和原资源；不报成确定未执行、未开始、已停止或已完成 |

completed记录有30天及10000行容量清理规则；不能把同ID理解为永久安全重放。本版不依赖重放send取得回执，优先使用现成只读查询。

## 5. 身份、执行与本机/远端一致性

继续以 ProcessIdentity、SessionID、Turn和NativeProof绑定原runtime、worktree、terminal、incarnation、pane、provider session、command/request、authority/revision及回合基线。accepted/started/settled分别保存，原输入唯一性、完整final、工具/子代理排除、旧done排除、中断和会话边界检查不放松。保留原生outcome与workflow结果区别。

当前本机Codex可能选择native-command/model包装及observeRecorded/helper完成查询。新原生Codex必须迁到共同的正文编译、通用提交、原输入及final核验；不能只合并submit而留下本机专用完成旁路。清理仅服务该分支的reader/receipt接线。既有执行主机准备、认证资格、模型参数传递和停止证明必须保留；耦合无法安全解除时先解决具体Runner接线，不保留旧执行路径或静默删功能，不新增本机runtime/凭据/structured-chat替代。

复用现有目标选择与连接层、Turn.Request/InputDigest、operation receipt、NativeProof和状态envelope。默认不新增状态字段或codec；若实现证实确需保存额外原回执事实，须最小变更并验证保守读取，不能引入第二套状态机。旧未知记录只读/精确停止，不自动迁移为已受理或重投。

## 6. 删除范围

| 删除对象 | 连带清理及保留边界 |
| --- | --- |
| native_paste.go | 屏幕全文比较、显示LF推断、折叠标签regex、连续两次相同投影门禁、250ms轮询和paste专用40秒期限；输入安全检查移到共同前置 |
| Lifecycle.fitComposer及初始化分支 | composerViewport、FitBoundComposer、ComposerViewport、ViewportRPC/Recorder、fit/restore回执；仅供正文匹配的terminal_viewport.go及协议测试 |
| 分开的paste/Enter提交分支 | 手工bracketed框架、按是否多行/本机远端选择提交算法的分流；保留共同准备和身份检查 |
| 本机Codex专用wire/完成选择 | prepareInput的native-command/model选择、observeOnce的observeRecorded选择，以及仅为该分支存在的reader/receipt接线；不误删其它agent extension或共享准备/停止能力 |
| 旧测试、脚本、配置/fixture与当前文档 | native_paste*、native_reflow、viewport专用断言，live probe草稿/resize门禁及segmented-shape专用模式；将内容/归属/故障反例迁到共同提交行为测试 |

实施时全仓搜索引用，删除仅服务以上逻辑的状态和配置。当前未发现公开fit/stable专用配置，仍需复核；不能误删共享WaitTimeout。历史原始证据不改字节、不改判通过。最终只留共同通用提交路径，无legacy回退、实验模式或隐藏旧分支。

## 7. 验收与交付

Plan的NS-T01–T10映射正式入口、条件与证据；实现后登记真实测试/case ID。核心确定性故障各目标每叶例30次，报告执行/失败/跳过；重复次数不能代替覆盖。夹具控制回执顺序、延迟、丢失和身份变化，按实际调用、冻结正文/摘要、请求身份和原结果验证，不靠时间碰运气。

真实验收在全新隔离现场，用无副作用最小任务，本机/远端各至少三个独立native session。记录唯一请求、Runner通用提交次数、原accepted/started/completed证据、原输入/final和精确停止清理。明确计数层次：一次Runner RPC不等于一次底层按键；Orca内部不可观测的次数记为不可观测，不反推或伪造。最终原生历史须证明原输入唯一。

不要求Unity、完整业务交付或创建业务PR。缺真实路径条件记未验证，不用受控通过代替；历史三轮活动和旧草稿重放不改写为历史原生问题已修复。完成项目检查、稳定回归、真实验证及独立复审后，更新单独PR；本轮只修订Spec/Plan，生产实施与删除数仍为0。
