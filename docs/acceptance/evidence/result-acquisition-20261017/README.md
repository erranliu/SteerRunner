# 结果获取与 Workflow 解耦实施记录（2026-10-17）

状态：已实施并验证非 parser 分支；P0 原生隔离不合格，完整需求未完成，不能发布“三方式均支持”的结论。SteerLine 未修改，未部署。目标及测试条件继续以 [Spec](../../../spec/result-acquisition.spec.md) / [Plan](../../../spec/result-acquisition.plan.md) 为准。

## 构建与原生前置

Runner 基线 `f211e316b89e3b86527358f1bbd48eac9f58513a`，本次为 dirty 工作树，开发版本 `0.2.0-dev.1`、资源版本 2、workflow/2、state/v3；CLI 信封保持 v1。环境 Windows/amd64、PowerShell 7.6.5、Go 1.26.4。源码/测试文件摘要及精确命令保存在仓库 `.evidence/ra-directed-20261017/`；完整入口历史日志在 `.evidence/ra-controlled-20261017/`、`.evidence/ra-live-workflow-20261017/`、`.evidence/ra-live-workflow-final-20261017/`；最新批次为 `.evidence/ra-live-workflow-verified-20261017/`。

本机原生 EXE/version/SHA 和上游源文件摘要见 [qualification](native-qualification.json)。固定源码来自 OpenAI Codex `rust-v0.159.2` / `ff6aec96948b70d94983af2641a6b67c94faeff5`，只读 sparse clone 位于 `.evidence/ra-native-source/codex/`。没有读取或复制 Codex 认证凭据、没有新增账号、没有启动解析用模型会话。

P0 的具体阻断：

- 上游 [`ToolPolicy`](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/ext/extension-api/src/tool_policy.rs) 提供 `allowed_tools=Some([])`，要求由宿主在 `ExtensionDataInit` 启动前注入；默认 `None` 保留普通工具选择。
- [`spec_plan.rs`](https://github.com/openai/codex/blob/ff6aec96948b70d94983af2641a6b67c94faeff5/codex-rs/core/src/tools/spec_plan.rs) 的 shell、utility、extension 工具注册独立；例如 apply_patch 还按 model/environment 注册，关闭 shell 不构成完整工具禁用。
- 同版 `core/config.schema.json` 的 `ToolsToml` 只有 experimental_request_user_input、update_plan、web_search，没有该空启动白名单；stock exec 的固定 help/配置及当前调查未提供可核实的完整入口。另未证明全部 managed hook/隐式上下文可被排除。这个结论是“无法通过资格门槛”，没有把未证明的能力断言成已隔离。
- 官方 [CLI 参考](https://learn.chatgpt.com/docs/cli/reference) 说明 ignore-user-config 跳过用户配置且原生认证仍走已有 CODEX_HOME；它没有承诺关闭全部工具或系统管理上下文。[配置参考](https://learn.chatgpt.com/docs/config-file/config-reference) 也区分 managed 来源。未用提示词、read-only 或一次无工具事件替代工具面证明。

因此生产 parser owner、解析资源恢复/停止接线和 T7 前两项尚未实施。`ParserSupport()` 明确返回 unsupported；审计获取需要它时先记录资格失败，再按同一原源的一次 agent report 预算后备。完整可信机器审计、实施/发布成功行及非 parser 路径不探测 EXE/认证/模型。没有新 parser 进程，就没有伪造 parser Job/会话或 agent Turn。也不增加 Codex fork、替代代理/沙箱或第二持久资源服务。

实施期间只读复核共同生命周期：SteerLine 当前 HEAD `bfe392447591137e6d762f19daf90cdc828aa428`，文档 SHA256 `5378389FB393871B4939022B67FC9FAC1A10843441E7EE807A1A3EF94712CDE9`，工作区干净。本任务没有写入 SteerLine；与设计时固定来源相比，其外部新增内容是 caller 协作/实例准入说明，W/E/V 核心语义未变。它仍说明当前接入 workflow/1；本任务 workflow/2、state/v3 的独立验收与未来部署限制继续按 Spec §8.1 处理。

## 已接线的范围

- 运行默认提示来自编译定义，显式覆盖在原 Run 冻结；资源摘要同时覆盖编译定义，静态 prompts 资源不作为另一路运行默认值。补报只带原源绑定/摘要/事实，利用原会话已有上下文，不重复注入完整长 final；原文完整保留。
- 两份编译内置定义承载状态、提示、契约指令、成功行、模型角色、单次报告预算与后继；共享执行按能力/契约运行。完整内部 `Workflow.StepID` 是业务游标，真正提示才有 StageInput/PendingTurn/NextStage；程序提取不伪造原生报告。
- 普通实施/发布自然语言执行，末尾可信成功行或完整机器结果直接取得候选；发布唯一 PR 线索仍须 provider 核验，核验前 acquisition 为 unverified。原生失败/未知、冲突、信息缺失分别保留，一次补报无重试。
- 审计只有代码审计及同轮 P0–P2 修复，保存 fixed_count/逐项修复/遗留情况；无 Spec 审计或自动返修轮。原实施结果/Command/Turn/版本→pre_head→真实审计→post_head→发布结果/PR head 的引用均留在原 Record。
- Git owner 补普通完整工作区、索引标志/gitlink/稀疏、稳定 HEAD、祖先及 tree 条件；缺原终态检查点不以当前干净 HEAD 重建。已观察来源漂移持久失效资格。补报原生事实确认有写文件、提交或测试时拒收，不声称全面工具隔离。
- 完成接受/结果/来源/后继复用原 Commit/CAS。核验 I/O 在 actor 锁和派发 gate 外，返回重核 revision/源/stop；技术 resume 不重新实施/审计，审计补材料保持等待，明确 feedback 才新实施。
- Runner 配置惰性读取/冻结、原 request_id 去重先于重读配置；新增获取公开投影，原 final 不替换。旧格式拒绝，不迁移。

## 自动验证登记

定向与完整入口日志已归档；[完整入口](test-summary.json)、[计数](case-counts.json)、[源码/构建身份](source-identity.json) 可复核。禁止将下列迭代输出或 P0 降级记成 RA-A06 通过。

| 批次 | 状态 | 原始证据 |
| --- | --- | --- |
| 迭代组件测试 | 早期两次编译失败（测试使用了不存在的 Fault.Code，已改为 Detail.Code）；另一次验证中发现发布候选 unverified 不能被误判为源码漂移，已修正。保留原失败 | `.evidence/ra-l1-initial.jsonl`、`ra-focused-preflight.jsonl`、`ra-component-final.jsonl` |
| 首次新增 Controlled 场景 | 退出 0；1 个场景、5 个 subcase，通过。随后有来源与核验接线变更，最终批次须复验 | 本会话命令 `go test ./tests/integration -run '^TestSRResultAcquisition$' -count=1 -timeout=4m -v` 输出；最终以完整入口证据为准 |
| T1/T2/T3/T4/T5/T8 定向 | 最终已有入口全部退出 0，T1/T2/T3/T4/T5/T8 分别 8/7/5/1/5/6 个顶层测试（共 32）。T4 仅资格拒用，T5 未含 parser 资源；不代表未建用例已过 | `.evidence/ra-directed-verified-20261017/`；精确命令/退出码见 [directed-results](directed-results.json)，各组脱敏文本见本目录 t1/t2/t3/t4/t5/t8.log |
| 首轮完整入口 | L1 228 通过；Controlled 编译失败（测试变量重复声明），已修正并保留原日志 | `.evidence/ra-controlled-20261017/` |
| 第一次 L2 前置 | L1 230 通过；Controlled 揭示 Codex adapter 原有空报告权限声明，已在 owner 显式声明既有权限事实。未启动真实模型/PR | `.evidence/ra-live-workflow-20261017/` |
| 第二次 L2 前置 | L1 233 通过；13 Controlled 场景中 12 通过、1 失败（发布补报预算漏填），已修正并补行为测试。未启动真实模型/PR | `.evidence/ra-live-workflow-final-20261017/` |
| 当前完整入口 | 退出 0；L1 237 项通过，6 个仓库显式允许的 helper/真机场景 skip；Controlled 13 场景全部通过，新增场景含 5 cases；既有 L2 smoke 1/1 通过 | [summary](test-summary.json)、[Controlled 日志](controlled/integration.log)、[新增 L1 断言](l1-ra-results.json)；原始现场 `.evidence/ra-live-workflow-verified-20261017/` |
| T7 parser 两项 / 三 case driver | P0 阻断，未建设/未运行，非跳过通过 | `native-qualification.json` |
| 不依赖 parser 的既有真实 smoke | 退出 0，173.53 秒；真实实施/发布、原 PR 核验、合并后内容和 run/service 停止均通过；两个 program 结果 satisfied，report/parser 均 0 | [场景](native/scenario.json)、[日志](native/native.log)、[来源](native/result-provenance.json)；原始现场 `C:/srt/4316df69fdd2/evidence/` |

没有将未运行、零测试、skip 或夹具解析计入真实 parser 通过。RA-A01（三方式）、RA-A05（parser 资源部分）、RA-A06 明确未完成；其他自动条件按当前实测逐项登记，不以本记录代替自动验收。

最终静态检查见 [static-checks](static-checks.json)：本地导航、JSON 样例和证据、源码清单逐文件摘要、`git diff --check` 均通过；65 个本次修改/新增 Go 文件的 `gofmt -l` 无输出。SteerLine 工作区仍干净；本任务未改动其实现与文档。检查不替代产品测试。

## 最终证据与边界

精确入口命令为 `./scripts/test.ps1 -Level L2 -Scenario local-codex.smoke -Config .evidence/ra-live-config.json -OutDir .evidence/ra-live-workflow-verified-20261017`，退出 0。真实执行未指定模型，由原生默认配置选择；没有把这次执行模型的可用性当成独立解析模型资格。测试 PR [SteerlineTest #436](https://github.com/erranliu/SteerlineTest/pull/436) 已由授权测试驱动合并；原 head `a210a00ff6eff882cd8557ef0cce8903b57536cf`，merge `adfcbd4fe300d4409938d05fe85d67f939d9e8ba`。这不涉及 SteerLine 仓库。授权认证资源回收见 [cleanup](native/authentication-cleanup.json)，此处属于原执行沙箱契约，不证明 parser 空工具隔离。

[定义轨迹](definition-traces.json) 保存 Controlled 轮询实际看到的公共状态，可能省略轮询间的中间状态；不会伪造成完整逐边 trace。[状态权威](state-authority.json)、[故障窗口](fault-windows.json) 与 [公开/版本断言](public-fixtures.json) 索引本批实际通过的 L1 断言，不以索引替代测试结果。所有 72 条新增 RA 测试及子测试通过记录在 [L1 结果](l1-ra-results.json)，32 项定向与它们有重叠，不能相加当独立覆盖数。原始 SQLite/协议现场由测试宿主管理，验收目录不另建状态权威。

[源码清单](source-files.json) 固定本次工作树中 351 个源码/测试/脚本/运行资源文件 SHA256；包括未提交文件，排除验收文档以避免摘要自引用。Runner EXE SHA256 为 `a178ceda48ddf11f3097e7e6ce24b00d29bc7e5a573c5c76b5ac85a870db387e`，resource_digest 为 `1d82aa6517e3e661ee0ebb88b71fc858e14bea9a8a17453e1fbbbdf738924b91`。未提交、未安装、未部署；旧 workflow/1/state/v2 继续明确拒绝，不覆盖既有 SteerLine 固定发行包。
