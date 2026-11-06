# Plan A：Orca 停止合同解释收口

日期：2026-11-06。状态：P0–P4 已完成，见[独立验收](../acceptance/orca-stop-contract-boundary.md)。依据 [Spec A](orca-stop-contract-boundary.spec.md)，调查基线 main `6dd4360a42314e65b427627508ffd2ee3e4287da`。用户随后授权先 A 后 B 实施；本项已独立验证，不自动提交、提 PR 或部署。

## P0：固定基线并审计设计

- [x] 刷新 main，在实际实施基线上复核 #123 单次解码、#125 开发消息与停止撤销；若相关实现变化，先更新本文和消费者清单。
- [x] 保留本次四份文档及其它已有工作，使实际源树包含上述调查基线或已审阅的新主线，再开始代码编辑和基线捕获。设计时的 `e2e4ee69` 不满足此条件；只用 git show 读过新代码也不满足。按实际分支关系合并或从主线建立实施分支，不为 Rebase 提示改写已验证历史；真实内容冲突按 AGENTS.md 在独立 worktree 解决。
- [x] 列 Engine.Handle、两处 settleRunStop、port、VerifyStopped、freeze 及停止诊断的真实调用；固定原谓词、首错顺序、诊断 guard、命令回执/提交点，不混用旧工作副本与新主线。
- [x] 捕获受影响源与既有测试摘要、生产持久声明/编解码和代表原字节。覆盖 #125 当前字段、旧 profile/State/NativeProof、操作回执、结果与 capabilities；捕获失败立即修复，不忽略 Marshal 错误，不以空输出证明相同。
- [x] 设计核对：分类只搬迁既有标记判断；严格验证仍在原入口；不得给 stop/结算新增 decoder、I/O 或失败门槛。确认不将 adapter 的 closeSettlesAgentSession 与 application 的开发 Workflow 谓词混为同一规则。

## P1：后端知识归 adapter

1. 在 orcatransport 增加窄纯查询，原样承接 protocol/profile 标记与后端冻结 scope 条件；不接收 Record/Workflow/Store，不引入接口平台、持久字段或可变缓存。
2. application 保留 Workflow 组合、已存 scope 非空条件、相等比较及原调用顺序，替换私有扫描。freeze 的合法写入接线保持。
3. 将成功诊断选择放入 adapter 窄方法/函数；保留具体 Orca 端口、端口 scope 非空及原 stopped guard，选择仍使用原已存 scope。mock/非 Orca 端口和失败分支行为不改。
4. 只改上述读取边界；原 stopBudget、意图/Operations 补入、结果合并、锁与事务正文不顺带重构。

## P2：行为和共享夹具验证

- [x] 用固定的基线预期验证 Spec OSC-A02 的分类矩阵，而非只对照两个同源新函数。最小/坏 payload 仍只是标记分类；已有 decoder 对非法配置的拒绝另在原入口验证。
- [x] 复用 result_snapshot_test、orca_structured_test、shared_native_stop_test；覆盖实际 Handle 准入顺序、两处结算、具体/替身端口诊断、scope 缺失/篡改、离线原错误。保留旧断言，不为了调用新函数把夹具统一改成完整 adapter 配置。
- [x] 跑完整 orcatransport 受影响包，以及 application 中停止、scope、结果封存和 `TestDevelopmentRunMessage*` 定向组；覆盖 stop 前后消息/答案 saved/frozen/intent/accepted/rejected 窗口、原 unknown 对账及恢复。host/storage/contracts/usage 按实际 diff 选择回归，编译 native driver/helper。

## P3：原格式与公开入口

- [x] 对 P0 声明和代表编码逐字节比较，差异必须解释为基线上游变更或本批缺陷；本批不允许格式差异。核对无新格式版本、持久字段和旧记录重写。
- [x] 生产/测试冻结后正式构建。以同一最终 EXE 经 scripts/test-foundation.ps1 明确 Pattern 运行 `TestSRSingleExecutionColdStop`、`TestSRNativeUnknownAndStop`、`TestSRRestartPreservesAuthorization`；保留零匹配保护和原断言，不将定向通过记成完整 Controlled。
- [x] 若公开替身不覆盖本次关键 scope/开发关闭路径，以组件及实际应用入口补齐，明确其证明范围，不能用替身通过代替 Orca 真实关闭资格。

## P4：独立交付点

- [x] 在 docs/acceptance/orca-stop-contract-boundary.md 记录修改前后知识所有者、仍保留的 scope 分支、持久对照、命令/产物、失败/skip 和未验范围；收益按私有解释减少验收，不承诺提速或净删。
- [x] 完成最终 diff/消费者核对并固定 A 源码摘要；OSC-A01–OSC-A05 完成后才允许 B 修改共享夹具。后续实施若失败，保持 A/B 证据分离，不将 B 或上游修改倒记为 A 初次通过。
- [x] 按 docs/testing.md 区分定向/完整、合并前/合并后、组件/公开受控/真机；没有正式实际结果时不更新长期能力资格。中央 README 无新增导航需求则不改。

## P5：用户追加实现复审与 PR

2026-11-06 用户授权“审计后提 PR”。已完成[实现复审](../acceptance/orca-stop-reader-reaudit.md)，未确认生产缺陷；修复原输入不变断言的 Payload 浅拷贝，并补真实在线/离线 scope 与错误优先级回归。生产文件保持，最终复验及产物另存，不改写 A/B 初轮结果；按追加授权提交、推送并创建 PR，不自动合并或部署。
