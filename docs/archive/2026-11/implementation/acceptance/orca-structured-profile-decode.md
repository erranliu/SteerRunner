# Orca 结构化 profile 单次解码验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

PR #123 合入前复审更正：本记录中基于忽略错误的 `json.Marshal(setup)` 比较，不能证明 setup 不变；setup 含函数字段，原比较实际为空。已修复新增构造测试及两项 #121 测试，完成最终候选复验，见[审计与修复记录](orca-structured-profile-decode-review.md)。下方保留首次候选及原证据，不把它们追认为修复后候选的结果。

2026-11-06，合并前验证。实际主线基线 `5b5487b56715dfa3a2bbbd9d4c2d207b6647e555`（#122），隔离分支 `codex/orca-structured-profile-decode`；依据 [Spec](../spec/orca-structured-profile-decode.spec.md) 与 [Plan](../spec/orca-structured-profile-decode.plan.md)。本轮未修改 Orca、SteerLine、安装现场、公共协议或门禁。

生产修改限 `frozen_record.go` 与 `lifecycle.go`（+14/-11）：结构化构造直接一次调用原完整 `decodeAgentSessionProfile`，填充原 Transport/version/host/development 和私有 audited；NewLifecycle 消费该结果，执行视图完整投影。原 agent → Clone → ProcessHost 顺序、旧协议执行前置拒绝、旧 terminal 解码、client/clock/nativeAccess/probe 初始化及 frozen 只读白名单保持。未改完整解码器或其内部校验，不新增缓存、工厂、策略接口或配置副本。

## 构造等价与受影响包

```powershell
go test ./internal/execution/adapters/orcatransport -count=1 -json
go test -overlay .evidence/profile-decode/baseline-overlay.json ./internal/execution/adapters/orcatransport -run '^TestStructuredConstructor' -count=1 -json
```

| 执行 | 顶层通过 | 叶例通过 | 失败/跳过 | 包耗时 |
| --- | --- | --- | --- | --- |
| 未修改主线完整 orcatransport | 73 | 261 | 0/0 | 56.532 秒 |
| 原生产代码 overlay + 新增构造夹具 | 2 | 21 | 0/0 | 1.922 秒 |
| 最终候选完整 orcatransport | 75 | 282 | 0/0 | 56.561 秒 |

原生产代码来自同一基线 `git show HEAD:<原路径>`，overlay 不改工作区；夹具仅去掉原版不存在的 frozen `agentAudited` 字段比较，执行对象原 audited 比较仍执行。候选对新字段另做全量断言。没有用改写生产 decoder、计数 hook 或性能测试证明“单次”；实际 diff/调用图确认 NewLifecycle → decodeFrozenRecord 的结构化分支仅一次调用原完整解码器，解码器内部校验保留。旧 terminal 分支仍走原 decodeLifecycleProfile。

新增 `TestStructuredConstructorFacts` 覆盖本机 execute、development v1/v2、audited、Pi v2，配对 Codex execute v1/v2 与 development：独立预期 Transport/version/host/development/audited；验证配置不被修改、克隆不受后续源 payload 污染、再次构造重新校验、ProcessHost 保持原共享关系、零原生调用。每个代表值均通过 frozen reader 不实现 LifecyclePort、mutation 在 I/O 前被拒绝的检查。

新增 `TestStructuredConstructorFaultPriority` 覆盖缺失/重复、codec 与坏 JSON 并存、JSON 与错归属并存、封套归属、未知字段、尾随 JSON、target、退役 launch、Pi v1、错误 host、非法 audited；构造错误类型、完整诊断及 BackendError.Code 与原完整解码器对照。各例叠加 nil ProcessHost、unsupported agent、旧 protocol 验证原首错顺序，失败不返回可用 owner。现有旧记录冻结读取、原停止冷读、旧执行拒绝及当前创建/续轮/模型/usage/progress/result/close 成功与未知零重放回归全部通过。

测试初跑纠正了新增夹具修改 payload 后未重建封套摘要的问题及一处测试声明语法；均在最终构建前修复。最终候选与对照通过，未改变生产校验来容忍夹具。

摘要、顶层测试及新增用例清单见 [tests.json](../../../../acceptance/evidence/orca-structured-profile-decode-20261106/tests.json)。原 JSONL 保留本 worktree `.evidence/profile-decode/`，不是完整 L1/Controlled 或完整 L2/L3 资格。

## 同一候选双端正式联验

```powershell
.\scripts\build.ps1 -OutFile .evidence/profile-decode/live/writer.exe
.\scripts\build.ps1 -Version 0.2.0-profile-decode-reader -OutFile .evidence/profile-decode/live/reader.exe
python .evidence/profile-decode/live/acceptance.py
python .evidence/profile-decode/live/verify-creation.py
```

两个 EXE 的 `source_digest` 均为 `473676b10a73bbe0a5ab8721c5eaeadc2fd26c55a8723da8c6c3fe5430b5c9e2`，source_commit 为上述基线、dirty=true，包含最终测试修改；验收后重新计算一致。[构建记录](../../../../acceptance/evidence/orca-structured-profile-decode-20261106/builds.json)：writer SHA256=`7619554464c63c31d8242f5ce4e910cae7773ef3bfbec0736aadc5693a495b75`；独立 reader SHA256=`72f305e124c90e22bca62c57669840b2f23dbad7df63cffe140877715e5ddf1d`。

| 目标 | 原 Run | 原 prepare_workspace operation | selector |
| --- | --- | --- | --- |
| 本机 Orca Codex execute/1 | `69fc3182df7e39e068b32ebdd87edb60` | `20c1d7e83321777a5537e37a40aebf56` | path |
| 配对远端 Orca Codex execute/1 | `dab25227b610f14c391c6ce2dc55f11d` | `5e66b99e94caf95778f78fcb7aaa1ec2` | id |

两端串行使用相同 writer、新目录与新 request_id。原配置经候选公开 `validate --kind run` 成功后，再走 service start/submit/inspect/result-file/service stop/verify-stopped。复用既有验收脚本，唯一调整为证据根、名称和额外纯校验；#122 原生配置 gate 未改变。过程中的 operation_outcome_unknown 只观察原请求至确认，没有补发或重建。

两端均得到预期三行可信结果。原多行/中文/emoji 输入摘要、唯一原已受理回合和会话绑定通过；创建 owner 计划、派发与输出摘要、runtime/worktree/instance/selector/base/HEAD、Workspace/Target 全字段对照通过。两端最终 completed/closed/stopped，恢复与新输入准入关闭。原 close_receipt 为 `{ "ok": true }`，候选原有核验及冷读判定通过，停止范围严格为 `orca_agent_session`；不证明 detached processes 或额外资源清理。

result-file 均为 `steerrunner-result-file/v1`、version=1、collection_status=closed，预置 `rpc-seed` 的 SHA256 与封存相符；此案例没有声明模型写文件。结果、原回执摘要、唯一输入/回合及排空证明见 [runs.jsonl](../../../../acceptance/evidence/orca-structured-profile-decode-20261106/runs.jsonl)，状态日志见 [formal-live.log](../../../../acceptance/evidence/orca-structured-profile-decode-20261106/formal-live.log)。[实际入口](../../../../acceptance/evidence/orca-structured-profile-decode-20261106/acceptance.py)沿记录中的历史本机脚本位置复用原验收，非新增生产框架；[绑定读取脚本](../../../../acceptance/evidence/orca-structured-profile-decode-20261106/verify-creation.py)只读原目录。未提交数据库、认证或配对正文。

## 原目录排空与最终审计

SteerLine 当前干净 checkout `6d12483213822db44727bdc146ecb2d181bfd326`，沿其原生产 `VerifyUpgradeDrained`：设置 `ORCA_WORKSPACE_RECEIPT_EVIDENCE` 为本轮 live 根，在该 checkout 执行 `go test -overlay <本轮 live/upgrade-overlay.json> ./internal/runnerclient -run '^TestOrcaWorkspaceReceiptOriginalDirectories$' -count=1 -v`。仅通过临时 overlay 加入[专项调用](../../../../acceptance/evidence/orca-structured-profile-decode-20261106/upgrade_test.go.txt)，未修改其源文件或门禁。

两端均在原 writer 停服后，由 writer 和独立同源码 reader 核对原 state 目录的同一个 Run/input；生产升级排空通过（案例 1.34 秒），数据库与 shutdown.json 冷读前后 SHA256 一致，见[原日志](../../../../acceptance/evidence/orca-structured-profile-decode-20261106/upgrade-gate.log)及 runs.jsonl。没有以停服或命令退出替代业务、资源、未完成操作的原 gate。

最后逐行审计：协议分支只依赖原 setup 协议；完整 decoder 与旧分支不变；五个派生字段来自同一结果，audited 未从 development/codec 推测。NewServiceFacts 只是原无副作用值的构造，不执行 probe。frozen 仍无执行、上传、observer/feed/Job 或状态所有权；白名单、配对身份、能力与错误、未知零重放及停止/文件合同未改。未发现待修复运行缺陷；gofmt、diff 检查与文档链接检查通过。创建 PR 与最终交付前另核对最新 main 和 GitHub 实际合并状态。

本轮只证明完整受影响包、双端默认 execute/1 与两个原目录排空；Pi/audited 只有组件及构造回归，未做其真机，不安装远端 Pi。旧记录证明复用原冻结夹具，未启动旧执行或 helper。完整 L1/Controlled、完整 L2/L3、合并后回归和部署均未执行；资格继续按[测试指南](../../../../testing.md)解释。
