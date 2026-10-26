# result-file 最终快照：实施与定向验证

日期：2026-10-26。状态：收缩实现及定向验证完成，等待审阅。依据 [Spec](../spec/runtime-result-file.spec.md) / [Plan](../spec/runtime-result-file.plan.md)。本稿取代持续观察设计的验收口径；仅本地及受控资格，无真实远端或部署资格。

## 实施结果

- 删除 result observer、定时采样、内容变化计数、采样竞争及恢复重开框架；恢复原 progress 的 prepare/read/observe/finish 组织。
- result 仅在原执行确认结束、生产者停止后进行一次最多 2 秒的采集，受原停止/服务退出父期限约束，先封存再释放原读取资源。pr_ready、单 turn、waiting_input、被动断线不触发。
- 成功 version=1；失败 version=0 加 diagnostic；两者均 closed 并持久封存。查询只读记录，重启/resume 不刷新。持久化失败不先发布，同一服务不重复已开始的物理采集；跨崩溃只保证最多发布一份快照。
- 保留一次初始化、未知只读对账、原主机/runtime/worktree、固定路径、两变量独立清理、64 KiB 和独立 schema/capability（collection=end_only）。未扩展支持组合、workflow、任意路径或 artifacts。
- 业务提交、progress 保存、result 封存从当前 Run 合并；迟到封存重验原绑定、结束资格、原准备事实和尚未封存，保留其他字段及业务 Result。

## 当前构建与证据

基线 HEAD `5df57b69e98382abcd63676b04a73079fd939fa6`，工作区未提交；Go 1.26.4，Windows/amd64。当前受控 EXE SHA-256：`171C68867C622A63CDA04D8E50DD6D92F5201FFB4EDDAAB824D034301DCB4DB9`。源码清单/摘要及结果汇总见 `.evidence/result-final-only-summary.json`。

| 验证 | 结果及证据 |
| --- | --- |
| 定向 L1，8 包 43 个顶层测试 | 全部通过；`.evidence/result-final-only-unit-complete.jsonl`。含原 progress 8 项、封存/保存保护、输入/schema/CLI、初始化、字节/原子替换、精确包装解包和原绑定 RPC |
| L1 增量，3 个顶层测试 | 全部通过；`.evidence/result-final-only-unit-delta.jsonl`。双向初始化失败不短路另一方、正常完成仍要求停止事实、原 Codex 环境回归；与前组按名称去重共 45 项 |
| 私有 IPC 反例 | 上述 L1 的 TestResultFileOriginalIPCBoundaries：实际私有 named-pipe、两 Run 哨兵、本机同路径不同哨兵、错 host/runtime/worktree/instance/path、断线、阻塞、超限；没有真实 Orca 远端 |
| 当前构建受控 result 闭环 | TestSRResultFile 通过（187.49 秒）；`.evidence/result-final-only-controlled-final/`。四组合、实际 native 环境、运行中 pending、原子替换任意字节、明确停止、封存、重启后不刷新；native turn/start 共 4 次，无查询/重启额外输入；最后服务停止记录 resources_settled=true |
| 原 progress 当前构建回归 | 同批一次 Windows 原子替换 Access is denied（56.56 秒），原日志及私有失败现场保留。相同 EXE/fixture 摘要隔离复核也失败（73.03 秒）；`.evidence/result-final-only-progress-isolated/`。本地 Root.Open 保持目标读句柄的最小复现重现 Access is denied；仅给受控 producer 的同一次原子替换增加 1 秒内对 Windows 共享/访问冲突的重试，Runner 算法及生命周期/投递期限不变；最新受控复核通过（80.74 秒），`.evidence/result-final-only-progress-producer-coordination/`；Runner 摘要仍为上述 171C…，只有 fixture 摘要因 producer 协调改变 |
| 检查 | git diff --check 通过；构建和正式 foundation 前置检查完成；未修改 progress 公共 schema/capability |

定向命令：

```powershell
go test ./internal/application ./internal/execution/workspace ./internal/execution/agentlaunch ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport ./internal/resources ./internal/contracts ./internal/cli -run '^(Test(ResultFile|Progress|Usage)|TestServiceGenerationTakeoverRejectsUnknownCodecAndKeepsOriginalFacts)' -count=1 -timeout=5m -json
./scripts/test-foundation.ps1 -Pattern '^(TestSRProgressFile|TestSRResultFile)$' -OutDir '.evidence/result-final-only-controlled-final' -TimeoutMinutes 15
./scripts/test-foundation.ps1 -Pattern '^TestSRProgressFile$' -OutDir '.evidence/result-final-only-progress-producer-coordination' -TimeoutMinutes 10
```

执行时仅测试进程使用 GOFLAGS=-p=1、GOMAXPROCS=4；未放宽产品或用例期限、断言或跳过规则。原 progress 的文件字节读取算法、完整内容断言和投递期限未改；受控 producer 的有限 Windows 共享冲突重试作为平台协调单独记录，未给 Runner 增加重试。复核不覆盖原失败。L1 增量详见对应原始 JSONL。

## 过程记录与范围

- 新语义首轮 `.evidence/result-final-only-controlled-r1/` 两个受控场景通过（progress 70.77 秒，result 111.51 秒）；其后增加服务退出父期限约束及最终提示/capability 标注，因此以当前构建证据为准。
- 初次新单元测试编译因未使用 import 失败，移除后通过。一次 `go test ./tests/integration -run '^$'` 在编译后被仓库零场景防护拒绝，不计验收；正式受控入口按实际选择场景执行。
- 先前持续观察设计及宽范围执行全部标记被取代，不计本稿通过；原记录完整保留于 `.evidence/result-final-only-superseded-notes.md` 及原 evidence 目录。上一轮宽范围 harness 已退出，其残留的私有 service-ownership fixture 服务经原实例核对后明确停止；未操作生产或共享 Orca 资源。
- 本轮未重跑完整 content recovery、resource creation、全 Controlled、原生 L2/L3、全 Agent 矩阵。真实远端双文件闭环与部署仍未执行；没有推送或创建 PR。
