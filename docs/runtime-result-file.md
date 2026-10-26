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

初始化一次，未知只读原文件对账，不覆盖已有文件；恢复只从原绑定主机/runtime/worktree 读取，不从本机同路径代取。已封存直接保留；未封存只有原结束资格和绑定仍有效时才补采，资源丢失登记失败，不创建替代工作区。

result 保存与 progress、业务记录互不覆盖，文件保存不增加 Run revision。支持范围沿 progress 现有可写开发组合，不扩展 conversation/generate 或 Agent 矩阵。详见 [Spec](spec/runtime-result-file.spec.md)、[Plan](spec/runtime-result-file.plan.md) 和[验收记录](acceptance/runtime-result-file.md)。真实远端验收、部署尚未执行。
