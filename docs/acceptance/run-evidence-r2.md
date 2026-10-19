# R2 验收记录

日期：2026-10-19。Spec/Plan 见 ../spec/run-evidence-r2.spec.md、../spec/run-evidence-r2.plan.md。

已实现。公开契约及最小 CLI 请求/响应见 [接入说明](../run-evidence-r2.md)，
内嵌资源为 `run-evidence.capabilities.json`、`run-evidence.schema.json`；随资源摘要发布。
实现不读取测试 ID/Plan 测试安排/级别/通过条件，不改 SteerLine。

| 验证项 | 结果与证据 |
| --- | --- |
| 同 Run 收集/重复读身份、摘要、原字节一致 | 真实 EXE/CLI/IPC/SQLite 受控场景通过；含中文 JSON 和含 NUL/非 UTF-8 的原始日志；[公开响应](evidence/run-evidence-r2-20261019/public-evidence.json) |
| 新记录不覆盖旧报告、跨 Run 不串内容 | 同路径更新后新 collect 分配新 ID，旧字节保持；另一个 Run list 空、读取原 Run ID 失败 |
| 缺文件/越界/超限/来源不明 | CLI 缺文件、..、绝对路径、ADS、1 MiB+1 拒绝；workspace junction 逃逸和 application 不明来源反例通过 |
| Git clean/修改/未跟踪/新 commit/缺工作区 | CLI 实际 Git 通过；查询 commit 与工作区 rev-parse 对照；已清理 verified=false、commit/clean=null |
| 观察中发生变化 | workspace 使用原 Git 进程启动边界在第二次采样前分别制造新未跟踪文件和 commit，均拒绝核实干净；重命名的原文件名伪装 branch.oid 时也不能替换真实 commit，见 workspace.log |
| 查询零 Agent/恢复/重跑 | CLI 证据操作前后完整 native calls.jsonl 字节一致；application 查询未修改 Record、未 wake worker，port=nil；五个新命令均通过原 instance 围栏 |
| stop/清理后保留证据 | 原 Run stop 实际达到 stopped，再从所属仓库移除其原工作区；read 原身份/摘要/内容不变，discover 明确不可用 |
| 持久性/完整性 | 原 Store.Runs 重读字节和元数据一致；旧请求在源删除后仍重放原回执；破坏已存字节时明确 artifact_integrity_failed |
| Orca local 归属 | application 使用原注册 checkout 的 Git common，对照实际 Workspace/Ref/Fact，允许不同的计划路径；受控单元通过，未声明 Orca 真机验收 |

最小验证命令：

```powershell
go test ./internal/contracts ./internal/resources ./internal/cli ./internal/storage ./internal/execution/workspace ./internal/execution/workflow -count=1
go test ./internal/application -run '^TestR2' -count=1 -v
go test ./internal/host -run '^TestSRExpectedInstanceFencesReplacement$' -count=1 -v
go test ./tests/integration -run '^TestSRRunEvidenceR2$' -count=1 -v -evidence-dir <独立证据目录>
```

owner 六包通过，application/host 定向通过，见本目录证据中的 owners.log、application.log、
instance.log。受控端到端修复夹具后通过，耗时 99.49 秒，见 integration.log。
首次迭代有两项测试问题：直接比较持久化 JSON map 与首次 DTO，已改为公开 JSON 值比较；
Windows 从待删除工作区内部执行删除，已改为从所属仓库执行。原失败记录保留在本地
`.evidence/r2-first`，没有将失败批次记为通过。

最终交付从干净实现 commit 构建 `bin/steerrunner.exe`，版本 `0.2.0-dev.3-r2`。
具体 source_commit、EXE SHA256、resource_digest 和最终同一 EXE 定向结果写在本地
`.evidence/r2-release/manifest.json`、`version.json`、`integration/result.json`；
最终响应引用这些实际值，避免把基线或 dirty 来源冒充最终构建。

未验：真实 Codex/Orca Agent、真实测试项目入口、真 PR、全量 L1–L4、macOS/Linux。
Orca environment 本期明确 unsupported，不具备远端文件收集/Git clean 资格。
无锁 Git 双样本不能排除样本间变更后完全恢复的瞬态，契约已公开此限度；未建设监控或快照。
工作区清理使用隔离测试的 Git 操作，本次没有新增产品清理命令或改变停止/发布门槛。
