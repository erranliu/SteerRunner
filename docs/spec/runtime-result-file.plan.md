# result-file 最终快照 Plan

日期：2026-10-26。状态：按主动收缩稿完成代码及定向本地/受控验证，等待审阅。依据 [Spec](runtime-result-file.spec.md)。本轮只完成代码及必要本地/受控验证，不执行真实远端验收或部署。

## 1. 复用位置与必要修改

| 位置 | 必要修改 |
| --- | --- |
| contracts/run.go、result_file.go；resources/input_schema.go、result_file.go、assets/result-file.schema.json | 保留可选输入及 64 KiB 严格校验；独立 contract/schema/capability 改为 final-only、version 0/1、disabled/pending/closed |
| CLI/host/service_binding | 保留 result-file 只读查询及原围栏；只返回持久结果 |
| workspace/progress.go；protocol/orca/progress.go；orcatransport/progress.go | 仅复用固定路径初始化、稳定字节读取、长度检查、原主机/runtime/worktree RPC，不引入文件注册集合 |
| engine.setup；codex owner/lifecycle/worker_runtime；agentlaunch；terminaltools | 保留 result 绝对路径贯通及禁用变量清理、包装精确解包；不改原 frozen command |
| application/progress.go、engine/worker | 删除 result observer/timer/采样计数/恢复重开；恢复原 progress 组织，仅补当前聚合保存保护及独立 result 准备 |
| runstate.Record、独立 result 封存逻辑 | 保存初始化事实和 durable sealed 标记；原 Record.Result/ContentResult 不占用；无独立执行账本 |
| application/control.go 及原 worker 生命周期 | 原 stop 确认后、读取资源释放前采集；恢复只凭原持久结束资格补采；正常完成须同时确认生产者停止。pr_ready/turn end/detach 不触发 |
| application/engine.commitLocked、progress.saveProgressLocked、result 保存 | 当前聚合合并，互不覆盖；迟到封存检查绑定/结束资格/unsealed；持久化成功才发布 |
| takeover 候选能力与 storage 严格解码 | 保留最小 result contract 兼容检查，不升级存储格式或恢复框架 |

基线 progress 初始化先保存意图、未知只读对账；300ms observer/最终采样/closed-detached 行为保持原样。SaveProgress 写整份 runs.body，必须从当前聚合合并，不能用旧 clone UPDATE。result 不使用 progressTerminal（包含 pr_ready）。Orca StopStatus 的停止范围沿原 adapter 事实，不能外推额外写入者保证。

## 2. 实施顺序

1. **修订 Spec/Plan**：以本稿取代双通道持续观察要求；定义结束资格、2 秒父期限、封存失败和恢复规则。
2. **删除持续观察代码**：去掉通用双通道 observer 框架及 result 计时器、内容变化计数、普通/最终采样竞争、恢复重开、对应测试；恢复 progress 原组织。保留输入/环境/固定 IO 接线。
3. **接入一次封存**：独立 result 初始化；原停止/完成确认后的有限采集，读完再释放原资源；无资格不读。成功 version=1；失败 version=0 加诊断；两者 durable sealed。查询/resume/restart 不刷新。旧整份记录提交保护三方字段，迟到提交围栏和保存失败保持未封存。
4. **定向验证及交接**：只运行下表集合，更新 caller 文档、catalog 和验收记录；旧持续观察及宽范围运行单独标记被取代/非本轮资格。失败先定位，不扩成全测试矩阵。完成后停止，真实远端和部署不执行。

## 3. 定向验收映射

| ID | 验证 | 层级与断言 |
| --- | --- | --- |
| A01 | 四种启用组合、输入与公开契约 | contracts/resources/CLI + controlled：不同路径/变量，禁用清除继承值；查询零 IO；独立 schema/capability final-only；未知 Run/围栏沿原规则 |
| A02 | 运行中零 result 采集，结束后任意字节/原子替换 | application/workspace/protocol + controlled：初始化对账不发布内容；waiting_input/turn end/pr_ready/detach 都 pending；停止确认后 base64/长度/sha256 等于原文件 |
| A03 | 只封存一次 | application + controlled：成功 version=1、失败 version=0；封存后改/删文件，查询/重启/resume 不读不刷新；空字节与未采集可区分 |
| A04 | 预算内失败及隔离 | application/workspace/protocol：缺失、65537 字节、不稳定、断线、错 host/runtime/worktree/instance；最多 2 秒且受原 stop deadline，失败封存不阻塞 Run 或破坏 progress；不得本机同路径/其它 Run 代取 |
| A05 | 旧记录覆盖保护 | application/storage：旧业务记录、progress 保存不覆盖最终 result，result 迟到提交不覆盖 progress/业务/已封存结果；绑定变更或 resume 使资格失效即丢弃；持久化失败不先发布 |
| A06 | 初始化与原绑定恢复 | application + controlled：一次初始化、未知只读对账、禁用无文件；已封存直接保留；未封存有原结束资格才补采，原资源丢失封存失败，无替代工作区/重复业务 |
| A07 | 原 progress 直接回归 | 原 progress L1 + TestSRProgressFile，保留原 contract/状态/版本/观察/原子替换/保存/停止查询；Windows 受控 producer 仅对共享/访问冲突在 1 秒内重试原子替换，不重试 Runner 采集或放宽投递/生命周期期限；不运行完整 content/resource recovery 长流程 |
| A08 | 一个受控闭环 | TestSRResultFile：公开 EXE+隔离 state/native fixture，启动→写双文件→确认 result pending→明确结束→查询最终字节→服务重启后查询不刷新；记录构建摘要、响应、原绑定和资源结算 |

本地字节/RPC/保存测试与受控 EXE 闭环分别记录。真实远端双文件闭环留待后续授权，不以 fake/native fixture、旧 continuous 测试或历史成绩代替。无新远端执行场景、部署或自动修复。
