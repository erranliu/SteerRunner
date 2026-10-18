# PR 前质量审查（2026-10-18）

本次审查以确认的 [Spec](../../../spec/result-acquisition.spec.md)、[Plan](../../../spec/result-acquisition.plan.md) 和两份 [Workflow 定义](../../../workflows/README.md) 为准，范围为 Runner；不改 SteerLine，不部署、不合并 PR。先接入 `origin/main` 的 `0e34595`，再接入审查期间合入的本机 Orca 支持 `7c48969` 后重新核验，不把此前原生构建的结果转记为本次构建通过。

## 审查结论与修正

| 发现 | 实际修正与验证 |
| --- | --- |
| 未开放远端审计 workflow，却携带 WebSocket 模型改写桥、额外令牌/线协议及远端审计模型配置 | 移除 `remote_models.go` 及其接线、测试。当前审计定义只支持本机 app-server；普通远端执行继续用原生原会话协议。保留真实运行所需认证引用、回收及原生来源核验，不新建通用模型路由平台。 |
| 主线公开 schema 与 workflow/2 需同步；审计模型正则的双重转义导致实际 schema 编译失败 | 保留主线 `ConfigurationSchemas()` 唯一组合发布入口、实例 fencing、environment-check；同步 workflow/2 和仅代码审计的 prompt keys，修正正则。schema 与产品 admission 的有效/无效组合对照测试通过。 |
| parser 的 question/blocker 字段误标为原 agent final 路径 | 改为具体 `parser:<operation_id>:$/…`，业务 Source 仍是原审计 final 的 ID/版本/digest；新增两种业务未完成反例，不覆盖原 final。 |
| 粗粒度 `Succeeded=true` 可与明确 native failed/cancelled 并存 | 归一化时明确失败/取消优先；保留原文及原生 outcome，不因 final 声称 done 而推进。加入矛盾原生事实的反例。 |
| 主线测试仍默认 Spec+代码双审计及发布后反馈返工；旧反馈资格门在 workflow/2 中已无用途 | 删除资格字段、校验器及脚本参数；默认 smoke 无反馈，独立 feedback 场景验证明确拒绝且原 revision/Turn/PR head 不变。原生审计验收仅接受一个代码审计，绑定 pre_head/post_head 与实际模型回切。多 Run 测试复核原 peer 资源而不制造返工边；Controlled 保留活动 peer 跨 stop 的隔离反例。 |
| 新增来源机制可能复制状态/恢复权威 | 核对后保留唯一 Record 及 Workflow.StepID；NextStage 只保存未派发原会话提示；parser 的 attempt 只引用同一 Intents/operation，journal 仅提供原回执。未增加状态服务、策略 DSL、脚本宿主、资源池或新恢复调度器。 |

新增主线整合时保留本机原生查询和远端连接恢复，并修正测试清理仍硬编码 `RemoteRuntimeID` 的接线：复用 `Manifest.OrcaRuntimeID()`，仅远端追加 environment。未建立本机专属的第二套清理账本或资源状态。

## 来源与修复后绑定

审查入口为 `application/results.go`、`workspace_source.go`、`parser.go` 与 `workflow/verification.go`：原实施 result/source → 原 clean pre_head → 原审计修复 Turn 的 post_head/tree → parser/report 的相同原源 → 发布前本地复核 → provider 身份/PR.HeadRevision → 再次本地复核与原 revision CAS。补报不能重建缺失的原 Git checkpoint；同 SHA 的新业务源不能继承旧审计。正数修复必须 tree 改变；工作区漂移、旧来源、停止授权和未知副作用均不能靠报告清除。

当前定义和统一机制分别保留职责：定义给出阶段、后继、提示、结果契约和本期选用方法；actor、native owner、Git/provider owner 执行原子提交、资源管理和核验。审计原记录属于事实及来源投影，不是第二个业务游标。

## 验证

最终批次结果、源码身份、命令及原始日志索引在 [verification.json](verification.json)：最新主线完整 L1 **335 项通过**（7 个允许跳过项单列），原完整 Controlled **16/16** 及最新构建接线回归 **3/3** 通过；最新 go vet 通过。最初接入主线后的 L1 发现 schema 编译失败，保留失败摘要；修正后完整 L1 329 项通过，并运行当前注册的完整 Controlled 16 项。后续本机 Orca 主线整合再跑完整 L1，以及流程发现/环境检查/多 Run 隔离三个进程回归。后者使用最新构建，原完整 Controlled 保留原冻结二进制。结果获取、Workflow、actor、契约、存储与 Controlled 测试源码相对原完整 Controlled 构建未变，等同性由 Git diff 及源码清单核对；不把两个 EXE 写成同一 SHA，也不重写旧来源身份。不会通过缩减断言、改变预期、把空测试或 skip 计作通过来结算。

## 未完成的原生可靠性项

此前全范围记录见 [原 L2/L3 现场](../full-l2-l3-20261018/README.md)：原构建 L2 2/2；当时注册的 L3 12 case 中 4 通过、1 失败、7 因专属管理员设施缺失阻塞。

`service-restart/kill-active` 的失败窗口是只读 inspector 的 `terminal.create` 意图已保存，但原生创建回执/资源身份尚未返回时服务被强杀。Orca 原生 create 无调用者幂等键和精确恢复查询；原完整空库存不能排除晚到创建。继续保留 unknown，不能按 title 补认、重建 inspector、重发业务或改写原结果。解决此项需要原生幂等创建/原创建查询能力，或另行设计持久派发握手；本 PR 不用第二套资源权威掩盖缺口。

接入最新主线后当前 L3 共 24 case；新增案例及本次最终构建未重新取得完整原生 L2/L3 资格。旧原生结果只作历史证据，不能代替当前完整通过。PR 应保持 draft，待该恢复缺口及相应原生验证结算后再决定可发布范围。
