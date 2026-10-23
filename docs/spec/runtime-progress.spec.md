# 运行时单文件汇报通道 Spec

2026-10-23；实现与注册已完成，实际构建资格见[专属验收](../acceptance/runtime-progress.md)。本次只修改 Runner，Orca 本地及远端均为必需范围。调用方拥有内容与解释，执行工具拥有写入；Runner 只传原始字节。无 Plan、测试语义或 SteerLine 接线。

- 可选 submit `progress_file: {initial_content_base64: "..."}`，严格 base64，最大 65536 字节。未配置无文件、无观察 goroutine。支持具有工作区写资格的 codex-app-server 本机开发流程与 Orca local / orca_environment 开发流程；read_only 内容流程不扩大权限。
- Runner 在原实际工作区的 `.steerrunner/progress/<run_id>/progress.bin` 分配固定文件。执行前准备目录及 Git info/exclude，O_EXCL 初始化一次，持久化主机/workspace/资源绑定，本机路径额外保存目录身份。重启、请求重放和重连不覆写；初始化崩溃窗口已有文件只读接续，不清空。
- 执行环境注入 `STEERLINE_PROGRESS_FILE` 绝对路径；提示说明按字节写入、可使用同目录临时文件原子替换。各阶段沿原绑定。不得增加其它写根。
- `progress-file --run-id R --json` 返回 contract、run_id、独立 version、observed_at、base64、size_bytes、sha256、collection_status、有限 diagnostic。版本只在完整内容变化时增长，独立于业务 revision；保存最后快照供重连和终态查询。未获得快照时内容字段不出现。
- 独立每 Run 观察；轮询重新打开固定路径，不以 mtime 去重，不持久绑定文件 inode。两次相同完整读取后接受，可合并中间变化；最终稳定内容在可读前提下收敛。不承诺每次写入必达。
- 按用户 2026-10-23 收敛：固定原 Run 的执行主机、工作区及相对文件；不接受任意路径，不扫描其它文件。核验原 runtime、worktree、资源及配对主机指纹；不承诺抵御读取瞬间的恶意链接/目录替换。文件正常原子替换受支持。缺失、超限、读取失败保持最后快照，报告有限异常，不更改业务状态/结果/输入授权。
- 正常收尾点为 development 的 pr_ready、completed，或明确 stopped/closed，有界最终稳定读取后停止本次观察；观察收尾不等于 native 资源退出。waiting_input、结果等待或核验未完成仍持续观察。服务被动退出同样最终采样但只标 detached，不停止原业务执行；新服务沿原绑定继续。
- Orca local / remote 均经原绑定文件 RPC 初始化与读取，不使用本机同名路径。files.writeBase64 独占创建；同一原终端启动前置设置环境变量及 Git ignore，既有工具白名单保留该变量。不部署远端常驻 Runner/helper；采集不要求 native final 或终端存活。首次初始化尝试前持久化，结果未知后只读对账，不重发初始化；若服务恰在意图保存与首次写入之间退出，原文件缺失报告采集异常，不自动重建。
- 不为本增量迁移/双写旧开发记录，原 R2 artifacts 接口保留。字节保存在原 Run record，专用 UPDATE 不增加 revision；业务提交总是保留当前独立快照，避免旧业务 clone 覆盖。

L3：`local-codex.progress-file`（实际注入/任意字节与替换/去重）、`orca-remote.progress-file`（原远端主机读写）与 `orca-local.progress-file`（本机 Orca 接线）、`local-codex.progress-multirun`（同服务隔离/独立 revision/停止一方）、`local-codex.progress-recovery`（原生结果等待、读取故障/恢复、服务换代零重发、收尾保留）。场景之间独立服务/state/worktree/native owner，可并行；快速反例放 L1/Controlled。真实及并行资格按当前执行记录，不以注册或受控通过代替。

Orca 执行准备沿服务恢复契约：工作区创建前保存原初始终端准备计划，首次尚未尝试可继续准备，创建结果未知只读原 owner 回执。辅助控制终端关闭后可能保留 disconnected 展示卡片；卡片不单独证明停止，完整原 runtime/worktree 清单核实同一身份缺席后才完成对账，不重复关闭或输入。当前 r12 同构建五项真实 L3 与实际并行资格全部通过，证据见专属验收。
