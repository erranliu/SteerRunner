# 运行时单文件汇报通道

Runner 保存原始字节；结构、合并、业务含义与展示归调用方及执行入口。支持本机 codex-app-server、Orca local / orca_environment 开发流程。read_only 内容流程不扩大写权限，本次不启用此通道；R2 artifacts 保留。

先查询 `capabilities --offline --json` 的 `progress_file`；`configuration_schemas.run_input` 发布提交字段，`progress_file_schema` 发布结果。实例围栏同样适用于新命令。

现有 Run 输入可加入（例为原始 hello 字节）：

```json
{"progress_file":{"initial_content_base64":"aGVsbG8="}}
```

省略时没有文件/采集任务，也不把父环境中其它 Run 的同名路径传给执行工具。规范 base64，解码后最大 65536 字节，允许空内容与非 UTF-8 / 非 JSON 字节。不接受调用方路径；固定在原实际工作区 `.steerrunner/progress/<run_id>/progress.bin`。

第一次业务输入前初始化。环境变量 `STEERLINE_PROGRESS_FILE` 为绝对路径，执行提示说明用途。Orca 原启动命令在实际执行主机按 cwd 构造路径，工具白名单保留变量。脚本可通过同目录锁文件协调写者，读入旧内容进行业务合并，再用同目录临时文件原子替换目标。Runner 不合并内容。Git info/exclude 排除整个 progress 子树（含锁/临时文件），不改受跟踪文件，不增加其它目录写权限。

```powershell
steerrunner progress-file --state-dir C:\owned-state --run-id R --json
# 可加原有 --expected-instance-id I 或 --expected-binding 围栏。
```

成功信封的 data 示例（身份/时间为示意）：

```json
{
 "contract":"steerrunner-progress-file/v1",
 "run_id":"R",
 "version":2,
 "observed_at":"2026-10-23T05:00:00Z",
 "encoding":"base64",
 "content_base64":"aGVsbG8=",
 "size_bytes":5,
 "sha256":"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
 "collection_status":"observing"
}
```

version 独立、持久化，只在内容变化时增加；不改 Run revision、不发业务事件、不唤醒业务 worker。observed_at 是已保存内容的观察时间，查询不刷新。重连直接查询最新快照；中间写入可合并，不承诺每次必达。

状态：disabled 未配置；pending/initializing 尚未完成绑定/初始化；observing 正常；unavailable 本次异常；closed 正常完成/明确停止后的收尾；detached 本次服务观察退出。异常保留最后内容/版本并返回独立有限 diagnostic，不伪造空内容。尚无成功快照时 version=0，省略 content_base64/sha256/observed_at；size_bytes=0 不表示收到空快照。

约 300ms 轮询。Orca 每次打开原绑定 IPC/配对通道，核验 runtime/worktree/路径及配对公钥指纹，限量读取固定相对文件；不扫描、不本机回退、不读终端画面、不用模型结果替代。重复读取检查内容稳定；不依赖旧文件 inode 或 mtime。采集不要求 native final 或终端存活。

每轮采集预算 2 秒，初始化 4 秒；终态/stop/被动停服最后采样共用 2 秒。被动停服只释放本次观察，保留原业务授权/执行。重启沿原文件、绑定和版本，零业务重发。首次写入前保存初始化尝试；若恰在保存与实际写入之间崩溃，原文件缺失只报告异常，不自动重建或重复初始化。保留 state-dir 即可查询终态快照，源文件之后删除不删除已保存内容。

本通道的正常收尾点为 development 的 `pr_ready`、`completed`、显式 `closed` 或已核实 `stopped`；这是观察收尾定义，不证明 native 资源退出。`waiting_input`、结果核验中和未知结果等业务等待仍持续观察。初始化结果未知时，只核对原文件是否仍等于初始字节；缺失或不完整均不重新初始化、不发业务输入，诊断保留为 unavailable。

终态已保存 `closed` 后，服务重启不会重新读取源文件或改写该快照；终态尚未收尾时仍完成有界采样。服务退出与终态同时发生时保存 `closed`，仅活动 Run 保存 `detached`。显式恢复活动的 Run 可以沿原绑定继续观察。

用户收敛后的边界：固定原 Run 的主机、工作区及相对文件，不承诺抵御采集瞬间的恶意链接/目录替换；不要求新增 Orca 安全接口，不部署远端常驻 Runner/helper。

详见 [Spec](spec/runtime-progress.spec.md)、[Plan](spec/runtime-progress.plan.md)、[验收](acceptance/runtime-progress.md)。
