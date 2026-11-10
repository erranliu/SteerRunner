# 已停止 Run 的受管 worktree 复用验收

日期：2026-11-10；工作树基线 `87b95cef`。本记录为合并前验证，未部署、未修改安装环境或合并 PR。公开行为见[工作区复用契约](../worktree-reuse.md)及[模型用量契约](../model-usage.md)。

## 实现

- 新 execute/1 请求用 `reuse_worktree_from_run_id` 指向同一 state-dir 的最后持有者；仍为单次输入、单次执行、结束停止。使用全新请求、Attempt、generation、Session、命令和用量身份，记录 worktree 来源及 origin；原 Run 保持终态。
- 复用原停止证明、服务 lease、worker gate 和受理事务；按稳定工作区 incarnation 仲裁，仅允许串行来源链。旧控制、后台派发、事件与清理不能改写已转交来源。未知操作或停止、未结算命令、未退出 parser、未封存原结果均不授予接管资格。
- 复用原 worktree、分支和 Git 管理目录，保留提交、索引、未提交及未跟踪内容、真实 rebase 冲突状态。没有创建身份、原目录被移除或同路径替换时拒绝，不自动 reset/stash/clean/recreate。
- 独立 parser 在原事件账本中记录 runner_internal 来源、每次 CLI 调用唯一 call_id、原 command/result/step、配置模型及 Token。先登记 pending；缺少可信用量为 uncovered/null。主会话缓存补归属不会覆盖 parser 模型，main_session 总量与覆盖含义保持。SteerLine 仍负责价格、跨 Run 金额汇总和预算。

## 验证与证据

测试使用 Windows/amd64、Go 与仓库 PowerShell 入口；受控原生协议夹具、临时 Git/SQLite，不调用真实模型或创建真实 PR。沙箱阻止 Git for Windows 创建 MSYS 内核对象后，Git/进程测试改在普通宿主权限下执行，未调整宿主安装或配置。

| 范围 | 结果与证据 |
| --- | --- |
| 本机工作区定向单元 | `go test ./internal/application -run '^TestWorktreeReuse' -count=1 -timeout=4m` 通过；三轮累计提交/脏文件/未跟踪文件、真实 rebase 冲突及结果通道归属、缺失/同路径替换拒绝、未知结果/命令拒绝、相同 incarnation 的另一 Run 竞争与旧事务拒绝 |
| Orca 包 | `go test ./internal/execution/adapters/orcatransport -count=1 -timeout=5m` 通过；本机/配对位置的夹具核验原停止、原 instance、只读接管、全新 session envelope，旧输入/close/shell 不继承；原工作区审计合同保持 |
| 计量与公开合同包 | `go test ./internal/usage/... ./internal/storage ./internal/textanalysis ./internal/contracts ./internal/resources -count=1 -timeout=5m` 通过；内部调用登记/结算、原配置归属、CLI 重试逐次记录、null、去重、缓存模型不覆盖、主会话总量和历史页不变、准入及退出码 |
| 四 Run 公开入口 | `.evidence/worktree-reuse-final-current/result.json` exit=0，SingleExecutionColdStop 通过；同 worktree/分支四 Run，累计成果、并发接管唯一胜者、同请求回放、活动来源拒绝、旧 stop 隔离、活动后继跨服务重启、最后一个新 Run 实际提交遗留修改，原 Run 和 result-file 不变 |
| 停止/文件/结果获取定向组合 | `.evidence/worktree-reuse-controlled-final/summary.json` passed；execute-stop、完整 runtime-files、result-parser 共六项顶层测试，包含 ResultFile 两类各四个 enabled 变体、Restart 四类、parser 三类。该构建早于最终 incarnation 仲裁及 parser 缓存归属修正；不冒充最终完整 Controlled 资格 |
| 最终 parser 公开计量 | `.evidence/worktree-reuse-parser-coverage/summary.json` passed；parser-unsupported、parser-complete、parser-tool。后两项明确检查主会话 scope 保持、内部记录恰一次、独立模型/effort、原命令/结果/阶段、covered Token=2 或 uncovered/null |
| 完整 L1 | `./scripts/test.ps1 -Level L1 -OutDir .evidence/worktree-reuse-l1-verified` passed，exit=0；框架、internal 包、21 项扩展测试、原生 driver 编译与零匹配保护通过。入口记录 test_count=767，按原允许列表跳过 12 个 live/helper 测试，未冒充真实原生环境资格；完整记录见该目录 summary.json/l1.jsonl |
| 产物能力发现 | `.evidence/worktree-reuse-contract/summary.json` 两项均 true；固定 EXE 的 `capabilities --offline --json` 发布 worktree_reuse 与 runner_internal 来源/覆盖声明 |

公开入口四 Run 构建与最终 parser 构建分别冻结在各证据目录的 `steerrunner.exe.build.json` 和 source-identity 中；两者之间最后的生产改动仅为内部事件缓存归属排除及恢复原 Orca 工作区读取校验。核心工作区仲裁已在四 Run 构建中复验。各构建不混用完整资格；定向组合均为 `qualification_eligible=false`，未取得完整 Controlled、L2/L3/L4 或合并后资格。

早期失败保留：沙箱下 Git/MSYS 权限错误；首轮 L1 因测试期间源码变化触发固定输入门禁；首次 CLI 夹具编译错误及新错误码退出映射缺漏；夹具按 cwd 固定 session ID 的碰撞；Orca 原工作区校验收紧导致一个原测试失败。对应代码/夹具已修正，相关路径分别重跑通过，失败记录不改写成通过。

## 合并状态与缺口

独立复审阶段再次刷新 origin/main 至 `bda3bc8ca54a01975420ef71acb8a59748edd904`，并通过 GitHub compare 确认目标相对工作树基线领先一个提交及实际文件变化。`docs/testing.md` 为唯一共同修改文件，三方内容合并检查 exit=0；生产源码没有共同修改。复审证据为 `.evidence/review-merge-check/summary.json`；PR 的 GitHub 实际合并状态在创建后另行核对，不用本地检查冒充平台判定。

- 尚未运行真实 Orca 本机/远端模型或真实 PR 冲突解决端到端验收；Orca 本次只有包级/协议夹具验证。真实 Git rebase 冲突保留已单元验证，四 Run 收尾已受控验证。
- 没有保存原创建 incarnation 的历史 Run 明确拒绝，不能根据当前路径补造创建证明。既有未知停止仍须原机制核实；无法核实时不复用。
- Orca 停止沿原 session 范围，不扩展为脱离受管范围的后台进程清场。子 agent、工具内部模型及无法观测的供应商调用仍未覆盖；崩溃前已登记但未结算的内部调用保持 pending/null，未按零估算。
- 未建设工作区服务、插件或调度系统；SteerAgent 仍只读，后续执行由提示词和现有公开接口驱动。

## 独立代码复审与修复

两名独立只读审查者分别检查接管/未知停止、旧清理/用量去重；确认问题后由主实现及接管审查者修复，原审查者复核。未发现待修复的新增确认问题。

- **P1，晚文件初始化双写**：原四秒期限结束后物理初始化仍可能继续；已停止、封存的来源原先可被接管。现在派发任何目录/exclude/seed 写入前持久登记 pending，活跃计数覆盖完整 I/O 及 close，物理写入结算后才 settled；新 Run 等待结算，原业务/停止预算不变。远端写丢回执，即使随后读取成功仍 pending；重启或旧快照不能抹除证明。补充封存并发结算检查，避免只因内部 marker 更新而丢弃唯一封存机会。
- **P1，Orca 路由变更导致无法执行及停止**：新配置若改变 transport executable/args，复制的原工作区路由与新 profile 不一致。现在在受理前明确拒绝，测试覆盖不同实际 executable 路径与 args，来源保持不变。
- **P1，首次输入前停止后无法继续复用**：未派发或已明确拒绝的原 submit 曾保持 pending/unknown。现在只在有未派发证明，或全部原输入具有 ValidateFor 成功且无接受/启动/完成证据的 rejected 回执时结算 failed；真正未知、无效回执或矛盾事实仍保留待对账。
- **P2，可选 usage 错误影响业务结果**：usage 改为独立解码，类型错误只使计量 uncovered/null，合法业务文本仍可采纳。
- **P2，冲突流误计 covered**：重复完成、失败、工具事件、坏 JSON 或扫描错误清空计数，不能沿用第一份 Token；完整可信用量与业务 final 失败仍分别解释。
- **清理隔离加固**：旧 closeWorker 的身份检查、封存、观察关闭和 shutdown 共同持有原 gate，与接管事务串行化；已经转交的旧清理不再进入后端。

新增定向回归通过：接管/RuntimeFiles/StopSettlement 合计命令执行通过（107.164 秒），结果封存并发结算及旧清理串行化回归通过（0.998 秒）；textanalysis、storage、usage 全包通过。两个审查者分别复核停止证明和物理写入/close 资格门，SQLite/file-channel 接缝独立复现修复前问题并验证修复后阻断。该接缝验证不等于真实后端 E2E。新夹具首版因初始化元数据使用业务保存入口而失败/超时，改用现有文件元数据保存入口后重跑通过；失败未改记为成功。

本轮固定构建 EXE SHA-256：`fee9ef87ab58cf333316bdab0527c91f20ba23066e373e9f7e2ae977bf9f2fc2`，生产输入摘要 `907ead9a883b12cf402b4d3fbcbfa7352c19d13b79e3a3a02eb7fb910398c617`。离线能力输出包含物理文件写入结算和冻结 Orca transport 声明，保存于 `.evidence/worktree-reuse-review-capabilities.json`。

本轮最终构建定向 Controlled：`./scripts/test-controlled.ps1 -Combination execute-stop,runtime-files,result-parser -OutDir .evidence/worktree-reuse-review-controlled`，2026-11-10 10:31 UTC 完成，status=passed、result exit=0，六项顶层测试及全部所选子场景通过，耗时约 484 秒。覆盖四 Run 累计收尾、并发接管与重试、旧 stop 隔离、活动后继重启、progress/result 全部组合、未知停止、四类授权重启及三类 parser 用量。固定构建与生产输入同上；该选测仍为 controlled-selection，`complete_controlled_passed=false`、`qualification_eligible=false`，不声称完整 Controlled 或真实后端资格。

本轮最终源码完整 L1：`./scripts/test.ps1 -Level L1 -OutDir .evidence/worktree-reuse-review-l1`，2026-11-10 10:39 UTC 完成，status=passed、exit=0；777 项 Go 测试、21 项扩展测试、框架检查、原生 driver 编译及零匹配保护通过，12 个 live/helper 测试沿原允许列表跳过。输入开始/结束摘要一致，未混用此前构建通过结果；该结果仅为合并前完整 L1，仍不代表完整 Controlled、L2/L3/L4 或合并后资格。

物理写入缺少结算回执的历史记录明确拒绝复用；本次未新增远端文件写入对账接口，不能以服务重启或文件可读推断远端写者已退出。其余真实 Orca/真实 PR E2E、未覆盖内部模型及合并后资格缺口保持。
