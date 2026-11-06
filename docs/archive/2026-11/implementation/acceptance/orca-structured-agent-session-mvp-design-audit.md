# 结构化 execute/1 与 DSH 退役：设计复审

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-03。范围：用户确认的本机 Orca Codex 新 execute/1、停止证据不足明确未核实、不修改 Orca，以及本 PR 删除 DSH 执行和专用历史/用量支持。审计对象为收敛后的 [Spec](../spec/orca-structured-agent-session.spec.md) / [Plan](../spec/orca-structured-agent-session.plan.md)。方式：主会话只读核对现有消费者，修订文档后再检查；未调用独立审计 Agent，不是实现审计或运行资格。

结论：原方案的全量迁移门禁与过早抽象已收敛；以下确认问题已在文档修正，未发现尚未处理、要求在编码前另行设计的平台或状态机问题。**本轮不实现代码、不构建、不调用模型/Orca runtime、不部署、不提交 PR。** 原 P0 证据不改写。

## 1. 确认问题与修订结果

| 项目 | 原风险 | 本轮处理及复核 |
| --- | --- | --- |
| 全量迁移范围过大 | 首 PR 同时要求 Pi/双端/development/audited/四组合与全量 helper 退役，输入修复被不相关门禁阻断 | Spec §1/9、Plan §1/7 只留本机 Codex execute/1 和 DSH 删除；B03 audited、远端 Pi 与其它迁移不作为本 PR 前置 |
| 已接受限制仍被当作阻断 | 旧全文要求 resources_settled 才可准入，与用户接受 unknown 冲突 | Spec §6、Plan §4 明确新 scope 的 unknown/false 与结果可读；不要求 Orca 补字段，不重复请求确认，不影响其它 scope |
| 原子首消息重复派发 | 原 create/准备/StartTurn 顺序可能在 create.firstMessage 后又 send 一次 | Spec §4、Plan §3 将业务首消息固定为一个原子外发，要求既有 action/回执约束验证及实际计数；不新增调度动作、空 create 或“补发” |
| 空终端身份假 stopped | `application/control.go` 有 External.Kind 为空且无 unknown 的无资源成功路径；`verify_stopped.go` 有旧 stopped 标签/OS/terminal 读法 | Spec §2/6、Plan §4 要求识别新协议原资源/已可能创建意图，不伪造 terminal/PID，不让 native 走旧成功分支；纳入正式受控验收 |
| 正常结果被清理未知掩盖 | `results.go`/`content.go` 先保存结果再 closeContent，stop unknown 后聚合状态可能保持 reconciling；若追求整体 completed 会误报资源退出 | 保留已有 ContentResult 与 stop 分离，明确结果仍可读、聚合状态不假完成；无需新增全局状态枚举或改所有 Workflow 的完成模型 |
| 旧链被当作新 native owner | 用原 terminal observer/close 或新 helper 输入桥保留两套 writer，随后还需再退役 | 原 native session 从创建开始持有；新路径无 helper/TUI/provider 旁路/terminal fallback；只复用工作区/辅助 shell 的真实能力 |
| 未来能力提前建设 | 单 Agent 就先建 Pi 映射、远端 stream、回答/feedback、队列、通用 receipt/Runtime/第二账本 | Spec §3、Plan §2/7 仅 create/history/close，必要时 item；其它操作有实际消费者时再加，复用现有事务和一次派发 |
| 选择与配置含糊 | 同一新 Run 可能按配置或报错切回旧链；旧 helper 配置被忽略，或新校验伤及 audited/Pi | 新组合在 freeze 早于 helper metadata/launch 选择并冻结协议；只对该组合拒绝无法兑现的显式启动设置，旧记录按原协议读，无用户运行时开关 |
| 删除 DSH 不彻底或误删共享面 | 只删执行、残留历史 parser/注册；或整文件删除损伤 Pi/terminal，甚至旧 DSH 记录使全目录读取失败 | Spec §7、Plan §5 明确执行与专用历史/用量一起删除，列真实消费者；普通记录级 unsupported 与非 DSH 隔离，原 evidence 和用户数据不物理删除 |
| 为迁移引入全局格式工程 | 新 opaque key 被扩大成 state/v9/强制新目录/活动任务迁移和旧格式矩阵 | 沿 state/v8 及既有 envelope；无真实不兼容不变全局格式，原构建先排空，不接管旧 DSH 活动任务 |
| opaque state 重复存大正文 | 原描述可能将输入/完整结果再复制到 64 KiB ImplementationDataEnvelope，引起无谓超限和第二份事实 | Spec §2、Plan §3 限 opaque 为身份/游标/派发事实；正文和结果复用既有原 operation/StageInput/ContentResult/消息存储，不新建 blob 库 |

## 2. 刻意保留的边界

- **资源结算未核实是已接受的产品限制。** close 回执、turn interrupted、空列表均不补成退出证明；verify-stopped 可能无法通过，不能自动认定升级已排空或删除工作区。此代价在 Spec §6 明示。
- **完整结果仍是硬门禁。** P0 的单标记短回复不证明工具/多段消息的完整 final 选择。Spec MVP-03 和 Plan §6 要求真实覆盖；实现若不能从现有公开事实取得充分原结果，应保留未核实并阻断对应交付，不能诉诸最后一段消息或 provider 私有读取。
- **显式配置必须兑现或明确拒绝。** 原子 firstMessage.selection/full_access 要在实施时核验；此前分步 send 的选择回执不平移。没有新的 Agent 配置语言或 Orca 配置迁移器。
- **DSH 删除不等于状态数据清除。** 不再保留其专用历史/用量能力；旧证据和用户文件留存。旧 DSH 不应阻断同目录 Codex/Pi 读取，但不新增 DSH 恢复/解码框架来追求历史可用。
- **未迁移旧实现有真实消费者。** 保留 development/audited/Pi/远端是分批迁移边界；不扩展其 helper 职责，新路径绝不回退它们。

上述待运行验证项不是已通过，也不需要为本轮文档另造模型调用。无关的完整退出、审计双模型、全远端资格不重新加入首 PR。

## 3. 文档检查与结果适用性

本轮同步了产品待实施说明、Orca 操作导引和 Spec 索引：只声明设计已确认，现行生产代码/能力尚未改变。交付检查覆盖 11 份文档、485 个本地链接目标，无缺失链接/正文尾空白，git diff --check 通过；原 192 份证据和 20 个所引用生产文件摘要保持。改动只在文档及既有证据字节属性，不在运行目录。检查脚本初轮的 git status 首行 trim 路径解析错误修正后重新通过；不是产品测试失败或运行资格。

实现阶段按 Spec MVP-01–06 验证。停止 unknown 不能计成完整退出通过，DSH 删除不能计成 Codex/Pi 全量迁移完成，设计复审也不能代替实现审计或正式验收。
