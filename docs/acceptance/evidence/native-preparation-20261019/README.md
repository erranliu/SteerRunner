# 原生准备阻塞定位与修复（2026-10-19）

本轮直接修复执行准备中的匿名配额依赖及 Pi/DSH 工具初始化失败诊断，并复验受影响路径。未修改 SteerLine，未部署、未合并源 PR；既有未知资源归属规则继续保留。

## 定位依据与确定性

旧批次只能证明 Codex 在 `network` 阶段失败，没有保存底层异常或 HTTP 状态，不能倒推出当时一定是 403/限额或 TLS 故障。本轮修改前的相同版本/权限/认证链路只读真机探针通过，说明该阻塞并非持续不可用。

源码发现原准备脚本在实际认证客户端之前，额外执行未携带认证头的 `Invoke-WebRequest https://api.github.com/meta`。本轮实际匿名响应的 `X-RateLimit-Limit=60`、`Remaining=56`、`Resource=core` 见 [配额响应](checks/unauthenticated-meta-quota.json)。它依赖共享 IP 的独立配额；高频测试或其它调用者可使该探针失败，即使已认证仓库访问仍然可用。该错误依赖是已确认的实现问题；旧现场恰好是否因此失败仍不可证。

Pi/DSH 的公共 `prepareTerminalTools` 在探针拒绝时抛出异常；插件管理器可能继续保留 TUI，而尚无 readiness，Runner 只看到等待。本轮为这一具体工具准备路径补足有来源的失败事实，不承诺任意配置/模块/SDK 错误都能产生这份回执。

## 最终修正与职责边界

1. 删除额外匿名 `/meta` 探针。相同 native sandbox、never、固定工作区/Git 根、原生 token 边界、凭据摘要、remote 身份与 helper 策略保持；真实的已认证 `gh` 身份/仓库查询、Git `ls-remote` 仍必须通过。没有降低 TLS/证书检查、扩大权限、增加模型重试或把 host 查询冒充 sandbox 工具事实。
2. 公共工具准备失败时，用原原子回执写入器在所属执行端证据目录保存 `preparation-failure.json`（内部 version1）。字段仅含 Run、会话、bundle SHA、worker PID、固定错误码与白名单 probe_stage，不保存异常正文、stdout/stderr 或环境。仍抛出原异常，失败后不执行后续核验 canary 或业务工作。
3. Runner 仅在初始化尚无原 readiness/owner/launch/turns 且 readiness 文件缺失时读取该事实；核对原 owner 的 runtime/config/bundle、Run/会话及 worker PID。它只解释初始化失败，不授予就绪、业务成功或资源结算；活动会话缺失证据不能被旧启动失败覆盖。原 Record/Intents/原子提交/停止和 unknown 语义不变，未新建工作流状态、队列或恢复服务。
4. 真实无效凭据负例原先在预期准备拒绝后，仍对已关闭 native client 发起额外 Git 探针，导致测试自身失败。本轮把这些追加探针限制为已合格 host-broker 分支；无效凭据仍须拒绝、零业务 thread/input 并停止原进程。独立 loose/packed Git 原生验证也保留。

CLI v1、Run state/v3、Orca profile/state 及 Workflow/2 的公开版本和格式不变。新文件是原执行端目录中的观察证据，不是第二状态权威，也不是 agent 报告轮；不改变三种结果获取方式、原始 final 或审计修复来源绑定。独立 parser 的配置/认证/模型选择及 SteerLine 接口均未改变。

## 源码与构建身份

修复前 head `94fcafd62c2f23bd17ecb3da8c474b132f1931f9`，本轮最终代码 `361589fdd890ed0bb1edb5cbc4be8cfea50e486a`，主线仍为 `59c7597`。

| 源码 | 用途/区别 |
| --- | --- |
| `3bba05d` | 初始化失败事实与临时 network 异常分类实验；只读真机分类探针通过。随后发现匿名配额依赖，最终不保留这套多余分类。其完整入口在 L1 期间、模型调用前中断，见 [中断事实](checks/interrupted-prerequisite.json) 和 [原 summary](interrupted-prerequisite/summary.json)，不计为完成验收 |
| `8f5f21c` | 删除匿名依赖，保留真实认证客户端；host-broker 真机通过，无效凭据负例暴露上述测试复用关闭连接的问题 |
| `361589f` | 修正负例。最终正反例、完整 L1/Controlled 和本轮原生用例均使用此源码 |

最终 helper SHA `128695A6788848CCFA3F67708289D5F179876D4A1C5AAD8B73A0863716FFD62E`，bundle SHA `0b2e7c5ea62a47dd52a9452281d61af6d29622d5548cc2d1643cea6ac457eb5c`，见 [构建记录](checks/helper-build-qualified.json)。中间 helper/source3bba 单独保留，未覆盖。各场景 manifest/source-identity/Runner build 文件保留自己的身份，不跨构建拼通过。

环境：Windows/amd64、Go 1.26.4、Node 24.13.0、Codex 0.159.2、Pi 0.87.1、DSH 0.2.1-rc.2；原机已有认证，无新增账号或凭据复制。本轮 Source/文件摘要、命令、时点和验收索引见 [verification](verification.json) 与 [hashes](hashes.json)。

## 验证范围与结果

真实模型之前执行完整 L1/Controlled；两项 L2 顺序复用同一 clean source 的成功前置，保持真机场景并行上限 2。不是全量 L3（当前 22 scenarios / 29 cases），没有新增庞大矩阵或取消原停止/身份断言。

| 层级 | 场景与通过条件 | 本轮结果/证据 |
| --- | --- | --- |
| L1 / Node | 有效/错 Run、会话、bundle、PID 和未知错误；缺回执仍 pending；旧初始化事实不覆盖活动会话；原子失败事实不泄露原输出/环境，失败后无后续工具工作 | 完整 360 Go 根、29 Node 通过，7 个允许的非本层/helper 项跳过；native compile/零匹配守卫、框架通过。[summary](live-qualified/summary.json)、[原 Go JSONL](live-qualified/l1.jsonl)、[Node TAP](live-qualified/terminal-extensions.tap) |
| 原生只读 P0 | 实际 sandbox 中有效认证、GraphQL 只读/dry-run、Git、兄弟目录拒写；无效凭据拒绝且不创建业务 thread/input；原进程/broker 停止 | 最终正反例通过。[有效](checks/native-qualified-host-broker.log)、[拒绝](checks/native-qualified-invalid-canary.log)、native-probes 下原回执/cleanup |
| Controlled | 全部 16 顶层，包括结果获取 8 个子路径、来源/报告/解析/重启、原输入/停止/恢复/发布核验/多 Run | 16/16 通过。[日志](live-qualified/controlled/integration.log)、[身份](live-qualified/controlled/source-identity.json) |
| L3 | Codex、Pi、DSH 各自的真实 needs_input/reply、丢接受回执只对原请求对账、原资源重启恢复、发布/provider 核验、里程碑反馈拒绝、真实父子 writer/Job、停止后无晚到写及原资源/服务清理 | 三项全部通过；最终模型与 helper 同源；逐 fixture 原证据如下 |
| L2 | 完整两项 local-codex.smoke / local-codex.result-acquisition；原生 Git loose/packed；独立 parser 的默认/显式模型、权限反例/取消及业务 smoke | 两项全部通过；见 [summary](live-l2/summary.json)、[Git](live-l2/l2-git-boundary.log) |

| 场景 | 现场 / 原 Run | 测试 PR / 合并 | 原结果 |
| --- | --- | --- | --- |
| orca-local.codex | `b0106f290a5e` / `882a957d37149a0de345d32884e52f86` | [549](https://github.com/erranliu/SteerlineTest/pull/549) / `ab1d71c8923a` | [scenario](live-qualified/orca-local.codex/b0106f290a5e/scenario.json)、[native.log](live-qualified/orca-local.codex/b0106f290a5e/native.log) |
| orca-local.pi | `f09f72df0eb8` / `de40abfbc9b2fea69d36eecc0d4bac87` | [548](https://github.com/erranliu/SteerlineTest/pull/548) / `52bbc9fcc662` | [scenario](live-qualified/orca-local.pi/f09f72df0eb8/scenario.json)、[native.log](live-qualified/orca-local.pi/f09f72df0eb8/native.log) |
| orca-local.dsh | `e115bfecb7b8` / `8f295d86d37e1eb8907151b70d245600` | [550](https://github.com/erranliu/SteerlineTest/pull/550) / `8d1c4ba3a057` | [scenario](live-qualified/orca-local.dsh/e115bfecb7b8/scenario.json)、[native.log](live-qualified/orca-local.dsh/e115bfecb7b8/native.log) |
| local-codex.smoke | `84fc08bff39c` / `d45f05126379c0713b5fa8cb913a7e09` | [552](https://github.com/erranliu/SteerlineTest/pull/552) / `3dd71a5d7513` | [scenario](live-l2/local-codex.smoke/84fc08bff39c/scenario.json)、[native.log](live-l2/local-codex.smoke/84fc08bff39c/native.log) |
| local-codex.result-acquisition | `448b2ef40ccb` / `91cc3c7c8618b5d3d6a716b0124acacb` | [551](https://github.com/erranliu/SteerlineTest/pull/551) / `015dd50d7ce7` | [scenario](live-l2/local-codex.result-acquisition/448b2ef40ccb/scenario.json)、[native.log](live-l2/local-codex.result-acquisition/448b2ef40ccb/native.log) |

实际 Codex 路径保留 needs_input 历史，实施结果使用单次原 agent 报告后备，发布结果消费程序线索并核验 provider；其它场景按实际 marker/报告选择取证，不强制统一三级流水线。独立 parser 的实际模型/认证/停止结果只从 L2 parser-cases 记录认定，不据运行环境或模型列表推定任意模型可用。本轮 parser 的 observed_model 未报告，明确记为不可得；显式请求 gpt-6-luna 的本次成功不推定其它模型有权。

## 停止、剩余缺口与合入判断

本轮模型用例、测试 PR、原 Run/服务停止及 resources_settled 独立核对，详见 [清理摘要](cleanup-summary.json)、逐 fixture shutdown-calls/final public status 与 [测试 PR](created-test-prs.json)。测试 PR 的合并只由原驱动按用例执行；未合并源 PR25、未部署或修改 SteerLine。

准备阶段误拦截已修正并完成本轮实际路径复验。先前 `service-restart/kill-active` 的原 terminal.create 身份持久化窗口仍未解决：[旧完整范围记录](../full-l2-l3-20261018/README.md)。现用公开 CLI 的 [create](checks/terminal-create-help.txt) 无 caller 请求 ID/按 ID 读取原创建结果；[send](checks/terminal-send-help.txt) 的 retry-request 不能用于恢复创建。保存 intent 后、返回 handle/incarnation 尚未提交便强杀，空 inventory 仍不能排除迟到创建。

不能在 Runner 里按标题补认、因查无资源就重发、伪报 resources_settled，或以新状态/恢复平台掩盖原生保证缺失。最小原生补齐应是在创建前绑定稳定请求 ID 与所属工作区 generation，并允许只读查询原 handle/incarnation 或明确的未决状态；Runner 继续引用原 Intent，不新增另一套操作权威。本轮没有修改 Orca 仓库、升级宿主或伪造这项能力。

**合入结论仍保持 Draft，直到这条已知基线缺口有明确处置。** 本轮五项受影响原生流程通过不等于完整 L3/全部强杀恢复承诺已通过，缺管理员宿主的其它原生场景也未在本轮宣称通过。失败历史保留，不因新成功改写旧 Run/批次。
