# result-file 最终快照：实施与定向验证

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-10-26。状态：收缩实现及定向验证完成，等待审阅。依据 [Spec](../../../../spec/runtime-result-file.spec.md) / [Plan](../spec/runtime-result-file.plan.md)。本稿取代持续观察设计的验收口径；仅本地及受控资格，无真实远端或部署资格。

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

## 2026-11-01 通用 execute/1 增量

用户明确授权最小扩展，并要求在独立 worktree 实施。工作区为 `C:/Users/lmas1/.codex/worktrees/runner-project-detection/SteerRunner`，分支 `codex/project-detection-execute`，基线 `e2729563562314659a799a5f82debe427fcd7c24`；原 `C:/Project/SteerRunner` 未修改。

execute/1 在普通可写 Run 工作区只发送一次调用方正文，直接消费原可信 final 后关闭；不要求 publication，没有审计/发布/结果获取、项目策略或业务检测知识，复用 result_file 最终封存和原停止。拒绝 reply/resume/message/interrupt/close，停止后不恢复，提问按 human_input_required 关闭。未新增服务或框架。

独立审核 `/root/prompt_review` 已审阅实际 Git 差异并批准；首审发现的额外输入/恢复漏洞已修正，后续复审无剩余可执行 P0–P2。新增默认正文为“执行调用方指定的操作一次，保留原始结果与失败。”，69 UTF-8 字节，SHA256 `d4ae6da49ec392028096e1f48fbd0935819a63c5d99e022663ec0d12741bb9c0`；prompts.json 1088→1174 字节。该正文是公开默认定义，正式 execute 请求必须提供唯一非空 execute 覆盖。ResultFileInstruction 正文不变；公开 result_file.when 同步增加 execute/1，未改变 progress 规则。样本和资产测试更新不替代独立批准。

证据根 `C:/SteerlineDebug/wt-68d4-project-environment-workflow/result-consumption/runner/`：

- `L1/01/summary.json` 正式 L1 passed；配置/catalog/framework、721 个 Go 测试及29个终端扩展测试通过，现场 opt-in/子进程角色跳过按原 L1 约定记录，不代表原生目标全部可用。application 全包 594.546 秒、coverage71.6%。
- `result-file-01/result.json`：正式 TestSRResultFile 公开封存回归 exit_code=0，1 场景，原最终快照和停止证明保留。
- 定向 workflow/contracts/resources/application 全包通过；新增 SingleExecution 控制/停止/提问反例通过。公开能力描述最终修订后 workflow/resources 全包再次通过（1.375/2.311秒）。最初资源 catalogue 旧数量/版本期望失败已修正，未放宽实际 schema。

跨仓真实单次调用由 SteerLine 当前专属验收记录；本段只声明上述 Runner 自有测试，不把受控结果当原生成功，不复用旧开发PR证明 execute；未部署或合并业务主分支。

## 2026-11-01 两仓 PR 前主干整合复审

用户要求审核后两仓提PR、不得合并。独立worktree整合main=8bed4c4；engine stop冲突保留PendingResume取消后对execute永久关闭，result_file能力保留主干best-effort初始化并公布execute/1，合并两文档定义；自动合并freeze/observation/runstate也逐项复审。`/root/prompt_review` 批准相对该最新主干的最终实际正文/接线/资产，无剩余可执行P0–P2。manual pause不恢复或追加execute输入，stop优先仍可收尾；无项目业务检测知识或新框架。

证据 `C:/SteerlineDebug/wt-68d4-project-environment-workflow/result-consumption/pr-audit-02/`：正式runner/L1/01 passed，728项Go测试及29项终端扩展，应用全包551.027秒、coverage74.6%；按原分层记录的native opt-in/子进程角色跳过不代表原生资格。整合后的SingleExecution/manual/workflow/result-file组件回归应用包34.032秒通过。

干净pin源码50f12791abd28e241298643b2ceb0b510899d370，source_digest cc89825927cec77174d04e4bffc91955d13c818b5928bef7bfb44ae0324fc201，EXE SHA256 f247b1332fb4e5f24aea92db8f9c14dcf044fb0c783a5996ee75e13ed97a570c。SteerLine/L4/local-01在该新pin实际通过（执行175.960秒、cleanup_complete=true）：项目入口只调用一次，原结果completed/ready，原Run可信native完成、停止、最终字节封存，准备前拒绝和准备后真实Task准入/停止，全部自有资源清理。平台不因不透明可选明细含failed而重判。SteerLine同轮固定L1/L2亦通过；其前轮原生负结果保留，不冒充当前pin重跑。

本次收尾只补文档与空行规范，生产源码与50f1279相同。execute默认69字节、prompts.json 1088→1174字节保持，不以资产期望代替批准。Orca新增工作流原生矩阵未重跑；未部署、未合并、不启用自动合并。


## 2026-11-01 PR 复审修复

PR79 复审发现 late final 自动关闭会覆盖已受理 PendingStop，导致原 stop 回执未完成；已改为保留原 stop，拒绝另一不同 stop，原请求仍可幂等重放。配套调用方同构建取消恢复不再调用退役 takeover；新增 public inspect --offline，非创建地取得原 writer 租约，只读 current/v7 snapshot，拒绝驻留写者、未知 bootstrap、旧格式、refresh 或实例绑定，不启动 worker、不迁移、不封存结果。调用方在原 pending_stops 入口于 worker 前受理必要 stop；Closed 原 Run 由既有持久停止继续结算。

独立 /root/prompt_review 已审阅实际正文/接线/能力资产和反例并批准，无剩余可复现 P0–P2。Runner prompts.json1174字节、execute默认69字节均未改动；新增 offline_inspect 仅能力事实，没有项目要求、提示词副本或新框架。生产修复提交9fd309f，干净构建source_digest318c6fbc2fbaace541e240c843cbf7a1cb658e1adbf02d028e90b172c110f349，EXE SHA256 9a0d638479aa5e32599f868d114b0472b09d7f3a3a1116edf3b00c1b5d9ac5a9。

证据根 C:/SteerlineDebug/wt-68d4-project-environment-workflow/result-consumption/pr-fixes-04/runner：cold-public-recovery 既有四种恢复路径通过；single-cold-stop-02 公共 EXE 与受控原生进程的 execute 冷恢复通过19.08秒，原请求/Run公开查询、worker前stop、一次输入、原stop回执completed、result closed。host/CLI非创建只读/驻留租约冲突/无原状态/不支持格式及application late-final定向回归通过；新增测试首轮编译错误保留，修正后新目录通过。上述受控恢复构建来自提交前 dirty 8c3ee3f，与新干净 pin 的生产 source_digest 相同，但 EXE/pin 不同，不当作新 pin 或真实模型冷恢复实跑。新干净 9fd309f/9a0d6384 pin 的实际本机 L4 由 SteerLine 4bdeeca2 执行并通过（准备21.712/执行267.119秒、cleanup_complete=true）：一次项目调用、原生可信完成/停止/封存、实际Task准入/停止与自有refs清理。正式 L1/summary.json passed，732项Go测试及29项终端扩展通过；native opt-in和子进程角色跳过沿原分层列出，不代表原生全矩阵。该次L1在8c3ee3f+dirty生产修复快照启动，随后仅提交原有内容，最终9fd309f生产代码未再改变。未部署、未合并、不启用自动合并。
