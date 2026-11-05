# 固定 Git 审计读取验收

2026-11-05，基线 `5275d880`，候选源码 `211471de49cfbc7e8ca7a45ba7ed36409a9d4384`。两份 Windows EXE 的源码摘要均为 `7ab01e3ab9403d93061143ce612ed76151cb62813bf20467dd8cb05d5a466732`。

| EXE | SHA256 |
| --- | --- |
| 两场共用写入 EXE | `c5cc93eaa82de6b88a53fed8b6bcadb3103bbd7e5ba5f38ea537c022e3d83cc9` |
| 独立冷读 EXE | `35e051cacc0be64b223736146cdfd7e1837839d5313a496333bc66b96d646d15` |

## 等价核对与受控回归

固定 Git 参数、顺序、查询边界与错误传播沿用基线。原 Git 读取器按 Kill、原 handle 树核对、原进程身份缺席和 Close 的顺序处理。树证明无法取得时仍清除 `Qualified`。读取器专用实现位于 workspace，原 Codex history 和运行 owner 结清调用保持原处。Codex 的 wire、原 owner 校验与证据判定、Orca 的读取前后原 worktree 检查均保持。

Windows 执行 `go test ./internal/execution/workspace ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport -count=1 -timeout 12m`，三包通过。应用层六个定向用例通过，覆盖无 caller Git fallback、原完成检查点、祖先检查和 audited 门禁。新增或扩展负例验证固定 Git 参数、读取器缺失树证明、读取期间 HEAD 变化及 Orca 读后 worktree 换代。现有 Codex 夹具仍核对原 owner 活性、HEAD 和 index 字节。

## 同一候选真机

**本机 Orca audited** Run `88cb246928e7a3b634e503faa44e548f` 用同一会话完成实施、审计修复和发布。实际 `gpt-6.1-sol` 的 effort 为 `medium → high → medium`，三个原 turn 均有独立来源摘要，三次临时登记均已撤销。

| 原检查点 | HEAD | tree |
| --- | --- | --- |
| 实施完成 | `256c36960c3d805c89050fc420861a4efb9fa970` | `36d765fc1a1f8bc97c9bfd77988756b6c6deb82f` |
| 审计修复及发布 | `453dacdec9bab673a1fd71c4cdc22a90984cde89` | `891351db771e17acecdf4d0374bf6ae1874d5c28` |

固定一项修复后取得真实 tree 变化，GitHub 的原 PR base/head、所属仓库及开启状态核对通过。显式停止关闭原会话并撤销恢复授权，前后均保留同一版本、观测时间和 SHA256 的单次结果快照。使用 SteerLine `e010373e` 的生产 `VerifyUpgradeDrained` 和两份固定 EXE 冷读原目录，原 Run/input 的 `orca_agent_session` 结算通过，数据库和原停机证据摘要一致。 [专用测试 PR #768](https://github.com/erranliu/SteerlineTest/pull/768)

首次 Orca 试跑 `9630ce90acdda8df846a7d89ea6de083` 沿用同时启用 progress/result-file 的旧模板，Runner 生成部分超过现行输入预算，业务未派发，原 Run 已停止且离线结算通过。上述通过场使用同一 EXE 在新目录提交，测试模板仅保留本场所需的 result-file。生产输入预算和判定不变。

**非 Orca Codex audited** 使用 `scripts/test.ps1 -Focused -Level L3 -Scenario local-codex.audited`，固定同一写入 EXE，报告为 `passed`，执行并通过一个场景。Run `b3083e724557ebca044cbe6a213df771` 取证审计模型 `gpt-5.6-sol` 和发布切回 `gpt-6.1-sol`。三个原工作区检查点均合格，HEAD `75d8baba413792ad82917de9c477a20b1340114f`、tree `0440a7e019d7a5ac2735cc397e1ac565fd508eb7`，该场沿现有无修复场景规则通过。独立核对真实 PR base/head，原停止收尾通过且禁止新输入。 [专用测试 PR #767](https://github.com/erranliu/SteerlineTest/pull/767)

原始 CLI、冷读与 focused 报告位于本机 `.evidence/workspace-fixed-git-audit-live/`。本记录覆盖受影响组件、定向应用回归和两场专项真机。
