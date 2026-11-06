# Orca 会话停止与检测结果文件修复

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-03，独立 worktree `codex/fix-native-execute-result-settlement`，基线 `0991577582712e67647eb37385e39fef5152f59a`。用户要求：同一 PR 收口长期设计并修 Runner；真实环境检测必须通过后才提 PR。

## 原现场与范围

安装版检测 `75ab883ae6074cdc8d979b071fa253d1`，Run `cabea8df00c59e3c4b4ae09157b8974a`，脚本因缺少 `STEERLINE_RESULT_FILE` 退出 1；原 close 已保存 ok 仍被无条件保持 unknown。失败证据在 `C:/SteerlineDebug/install-orca-native-check-20261103/`，不改写、不补写其报告。

用户明确接受 Orca 额外资源清理不保证，原 session 关闭确认即可正常流转。长期权威为 [Orca 执行链路设计](../../../../orca-execution.md)。原 native close 成功回执只证明 `orca_agent_session`；不置 `WritersAbsent`、不假称所有后代/凭据/主机资源已清理。

## 固定指令与接线

旧接线把 ResultFileInstruction 的 `STEERLINE_RESULT_FILE` 替换为相对路径，仅告知文件位置，没有要求调用脚本时设置变量。新接线保留原通道指令，增加固定资产 `internal/execution/workflow/orca_file_environment.md`，动态部分仅含原 Run 的相对文件绑定。Caller 正文、脚本与参数不修改；已有 1000 字节 Runner 预算不扩大。业务原因是现有检测脚本读取命令环境中的绝对结果路径，Orca 原生会话不直接支持这组环境注入。

固定新增正文：

```text
Orca file bindings are worktree-relative. Before invoking a script that reads them, resolve the existing bound files to absolute paths and set the named environment variables in that command process. Preserve the caller executable and arguments; do not initialize the files.
```

固定新增资产为 LF / 275 UTF-8 字节，并以 .gitattributes 固定 LF。仅 result-file、32 位 Run ID 时，通道附加正文由 226 增至 574 字节（+348，含换行及绑定 JSON）；双通道仍受原 1000 字节预算约束并有回归。独立只读审核确认正文与接线，发现并已修复原 Ref 围栏、公开诊断、当前文档冲突三项；最终修复后复审及真实资格另记。

## 验证与现场

持续证据根：`C:/SteerlineDebug/runner-native-result-session-20261103/`。初次文件绑定测试发现新增 provenance 来源未声明；保留失败日志，声明封闭来源后重测。此前测试不当作最终通过。

当前状态：真实检测、会话范围结算、SteerLine verify-stop、同构建重启、正式完整 L1 和所选受控回归均通过；独立复审通过，完成这些验证后才提 PR。不声明所有后代退出、完整 Controlled 或全组合原生发布资格。

### 实际通道正文差异

以下固定示例 Run ID 为 `0123456789abcdef0123456789abcdef`，仅计算结果通道附加正文（含各输入片段前导换行，不含用户正文和其它既有指令）；实际受测输入使用原 Run ID。

改动前（226 UTF-8 字节）：

```text

.steerrunner/result/0123456789abcdef0123456789abcdef/result.bin: independent raw<=65536B; coordinate writers, same-dir atomic replace. Sealed once after execution/producers stop, never turn end/pr_ready; not a success signal.
```

改动后（574 UTF-8 字节）：

```text

STEERLINE_RESULT_FILE: independent raw<=65536B; coordinate writers, same-dir atomic replace. Sealed once after execution/producers stop, never turn end/pr_ready; not a success signal.
Orca file bindings are worktree-relative. Before invoking a script that reads them, resolve the existing bound files to absolute paths and set the named environment variables in that command process. Preserve the caller executable and arguments; do not initialize the files.

Runner file bindings: {"STEERLINE_RESULT_FILE":".steerrunner/result/0123456789abcdef0123456789abcdef/result.bin"}
```

模型须显式为原脚本绑定变量；该提示不冒称 Orca 已注入环境。真实脚本报告和原会话回执共同验证接线，而不是以黄金样本更新代替批准。

### 固定受测产物

- Runner EXE SHA256：`1b8b420128c9fe9a582034d0045c89823d30ab46eb159bab005fd62e21a6fd41`。
- 构建来源基线：`0991577582712e67647eb37385e39fef5152f59a` + 本 worktree 修复（dirty），源码摘要：`7ec8556b025733b73c7c56e9efd61235e4d66212a05db4e9fffd2a3c4fd3ebea`；构建记录 `candidate/steerrunner.exe.build.json`。受测之后仅补验收文档，不把待提交 SHA 冒称受测二进制来源。
- 调试应用：SteerLine `cca5114297564df176b27145e66fda71d2582682`。通过正式 `new-debug-release.ps1 -SteerRunnerExe <candidate>` 创建全新 home、状态及项目仓库，原安装和业务历史未修改。
- 真实后端：已有 Orca 本机 native Codex、`gpt-6.1-terra / high / full_access`。使用 `C:/Projects/SteerlineTest` 原环境文档和脚本，在新测试仓库固定提交，不修改脚本逻辑或代写报告。文档与原失败提交 `9d81d9c` 字节一致；脚本仅由 Git 将 CRLF 规范化为 LF，规范化后逐字一致（原 SHA256 `1ebc26ddcc91f79db3632e52dc2faaedfc515492bafc917526e3d8424f2fb11c`，受测 LF SHA256 `ccb339f9a4b063f0106c6a228a2b34691a12ddf3af221f24020f50195b40ac42`）。Orca 注册、测试专用 GitHub base 分支推送属于前置；没有创建测试 PR。

### 首场前置失败保留

首包 `packages/dev-20261103-185041-cca51142` 未先注册到 Orca，检测 `2be27c5e03bd4ceaa2de50b1fdf8a9fa` / Run `a5a24d3016dd90060393082b560d8769` 在 prepare_workspace 返回 `repo_not_found`，没有进入 prepare_session/start_turn。正式 environment_stop 撤销授权，随后正常停服，writer 已退出；原创建仍以 pending/unknown 保存，停机结算未通过，不将它算作正向通过或清理成功。保留 `live-first-prerequisite-failure.json`、`failed-prerequisite-stop-*`、`first-fixture-stop.log`、`first-fixture-runner-after-stop.json`。没有重放该原创建，没有改状态、删库或用新 home 授权接管原 Run。

修正的是新测试仓库的 Orca 注册前置。下一包使用新仓库、新 home、新请求独立测量；不是重试一个未知副作用，也不改写首场结果。

### 真实环境检测通过

成功包：`registered-packages/dev-20261103-185647-cca51142`，固定测试源码 `13badf4aa8cfbeb0718c60c853ad8d8b2139031d`；先注册新仓库，再推送该包自己的 `codex/debug/dev-20261103-185647-cca51142/base`，正式 live prepare/start 后通过应用 `/commands` 提交 environment_start。

| 核验 | 原事实 |
| --- | --- |
| 检测 ID | `74d6a382749b4211884430b16aabbf3b` |
| 原 Run | `5d18c14b936bbb8002685244792207d6` |
| 脚本执行 | Agent 解析绑定的现有文件绝对路径并设置命令环境；原入口一次调用，报告工具全部 passed，原生可核验回复记录退出码 0 |
| 平台状态 | `state/status=ready`，`settled=true`；round 的原报告 `completed=true, ready=true`，三项 PowerShell/Git/Python 检查 passed |
| Runner 状态 | `status=completed`、`closed=true`、`stop_status=stopped`、scope=`orca_agent_session`；诊断正确表明额外清理未保证 |
| 原始结果文件 | `collection_status=closed`，1782 字节，SHA256 `35ff1bddb824a8575b859c0d42ba95e3a28802993ada3a814d8bbd325315df94`；原 activity/round/source/input/context 全部匹配 |
| 正常停服 | 原应用 stop 成功，Runner session scope 离线 verify-stopped 成功；SteerLine verify-stop 成功 |
| 同构建重启 | 项目仍 ready，原检测 ready；原 session/turn 不变，start_turn/stop_execution 各一条，封存结果 SHA 不变 |
| 最终收尾 | 重启验证后再次正常停服，离线 session scope 结算通过；不保留该成功包的活动 writer |

原证据：`registered-detection-*`、`registered-observation-*`、`registered-run-completed.json`、`registered-result-file.json`、`registered-script-result.json`、`registered-offline-stopped.json`、`registered-steerline-verify-stop.json`、`restarted-*.json`、`live-verification-summary.json`。这是新会话 scope 的真实资格，不声称额外资源全部清理。

### 自动回归与独立复审

- 初次 provenance 来源未声明的失败已在本聊天原执行结果保留；修复后 `focused-first.jsonl` 为 21 根通过。
- `owners.jsonl`：orcatransport、contracts、resources、model、workflow 五包完整普通集，261 根通过；3 个明确 live/probe opt-in skip 不计真实资格。此执行早于最后 Ref 围栏修正，不能冒称修正后的完整集。
- `application-regression.jsonl`：28 根通过；最终 `review-fixes.jsonl`：29 根通过，包含原回执、替换 runtime/worktree 拒绝、文件环境绑定、一次执行和停止。
- 独立只读复审发现三项并修复：原资源 Ref 围栏不足、成功诊断误称 terminal、当前合同正文残留旧规则。修复后复审通过；提示原文、实际 LF 字节与接线独立复核通过。
- `controlled-content-stop/summary.json`：content-flow 与 execute-stop 两组合的 10 个顶层场景通过（401.33 秒顶层测试耗时）；覆盖 result-file final-seal 四变体及 graceful/kill/creation_receipt/pending_stop 重启。该报告明确为 controlled-selection，不冒称完整 Controlled 或 L2/L3 资格。
- 正式入口 `scripts/test-prerequisites.ps1 -Stage L1 -OutDir <evidence>/L1`：`stages.json.status=passed`，完整 L1 core 与 native preflight 均成功；Go JSONL 801 个顶层通过、16 个已登记 opt-in/helper skip，0 fail。框架守卫和 Windows PowerShell 检查通过，native driver 编译及零匹配保护通过。16 个 skip 不计为真实模型资格。代码与资产在受测期间保持固定，随后仅补文档。
- 最终候选源码摘要复核通过：`candidate-source-recheck.json`；所改 Markdown 本地文件链接与 git diff --check 通过。
