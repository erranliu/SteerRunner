# Orca 提示词迁移 P0 与首轮 P1 候选验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-04。P1 候选位于 `codex/orca-structured-execute-integration`，基线 `1df8e26d` 含 #97/#98，从 rebase 后的 M1 `dc8090da` 取运行代码和定向回归。修正远端快照提示和两端停止能力声明，保留脚本环境绑定。M2 生产切换、Pi、audited、显式启动覆盖和非 Orca Codex 未纳入候选。

## 固定产物

| 受测构建 | 源码摘要 | EXE SHA256 |
| --- | --- | --- |
| 主线 `1df8e26d` | `3c1c93e842a8c063ca572bff2750bd0a9c85e27287bc334c85cd6d27a85acc74` | `53b15cad12c79041fcfc6ad3b014bda986f39ce366e49298f6ba0406b49b9bde` |
| P1 候选 `1df8e26d-dirty` | `7cc35fa62f027353fd416756e750264d569903d34590ec88434ea6515bec9a47` | `88298b1ca9548f33973b6aa59d98172dee6df809ff537f316b139b9c5eb8b469` |

SteerLine 源码及隔离包基线 `a589f618`。本机及明确配对远端 Orca 均为 1.4.246，状态 ready。所有实测用专用 SteerlineTest 仓库及独立状态；原安装环境和原有业务 Run 未被替换。[核验摘要](../../../../acceptance/evidence/orca-structured-prompt-input-20261104/verified-summary.json)保留输入、结果和原身份摘要。原始证据保存在调用 worktree 的 `.evidence/prompt-input-p0/`。P1 的运行代码在构建后未修改；后续只补设计与验收文档。

## P0 门禁

| 项目 | 结果 |
| --- | --- |
| 远端准入与原行为 | 按 SteerLine 正式单行检测正文、同一远端默认 Codex profile、同一仓库提交 `9d81d9c` 和相同格式的隔离结果信封，两个固定构建的 `validate --kind run` 均通过。主线正式 Runner 提交 `16c5337adb9c1bec4e2a5418ddfe2317` 使用 `orca_terminal_v3`，在有界观察内未核实原最终结果。对原 Run 发 stop 后为 `stopped`，原服务退出且离线结算为 true。该样本证明旧路径能排空，未证明旧 SteerLine 检测可进入 ready。 |
| 远端候选脚本 | 同一输入在 P1 Run `20103cca4ae8ddba3bd42c0809480eb1` 走 `orca_agent_session_v1`。回读原会话唯一用户项及完整回合。原回合报告按原命令调用一次，PowerShell 因执行策略拒绝载入脚本并退出 1。原生回合为 succeeded；快照为版本 1，但字节仍是初始报告，`completed=false`、`ready=false`。保留实际失败，未改脚本、参数或主机策略。 |
| SteerLine 消费端 | 使用临时 Go overlay 调用原实现，不写 SteerLine 源码。三个反例通过，`stop_status=unknown` 时零结果查询，`ValidateResultFile` 拒绝快照合同，`TrustedFinal` 拒绝 `unreported` 并接受同身份的 `succeeded`。 |
| 排空与升级 | 同一远端 profile 在候选 Run 的 writer 已退出，离线 `resources_settled=false`，Run 停止保持 unknown。SteerLine 的 `TestUpgradeDrainedUsesCandidateProofAndBothBusinessReaders` 与 `TestStoppedVerificationKeepsRunDiagnosticsAndGate` 通过，明确拒绝原资源未结算和未知控制。与主线样本的正向排空相比，候选失去已证明的排空行为。 |
| 普通开发输入边 | P1 继续选旧普通开发路由。P2 只可复用现有 M2 的原 session 首轮及业务续轮；普通开发的反馈、报告来源与发布仍由现有 Workflow 处理。原生活动问答按原控制合同检查，不能作为下一轮反馈派发。P2 的真实报告回退及实际配置资格待该阶段验收。 |

**首轮发布阻断。** 两个环境检测合同门禁均存在，原环境检测可用性基线仍未获完整证明；同一配置的排空退化已经实测。历史 M1 的停止与快照授权只用于对应 Runner 合同，未批准取消 SteerLine 的环境检测或升级门禁。该首轮候选未发起 PR 或发布。后续通过的固定构建及正式检测见 [P1 解阻验收](orca-structured-prompt-input-p1-unblock.md)。P2 等待 P1 合入。独立目录只用于隔离验收。

## P1 双端 Runner 专项

使用同一个 P1 EXE，经正式 Runner CLI 和原有专用仓库提交。各端一场普通多行及一场确实处于活动中的显式 stop。正常场景包含 CRLF、空行、Unicode、引号及尾空白，两次工具调用后得到原回合四条完整 assistant 项和 150 行最终内容。对原运行 SQLite 记录及公开 Orca history/item 只读核对，四场均只有一条 user 文本块，逐字节等于冻结输入，无 READY 初始化回合。正常场原回合聚合逐字节等于 Runner 结果。

| 场景 | 原 Run | 结果与停止 |
| --- | --- | --- |
| 本机正常 | `e52e80f47fcd1b9a139c99c07ed3589a` | 原完整结果 2,094 字节。原 result-file 合同封存 `P1_RESULT` 的 9 字节，原会话停止为 stopped。 |
| 本机活动停止 | `d3c2b3845d5018f1b691c0b6d455ce86` | 核实原 turn 在运行及原活动标记后执行一次 stop，停止为 stopped，封存原 4 字节 seed。 |
| 远端正常 | `19e844a72c399a41268a4ac8539bd30c` | 原完整结果 2,093 字节。原快照合同采集 `P1_RESULT` 的 9 字节，停止为 unknown。 |
| 远端活动停止 | `5fa871fe80f62ff1a6339464951b532c` | 核实原 turn 在运行及原活动标记后执行一次 stop，停止为 unknown，保留原取消回合证据和 4 字节 seed 快照。 |

两端同构建重启保留原 session、停止、文件版本和 SHA256，结束后测试 writer 均已退出。离线证明本机为 true，仅限 `orca_agent_session`，远端为 false。上述操作次数基于持久意图和公开会话事实，丢响应零重放另由受控测试证明。检测脚本失败的远端场仍取得两条原响应的 finalized usage，缺失 0。停服后 collection permit inactive，原临时 lease 为 revoked。

### 正式本机环境检测未签收

固定 P1 EXE 的 SteerLine 调试包 `dev-20261104-125247-a589f618` 已生成，原检测脚本和文档复制到专用 base `1390d4c`，测试仓库已注册。调试启动因子进程留在 kill-on-close Job 在 API 就绪前退出。改用同包正式 `steerline start` 仍返回 `Access is denied`，未提交 `environment_start`。本机 CLI 的原会话停止和封存通过不能替代 SteerLine ready。原 #97 的应用验收仍以其原固定构建为准。见 [#97 验收](orca-session-result-settlement.md)。

### 受控回归

P1 候选的 application、contracts、Orca adapter/protocol、workflow、resources、storage、usage/collect 定向测试通过。覆盖 #97 的成功/丢失原 close 回执和离线边界、#98 的 `unreported` 与明确 `unknown` 区分、远端原身份及快照窗口、模型选择与输入、以及公开 catalog 的停止能力范围。两端真机专项与上述测试均为合入前定向资格，没有取得完整 L1/L3、生产部署或远端环境检测资格。
