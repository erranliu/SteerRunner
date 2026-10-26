# result-file 最终快照 Spec

日期：2026-10-26。状态：按用户主动收缩修订，代码及定向本地/受控验证完成，等待审阅。配套 [Plan](runtime-result-file.plan.md)。基线 `5df57b69e98382abcd63676b04a73079fd939fa6`。本稿取代此前 result 持续观察和独立递增版本要求；原 progress-file 契约保持不变。

## 1. 范围

result 是 **同一个 Run 结束时保存的一份不透明文件快照**。它不持续观察，不与 progress 共用观察生命周期。Runner 不解释 JSON、脚本、测试、PR 或环境就绪，不根据文件停止、重试或推进 Run。采集成功只证明取得字节；文件采集、Run 完成、原生资源停止分别表达。

支持资格沿用 progress 当前实际组合：有原工作区写资格的 codex-app-server / Orca local / orca_environment 开发流程，development/2、development-audited/2。枚举不是任意笛卡尔积。不扩展 conversation/generate、权限或 Agent 矩阵。

## 2. 公开契约

可选 submit：`{"result_file":{"initial_content_base64":"..."}}`。严格规范 base64，解码最多 65536 字节；允许空内容及任意字节。拒绝 null、缺失初始字段、未知字段、caller 路径及超限。

固定文件 `.steerrunner/result/<run_id>/result.bin`，注入绝对路径 `STEERLINE_RESULT_FILE`；progress 仍用原路径和变量。四种启用组合均支持；禁用时清除继承变量。路径仅取原绑定主机/runtime/worktree，不从本机同路径代取，不扫描目录。支持同目录临时文件加原子替换。

查询 `result-file --run-id R --json`，沿原信封、Run selector、实例围栏和 run_not_found 错误，不接受输入。查询仅返回持久记录，不初始化、不读取文件、不启动 Agent 或脚本。

| 字段 | 含义 |
| --- | --- |
| contract | 独立 `steerrunner-result-file/v1` |
| run_id / encoding | 原 Run ID / 固定 base64 |
| version | 未成功采集为 0，成功封存为 1；永不递增至 2 |
| observed_at | 成功封存字节的 UTC RFC3339 观察时间 |
| content_base64 / size_bytes / sha256 | 完整原字节、长度 0–65536、SHA-256 小写十六进制 |
| collection_status | disabled：未启用；pending：尚未封存；closed：已封存成功或失败 |
| diagnostic | 可选原有文件诊断；closed 且 version=0 时必有失败诊断 |

version=0 时 size_bytes=0，省略 observed_at/content_base64/sha256；真实空快照 version=1、content_base64=""。封存失败也为 closed，以 diagnostic 区分；不增加业务状态。pending 可携带初始化诊断，但不暴露初始化读到的内容。诊断沿用 binding_invalid、workspace_unavailable、directory_unavailable、file_missing、read_failed、size_exceeded、file_changing、initialization_unverified、transport_unavailable。

独立 schema 与 capability 明示“仅结束时采集”、上述三种 collection_status、0/1 version、查询零读取、封存后不刷新及支持边界。原 progress schema/capability/格式不变。

## 3. 初始化与保存隔离

沿原准备 gate，在首次业务输入前持久化原绑定及初始化尝试意图，再独占创建文件；已存在不覆盖。初始化一次；重启、重连、请求重放不覆盖。未知结果只读原文件按初始字节对账，不重复初始化，不创建替代工作区。初始化验证读取属于准备阶段，不是运行中结果采集，不发布快照。停止已受理、执行已结束或停服后不补初始化。两通道准备都尝试；一方异常不清空另一方状态，业务派发仍遵守原准备资格。

原 Run record 增独立 ResultFile，不能占用业务 Result/ContentResult；保存初始化事实及封存结果，不增加收集任务、worker、队列、表或第二套执行账本。progress 保持原 observer 和组织，仅共享固定文件初始化、字节/长度检查及原 RPC 等小辅助函数。

文件保存不增加 Run revision 或业务事件。所有保存从当前 Run 合并：业务提交保留当前 Progress/ResultFile；progress 保存保留当前 ResultFile 和业务事实；result 保存保留当前 Progress 和业务事实。迟到 result 提交须重验原绑定、结束资格和尚未封存；旧初始化/失败回调也不能覆盖封存结果。持久化成功后才发布内存状态。

## 4. 结束资格、预算与恢复

采集顺序：**原生命周期确认本次执行结束且生产者停止写入 → result 一次有界采集/封存 → 原读取资源释放**。正常完成和明确停止均适用；waiting_input、单个 turn 结束、pr_ready、服务断线/观察退出本身均不触发。现有开发流程的 pr_ready 尚不满足此条件，须等原停止流程确认。不能把 progressTerminal 直接复用于 result。

接到原停止确认之后、观察/资源清理之前；正常完成使用原完成及停止事实，不根据内容或推测进程消失。原服务退出只有确实执行原停止并取得停止事实时才可采集，继续原执行的 detach 不采集。停止结果未知时保持 pending，留给原生命周期对账。

同一次采集总预算最多 2 秒，且不得超过已有停止/收尾父期限；取消、原绑定核验、读取、有限稳定性检查全计入。允许一次采集内两次完整读取核验稳定；不轮询等内容，不另给重试预算。文件缺失、超限、不稳定、断线、错绑定或读取失败即封存失败；初始化未确认则封存 initialization_unverified，不为收尾创建文件。失败不阻止 Run 收尾，不修复、不自动重试。

封存是持久单向状态，成功或失败后查询、重启和 resume 都不刷新；同一 Run 首次封存后永不重新采集，需要新结果须新 Run。仅承诺最多发布一份最终快照，不承诺跨崩溃严格一次物理读取。持久化失败不得冒充已封存；同一服务不重做已开始的采集，后续服务可按下面条件对账。

服务恢复：已封存直接保留；未封存只有持久原结束事实仍有效、尚无 resume 使资格失效且原绑定可核实时，才补做同样有界采集。原读取资源已丢失则封存失败，不恢复/新建资源或工作区。没有结束资格则保持 pending。原服务 writer 排他及候选 contract 兼容预检继续适用；旧 writer 不得忽略新字段写回。

## 5. 验收及非目标

本轮只执行 Plan 的定向本地和受控验证；旧持续观察版本证据作废，不计为本稿通过。真实远端验收、部署仍待后续。

不做远端无模型命令执行、artifacts 扩展、任意路径/目录发现/多文件平台、新 workflow、业务结果解析、自动修复、全 Agent 矩阵、完整内容恢复/资源创建长流程默认回归。删除 result 连续版本、交错采样和观察重开验收项。
