# Spec A：Orca 冻结记录证明与执行构造分离

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-05。状态：设计审计后已实施，A独立验证完成，见[验收](../acceptance/orca-reader-execution-boundary.md)。基线 main `6940c59872d2b857a97b97b0f98090493379a219`。依据[长期产品契约](../../../../spec/steerrunner.spec.md#长期产品承诺)及[现行 Orca 组合](../../../../orca-execution.md)，配套 [Plan](orca-frozen-reader.plan.md)。A 验证完成后才实施[B](orca-legacy-execution-retirement.spec.md)。

## 职责与消费者

新 Orca Agent 只走结构化协议，旧终端由原构建排空；旧记录仍有真实只读消费者：application.VerifyStopped → Orca.StopEvidenceScope/VerifyStopped，usage 的原状态/输入绑定，progress/result 的冻结工作区读取，历史严格解码。当前两个停止证明函数却构造完整 NewLifecycle，连带执行 state-feed、输入查询、创建/上传及 observer/Job 的实现依赖。它们目前没有实际启动执行，本批解决构造耦合，不宣称修复了意外执行。

新增包内 frozenRecord，只包含克隆的 setup、已严格解码 profile/version、原资源/会话绑定和只读 CLI/probe。它没有 Execute/Reconcile/Stop/observer/feed/Job/upload 能力。原验证函数正文迁入这一读取职责，执行对象通过窄内部投影复用相同核验；不复制第二套校验，不增加端口注册、运行开关或持久模型。

scope/VerifyStopped 不再调用 NewLifecycle；读构造保持原 agent→ProcessHost→profile/身份校验顺序和错误。只读 CLI 仍沿原队列、超时、命令长度与配对 host 绑定；不能弱化为信任 saved stopped、终端卡片或缺失状态。结构化 close profile v1/v2 的原成功/未知区别保持。

## 保留与格式

旧 Profile/State/Turn/Control/TerminalOwner/NativeProof、所有 codec、严格解码与原身份校验保留。原 local Codex 根进程、完整终端清单、Pi owner/退出文件证明和结构化 session close 回执含义保持。读取不发送 create/send/close/kill/upload，不重建资源，不更改记录。

Lifecycle 的活动执行、prepareWorkspace、结构化恢复/停止和测试注入暂不在 A 删除。factory nil、旧 profile、共享 fakeHost/fixture/structuredFixture/creation fixture、真实只读查询/usage/文件通道都核对并保留。A 不退役任何旧专属测试，也不改其断言。

state/v8、Request/Record/Operation/Receipt/opaque payload、结果/事件/命令及能力编码不变；不新增持久字段，不改预算/授权和原提交边界。PC-R01–R06/R08/R09 保持。

## 最小验收

| 编号 | 验收 |
| --- | --- |
| A01 | scope 和 VerifyStopped 的生产调用图不再进入 Lifecycle 构造；frozenRecord 不具备执行/输入/停止派发方法 |
| A02 | 新旧 profile/状态的成功、缺失、篡改、错身份、旧 close 未知及原 stopped 重核验保持；只读 transport 拒绝任何 mutation |
| A03 | 现有 reader/usage/progress/result/停止/当前结构化回归保持，既有共享夹具和断言不删 |
| A04 | 原持久声明/编解码与代表编码相同，记录实际命令/产物及失败/skip；不冒充全级或真机资格 |

收益是只读证明不再随执行构造扩展，且为删除旧写链建立可检查的边界；不是校验减少或未经测量的提速。
