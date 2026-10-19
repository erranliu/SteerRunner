# Rebase、质量审计与影响范围复验（2026-10-18）

**结论：暂不建议合入，PR #25 保持 Draft。** 已接入主线、解决冲突并修正本次发现的确定性缺陷；原生资格失败和强杀恢复缺口尚未闭环。失败记录保留，受控通过不替代原生通过。未合并源 PR、未部署、未修改 SteerLine。

## Rebase 与审计范围

目标主线 `59c7597ac8056fc5c29ad622034e10e74e4395de`。原 head `2ebc17d` 的树和远端身份见 [before.json](checks/before.json)，备份分支为 `codex/workflow-result-acquisition-before-rebase-20261018`。先将原分支相对 `7c48969` 的最终差异整理为单个提交并核对原树，再 rebase 到最新 main；解决十处冲突。不是把合并提交重放后直接选择 ours/theirs。

冲突逐项保留主线新增的 Pi/DSH 原生终端能力、动态 agent 名、TerminalOwner/Job/exit 身份与认证停止证明、DSH 输入 framing；接入本次 workflow/2、来源化结果获取和原聚合提交。审计重点为唯一状态权威、审计修复后来源、原生失败优先、恢复不重发、资源停止及配置边界。

审查后的机制边界仍是：具体 definition 决定状态、提示词、契约与有限后备；统一执行层调度/核验/原子提交。`StepID` 是业务游标；获取 attempt 引用原 Intents，journal 只提供证据。无第二队列/状态服务/DSL。原审计 Turn 的 clean post-head/tree 绑定原实施结果，解析/补报不换源，发布对同一来源核验 Git/provider 并 CAS 提交。模型报告不能覆盖失败、未知副作用或 provider 事实。

## 本次发现与修正

| 问题 | 修正及证据 |
| --- | --- |
| rebase 中终端所有权、认证证明与 Codex-only 配置可能被覆盖 | 保留 Pi/DSH 原生所有权，非 Codex 拒绝 `github_auth`；Codex 配置 schema 要求 helper。定向测试、schema 与完整 L1 |
| coarse `Succeeded=true` 可掩盖明确 unknown outcome | unknown/failed/cancelled 不转为成功；[回归](checks/unknown-outcome.log) |
| 死的 FeedbackIntent 及未使用字段增加状态理解成本 | 删除无消费者的字段与分支；业务游标和原操作授权不新增副本 |
| readiness 测试把 1ms 窗口用于 workspace 初始化 | 在初始化后设置 100ms 就绪预算；仍验证拒绝输入/零发送；[20 次复验](checks/readiness-repeat.log) |
| L2 夹具错误复制 publisher，被原生预检拒绝 | 恢复按 L2/L3 选择模板；没有放宽预检；两项 L2 真机复验通过 |
| QA 固定要求恰好两个结果，误拒绝保留 needs_input 历史 | 按当前 ImplementationResultID/PublicationResultID 选择并验证唯一性、契约、原生源；历史结果不能冒充当前完成 |
| 本地主机清理被无关远端 omitted host 阻断 | 原 local host 必须出现且未被省略；保留 total/truncated/worktree/runtime 检查，远端范围仍严格。原始 [inventory](checks/local-cleanup-inventory.json) 与负例测试 |
| active-stop 沿用普通业务材料；改成长 Base64 后又被模型抄坏 | 独立 Spec/Plan，driver 提供固定父子脚本及 hash，执行短 `-File` 指令；不豁免真实 Job/PID 和晚到写检查。[脚本组件证明](script-proof/proof.json)只证明父子可启动，不能冒充完整 L3 通过 |
| Windows Pi 原生 settled 回执停在 `.tmp`，正式回执仍 started | [原始两份回执](diagnostics/pi-atomic-publication/6c65f6ba50583a00f79ecdcbc5ad59c7.json)及其同目录 `.tmp` 保留。最终修复仅对同一 fsynced 字节的原子 rename 在 Windows EPERM/EACCES/EBUSY 下限时 1 秒重试，不重新调用 agent，不消费 tmp 推进。真实 Node/Windows handle 的短占用恢复、持续占用 fail-closed [回归](checks/receipt-publication-regression.log)通过 |
| DSH final 加粗 PR URL，提取器保留 `**`，核验了错误地址 | 去掉 Markdown 强调尾缀再去重；保留不同 PR 歧义和 query/fragment 拒绝。源 final 原样保留，仍须 provider 核验。[原失败 summary/final](native-writers-final/orca-local.dsh/fe8e07936163/latest-run.json)、[修复后测试](checks/markdown-pr-regression.log)。未重写旧 Run 为通过 |

Windows 读者 FILE_SHARE_DELETE 的中间实验未解决实际 rename，相关实现已撤除；[失败实验日志](checks/receipt-reader-regression.log)保留。最终没有新增 OS 读文件抽象，也没有把文件 I/O 重试扩大为业务重试。

## 构建与测试身份

每个批次保留自己的 manifest、source-identity、Runner/helper SHA；**不会把较早构建的通过改标为最终构建通过**。逐文件原始路径和 SHA 见 [raw-origins.json](raw-origins.json)，最终源码文件摘要与复验命令见 [verification.json](verification.json)。

| 标记 | clean source | 与前一批的差异 |
| --- | --- | --- |
| A | `a96556b06410139b63c1c419543cf87783b29f7c` | rebase、原生身份接入、unknown 优先、清理反馈死字段与 readiness 测试 |
| B | `67640c4365c6174e7aa7808a5b5b777c4bced1a5` | 仅 github_auth schema 与测试 |
| C | `1c4b08088bb73516eb3cd8fb0b147c02cb199807` | 仅 QA 来源选择、local 清理和 L2 模板 |
| D | `36a1cc346b68660ecfa69d250b36fc6251cbecff` | 仅 active-stop 独立材料 |
| E | `4472afb` | 原生 receipt 原子发布限时重试与 Windows 回归 |
| F | `4bcfb8cc3bd669a1f6f79dc17c8e589b1f8394e4` | 仅 driver 固定脚本，替代模型拷贝 Base64 |
| G | `8a7e339c78951dde1be7d698c1a81de4479fe7c6` | 仅 PR URL Markdown 尾缀及其回归；没有新的原生闭环 |

环境为 Windows 11 / Go 1.26.4 / Node 24.13.0 / Codex 0.159.2 / Pi 0.87.1。helper B SHA `4C5892221AC260CC92D27702A040D210314F04CCF4FDC8BBB7DB9D8B66583B56`；helper F SHA `2CE2B2821B2938F51D92B51E4F147B6A2AC624C597F54A39F36108D04D9170A2`。两个 EXE 分开冻结，均未覆盖正在使用的 helper。

| 层级/范围 | 实际结果 | 身份与证据 |
| --- | --- | --- |
| A 完整 L1 + Controlled | 356 个 Go 根、28 Node；Controlled 16/16 通过 | [summary](full-final/summary.json)、[16 项日志](full-final/controlled/integration.log)、[构建身份](full-final/controlled/source-identity.json) |
| B schema 定向 Controlled | 1/1 通过 | [身份](schema-controlled/source-identity.json)、[日志](schema-controlled/integration.log) |
| C 测试框架 | 16 项通过；testkit 修正、native vet 通过 | [框架结果](framework-final/result.json)、[testkit](checks/native-assertion-fixes.log) |
| E 原生回执修复 | Windows/Node 两分支、整个 orcatransport、28 Node、native agent freeze 通过 | [回归](checks/receipt-publication-regression.log)、[包日志](checks/receipt-final-package.log)、[freeze](checks/receipt-final-freeze.log) |
| F 最新完整 L1 | **358 Go 根 + 28 Node 通过**；7 个仓库允许的非本层用例跳过；native driver 编译/零匹配守卫通过 | [summary](l1-latest/summary.json)、[原始 Go JSONL](l1-latest/l1.jsonl)、[Node TAP](l1-latest/terminal-extensions.tap) |
| G PR Markdown 修复 | 结果获取、workflow、application 三个受影响包全部通过，退出码 0；额外 RA 定向无缓存复验也通过 | [完整包日志](checks/markdown-pr-regression.log)、[定向日志](checks/markdown-pr-focused.log)；最终身份另记 verification |

第一次完整 L1 的 readiness 失败保存在 [初始 summary](full/summary.json)，修复后才重新执行完整批次。7 个允许跳过项的精确名称在最新 summary 内，不将其计为通过。

最初 native 调度被失败的 L1 前置门槛拒绝，未调用真机；随后才在 full-final 通过后用 run-native-final.ps1 执行 B 批次。相关临时调度脚本保留在 checks，参数与前置条件不依靠回忆复述。

G 完整包调用中 application 测试用时约 303 秒，其退出后父 go 进程仍延迟返回，曾准备按 10 分钟预算结束调用；实际在预算内正常返回 0，未执行中断。等待期间启动的无缓存 RA 定向复验也正常返回 0（application 约 215 秒）。不将父进程延迟推断为产品故障或缓存问题。

## 真机批次：保留失败与源差异

本次选取**全部两项 L2 和五项受影响 L3**，不是全量 L3。目前 catalog 的 L3 为 22 个 scenario / 29 cases，未执行部分不能从本轮推定通过。

| 批次 | 源/原生 helper | 结果 | 原因与后续 |
| --- | --- | --- | --- |
| [native](native/summary.json) | B/B | 2 通过、5 失败 | audit、mixed 通过；两项 L2 错模板；三终端碰到 QA 历史数量/local omitted 断言 |
| [native-retry](native-retry/summary.json) | C/B | 2 通过、3 失败 | **两项 L2 通过**；三终端业务闭环、restart/resume、原 PR 与反馈拒绝通过，但 active writer 未产生，整个 case 仍失败 |
| [native-writers-final](native-writers-final/summary.json) | D/B | 0 通过、3 失败 | Codex 长 Base64 指令抄坏；Pi 原子回执未提交；DSH Markdown PR 地址提取错误。分别修复，不倒改该批次 |
| [native-final](native-final/summary.json) | F/F | **0 通过、3 失败** | 三终端均在原生准备阶段失败，未发送业务输入，没有命中预设的“已接受响应丢失”故障点；不是 workflow 业务失败，也不能计为受影响路径通过 |

独立 parser 在 B/C 的真实内部 cases 有默认/显式模型、原生认证、语义结构、权限反例、取消停止证据；完整 L2 通过仅记在 C。B 的 audited case 检查审计来源、模型恢复、PR-ready 与停止，未合并测试 PR。mixed 检查双原生资源与一方停止后对端仍在，合法拒绝 milestone 后反馈。详见各 fixture 的 scenario、parser-cases、original-pr-ready、audit-models 与 cleanup 证据。

F 的 Codex 原生 [sandbox-readiness](diagnostics/codex-sandbox-readiness.json) 为 `verified:false, stage:network`，没有可用于输入的 ready 回执；[原 Run](native-final/orca-local.codex/138d8becafb6/lost-native-unverified.json) 显示 `remote_agent_preparation_failed:remote_agent_execution_failed`。Pi、DSH 各自停在原 create_resource/native preparation，业务输入为零。网络可达性、sandbox 权限或原生准备失败的具体原因尚未查清，不能将其认定为普通网络波动，也未关闭资格检查、放宽权限或重发未知输入。

F 之后的 Markdown URL 修复只做受影响包回归，没有新的完整原生闭环。不能拿较早部分路径通过，拼成最终全部通过。

## 停止、证据与合入门槛

本次 18 个 fixture 的公共 `service status` 均为 `not_running`，实际启动服务的 16 个 fixture 的原 shutdown 均确认 `resources_settled:true`；两项最初 L2 预检失败未启动服务，记不可适用，不能伪造资源结算。见 [cleanup-summary](cleanup-summary.json) 和各 fixture 的原 shutdown-calls/final status。脚本组件实验只操作自己创建并验证身份的父子进程，均已停止。未清理或重试其他工作树的资源。

测试 PR #533–#547 的 [列表](created-test-prs.json) 保留，是否合并按各原生用例定义；失败/审计用例的 PR 不擅自补发或合并。PR #25 仍为 Draft。

合入前还需要：

1. 查清并恢复当前原生准备资格，在相同安全策略下，用最终代码与 helper 重跑三终端受影响 L3，覆盖真正的原提交响应丢失、原会话 restart/resume、active writer/子进程停止和无晚到写。
2. 明确历史 `service-restart/kill-active` 缺口的合入处置：[上轮完整范围记录](../full-l2-l3-20261018/README.md)。原 terminal.create 返回的资源身份未持久化便强杀，当前 Orca 无 caller-idempotent create 或精确 recover query；空 inventory 不能排除晚创建。保持 outcome_unknown、禁止标题猜测/替代创建的行为正确，但自动收敛尚无证明，不能标 resources_settled。该缺口也存在于主线，不因 rebase 自动解决。
3. 管理员原生场景的独立 host 仍未配置，不声称完整 L3 全绿。未为通过测试扩展本次 workflow 重构成新的资源恢复平台。

从结构看，本次没有再引入通用策略平台或重复状态权威，已发现的实现与 QA 缺陷均做了窄修复；从交付看，当前原生证据仍不足以放行。架构方向成立与当前版本适合合入是两个分别判断的结论。
