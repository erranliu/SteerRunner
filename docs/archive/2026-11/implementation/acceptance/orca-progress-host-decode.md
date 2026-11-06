# Orca ProgressHost 单次解码验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-06，合并前专项。实际基线 `6dd4360a42314e65b427627508ffd2ee3e4287da`（#125），隔离分支 `codex/orca-progress-host-decode`。依据 [Spec](../spec/orca-progress-host-decode.spec.md) 和 [Plan](../spec/orca-progress-host-decode.plan.md)。调查基线 #123 后的 ProgressHost/两份 decoder 至实施时未变化。未修改 Orca、非 Orca Codex、SteerLine、真实配对或安装现场；未合入或部署。

## 实现、等价及审计（PH01–PH03）

生产只改 `progress.go` 中 ProgressHost（+10/-7）：结构化分支直接一次调用原完整 decodeAgentSessionProfile，成功后返回本机 local 或原 PairingHostID；其他协议保持原 decodeLifecycleProfile、local 分支及 UsageHostID。原完整 decoder、所有内部严格校验、fileSource 独立核验、OpenWorkspaceFiles/运行调用方及公开签名未改。不新增类型、抽象、缓存、I/O 或 ProcessHost 前置要求。

复用 #123 的构造字段和首错矩阵，分别对八个合法组合核对 ProgressHost 返回值、无 ProcessHost、缺失当前配对存储仍可解析、输入不变、源配置后续污染及下次调用重新校验。非法封套/codec/JSON/归属/host/target/agent/审计与退役字段仍比较原错误类型、诊断及有分类时的分类。新增六项旧路由用例核对本机忽略坏配对、旧远端读取当前公钥、缺失/损坏配对错误，以及 profile/protocol 错误优先于配对错误；另核对本机 read_only 配置不能绕过完整校验返回 local。

相同测试文件用 overlay 仅将生产 progress.go 替换为固定主线原文，取得对照结果，不改预期或 decoder。输入比较使用独立 Clone 与有效结构比较，没有忽略 setup 函数字段的 JSON 编码错误。新增测试的协议类型拼写在固定候选前修正，未为夹具放宽生产校验。

| 执行 | 顶层通过 | 叶例通过 | 失败 | 跳过 | 包耗时 |
| --- | --- | --- | --- | --- | --- |
| 原主线 orcatransport 全包 | 76 | 284 | 0 | 0 | 57.455 秒 |
| 原主线 protocol/orca 全包 | 62 | 321 | 0 | 4 | 23.867 秒 |
| 同夹具主线 host/构造等价对照 | 4 | 28 | 0 | 0 | 1.850 秒 |
| 候选 orcatransport 全包 | 78 | 291 | 0 | 0 | 55.619 秒 |
| 候选 protocol/orca 全包 | 62 | 321 | 0 | 4 | 24.171 秒 |
| 候选 application 定向集合 | 24 | 42 | 0 | 0 | 38.599 秒 |

命令按 Plan 原入口：两个适配包 `go test ... -count=1 -timeout=6m -json`；应用 `go test ./internal/application -run '^(TestProgress|TestResultFile|TestSingleExecution)' -count=1 -timeout=10m -json`。后者不是全 application 或完整 Controlled。协议包四项跳过为 TestOrcaLocalPipeNativeEvidence、TestOrcaLocalIPCNativeProbe、TestOrcaTerminalQueryLive、TestUsageLocalAcceptanceRepoCleanupLive，未提供其原 opt-in 前置，不视为通过。包级预算不改变产品时间合同。计数、测试清单与原日志摘要见 [tests.json](../../../../acceptance/evidence/orca-progress-host-decode-20261106/tests.json)，完整 JSONL 留在本次 worktree `.evidence/progress-host/`。

diff/调用图确认只移除配对远端第二次完整解码；本机仍一次，旧 terminal 分支未变。原 decoder 不变，错误顺序未重排；nil ProcessHost 不影响文件 host 解析。#121 冻结只读/旧执行拒绝、#113 不同 endpoint/公钥/token 且同 runtime 的配对替换、一次解析和同配置拨号的既有断言通过，替代端点零连接/文件请求。没有把“文件连接解析一次”误扩大成整个旧 terminal 调用链仅一次配对读取。审计未发现待修复运行缺陷。

## 同一候选双端文件验收（PH04）

两个 EXE 的 `source_digest=9bca4d6c84ddb7542497b7ffbe6ec15f55e1da55d16e969e525ffd92fc075a46`，source_commit 为实际基线、dirty=true，包含最终测试修改；真机及排空后重算一致。[构建记录](../../../../acceptance/evidence/orca-progress-host-decode-20261106/builds.json)：writer（0.2.0-dev.4）SHA256=`38c0ce95c72c8469e25904da0902b0477c23c9ec4d762328ca1a4aeede5df764`；reader（0.2.0-progress-host-reader）SHA256=`0c296c2076236e927284dfc4d966e49815a7b676f9776252300346705951c6bf`。

| 目标 | 原 Run | 原 turn |
| --- | --- | --- |
| 本机默认 Codex development/2 | `dfc2856f89a67a8453f93e858604331c` | `01a1afca-74a6-79c3-ac63-03c56835f1dc` |
| 配对远端默认 Codex development/2 | `4814c858bb4bd4862bd5e3ff8306e4fa` | `01a1afcb-caf1-7573-98db-84829775588e` |

同一 writer、新目录、新请求，经正式 service start/validate/submit/inspect/progress-file/result-file/stop/service stop/verify-stopped。复用 #113 文件验收流程，仅换证据根、名称、旧 seed 的固定位置并增加纯 validate，不修改共享历史脚本。原多行输入各受理一次，operation_outcome_unknown 只继续原只读核实，未补发。原回合写入 FILE_PROGRESS_LOCAL/REMOTE 和 FILE_RESULT_LOCAL/REMOTE，均不同于初始化种子；同一工具保持 45 秒活动窗口，没有提交、发布或真实 PR。

每端进度查询前后均确认原 Run 运行、原 turn 不变，progress-file 返回实际新字节，result-file 为 pending。可信最终结果进入 waiting_input 后，按原 revision 受理一次 stop，确认原会话关闭且撤销恢复/新输入准入，随后只读等待 result-file/v1 closed/version=1；未延长生产封存窗口或重采集。重复结果查询字节、观测时间、版本和摘要相同。原持久记录的输入/回合、文件 host/工作区/instance 与原会话交叉核对通过；本机 host=local，远端恰为原 profile PairingHostID。

| 目标 | 实际进度 SHA256 | 实际结果 SHA256 |
| --- | --- | --- |
| 本机 | `0b6cd255ace38fb1004f9aa4663ea2cb82fe46ecbda85fdc89fe6cc031604d42` | `eb147986f4752c36f68a6ec6bcc2f74516ded15516883f22a5e4c3694671285c` |
| 远端 | `672f0799c8f1affd53800e3a28a63817cd52169e8141df751f8f671eb8b3adcd` | `f22f45d2c8bc7af18bb8ae9deacac7b1e8d8ed4ada9d2a77ae98a9b96c9bbe28` |

实际 [Run/文件/停止记录](../../../../acceptance/evidence/orca-progress-host-decode-20261106/runs.jsonl)及[状态日志](../../../../acceptance/evidence/orca-progress-host-decode-20261106/formal-live.log)保留原身份，完整原 CLI 与数据库在 live 两个叶目录。控制台日志以 UTF-8 内容归档，换行统一为 Git 的 LF；原始及归档各自字节摘要和编码见 [harness.json](../../../../acceptance/evidence/orca-progress-host-decode-20261106/harness.json)；其中“未完成”的控制台显示有编码乱码，结果判定以原 JSON 回执逐字比较为准，两端均通过，未重写回执或把日志当正文证明。可复核[文件持久绑定](../../../../acceptance/evidence/orca-progress-host-decode-20261106/verify-files.py)、[原查询先后顺序](../../../../acceptance/evidence/orca-progress-host-decode-20261106/audit-raw-files.py)与[实际专项入口](../../../../acceptance/evidence/orca-progress-host-decode-20261106/acceptance.py)，其复用的[原入口副本](../../../../acceptance/evidence/orca-progress-host-decode-20261106/base_acceptance.py)只改 seed 的定位。未归档数据库、认证或配对正文。

## 原目录排空及资格（PH05–PH06）

SteerLine 干净 checkout `d3a76ef8c35f092e969dd5a8bea4c950f1c25cff`，沿原生产 VerifyUpgradeDrained，使用 test overlay 加入[专项调用](../../../../acceptance/evidence/orca-progress-host-decode-20261106/upgrade_test.go.txt)，未改其源码或门禁。设置 `SR_WORKSPACE_FILE_EVIDENCE` 为本轮 live 根，在该 checkout 运行 `go test -overlay <本轮 live/upgrade-overlay.json> ./internal/runnerclient -run '^TestWorkspaceFileOriginalDirectories$' -count=1 -v`。两端实际原目录双 EXE 冷读通过（案例 1.43 秒），[原日志](../../../../acceptance/evidence/orca-progress-host-decode-20261106/upgrade-gate.log)。

原服务已退出，原同一个 Run/input 的业务、资源及未决控制通过现行门禁；两端数据库及 shutdown.json 冷读前后 SHA256 相同，实际值见 runs.jsonl。停止范围仍为 orca_agent_session；额外进程清理仍未核实，没有把服务退出当成资源结清证明，也没有启动旧 terminal/helper。未安装远端 Pi。

本轮 PH01–PH06 无遗留阻断；只证明受影响包、应用定向集合、双端默认 development/2 文件专项及原目录排空。Pi/audited 仅构造/组件覆盖，完整 L1/Controlled、完整 L2/L3、Pi/audited 真机及合并后测试未执行。后续 PR 交付前仍须刷新主线与实际合并状态，按内容影响复验；交付前复审通过，进入已授权的 PR 交付，实际 PR 状态以 GitHub 为准。

## 交付前复审

再次刷新 main 仍为上述实际基线。逐行核对 ProgressHost 原/新调用：旧结构化解码器转调与当前直接调用为同一完整 decoder，错误处理均先于 environment 判断；旧 terminal 后续解码及 UsageHostID 保持原样。文件 setup 不调用 Lifecycle/frozen 构造，不新增 ProcessHost 要求；实际文件打开仍走 #113 的单次配对解析、公钥比较及同配置拨号。本机 local、远端原 PairingHostID、严格校验/首错及 #121/#113 边界保持等价，未发现运行或测试缺陷。只澄清 Spec 的调查基线和交付状态，未改变已测生产或测试文件。

直接复算基线/候选/应用原 JSONL 摘要和计数，确认 overlay 的 progress.go 等于固定 main 原文，实际执行的验收脚本与归档副本一致。重新核对两份 EXE 及 source_digest、原输入摘要、唯一原回合、原 close、两条文件通道的冻结 host/workspace/instance，并从原 CLI 顺序验证活动进度 → pending 结果 → 显式 stop → closed 快照。控制台乱码不参与正文判定，原 JSON 的“未完成”两端均逐字相符。

SteerLine 此时为干净 `01356834e4c791aba9c94064d5dfcb75f0d6b487`，相对首次验收提交，其整个 internal/runnerclient 无差异。用相同 EXE/overlay 对两端原目录再调用生产 VerifyUpgradeDrained，均通过（案例 1.25 秒），[复审日志](../../../../acceptance/evidence/orca-progress-host-decode-20261106/upgrade-audit.log)。新的 proof 与已归档 proof 完全相同，DB/shutdown 前后摘要不变。审核事实及保留门禁测试清单见 [audit.json](../../../../acceptance/evidence/orca-progress-host-decode-20261106/audit.json)。本次复审未重投模型输入或把专项结果扩大为完整发布资格。

交付时将三份文本日志的归档换行固定为 LF，并分别记录原始与归档 SHA256，避免 Git 换行规范化后把原始摘要误用为仓库文本摘要；原执行回执、原始日志及已验收 cmd/internal 源码均未修改。
