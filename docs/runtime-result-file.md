# result-file 最终文件快照

result 是 **同一 Run 结束时保存的一份文件快照**。运行中不采集；progress 保持原持续观察行为。文件是不透明字节，取得快照不证明业务成功或进程退出。

submit 可选增量，两字段可分别启用：

```json
{
  "progress_file": {"initial_content_base64": "cHJvZ3Jlc3M="},
  "result_file": {"initial_content_base64": "cmVzdWx0"}
}
```

初始内容使用规范 base64，最多 65536 字节；结果也最多 65536 字节，允许空内容、非 UTF-8 和非 JSON。工具使用绝对路径 `STEERLINE_RESULT_FILE`，固定在原工作区 `.steerrunner/result/<run_id>/result.bin`；caller 不能指定路径。禁用变量会清除继承值。可用同目录临时文件加原子替换。

```powershell
steerrunner result-file --state-dir C:\owned-state --run-id R --json
```

查询沿原实例/绑定围栏，只返回已保存记录，不读取文件、不启动 Agent 或脚本。独立 contract 为 `steerrunner-result-file/v1`；capabilities 提供 `result_file`、`result_file_schema` 及 run_input 输入 schema，明确仅结束时采集。

| 状态 | 版本和内容 |
| --- | --- |
| disabled | 未启用，version=0 |
| pending | 尚未封存，version=0，无内容；可携带初始化诊断 |
| closed | 已封存成功：version=1，完整 content_base64/size_bytes/sha256/observed_at；已封存失败：version=0、diagnostic，无内容 |

version=0 的 size_bytes=0 不表示读到了空文件。真实空快照 version=1，content_base64=""。

在原生命周期确认整次执行结束、生产者停止写入后，且原读取资源释放前采集。正常完成和明确停止适用；waiting_input、单个 turn 结束、pr_ready 或服务断线本身不触发。开发流程 pr_ready 后仍需原停止确认。一次采集最多 2 秒，并受原停止期限约束，允许有限稳定性检查，不轮询等内容。缺失、超限、不稳定、断线或读取失败即封存诊断，不阻止 Run 收尾，不自动重试。

首次成功或失败封存均永久保留；查询、重启、resume 都不刷新，需要另一份结果须新 Run。仅承诺最多发布一份快照，不保证跨崩溃只物理读取一次。保留 state-dir 后可继续查询。

首次业务输入前，result 和 progress 各有最多 4 秒的尽力准备窗口；首次写入前记录尝试，记录失败不发文件写 RPC。初始化返回成功后直接结束准备，文件内容由消费者在使用时读取。每通道初始化最多一次，失败或超时均不阻止业务、不重复准备；未知只读原文件对账，不覆盖已有文件；恢复只从原绑定主机/runtime/worktree 读取，不从本机同路径代取。初始化未确认也可在结束资格成立后接受原绑定的实际稳定字节，无需等于种子，不创建缺失文件或目录。已封存直接保留；未封存只有原结束资格和绑定仍有效时才补采，资源丢失登记失败，不创建替代工作区。

result 保存与 progress、业务记录互不覆盖，文件保存不增加 Run revision。支持范围沿 progress 现有可写开发组合，不扩展 conversation/generate 或 Agent 矩阵。详见 [Spec](spec/runtime-result-file.spec.md)、[Plan](archive/2026-11/implementation/spec/runtime-result-file.plan.md) 和[验收记录](archive/2026-11/implementation/acceptance/runtime-result-file.md)。2026-11-01 的 P2 实施及各路径结果见[独立验收](archive/2026-11/implementation/acceptance/personal-executor-p2.md)；未部署。

## execute/1 调用

execute/1 复用同一可选 result_file、STEERLINE_RESULT_FILE、原停止与最终封存契约，支持 codex-app-server / orca-terminal 的既有 local / orca_environment 组合。输入不接收 publication 或 progress_file；结果对 Runner 仍是不透明字节，不因结果包含 ready 等字段获得特殊判定。调用方须同时核对原 Run completed、可信原生 final、stop_status=stopped 与 closed 最终快照。该通用工作流的单次输入及控制限制见[工作流说明](workflows/README.md#通用单次执行-execute1)。


取消恢复时，公开 `inspect --offline` 仅返回原持久 Run/命令快照，不采集 result-file，也不证明停止。调用方不得因冷查询读到 ready 或 Closed 就跳过原停止和最终封存；同构建启动继续原结算。详见[启动前读取约定](workflows/README.md#启动前读取原执行)。
