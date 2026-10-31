# Orca 原生输入通用提交 Spec

2026-10-31。PR #73；主线基线 `a086a9984dc0bd32b3f02b9698cd26c9ba313d90`，本次再审计基线 `597d669`。配套 [Plan](orca-native-submit.plan.md)。

当前再审计修正已实施：冻结源 `c21d915a6d5ce1d03e7b437b94048614781052312279076ac2cb678f2c3ad894`，正式矩阵 239 叶各 30 次＝7170/0/0，原查询回归 450/0/0；完整L1与新源managed本机3/3、远端3/3均通过，最终独立补充复核通过，无未修P0–P2；按本Spec及验收报告边界放行。原构建资格不能平移为本次通过。

## 1. 边界与单一提交路径

删除原远端普通 Codex 多行专用 paste/草稿/Enter 分支，所有本机与远端原生 Codex 进入既有通用 `terminal send --text --enter --retry-request <原请求ID>`；仅目标路由不同，不新增本机专用 runtime、凭据或完成旁路。

保留进入发送前的冻结正文、wire 包装、预算和规范化。普通 Codex 继续 CRLF→LF、去尾 CR/LF，内部空白和空行保持；所有原生 Codex 在同一 send 前将逻辑 LF 编译为终端 CR，Codex paste 规范化恢复 LF。原逻辑 wire/hash/预算不变，实际 CLI 参数摘要单独记录，正式原输入必须精确匹配原逻辑正文。保留原草稿输入范围的 NUL/ESC 拒绝，检查位于传输编码之前。

不修改 Orca，不引入新或实验性 Orca API、提交重试/预算、固定 sleep、request-show 恢复、第二套状态机、codec 字段/版本、通用执行框架、兼容回退或实验模式。不增加环境检测、项目脚本专用分支，不改业务材料，不部署安装或修改全局配置。

本机既有native-command/model、实际模型/effort/clientUserMessageId、recorded history与32KiB结果能力保留；远端既有NativeProof、正式原输入和新done证明保留。远端仅冻结启动模型偏好，实际model/effort未独立核验、audited不支持，不得概括为两端实际模型证明等价。既存准备、认证和完成来源不在本次重构为一套；这不代表新增目标专用实现。

## 2. 五种事实与权威边界

| 事实 | 原权威来源及能证明什么 | 不能证明什么 |
| --- | --- | --- |
| 传输写入成功 | 原调用/传输回执，按 settlement 解释 | bytesWritten 不证明 TUI 全文消费、业务开始或完成 |
| 完整正文交付 | Runner 冻结并一次交付完整编译正文和 Enter，Orca 通用入口承担 framing/处理/提交顺序 | 无新增全文原子消费 ACK 或摘要，不承诺任意人工写者排他 |
| accepted | 原 runtime/terminal/incarnation/request 绑定且 provider=codex、observation=supported、阶段一致的原回执 | 不是 started、completed 或全文消费证明 |
| started | 本机 managed 原 history 唯一原输入/turn；普通原生的独立唯一原输入及新 done 补证 | send 的 turn_started、working、idle、READY 或屏幕变化不能单独认领原业务 |
| completed 及结果归属 | 本机原 recordedTurn/validateRecordedTurn；普通原生原 NativeProof/完整原输入/新 done/完整 final，保留唯一性、冲突和 workflow 校验 | mutation completed 不等于模型完成；模型完成不等于业务交付 |

先冻结 Turn.Prompt/InputDigest、wire、预算和 operation intent，再进行唯一提交。保留 runtime、terminal、incarnation、session/provider/pane、command/request、authority/revision、原回合基线和资源所有权绑定。accepted、started、settled 分别判定，不降低工具输出、旧结果、重复输入、模型或完整性围栏。

提交阶段由原通用入口负责顺序，Runner 保证原身份的一次完整参数交付、统一编码和未知不重复；事后正式原文核验不能阻止错误文本已经执行。不得把它改称执行前原子消费保证。保留准备、权限、原资源核验、串行化和已知外来输入/身份冲突阻断；已知冲突不清空或合并。idle 不证明无草稿，任意未知草稿或人工输入竞态的完全排除仍未获证。

## 3. 受理、开始和未知处理

沿用原 runtime/handle/request/incarnation/CallUncertain 与错误校验；Codex 只接受 input_accepted，或依次 input_accepted/turn_started。unsupported、old-host、缺字段、替换、未知阶段不升级成功，保留原因及原回执；可能有副作用时不能谎报“未发送”。

Codex 的 turn_started 只保留为宿主提示，不设置业务 Started 或发布 EventStarted。本机 managed 继续 observeRecorded；普通原生继续 queryCompletedOriginalStart。后者可能直到 done 才建立开始事实，之前不发布依赖 Started 的 working。其它 agent/extension 规则不变。

发送超时、回执丢失或不可验证，沿原 operation journal、Reconcile 和 observer 保留身份、原因及未知事实；不再次 send、不补 Enter、不换 ID、不建替代资源、不重派旧 Run。同 ID 也不构成安全重投许可，原 mutation 可能已清理。缺 accepted 的旧流程不承诺恢复完成，不能伪造 accepted 或确定未执行；已有强事实和冲突不能被弱观察清除。

NativeRemoteProfileVersion 保留既有 --wait-submit 20 和 50 秒 CLI 上限，其他路径参数不改，更短调用方期限优先。观察不重投，CLI 仍持原共享锁；排队和执行共用原 deadline，取锁不刷新，不保证 stop 抢占或全局 FIFO。不得另造锁、轮询器或回退规避锁回归。

## 4. 原始控制读取

READY/control 与本机 launch READY 从公开 cursor 0、limit 1000 读取，要求原 stream、保留起点、明确未截断/未受限及原身份；完整空流仍 pending，不拼接屏幕行。

持续 original-turn/workspace-audit/authentication-stop 读取当前保留窗口：首读可报告前缀淘汰；按 nextCursor 分页，只读到首次 latestCursor，不追赶新输出。逐页前后核验原 runtime/terminal/incarnation，核验来源、cursor 单调性及完整行；中途丢页、倒退、无进展、身份变化或取消失败并保留原因。分页仅用本次查询的局部变量；沿原解码、request/binding/digest 和 checkpoint 决定采纳，无持久 cursor 或新发送机制。

### 4.1 模型桥保真与提交阶段

原生模型桥上游消息上限从 16MiB 对齐既有 Codex 原客户端的 128MiB 有界上限，下游 2MiB 不变。真实诊断已有 16777217 字节 read-limit 首错和完整 23,771,840 字节回复证据。限额断线不能解释全部缺 LF：单独 CR/16MiB 和 128MiB/LF 都曾失败，不能把任一单项或诊断会话计为完整通过。

rewrite 拒绝时向原数字/字符串 RPC id 返回结构化错误，零转发、不重试、不以静默断连掩盖原因；缺失/非法 id 关闭连接，错误不带正文或凭据。认证、单活动连接、原 thread、冻结正文/模型及 clientUserMessageId 约束保持。不得以安装包内部函数作为生产依赖。

### 4.2 再审计：普通本机正式原输入查询

597d669 的 P2 回归：Started 收紧覆盖所有原生 Codex，但 NewLifecycle 仅给远端接入 queryInput。development/2 仍允许非 GitHub、无 managed helper 的本机 Codex，其 WireFormat 为空；既不走 observeRecorded，也无独立原输入查询，accepted 后无法恢复 started/completed。

最小修正取消既有 queryInput 闭包的远端限定，继续由同一个 QueryOriginalInput 和 Environment 路由目标。保留唯一完整原文、原 process/terminal/session 的前后围栏与新 done 判定；不恢复 send 阶段授权。本机 managed 仍走 observeRecorded，其他 agent/extension 不进入 Codex 原输入补证。没有新增查询 API、完成算法、runtime、凭据、状态或回退。

生产构造器回归不注入 Started/queryInput，实际 IPC 计数证明查询发生一次；thin/corroborated 正例及 duplicate/altered 反例覆盖中文、空行、CRLF、超过预览的长文。本次既有正式本机目标为 GitHub/managed，未覆盖无 helper 的非 GitHub 真实目标，确定性 IPC 证明不能冒称该分支真实通过；managed 本机新现场只证明其自身路径。

## 5. 清理与保留

| 对象 | 处置 |
| --- | --- |
| native_paste.go / submitNativePaste | 删除手动 paste/Enter、草稿全文/标签匹配、显示换行推断、双稳定、专用轮询/期限 |
| fitComposer/composerViewport/terminal_viewport.go | 删除正文匹配用的 fit/restore、相关字段、RPC/Recorder、回执、实现及专用测试 |
| native_paste*/native_reflow/segmented-shape | 删除专用测试、模式和配置；内容/身份/未知反例迁入通用行为回归，live probe 不再分段手动 paste |
| wire/model/history/NativeProof | 保留逻辑规范化、预算、模型、正式原输入/final；普通本机仅补既有查询接线 |
| 准备/认证/stop/codec | 保留原证明链和所有权范围；不增加字段或迁移旧未知为成功 |
| 活动文档与历史 | 活动 Spec/Plan 只描述当前路径；历史验收、原字节和失败结论原样保留 |

逐引用搜索确认屏幕专用项无生产残留，保留共享 WaitTimeout、通用 terminal read 及其它 agent 所需能力。RPC 单测夹具的 stop/wake 停 Accept 后 Close 修正仅在测试中保留，不升级依赖、不改变生产 RPC、预算或清理判定。

## 6. 验收与交付

测试 ID、入口、条件和证据见 Plan 的 NS-T01–T15。核心确定性每叶各 30 次，夹具用 channel/屏障控制延迟、分段、顺序和故障位置，以实际调用、请求身份、正文/摘要和正式证明判定；报告执行/失败/跳过，不能以单快照重复替代覆盖。完成项目 L1、受影响正式 L3/controlled 及独立复核。

新源采用隔离构建的现有 Runner agent；全新本机与远端各至少三个独立原生会话，仅无副作用 nonce 任务。记录源/EXE/CLI摘要、Run/request/runtime/terminal/incarnation/session、accepted/started/completed、原文/final及正式 stop/完整原 host 空清单。Runner 一次 RPC 不等于宿主一次按键，不可观察次数如实标记。远端 stop 范围仍为 orca_terminal；物理路径未独立核验不得宣称消失。

不重跑业务/Unity或创建业务 PR；更新原 PR #73，不合并或部署。缺少真实条件如实标未验证。历史 dirty 原源码未恢复，不宣称完整复现或修复历史原生故障。

历史与原字节分别见[原实现](../acceptance/orca-native-submit-implementation.md)、[原双端验收](../acceptance/orca-native-submit-remote-final.md)、[持续读取批次](../acceptance/orca-native-submit-control-stream.md)、[保真与开始凭据批次](../acceptance/orca-native-submit-integrity.md)；各批资格互不平移，失败不改判通过。