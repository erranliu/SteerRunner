# 原生目录查询与调用方边界审计及验收

关联 [Spec](../spec/native-catalog-boundary.spec.md) 与 [Plan](../spec/native-catalog-boundary.plan.md)。2026-11-05 在基线 `e7876c0d` 审计，R1–R3 已实施于独立 `codex/steerline-boundary-spec-plan` worktree，约定自动验收通过。独立只读审核覆盖协议/清理、提示词资产与测试选择，确认的问题已修正。全部运行证据位于仓库外 `C:/SteerlineDebug/wt-4ba6-boundary/runner/`，以下按实际范围登记，未修改安装版。

## 设计审计及修正

| 发现 | 源码证据 | 修正 |
| --- | --- | --- |
| 原建议未充分利用已经抽出的 Orca 能力层 | protocol/orca 的 state_config/state_local/state_stream 已负责原生目标与连接 | 在原层补有限目录操作，CLI 编排脱离 Run/Workflow |
| 策略槽位其实已经公开 | resources.ModelInputContract → workflow.InputContract → project_policies.slots | 调用方直接读取既有字段，Runner 补说明与回归，取消新目录设计 |
| 原发现缓存依赖 SteerLine 所持私有配对上下文 | SteerLine web/server/discovery.Query 把 env.pairingKey/local.identity 纳入缓存键 | 迁移后的查询取消跨请求新鲜缓存，保留在途合并、原生来源和过期展示 |
| 目录注册与支持资格存在混淆风险 | 现行 Runner 已退役 DSH 执行，原生目录可能仍返回它 | 原样表达发现事实，执行支持由独立 capability_catalog 判定 |
| 完整执行 adapter 初始化包含额外职责 | Codex codexapp.Client 引用 execution/model，Orca Lifecycle 持有执行 owner | 只复用窄协议/进程能力，按实际依赖提取最小部分 |
| 常驻 Client 参数会破坏无服务边界 | SteerLine runnerclient.Client.Call 默认带 state-dir 和部分实例检查 | 调用方使用无服务查询路径，公开 CLI 在 host.Normalize 前分派 |

## 自动验收矩阵

| ID | 行为及反例 | 状态 |
| --- | --- | --- |
| RNC-A01 | 严格单 JSON、重复键/未知字段/超限拒绝；离线 capability 自包含；查询响应/退出码同源 | 通过：contracts/resources gate/schema 对照，public-cli-second |
| RNC-A02 | 本机/远端共用目录读取，明确 runtime/目标；错误配对、未知目标、失联、协议不足及隐式路由拒绝 | 通过：迁移的本机 14 模式、加密远端 5 模式及安全 metadata 反例 |
| RNC-A03 | 空/重复 ID、effort、分页、listing、来源时间、完整空目录、未实现执行的已注册 Agent | 通过：原生组件矩阵及 Codex model/list 矩阵 |
| RNC-A04 | 无服务/空 home 可查询，零状态目录/DB/Run/Agent/终端/worktree/配对动作；非法在线参数先拒绝 | 通过：公开 EXE 受控专项，分派早于 host.Normalize |
| RNC-A05 | Codex 仅 initialize/model/list，hidden/分页/版本/原始默认准确；零 thread/turn | 通过：迁移的分页/隐藏/重复/断开/超限夹具、真实只读目录 |
| RNC-A06 | 30 秒总限额、1 MiB、取消/断开/父进程强杀与五秒清理；原业务进程不受影响，清理失败不报成功 | 通过：组件及最终完整 Controlled 的公开目录 CLI，精确 child PID/Job 退出 |
| RNC-A07 | 正式 L1/Controlled，受影响协议/进程 owner，原请求/停止/Workflow 行为保持 | 通过：原指纹 L1 853 根/21 扩展，后置 owner 检查及最终完整 Controlled 28 根，验证边界见下文 |
| RNC-A08 | 既有公开 slots 与唯一生产定义、顺序及请求校验一致，显式空数组和默认路径/路由有证据 | 通过：原 input_contract/project_policies owner 回归及 SteerLine 正常入口字节对照 |
| RNC-A09 | 固定最终 EXE 的真实本机 Orca、明确远端 Orca、原生 Codex 无模型目录查询分别取证 | 通过：live-native-final 六项 ready，与最终 EXE→Web/零项目页面同构建 |

RNC-A09 为可自动执行的真实依赖联验，条件缺失时保留待验目标。目录查询无需真人主观判断，未新增人工必验项。日志不保留认证正文或原始外部 stderr。

## 实施与组件证据

`internal/contracts` 定义公开纯值/严格 gate；`internal/resources` 发布自包含 schema。`internal/cli` 在 state-dir/host/service 前完成无服务分派，固定 SHA256/version/resource_digest，安全错误码不回显原生 stderr/凭据。`protocol/orca` 共用既有连接认证并增加有限目录操作，独立于执行 Lifecycle；`nativecatalog` 只编排目录；`metadataprocess` 从现有 textanalysis 进程 owner 提取 Windows 挂起启动+Job，原调用保留等价关闭接口。

原 SteerLine Codex/Orca 生产协议和夹具迁入 Runner，保留分页、空目录、努力档位、listing、缓存时间、认证、协议不足、错误目标、断开和超限。新增固定用户目录/metadata 安全读取、相对 cwd/PATH 排除。旧 textanalysis 与新元数据 owner 在原包测试回归，不引入完整执行 adapter 初始化。

`TestSRNativeCatalogPublicCLI` 已登记至完整 Controlled 与 foundation 归属，完整根由 27 增至 28，原 27 项未删除。公开 EXE 专项 `public-cli-second/result.json` 通过，覆盖无服务/零状态副作用、非法在线参数、源 SHA 与凭据不泄露、真实子进程 model/list。强杀父 CLI 时先打开精确 child PID handle，再等待 Windows Job 关闭子树；不按进程名批量结束其它业务进程。helpers/native 仅补标准 app-server stdin 方式与目录夹具、opt-in 挂起/PID 证明。

## 正式基线、历史失败与独立审核

`formal-first` 在框架当前完整根数量仍硬编码 27 时阻断；同步 catalog、framework 和测试目录至 28 后复跑。`formal-second` 完整 L1 中 application 包通过（1159.056 秒，覆盖率 80.9%），唯一实际失败为历史 `TestSRNativePreparationReferencesAreStaticConstraints/absolute-uninstalled` 把退役 helper 当原生 Codex 正例；生产当前 gate 正确拒绝。

最终只修改该测试夹具为 native driver：绝对不存在 transport/no helper 可用、相对 transport/no helper 不可用、绝对及相对 retired helper 都不可用。直接断言生产 ValidateRunConfiguration 接受无 helper 并以 unsupported_capability/exit3 拒绝 helper；测试专用可执行路径检查另断言。生产 gate 和 assessCase 未改，定向复验通过。早期尝试改 Pi 仍为结构化 Agent，不满足 gate，已撤回；独立 reviewer 撤回该建议并批准最终四例。

`formal-third` 保留两项失败：原 `TestR2SlowEvidenceDoesNotBlockOtherRunControl/workspace-git` 在释放 I/O 屏障后的返回等待超过原 30 秒（该用例其它 Run 控制/持久化断言已过），随后 application 整包在 `TestSharedNativeEngineStopReconciliation/paired-worker/wrong-runtime` 子例仅运行约 6 秒时触及 1200.59 秒总上限。原 evidence 用例在 `evidence-latency-recheck/test.jsonl` 定向 9.825 秒通过，不改其 30 秒断言；基于先前整包 1159.076 秒通过、当前仍连续完成子例的证据，经独立审核仅将 Go 包级聚合 watchdog 20→30 分钟。原包集合、覆盖率、skip 白名单、产品/单用例期限不变。全部最终输入冻结后，以 `formal-final` 新目录完整复跑 L1,Controlled，不把第三轮记为通过。

文档复核发现 hidden/重复/空 ID 和 effort 原反例缺项后，补充 Codex hidden 行过滤、includeHidden=false、原默认 model/effort，以及四个身份失败模式；失败断言要求 context 未超时且得到原校验错误，不能由断开/期限冒充。Orca 共用 `readCatalog` 增加重复/空 Agent、模型、effort 六例，按原响应调用数确认拒绝位置。先用独立 Go overlay 验证，再于第三轮结束后落盘，`catalog-identities-final/test.jsonl` 两包通过；独立 reviewer 已审查并批准，生产代码无更改。

独立审核要求并确认修正三处：Orca 第二个 listing 响应重置旧标志、公开 schema 与 local 空 environment/缺失 adapter gate 一致、调用方配置变化拒绝旧 ready 数组。还审查新增 metadata/PATH 矩阵、精确 Job 清理、28 根完整选择及零项目停服接线。L1 原有 16 个显式 opt-in/helper skip 按既有白名单登记，不算真实目标资格；未新增 skip。Controlled 要求完整 28 根非零执行且不跳过。

`formal-final` 的正式 L1 通过：853 个根测试、21 个扩展，原 16 个允许 skip，native compile/zero-match guard 通过；application 包 1625.679 秒、覆盖率 80.9%。其完整 Controlled 实际执行 28 根，失败仅在两组历史 terminal 夹具：`TestSROrcaCLIWaitRetry` 的 7 个旧 scope 断言、`TestSRResourceCreationRecovery` 的 5 个旧 terminal session/故障门断言；其它 26 根及新增公开目录 CLI 通过。整份 formal-final 仍为 failed，未覆盖或改写。

经独立审核只修测试和 helper：排队精确要求当前 native Codex 的 `orca_agent_session`/`orca_agent_session_v1`，保留七个排队、重启、CAS、撤销和零调用反例。恢复七场对应当前 structured workspace；receipt-before-commit 的真实故障门放在 worktree show；缺原计划负例删除原 CreationOwnerConfigFile；缺 create terminal 字段的正例不依赖 legacy initial-terminal 计划。四个可恢复场景保持原 worktree/branch、原 prepare_workspace ID/action/confirmed、空业务 session、TurnStarted=false，并核实唯一 close 指向原辅助 shell handle 且 marker 已写；原三种创建未知场景保持 Workspace/Session 空、禁止输入、零关闭/发现/重建。CLI 业务发送为零，另记录 RPC method-only 日志，白名单只允许三个只读方法，禁止业务 session create/send/close。

早期适配额外假定 public AttemptID 准备前后相等，诊断后撤回此非原契约约束，改核对原准备操作 ID。独立 reviewer 一度建议辅助终端 close0，后查明完整原工作区 inventory 会绑定并关闭已核实的辅助 shell，撤回该建议；正常四场精确 close1、未知三场 close0，业务 agentSession 仍零操作。失败 overlay 和逐字段诊断全部保留。`queue-overlay/result.json` 七变体通过；强化 RPC 审计后的 queue 根也通过；最终 `creation-overlay-corrected/result.json` 七场通过。

最后规整 resources.go 唯一新增 map 项的缩进，并执行 `post-format-owner/test.jsonl` 的 resources/CLI 两包完整回归，通过。`post-l1-source-delta.json` 精确确认正式 L1 后只有该一行无 token 变化及三份集成测试/helper 修改；原 L1 指纹 `3672fb6e21de6db5c88987f27ae955c1a33cc4ec10ef44d15367b77594276dcd` 保留，不声称新指纹可复用旧前置资格。经独立审核不因无语义缩进重复整包 L1，以官方 `test-foundation.ps1` 从原 catalog 动态选择全部 28 根，在 `controlled-reviewed-final` 完整复跑最终受控集合。最终 **exit0、28/28 根通过、零失败、零 skip**；Pattern 与当前 catalog 逐字符一致，七个排队和七个恢复窗口全部通过，新增公开目录 CLI 含精确子 PID/Job 清理通过。完成时间为 2026-11-05 16:31:51 +08:00。外层 `../final-verification.json` 汇总两仓正式结果、实际根数、同 EXE 摘要及真实页面清理，不替换原报告。

## 提示词和资产

原模型可见提示词及 Workflow 正文无修改，实际全文差异为空；没有黄金样本更新。新增 `internal/resources/assets/native-catalog.schema.json` UTF-8 **0→11039 字节**，用于公开只读查询 schema，与模型正文隔离。对应该资产的 CLI/capabilities/fixture 与测试选择变更已独立审核。

既有 `model_input_contract.workflow.project_policies.slots` 的 PolicySlots 唯一生产定义未变。SteerLine 从正常 PrepareTestPlan 入口比较旧/新完整 Run 输入 **2706→2706 字节、逐字节一致**；四份 PromptOverrides 分别 **830→830、312→312、372→372、319→319**，全文差异为空。业务原因仅为 caller 改读同源槽位目录，预检仍由 caller 负责。完整对照日志在相邻 `../diagnostics/policy-byte-comparison/go-test.jsonl`（相对证据根）及 SteerLine 专属验收。

## 最终构建与真实无模型联验

固定 EXE=`reviewed-final-build/steerrunner.exe`，version=`0.2.0-dev.4`，base commit=`e7876c0dcbfbbc87a92543a66fdf14fb1de84834`，dirty=true，source_digest=`3b0dd992f94a37465c2f749ffd4b6d36b822edb63567e4dd416e77447c857d3f`，SHA256=`87d23fecaeccf253bb9920f0d6124be66cb1ed671e18dfd79160ecca8120e742`，resource_digest=`d575984a2d64b37a5325edf5abe8313a1c8ab7c98f8ba14adf6daf4e0aa124bb`。测试补充和最终 map 缩进进入构建来源清单，故重新构建并刷新 source_digest；实测 EXE 字节与已做真实联验的 `final-build` 及 `final-qualified-build` SHA 完全相同。以下证据按相同固定字节复用；最终文档修改不属于生产构建或测试前置输入集合。

`live-native-final/summary.json` 全部 ready/exit0：登记 2 环境、本机 Orca agents=8/models=8、远端 `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8` agents=7/models=8、本机 Codex models=8。Orca `1.4.246`/protocol3，Codex `0.166.0-alpha.6`；单次 0.081–3.826 秒。原始公开响应保存 ID、effort、来源及 runtime，未保存认证正文或原始外部 stderr。早期另一个远端 `9cf377a1-42de-4f2b-8874-70f63a7bbab2` 返回 unavailable，原因未确证，不计通过。

相同最终 EXE 的 SteerLine `diagnostics/published-runner-web-final-matched` 真实接线和 `live-page-final` 正式零项目页面通过。页面浏览器 1/1（19.6 秒），覆盖本机 Codex/Orca 与指定远端；stop_exit=0/cleanup_exit=0。仅目录查询，不推导模型、额度、项目、全 Agent 或 PR 执行资格。

## 文档验证

六份修改文档的 185 条本地链接全部有效；21 份新增文件 UTF-8、末尾换行及逐行空白检查通过，`git diff --check` 通过，全部修改 Go 文件经 gofmt 核对。静态验证不代替正式基线与真实目标结果。

交付前按 AGENTS 再次刷新 origin，`origin/main=f8bd932b49ac1d8adb7dd9bb865b91e5baba9f8d` 比本次固定基线多三个提交；唯一同改文件为 state_stream.go。独立临时文件中的 `git merge-file` 三方组合 exit0，同时保留本次 catalog 分支和上游 hostID 提取，没有文本冲突（`target-refresh.json`）。本次保持已选基线 worktree，不合入额外 Workflow/提示词变更；当前没有 PR，不宣称 GitHub 已合并或合并后运行资格。

## PR 发布记录

用户另行授权提交、推送和创建 PR 后，分支 `codex/steerline-boundary-spec-plan` 已提交实现 `742c9b3a`，推送并创建 [PR #117](https://github.com/erranliu/SteerRunner/pull/117)。创建前再次刷新目标 `main=6940c59872d2b857a97b97b0f98090493379a219`；GitHub 原始接口返回 mergeable=true、mergeable_state=clean。以上为本次分支发布状态，不表示已合并，也不将历史固定构建资格改写为提交后或合并后新 EXE 的资格。

配套 SteerLine 为解决其上游部署根清单组合而接入 `main=c94cc5aa`，其提示词正文相对该 main 没有 PR 差异。新的正常 PrepareTestPlan 字节对照为完整 Run **1796→1796**、implement.execute **167→167**、其余三段 **312→312 / 372→372 / 319→319**，全文相等；上文 2706/830 为原固定基线历史证据，未被覆盖。配套主干接线和测试清单合并另经独立只读审核，实际新结果在其专属验收登记。
