# Parser 解耦实施与验证

日期：2026-10-31。源码基线 `e78c0db6f4e9be6242c6928cfeae596c8358ca5f` 加当前工作区修改；未安装、部署或操作真实业务任务。依据 [Spec](../spec/parser-decoupling.spec.md) 和 [Plan](../spec/parser-decoupling.plan.md)。状态：已实施并完成实现审计四项修复及定向复验，见下节。此前正式 L1、八个分支的定向受控闭环证据保留；本轮未重跑全量 L1、Controlled 或 L2/L3，不将旧构建证据计为修复后全量通过。

## 实现审计四项修复与本轮复验

1. **冲突不自动重算**：获取协调只接受 `status=running` 且 parse 游标、获准推进的等待。矛盾候选仍沿原业务规则 blocked；即使游标保留 parse，多次唤醒也不重建任务。blocked 的状态提交同时失效在途句柄；恢复仍使用既有业务状态，不新增任务消费字段。
2. **单层局部重试**：Analyzer 仅对已识别 transient 最多调用两次 CLI；CLI 的 HTTP/流重试为 0，WebSocket/无限连接重试关闭。参数使用本次调用内的 `steerrunner_text` provider，以避开内置 ID 不能覆盖的限制；保留原生 OpenAI 认证和默认地址解析，不设置 endpoint、env_key 或 token，不写宿主配置，不新增 API 服务或密钥。依据官方 [配置参考](https://learn.chatgpt.com/docs/config-file/config-reference) 和 [provider 约束](https://learn.chatgpt.com/docs/config-file/config-advanced)，并用现有登录真实 CLI 验证接线。
3. **删除闲置权限探测**：移除开发 adapter 的 `parser_configuration.go`、对应专属测试，以及残留 CancelParser 注释；ParserConfiguration/hasHookHandler 无生产消费者的旧路径不再保留。
4. **完整材料限额**：在容量等待及 CLI 启动之前检查最终 stdin prompt 加 schema 的 UTF-8 字节总量，包含 JSON 编码扩张；恰好上限可用，超一字节失败且零 CLI 调用。此限额针对调用方提供的材料，不包含 CLI 自有系统说明及协议封装。

| 本轮命令 | 结果与边界 |
| --- | --- |
| `go test ./internal/textanalysis -count=1 -v -timeout=90s` | 通过，包耗时 4.185 秒；覆盖真实受控子进程、transient 成功重试/持续失败均恰好两次、工具/认证/非法 JSON/schema/输出超限零重试、prompt+schema 精确边界、实际排队中取消、取得名额才创建推理期限及实际超时、默认并发 2/可配置 3。 |
| `go test ./internal/application -run '^Test(TextAcquisition\|BudgetParser)' -count=1 -v -timeout=240s` | 通过，77.359 秒；新增冲突后多次唤醒零重算，暂停/源替换/blocked 后新旧句柄屏障，旧结果迟到时当前任务不受影响；同一个 Engine 中另一 Run 的业务推进和两个 Run 的 stop 均在 parser 仍被屏障挂起时完成。原有成功/失败后备、提交失败、revision、停止和重启重算回归也通过。 |
| `go test ./internal/execution/adapters/codex ./internal/execution/resultacquisition ./internal/execution/workflow -count=1 -timeout=180s` | 通过，三个包分别 42.597 / 0.884 / 0.932 秒；确认旧探测删除不破坏其余 adapter/获取/业务路由。 |
| `go vet ./internal/textanalysis ./internal/application ./internal/execution/adapters/codex ./internal/execution/resultacquisition ./internal/execution/workflow` | 通过。 |
| `TestCLIParserLive`（本轮实际命令见下） | 修复后的 CLI 参数下通过；现有 CLI 0.159.2、现有登录、`gpt-6.1-sol`、effort=low，测试耗时 13.39 秒。 |

```powershell
go test -tags=live ./internal/textanalysis -run '^TestCLIParserLive$' -count=1 -v -timeout=180s -args '-parser-cli=C:/Users/lmas1/.local/bin/node_modules/@openai/codex/node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc/bin/codex.exe' '-parser-model=gpt-6.1-sol'
```

本轮真实 CLI 证明参数与现有登录的成功分析兼容；重试分支/调用次数由受控子进程覆盖，没有对真实提供方进行网络故障注入，也未声明真实多机、PR 或全量发布资格。暂停/恢复与源替换测试使用已有业务状态事务和 resumeWorkflow，未新增原生 parser 恢复流程。组件排队使用显式 context 屏障，业务竞态使用完成/释放 channel；期限只用于测试超时与防挂死，不靠固定 sleep 推断并发正确性。

四项修复后没有新增持久字段、数据库表、跨重启预算、全局版本或额外服务。以下记录属于此前实施批次；本轮结果以本节为准。

## 实现范围

- [独立组件](../../internal/textanalysis/analyzer.go) 直接调用现有 Codex CLI 和已有登录；忽略项目/用户执行规则和功能配置，内部设置 read-only/never、关闭工具、拒绝异常工具事件、管理临时目录及 kill-on-close Job。不读取/复制登录 token，不引入 API 服务或密钥。
- [获取协调](../../internal/application/text_acquisition.go) 只保存当前内存任务、原业务 Source/ImplementationResultID、取消和完成值。复用 audit.parse 游标及已有结果，候选/失败与业务转移沿原 Store.Commit 原子保存。没有 parser operation、持久解析槽、派发计次或跨重启预算。
- 取得名额后开始单次 CLI 超时，默认 120 秒；默认并发 2，可配置 `codex_parser.max_concurrent`，省略/0 均为默认值。组件只对已识别的暂时网络失败局部重试一次；工具、结构或配置错误直接走原一次 report/等待。
- 业务停止/暂停/源替换成功提交后失效旧内存句柄，不等待模型退出。服务关闭取消任务；重启可对尚未取得结果的原源重新分析，接受额外费用。重复唤醒、候选提交失败或无关 revision 变化不重复推理。
- 原审计 final、原生来源和 checkpoint 不被候选覆盖；工作区/PR 资格仍在业务及后继派发边界核验。完整结构直提与普通开发不触发 parser 配置/登录检查。配置错误只记获取失败并走原补报，不阻断普通受理或接管。

删除旧 application/parser、backend ParseInput/ParseEvidence/ParserTask、reliable parser operation、codexexec 资源/journal/回执、owner 的 OwnedParser/cancel_parser RPC、恢复资格及停止取证。原生 parser-owner 换代专项和对应权限/停止证据要求一并退役；主执行自身恢复、停止和只读行为保持。

保留 state/v6、默认目录、CLI/workflow 版本。旧 parser 操作仅在 [存储加载边界](../../internal/storage/legacy_parser.go) 非破坏拒绝；不保留旧执行实现。旧 owner 的 `retains_parser` 值仅作为签名元数据透传，新 owner 不写、不判断；避免破坏普通旧 owner 的身份/checkpoint 编码。旧 result_acquisition_config 同样只作惰性历史数据，新 Run 不冻结或输出它。

## 已验证

| 检查 | 结果及范围 |
| --- | --- |
| `go test ./internal/textanalysis -count=1 -timeout=60s -v` | 通过；受控原生子进程覆盖正常输出、工具事件/非法 JSON/schema 不符拒绝、排队/在途取消及内部工具关闭参数 |
| application `TestTextAcquisition*`、`TestBudgetParser*` | 通过；成功/需要输入/失败后备、原 final 保留、一次采纳、提交失败保留候选、无关 revision、停止不等待及重启重算 |
| `go test ./internal/execution/workflow ./internal/storage ./internal/resources ./internal/execution/reliable/... ./internal/execution/adapters/codex -count=1 -timeout=180s` | 通过；包含保留的原执行/owner 回归及新的来源定位，旧 parser 的原生 operation_id 不再作为解析来源 |
| `go vet ./internal/textanalysis ./internal/application ./internal/execution/... ./internal/storage ./internal/host ./internal/resources` | 通过 |
| `pwsh -NoProfile -File tests/framework/native-result-test.ps1 -Repository <仓库路径>` | 通过；正式 parser 专项结果改为文本分析/取消，不再要求虚构 Run/session、权限 canary 或主执行资源停止证明 |
| `go test -tags=live -c -o .evidence/parser-decoupling-native.test.exe ./tests/native` | 编译通过；未运行全 L3 |
| 真实 CLI 文本分析（见下） | 通过；CLI 0.159.2，现有登录，显式 `gpt-6.1-sol`、effort=low，原审计文本得到 schema/审计契约均有效的 JSON |

真实验证命令：

```powershell
go test -tags=live ./internal/textanalysis -run '^TestCLIParserLive$' -count=1 -v -timeout=180s -args '-parser-cli=<现有原生Codex绝对路径>' '-parser-model=gpt-6.1-sol'
```

此前实施批次的真实 CLI 验证在移除临时诊断、保留纯失败代码后的实现上通过，耗时 11.13 秒；修复后复验为上节的 13.39 秒。未创建 worktree、PR、业务 Run 或新的 API 配置。CLI 能力限制及工具事件拒绝不是操作系统级无工具保证；本次不扩大到其他模型、平台或全 L2/L3 资格。

## 正式入口及证据

- 正式 L1：`pwsh -NoProfile -File scripts/test.ps1 -Level L1 -OutDir .evidence/parser-decoupling-l1`。已通过；summary.status=passed，L1 报告 test_count=710，29 项扩展检查，native driver 编译与零匹配保护通过。原有显式 opt-in/helper skip 按正式入口白名单记录，不计真实后端资格；新旧数据边界及内存任务重启测试确实执行。
- 最终受控 EXE 闭环：`pwsh -NoProfile -File scripts/test-foundation.ps1 -OutDir .evidence/parser-decoupling-controlled-final -Pattern '^TestSRResultAcquisition$' -TimeoutMinutes 12`。覆盖普通开发、零/正数审计、业务 owner 离线、一次报告、parser 不可用/成功/工具事件八项内部 case。该入口异步启动独立 harness，脚本退出不代表场景完成；以该目录 result.json 和 integration.log 为准。本次 exit_code=0，scenario_count=1，八个内部 case 全部通过，耗时 404.40 秒。固定 Runner SHA256 为 `f6553375d64dcb38c014f40826e7eb975f81bf81188f6a5eaa16860154fa12f6`，对应 source-identity.json。
- 此前失败批次 `.evidence/parser-decoupling-controlled` 保留：parser-unsupported 的失败获取记录尚未标 failed，以及旧假 CLI 强制要求退役的 MCP 单项配置查询，导致 parser-complete 走补报。已分别修正业务失败记录和夹具断言；不得将该批记为通过。

## 调查与失败边界

先前 API 方向及无凭据原型记录在 [P1 历史](parser-decoupling-p1.md)，用户明确采用 CLI 后原型已删除，不再作为前置。

真实 CLI 最初的命令行中模型参数未整体引用，PowerShell 将带点号的参数拆分为 `gpt-6`，出现元数据/不支持模型错误；正确引用后实际配置为 `gpt-6.1-sol` 并通过。临时诊断已移除，生产错误不返回原生正文。期间人工临时目录诊断也只使用合成文本及已有登录，未操作账户配置或业务资源。

曾使用 `go test -run '^$' ./tests/integration` / native 包尝试只编译，但其 TestMain 正确拒绝缺 evidence 或零场景；已改为 `go test -c` 及正式受控入口。这些是 harness 调用失败，不计场景通过或主执行失败。

完整真实开发交付、其他机器/Orca 的真实 parser 接线及其他模型未在本次执行；定向受控、组件真实分析和完整发布资格分别报告，不互相替代。

## 验收映射

PD-A01：原生操作/owner/恢复/停止消费者及闲置 adapter 探测已删除，本轮依赖守卫通过；旧签名元数据仅透传。PD-A02：本轮已有登录真实 CLI、受控子进程及完整材料限额通过。PD-A03/04：此前 L1 只证明其实际执行的原有场景；本轮新增/复验冲突不重入、单层局部重试、实际排队取消/超时、提交失败、revision、暂停/源变化后的旧句柄隔离、同 Engine 的其他 Run 推进/停止及重算，具体范围以上表为准。PD-A05：此前八项受控闭环覆盖提取/解析/后备，本轮定向业务路由回归继续保留原 final/来源。PD-A06：v6/默认目录和旧数据加载策略本轮未改；此前相应边界测试证据保留。

本次无安装部署、无新 API/凭据、无全量 Controlled 或真实多机/PR 支持扩展；全部修改留在工作区供审阅。
