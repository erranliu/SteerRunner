# P2 Orca Codex 普通开发结构化验收

2026-11-04。以 #100 合入后的 `ee1619b4` 为基线，复用 `edda8f8b` 及其 M2 协议和用量实现。仅新受理的本机与明确配对远端默认 Codex `development/2` 启用。原 Workflow 继续拥有实施、反馈、报告回退与发布验证。外部使用既有 `submit`、`reply`、`stop` 和查询接口。显式启动覆盖、受管 Codex、audited、Pi、非 Orca Codex 和 parser 保持原路由。

## 先行停止与原目录升级门禁

第一版双端各跑两个原回合，已确认原会话关闭、v1 文件封存及资源离线证明。两条 Run 的持久 `resume_required=true`、`can_resume=true` 仍被原 EXE 和第二 EXE 读出，SteerLine 正确拒绝排空。根因是 Runner 沿用多轮 stop 的恢复授权投影。修正限定在新冻结关闭合同的结构化 development，确认原停止时同事务撤销恢复授权。原会话关闭确认或创建未派发才可进入该停止状态。新 resume 和重复 stop 均拒绝，已有原请求回放沿原机制。缺失回执或未决控制仍不能签收排空。

修正版重新创建本机 `ce9914beaeaedfe032c2a05db1d3146d` 和远端 `98e7f8ae3fa043b4822204c6d6a9c040`。各自两个原回合的 close、停止后 v1 封存、原 writer 退出和 `service verify-stopped` 均通过。SteerLine 未改动的 `VerifyUpgradeDrained` 对同一原目录、两个不同 SHA 的 EXE 冷查通过。测试入口仅用 Go overlay 调用原消费函数。旧 M2 的四条 Run 与旧配对 P1 的两条 Run 冷查仍未结算，未改记录或补发原操作。

## 同一候选双端正式验收

运行 EXE SHA256 `cf82b330ed702b580b63870b0781322c452e08256d8bd8cf1686f94da0ff91e9`，构建源码摘要 `074086f51ae934a101d77fae5811de8f1e1b115c4a2cd69114387e2d25b91a56`。第二个 EXE `e26a15de3819e8c8e06b34c967648e20033314aae16b58434f00ea6c83505393` 来自同一运行源码，仅用于冷查。构建后修正了一条应用层测试的合成 profile fixture，运行代码未改。见 [构建记录](evidence/orca-structured-development-p2-20261104/runner-build.json) 和 [逐 Run 摘要](evidence/orca-structured-development-p2-20261104/verified-summary.json)。

| 项目 | 本机 | 明确配对远端 |
| --- | --- | --- |
| 正式普通开发 Run | `4a9a886fe6228116762016a0668c424a` | `c73d29144e71e34e49165b93be2a6100` |
| 发布验证 | [SteerlineTest #763](https://github.com/erranliu/SteerlineTest/pull/763) | [SteerlineTest #762](https://github.com/erranliu/SteerlineTest/pull/762) |
| 结果和用量 | v1 封存 31 字节，11 条计量响应 | v1 封存 32 字节，10 条计量响应，临时授权已撤销 |
| 活动停止 Run | `b748a973e461f1f938fb25ae60380cbf` | `be195950cf70b68bc3a40c7e4f2c35a2` |

两端正式 CLI 均经 `needs_input` 接收一次合法多行反馈，随后实际触发 `implement.report`。触发源为原生成功、归属已核实的 `Violet pebble Q7.`，该结果缺少原 Workflow 所需的完成标记或结构化报告。Runner 生成同会话的独立报告输入，原 command、turn、结果 ID、输入版本及原工具策略逐项可核对。报告返回 `done`，后续发布验证对照真实 repo/base/head、原测试文件及实际提交。两条测试 PR 保持开放，发布后迟到反馈被拒绝。

独立只读回读确认每条普通开发恰好四个原 user 文本块和四个不同的 turn，正文与原冻结输入逐字节一致。各回合取最后一条完整 assistant 消息。两条活动停止各有一条原输入及活动标记，停止时仍在原活动回合。四条 Run 各有唯一持久停止意图和同一原会话 `close ok`。普通 Run 在等待反馈和 `pr_ready` 时结果仍 pending，显式停止后才封存。活动停止保留原 4 字节 seed。停止、重启后文件版本及字节一致。逐 command 用量归属、费用与结果分开验证。原 writer 退出后，四条正式 Run 均通过正向离线会话范围证明及 SteerLine 双 EXE 原目录升级预检。该证明的范围是原会话。

## 定向复验

受控覆盖两端结构化会话、续轮空闲与 fence、丢响应零补发、输入归属、公开合同、用量、历史快照、#98 outcome 边界、确认停止后的授权撤销及报告回退策略。实际执行以专项资格记账。未运行完整 L1/L3。

同一 EXE 的 P1 交叉回归完成。本机和配对远端分别跑多行 `execute/1` 正常结果和活动停止，四场均 `stopped`、v1 封存，同构建重启后原状态与字节一致，离线会话范围证明通过。SteerLine `3996bf20` 使用独立应用 home 和既有登记的隔离仓库执行本机正式 `environment_start`。Run `a1d6a49c556bbcd346680a1446719acb` 取得原生 `succeeded`、报告 completed/ready、原 v1 封存和应用 ready，正式停服及双 EXE 原目录升级预检通过。新仓库登记尝试在执行前被自动审批拒绝，复验使用已有登记，未改变 Orca 设置或登记。

原始证据在调用 worktree 的 `.evidence/p2-fixed-live/`，包括固定 EXE 构建记录、`gate-upgrade.log`、`formal-upgrade.log`、原 Run 回执、公开历史和 P1 交叉结果。报告通过范围仅对应此次合入前固定构建。

## 合入前审计补验

核对原报告来源和公开 turn。两端正式报告均绑定原 `Violet pebble Q7.` 的 command、turn 和结果 ID，独立的报告 turn 均无工具调用，随后由原 Workflow 核验真实 PR。原正式开发与报告证据仍对应上文固定构建。

修正旧结构化 development 的文件提示及公开说明，使快照仅由显式停止触发。保留旧 M2 离线未结算诊断，将恢复授权收尾明确限定于新 `development/2`。新 Run 的 v1 提示、发布判定和原目录排空条件保持。

用审计构建新跑双端活动续轮停止。两端均先取得原 needs_input，再各派发一次结构化反馈。停止前确认活动的第二个 turn 不同于第一轮，公开历史回读确认第二轮为 interrupted/cancellation。每条仅一个原 stop 意图和成功 close，v1 封存，拒绝新的 resume 和重复 stop。原 writer 退出后，未改动的 SteerLine `VerifyUpgradeDrained` 通过。最终冷读取器也对原门禁、原正式及新增活动续轮三个目录通过双 EXE 原排空核验。最终读取器与活动续轮构建的运行源码差异限于旧 M2 的离线诊断。

使用原 EXE 和最终冷读取器逐条核对旧 M2 四条及旧远端 P1 两条 Run。冷查视图、未结算 scope 和输入身份相同，六份原快照摘要与字节一致。原数据库文件摘要一致。受影响 Go 用例通过，验证范围仍为专项与定向。见 [审计摘要](evidence/orca-structured-development-p2-20261104/merge-audit-summary.json)。
