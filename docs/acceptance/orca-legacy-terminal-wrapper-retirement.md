# Orca 旧终端 create / send / read Go 包装器退役验收

2026-11-11，合并前定向验收。实施和交付前的 main 均为 `e6cac6e17bf1b8de198c115a06308361b42d2b3c`（#142），实施分支以该提交为基线。[Spec](../spec/orca-legacy-terminal-wrapper-retirement.spec.md) · [Plan](../spec/orca-legacy-terminal-wrapper-retirement.plan.md)

## 独立审计与变更

独立子代理逐项核对六文件范围、两个专属测试、共享 worktree 子例、诊断入口和选测命令，确认删除与保留清单成立。按审计修订补录 stability 辅助关闭调用链及两项现有隔离诊断测试。类型捕获改为记录完整前后集合，只允许七个指定声明退出。

- 删除 `terminal_send.go`、`terminal_read.go` 和两份专属测试。只从 `resource_create.go` 移除 terminal 参数/回执编译与解码，从 `resource_create_test.go` 移除七个 terminal 子例和独占的 `ptrString`。生产净删 169 行，测试净删 275 行。原十个 worktree 子例的名称、输入、断言、顺序和字节保持。
- 保留当前 worktree 创建、回执和原 owner 对账，原 `Terminal`、`StartupTerminal`、`AgentTerminalHandle`、终端清单及关闭。历史 reader 继续只读并在外发前拒绝 mutation，历史停止 scope、结构化 session 和实际派发 checkpoint 的文件保持。
- Pi 仍由 `Invoke-ProbeOrca` 直接执行脚本组装的原 argv。stability 诊断沿 `DiagnosticRead` 与 `ParseEnvelope`，辅助关闭沿 `closeAuxiliary → CloseAuxiliaryTerminal → CloseReadonlyTerminal`；状态查询与订阅、脚本、driver、helper 原字节保持。隔离诊断测试覆盖 raw mutation 零重放和同一辅助终端身份、完整清单及丢回执对账。

原清单 882 个非文档文件（另列证据属性），其中 876 个原字节相同，四个删除，两个按批准块精确修改。当前可执行源码及测试无待删符号引用。[原引用](evidence/orca-legacy-terminal-wrapper-retirement-20261111/before-references.txt) · [最终扫描](evidence/orca-legacy-terminal-wrapper-retirement-20261111/after-references.txt) · [文件与类型差集](evidence/orca-legacy-terminal-wrapper-retirement-20261111/scope-audit.json)

## 实际最小验收

Windows/amd64、PowerShell 7，定向 Go 测试均用 `-count=1`。执行节点包含顶层与子例。无失败、无 skip。

| 范围 | 删除前 | 删除后 |
| --- | --- | --- |
| protocol | 7 项，105 节点 | 5 项，64 节点 |
| adapter | 8 项，57 节点 | 8 项，57 节点 |
| livetest | 2 项，12 节点 | 2 项，12 节点 |
| Pi 静态解析 | | 0 个解析错误 |
| stability helper / native driver | | 均编译通过，driver 含 `live` tag |
| 正式 Windows 构建 | | 退出 0 |

协议原两项专属测试已从源码删除，混合测试只减少七个指定子例；其余执行节点前后相同。adapter 覆盖 worktree 原参数、创建回执及 owner 恢复、未知/错来源不重派、历史 reader 拒绝 mutation 和原停止范围。Pi/stability 静态与编译结果未执行真实探针、网络故障或 Orca。[执行命令](evidence/orca-legacy-terminal-wrapper-retirement-20261111/commands.txt) · [测试名及结果](evidence/orca-legacy-terminal-wrapper-retirement-20261111/test-summary.json)

## 编码、资源与产物身份

捕获的 301 个类型声明中只移除批准的七个，其余 294 个声明、代表 profile/state/record/receipt 编码、schema/capabilities 和资源摘要相同。代表样本与六文件字节核对共同举证持久边界。resource_digest 为 `732a1c7e211563648ea5b4d5c155345419693f4c78e29c724bb3168244714502`。[删除前捕获](evidence/orca-legacy-terminal-wrapper-retirement-20261111/before-capture.json.gz) · [删除后捕获](evidence/orca-legacy-terminal-wrapper-retirement-20261111/after-capture.json.gz)

正式构建 source_commit 为 `e6cac6e17bf1b8de198c115a06308361b42d2b3c`，source_dirty 为 true，source_digest 为 `3f933f02fbfb9a1944e5a6cc50808bdab8470ab96473ab3a9da7ed4ba31eef4b`。工作副本 EXE SHA256 为 `6f5dbe1a022f2b8e06d0a6a5aed8c204265570be14c57df798613dfb711642cc`，与构建记录和最终输入一致。[构建记录](evidence/orca-legacy-terminal-wrapper-retirement-20261111/build.json) · [三项产物摘要](evidence/orca-legacy-terminal-wrapper-retirement-20261111/artifact-hashes.json) · [证据 SHA256](evidence/orca-legacy-terminal-wrapper-retirement-20261111/SHA256SUMS.json)

OTC-01–05 完成。资格限于上述定向测试、静态解析和编译。完整 L1–L4、Controlled、真实 Pi/stability/Orca、模型或 PR 场景未运行。实施尚未提交、推送、创建 PR 或部署。

## 提 PR 前主线补验

提交前刷新至 `2de536cbe5d0cf4b0011568becf0a070bdc8520c`（#143、#144、#146）。上游未改本批六个退役文件，快进后六文件 diff 与初次验收完全相同。以上初次验收和构建身份仍按原基线记录。

在更新后的候选点通过 6 项 protocol、10 项 adapter、2 项 livetest、2 项 application observation 定向测试，无失败或 skip。额外五项覆盖上游修改的 history 完整回执、分页恢复、身份冲突、确认 turn 重连和 stop 撤销。stability helper、带 `live` tag 的 native driver 和正式 Windows 构建重新通过。使用同一捕获程序比较最新 main 与候选，类型差集仍恰为批准的七项，其余 294 个类型、代表编码、schema/capabilities 与该主线的资源摘要保持。Pi 脚本原字节相同，沿用初次静态解析结果。范围仍为合并前定向与编译。[补验汇总](evidence/orca-legacy-terminal-wrapper-retirement-20261111/pr-refresh.json) · [原始输出和捕获](evidence/orca-legacy-terminal-wrapper-retirement-20261111/pr-refresh-raw.zip)
