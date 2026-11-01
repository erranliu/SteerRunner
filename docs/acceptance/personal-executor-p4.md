# 个人执行器简化 P4：原生问答输入职责收口

日期：2026-11-01。依据[修订 Spec](../spec/personal-executor-simplification.spec.md)及 [Plan P4](../spec/personal-executor-simplification.plan.md#6-p4按能力局部收口不搬通用准备逻辑)。本项按 PES-A06/A08 独立验收通过。

本轮只收口一项已有重复私有解释：原生问答的输入表示。profile/helper、权限资源、checkpoint、文件源本轮不继续展开，避免只换文件或新增转发层。共享准备、业务来源与审计核验保持原所有者。交付为 P3 已验收工作副本上的未提交变更；基线提交仍为 e2729563562314659a799a5f82debe427fcd7c24，source_dirty=true。

## 收益及职责变化

| 核对项 | P3 基线 | P4 本项 |
| --- | --- | --- |
| 私有输入选择入口 | application/control.go、codex/lifecycle.go 均解释 Delivery 并选择 AnswerParts/原始文本 | 仅 codex/ask.go 的 answerInputParts；准备与实际发送复用 |
| application 私有 route 解码 | DecodeQuestionRoute 1 处 | 0 |
| application 私有 Delivery 分支 | 1 处 | 0 |
| 业务所有者 | application 校验答案、控制资格、冻结预算、提交意图、调用 Execute/Reconcile | 保持；通过可选窄能力请求纯输入片段 |
| 协议所有者 | adapter 已拥有 route codec 和发送机制，application 重复解释输入表示 | codec、输入表示与发送前复核均由 adapter 负责 |
| 新增强制 backend 方法/组装 facade | — | 均为 0；LifecyclePort 不扩大，不新增 resolver/动态注册器 |
| 生产改动 | — | 4 个已有文件，新增 32、删除 9，净增 23 行；不将本项宣称为净删代码收益 |

收益是消除一个重复私有解释入口及其跨层同步需求，符合 P4 的变化面门槛。后续问答字段或输入框架变化不再要求 application 解码或分支；真实发送机制仍由 adapter 自己调整。新增的 opaque route/替代输入片段用例证明：即使该 route 不属于 Codex，业务协调也可校验答案、冻结预算、持久化原意图并处理 unknown，不需要添加后端知识。

## 实施边界

- backend 新增可选 LifecycleAnswerInputPort：只传原 envelope、SessionContext、Answer 和原文，返回模型输入片段。无 Record/Engine/Store/提交回调，也不创建资源或发送输入。
- Codex 的纯准备能力验证原 route 后提供片段；实际 deliverFeedback 在原位置重新校验原绑定/问答、核对冻结预算并发送。同步工具答复的 JSON 字节、转义及所有权、async 原文的字节及 caller 归属均保持。
- application 继续进行 nativeAnswers 的选项/歧义校验和 m.FreezeInput；之后仍先 saveIntent 再 invoke。不能提供能力、route 无效、预算溢出或提交失败时零发送；unknown 仅 Reconcile 原操作。
- 依赖守卫禁止 application 解码私有问答或在 answer 中解释 Delivery，同时允许具体实现组装；原 adapter 对 application/runstate/storage/workflow 的反向依赖禁令保持。
- 所有 reliable 包生产文件、application/preparation.go、application/worker.go 和 storage/store.go 与 P3 基线一致。Next/QualifyAction、共享准备顺序、原发送门禁、状态格式、CLI/workflow、启动/回执/停止机制及审计来源未改。

## 独立验证

| 检查 | 实际结果 |
| --- | --- |
| 受影响组件 | Codex adapter、backend、model、reliable、operation、workflow 六包全部执行，145 个顶层测试通过；1 个既定 helper 入口跳过，不算原生通过；最慢 Codex 包 35.17 秒 |
| application 定向 | 问答、内容只读流程、操作意图、人工对账、停止竞争、resume 相关 25 个顶层测试通过，6.36 秒 |
| 最终能力/边界反例 | 4 个顶层测试通过；与上述有重复，不相加伪造总数。覆盖未来 opaque 编码、accepted/unknown、提交失败、不支持能力、无效 route、Runner 预算溢出，以及依赖守卫正反例 |
| 纯值字节/身份 | RPC JSON 和 async UTF-8 原文的固定字节/摘要、caller/Runner 字节归属保持；零 lifecycle 原生能力即可准备；外部 generation/未知 codec 拒绝，不改原对象 |
| 公开 CLI 受控 | TestSRContentGenerate、TestSRContentConversation、TestSRContentQuestionAndUnknown，3/3，通过且无跳过；问答/unknown 组 12.91 秒，包含 answer 幂等重放、旧 question 拒绝、未知输入不重发及 close/stop 收尾 |
| 实际来源 | 新构建 Runner SHA256 57e1fd2fc06b660fbb2a03cf862e71ca2223c161aada259192e6cf79b3972720，source_digest b73832d145a81aaa951381055316505b9294de843e0733174786adef75f06562 |

本项没有协议/原生机制变化；按 Plan 使用受影响组件及新的公开受控闭环，不为纯值职责收口重跑真机或所有后端。此次未取得完整 L1、完整 Controlled、完整 L2/L3 或发布资格；P3 的完整证据仍只证明其原版本，不冒充本项新验证。未安装、部署、发布或改动外部 SteerLine。

证据：[汇总](evidence/personal-executor-p4-20261101/verification-summary.json)、[收益基线与实测](evidence/personal-executor-p4-20261101/metrics.json)、[本项生产 diff](evidence/personal-executor-p4-20261101/production.diff)、[公开闭环](evidence/personal-executor-p4-20261101/controlled/integration.log)、[构建记录](evidence/personal-executor-p4-20261101/controlled/steerrunner.exe.build.json)。[文件清单](evidence/personal-executor-p4-20261101/files.json)核对归档字节；JSONL 无损 gzip，解压后摘要见[原始流摘要](evidence/personal-executor-p4-20261101/raw-files.json)。P1/P2/P3 历史报告和原证据未改写。
