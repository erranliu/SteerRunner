# 结构化结果获取：设计调查记录

日期：2026-10-16。仅调查，不是产品验收证据。[Spec](../../spec/result-acquisition.spec.md) · [Plan](../../spec/result-acquisition.plan.md) · [验收](../result-acquisition.md)。

## 仓库与共同规则

- Runner 工作树：`C:/Users/lmas1/.codex/worktrees/1c3c/SteerRunner`，HEAD `f211e316b89e3b86527358f1bbd48eac9f58513a`。开始时 `git status --short` 为空；本轮最终仅 Markdown 变更。
- 本工作树、主 checkout 与跟踪文件未找到仓库 AGENTS.md；祖先 `C:/Users/lmas1/.codex/AGENTS.md` 存在但为空。没有据此启动 roadmap 或委派子 agent。
- 读取 README、docs/testing.md、首版 Spec/Plan、workflow-selection-audit Spec/Plan、reliable-modularization Spec、environment-check Spec/Plan/验收及现有 acceptance 组织方式；本期单独登记 RA-A01–A06，不重用其他任务的通过结论。
- 共同生命周期文件：[worker-lifecycle.md](C:/Project/SteerLine/docs/product-design/worker-lifecycle.md)，读取时 SteerLine HEAD `04ba59542718e2d3b4d1db3d10a290955135f428`，文件 SHA256 `7CFAB09F2F7E46797E0B2436DD345C6C74F7B94A3769879A956AFF8F2C78DBF8`。这是读取现场身份，不推断整个 checkout 干净。重点核对 W08–W16/W18、E01–E07、V05–V08、原输入/未知/恢复/格式预算；完整共同规则不复制到本任务。
- SteerLine 仅只读调查 `internal/runnerclient/{catalog,views,selection,client,input}.go`、`runnercontract/input.go`、`app/runner_config.go`、`taskrunner/service.go`；响应扩展使用 json.Unmarshal，catalog 校验 CLI/resource identity 和 configuration_schemas.version=1。workflow 从 profile 透传，空值回填 development/1（service.go:143），不能自动采用新版流程；当前 DTO 也不提供新增审计模型能力，本期不顺便补齐。固定旧发行包、profile 版本与 v2 目录三个障碍见 Spec §8.1；本期验收独立 Runner，不给 SteerLine 增加配置步骤。

## Runner 源码调查定位

| 读取范围（相对 internal） | 核实结论 |
| --- | --- |
| `execution/workflow/{route,result,profile,types,input,prompt,feedback}.go` | 固定实施报告；publish/audit JSON；审计源硬绑定 StepReport/Command/Turn；两次修复；反馈与重入依赖报告 step |
| `application/{results,freeze,worker,feedback,control,engine,workflow}.go` | finish 原子保存原生事实/路由/下一输入；成功缺 final 停在原源补读；resume 默认可回实施；stop 当前主要枚举 executor resource，parser 必须接入 |
| `application/{report_policy_test,shutdown_test}.go` | 现测试明确报告只读是 adapter 的提示词策略；shutdown 需使用已提交最终原生事实，不能借历史状态结算新增 parser |
| `execution/reliable/result.go` | 已有 native termination/content verified/session eligibility 分离和相同原结果幂等/冲突校验 |
| `execution/adapters/codex/{native_params,policy,launch,observe,history,executable}.go` | 受管 native EXE、固定 0.159.2 执行权限、独立 app-server、来源日志/历史读取、主机请求拒绝和已知 shim 解析；这些不是 parser 无工具隔离的既成能力 |
| `execution/process/{host,facts}.go` 及 Windows 实现 | managed process/Job、有界输出、原子文件；现 OutputCommand 不能代替先持久 parser 身份/意图的异步 owner |
| `execution/publication/provider.go` | 当前 Provider 只有 Query(URL)，没有按 head 列举/反查 PR 的接口；本期缺 URL 用原 agent 补报，避免为此扩大 provider API |
| `runstate/record.go`、`storage/{store,runs}.go` | 唯一 Record 和单写 Commit/CAS；当前 steerrunner-state/v2/user_version=2 |
| `contracts/{run,view}.go`、`resources/resources.go`、`resources/assets/{prompts,development,settings.schema}.json` | 现公开 v1、workflow/1、真实 final/audit 投影和资源摘要；本期版本方案见 Spec |
| `scripts/test.ps1`、`tests/catalog.json`、`docs/testing.md` | L1、完整 Controlled 为 live 前置；当前 12 个 Controlled 场景；新测试入口须先建设并登记，不能直接声称运行 |

## 2026-10-16 本机只读 Codex 探针

执行 `Get-Command codex*`、`codex --version`、`codex --help`、`codex exec --help`、`codex app-server --help`、`codex features list`、`codex app-server generate-json-schema --help`，以及 `generate-json-schema --experimental --out <系统临时目录>`。未读取用户 config/auth.json，未登录、未启动模型任务。生成协议 schema 是本地工具操作，不创建业务会话或服务。

发现同机有不同发行包，不能用命令名假设版本：

| 路径 / 解析入口 | 实际版本与 SHA256 |
| --- | --- |
| shell 中 `codex`，已进一步用准确 native 路径复核：`C:/Users/lmas1/.local/bin/node_modules/@openai/codex/node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc/bin/codex.exe` | `codex-cli 0.159.2`；`52F75C649BEBB8001102A1DD129C1EA6D02B0940321E6D7E82EE0526753BD58A` |
| `Get-Command codex.exe`：`C:/Users/lmas1/AppData/Local/OpenAI/Codex/bin/9614f1d9b40d9c4f/codex.exe` | `codex-cli 0.162.0-alpha.17.2`；`E98C516F1C8935789AABE9122EE7E05EC4C3EFD1BA6ACF179242E031C495B99E` |

分别执行了准确 native 路径的 `--version` 与 `exec --help`；均列出 ignore-user-config/ignore-rules/output-schema。Spec 首个候选为 0.159.2；默认发现若落到不同包应明确资格不符，不能未经验证放行 alpha 或混合证据。没有改 PATH/全局配置。

0.159.2 生成 schema 位于系统临时目录 `sr-result-design-188c3e3900b243f0ad354467934a5a4c`（可重新生成，不作为持久结果权威）：

- `v2/ThreadStartParams.json` SHA256：`80A40A7FAC15B4BF70EFB7F893FB353ACC0A0D30C68F54AEE4F01923DECA85DE`。
- `v2/TurnStartParams.json` SHA256：`07771223642E1B61BD9AAC0069FC0F98143A1C047724CA02C7CEB13653442738`。
- ThreadStart 声明 cwd/model/config/baseInstructions/developerInstructions/dynamicTools/ephemeral 等字段；没有“空 dynamicTools 就关闭所有工具”的声明。
- TurnStart 声明 outputSchema（约束本 turn 最终 assistant message）。只证明协议字段，不证明模型/账户支持或语义正确。
- exec help 列出 `--json`、stdin `-`、`--output-schema`、`--output-last-message`、`--ignore-user-config`、`--ignore-rules`、`--ephemeral`。其中 ignore-user-config 的说明明确 auth 仍使用 CODEX_HOME；本设计为可对账而不启用 ephemeral。
- features list 可见 shell_tool、unified_exec、apps、plugins、hooks、multi_agent、browser/computer/image 等能力；该输出不是 parser 有效配置清单，更不能证明完整关闭组合。实际工具注册与隐式上下文排除尚待 P0。

alpha 包 schema 另存临时目录 `sr-result-native-design-74eae5fc8da84b26a9947049bc263398`：ThreadStart 同上述摘要，TurnStart 为 `062373A7DFC290B506E591BC5C2A35005211DF27B3C60358CEE3EFED12859062`（仅记录另包存在；其能力不用于 0.159.2 支持结论）。

## 官方资料及能力等级

于本轮搜索并实际打开以下官方页面（部分 developers URL 跳转至 learn.chatgpt.com）：

| 官方来源 | 用于本设计的窄结论 |
| --- | --- |
| [Codex App Server](https://learn.chatgpt.com/docs/app-server) | 独立 thread/turn、每 turn outputSchema、完成/中断事件和读取历史接口存在；动态工具不是全部工具开关。具体本机 schema 另行核对 |
| [Authentication](https://learn.chatgpt.com/docs/auth) | Codex 原生 credential store 与 CODEX_HOME 相关；认证环境是引用，不等于可随意复制的 token。原生认证管理继续由 Codex 承担 |
| [Configuration Reference](https://learn.chatgpt.com/docs/config-file/config-reference) | 有 shell_tool、MCP/app 工具、hooks、项目指令等配置面；read-only/never 和提示词本身不足以证明所有工具/上下文均被排除 |

上述页面没有证明本任务要求的完整隔离组合，也没有证明当前登录可使用任意模型。未采用搜索中出现的 token-sharing 自行注入/续期 token 方案；这与用户要求的原生 Codex 认证所有权不同。

**已证实**：精确版本/帮助/部分协议能力及仓库当前耦合。**设计选择**：首个 parser 使用独立 codex exec、默认关闭、有限材料包、无业务工具、原生认证引用、复用原 Record/Operation/managed owner。**待验证**：完整工具/隐式上下文关闭、原生 store/续期与 ignore-user-config 组合、模型资格、exec 完整来源及恢复、实际 Windows 解析权限与停止。P0/T7 未通过前不声明真实 parser 支持。

## 静态交付检查

检查范围为本轮新增 Spec/Plan/acceptance/本记录与 README 导航；本轮 5 文件静态检查、JSON 样例解析、RA-A01–A06 覆盖、尾空白/冲突标记、`git diff --check` 均通过。全仓 Markdown 导航检查为 43 文件/219 本地链接/0 错误，结果在验收文档登记。只检查设计文档，不运行产品测试或调用其他仓库调度。

## 2026-10-17 范围、状态权威与 Git 来源复核

本轮只读复核 Runner 同一 HEAD `f211e316b89e3b86527358f1bbd48eac9f58513a`；当前工作树已有本会话设计文档及 README 变更，不再称本轮开始时干净。仓库无 AGENTS，祖先 `.codex/AGENTS.md` 仍为空。共同生命周期本次读取 SteerLine HEAD `28d0a9500caf217d5e9804591173858b06bb0998`，worker-lifecycle.md SHA256 `B16D74F596D8720243C5C569B8E8AC47758970D96D8B0D2C03CC7D0FBB38808C`；只引用其 actor 单写、三层归属和 W14/W15/W16/W18 边界，不更新该仓库或将本次哈希替换原历史读取身份。

| 当前源码 | 已核实事实及本期设计结论 |
| --- | --- |
| `runstate/record.go`、`application/{results,feedback,worker}.go`、`workflow/{types,input}.go` | Record 持有 Workflow.StepID、Status/Phase、NextStage、真实 StageInputs/Results、Snapshot/Intents；finish 接收原生证据并调用 Route 后同次提交。新设计将 StepID 固定为唯一业务游标，另外字段作为历史/原生事实/对应载荷/公开投影，不能各自决定下一阶段；这是待实施约束，非当前代码已验证结论 |
| `execution/backend/workspace.go`、`workspace/{inspect,lifecycle,git}.go` | 已有 WorkspaceInspection 提供固定 repository/worktree/branch/base/HEAD/Dirty；Verify 核实 common dir、工作区、分支和 base 祖先，Inspect 用 `status --porcelain=v1 -z --untracked-files=all`。没有未提交内容快照/通用工作区版本系统。本期直接使用干净 pre_head/post_head，补读取稳定性与两提交祖先核验，Git 写入由正式执行 agent 完成 |
| `execution/publication/{provider,pr}.go`、`execution/evidence/pr.go` | provider 已有 HeadRevision 字段；ValidateWorkerPRIdentity 注释明确允许同分支 head SHA 前进，实际核验只比较 URL/repository/base/head 等身份。故审计版 PR.HeadRevision=post_head 是新增定义谓词，不能称原核验已证明审计版本；不需扩展 provider 查询端口，普通流程身份规则保持 |
| `workflow/{prompt,profile,result}.go`、`resources/assets/prompts.json` | 当前审计不修复是提示词/路由业务规则；现原生工作区权限事实沿此前调查。新定义的正式输入需允许修复并要求提交干净版本，parser/report 仍只能整理。未运行任何真实模型或写权限探针，不能据此宣称原生解析隔离/新审计闭环已通过 |

首期收敛选择属于设计：只做两个内置定义和三种获取能力，删除定制脚本端口/宿主及资格矩阵、DSL/插件/任意层级引擎和通用快照/字段编排。commit 级来源约束是为避免新增脏工作区快照；只读检查与 SQLite CAS 不构成 Git/文件系统事务，也不保证对任意外部写者的隔离。已记录原 post_head 后才能恢复比较；缺原定位不得以当前 HEAD 重绑。新测试入口和结果仍按 Plan 保持未实施、待验。

## 2026-10-17 本轮文档复审补充

主会话复审 Spec/Plan 与两份定义，未调用子 agent、真实模型、服务或产品测试。只读核对 `application/results.go:16–18`，finish 当前确实持有 e.mu；新增外部 Git 检查不能直接放进该临界区，未来需锁外冻结请求取证、回 actor 核源/授权/stop/CAS。现 WorkspaceInspection 只返回所观测 HEAD/Dirty 等，不构成“原 Turn 已处理此提交”的持久证明，更不包含本轮提出的 tree 差异或隐藏索引/子模块范围检查。

本轮修正均为待实现的设计：NextStage 限于真实提示输入；获取操作/等待复用原 Record，不建第二队列或 Git journal；原生与业务问题关联分开，业务未知不新增 outcome 枚举；原检查缺失不以模型 SHA/reflog/当前 HEAD 重绑；源 Turn 已结束即可解析而无须关闭原会话；同/空 tree 不支持 done 的正数修复；查询前后锁外复核并以 CAS 接受，只声明观测时点一致；重启从原 parser 资源事实恢复限额。完成来源条件不阻止部分/失败报告，已观测补报违约不能靠最终 HEAD 相同消除。具体问题与 T1–T8 覆盖见专属验收本轮段落，未将文档检查当产品通过，也不声称程序能独立证明每个修复的效果或排除任意外部写者。
