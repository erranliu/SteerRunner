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

## 合入主线 #111 后的同候选复验

提 PR 前主线合入 `a8284d13`，改到服务启动及直连 Codex usage 采集。将其合入候选后，从干净源码 `94b5728753dc7ee84b45e0775823f03702fe6cb2` 重建两份 EXE。源码摘要为 `8595c7a1887079d3ce1b13f0d57c11e5a2979b71fd6a3732a0c5450f08bc737a`。host、usage/collect 和 resources 定向包均通过。

| EXE | SHA256 |
| --- | --- |
| 两场共用写入 EXE | `52045b385765c2af89d0e9163fc4d5fdff6403dc7ade352d10faaf1cae58326d` |
| 独立冷读 EXE | `782563a2fdf59567425578182b989870c86259258f2279a6b72e4c8dcf4dea00` |

- 本机 Orca audited Run `0ca1341e22937c5566cf54eb219ec0b9` 通过。实施 HEAD/tree 为 `16cf651c79eb239eef2ad571c402ca7c2b31a187` / `60f48469102fead07c967a66757dc8fd04bda91f`，审计修复和发布 HEAD/tree 为 `8e81b2668d86f0fdfaaa89579b72e396df40b28f` / `b0f9026d2bcbce3883899cb900f84ea08afab199`。实际角色切回、三次临时登记撤销、真实 PR base/head、单次快照和原会话停止均再次核验。双 EXE 通过原目录排空。 [专用测试 PR #769](https://github.com/erranliu/SteerlineTest/pull/769)
- 非 Orca Codex audited Run `972a6f1c73c0db6b8eb4ac8e9c6cb5d5` 的 focused 报告通过。审计模型为 `gpt-5.6-sol`，发布切回 `gpt-6.1-sol`。三个原检查点均合格，HEAD/tree 为 `38b9ed985e9c6069bb6a728c910a059fc20e78f4` / `bbedb31f432e9e913f790e96f9a161a5d1e87ac7`。真实 PR 核验和停止收尾通过，原 Run 禁止新输入。 [专用测试 PR #770](https://github.com/erranliu/SteerlineTest/pull/770)

最终候选原始材料位于本机 `.evidence/workspace-fixed-git-audit-main-live/`。

## PR #112 合入前审计

与主线 `a8284d13` 对照，固定 Git 读取和读取器结清的实现体在包名及入参改写后相同。路径比较增加的非空检查在原绝对路径条件下不改变有效输入。现行 `InspectAudit`、原完成检查点、Workflow 发布判断及运行停止代码保持原样。Orca 的前后 worktree 核验和 Codex 的原 owner、readiness 与远程证明校验仍在各自调用方。

复验核对 #111 后的两份 EXE 实际摘要、源码身份和 PR 生产文件。重新读取原始检查点及真实 PR 回执。Orca 原目录用 SteerLine `57e3c52` 再次通过双 EXE 升级排空，数据库与原停机证明摘要一致。直连 Codex 原目录另以固定写入 EXE 冷读，确认 `service status` 为 `not_running`，`service verify-stopped` 返回同一原 Run 且 `resources_settled=true`。直连记录的资源结清与 Orca 的升级排空分别取证。

合入前重跑三个受影响包的 Git 读取、HEAD 竞争、读后工作区换代及读取器结清定向用例，并重跑六个应用层来源与检查点用例，均通过。
