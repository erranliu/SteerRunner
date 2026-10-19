# Spec：无人执行权限与 Git 环境检查能力

日期：2026-10-16。状态：已于本会话获用户授权实施；实现与验收状态见专属验收记录。Runner 独立前置；[Plan](environment-check.plan.md) · [专属验收](../acceptance/environment-check.md)。本文件是 Runner 权限模式、检查输入/结果和执行行为的权威；SteerLine 只消费公开契约，不另行定义这些规则。

## 1. 范围与调查基线

提供主动环境检查：在指定目标上，准备与普通执行相同权限的 Worker，让实际 Agent 调用模型和工具，验证工作区、Git 及必需 PR 发布能力，输出可核验报告。检查不接收 Spec/Plan、Task 或 development workflow，不做任务交付。构建、测试和业务资源留待项目定制用例；本期不建插件或通用资源声明框架。

读取的本仓主 checkout 为 `C:/Project/SteerRunner`，HEAD `e04d07fde31710b5b93d3186010074927a44effc`，检查前干净。已调查 `internal/application/{engine,freeze,preparation}.go`、`runstate/record.go`、`contracts/`、`cli/cli.go`、`resources/assets/settings.schema.json`、Codex/Orca adapter、`docs/testing.md`：当前 Run 聚合与 development 流程耦合；原请求、Operation、原生生命周期和服务停服结算可复用，没有本规格的独立检查契约。

另已读工作树 `8876/SteerRunner` 同 HEAD 上未提交的 `sandbox-authentication.spec.md` / `.plan.md`、Codex `authentication.go` / `sandbox_readiness.go`、Orca `authentication_test.go`。它们不是主干已交付能力；实现前须记录最终提交/构建和验收状态。本次不编辑该工作树，不复制它的未提交代码或凭据。

跨仓共同 W/E/V 语义沿 SteerLine 的 `docs/product-design/worker-lifecycle.md`；该文档的运行/恢复规则继续适用，本文件仅定义本功能增量。独立 Runner 执行不要求 SteerLine 文件存在。

## 2. 与沙箱认证工作的交接

| 现有沙箱认证项 | 本功能的处理 |
| --- | --- |
| SA01/SA03：执行主机准备单仓库短期认证、隔离继承、到期/撤销 | 由原认证 owner 维护，检查与普通执行复用同一来源/策略。统一使用执行主机已有 gh 登录的单仓库 broker，缺登录阻断；旧 github_auth（含 null）拒绝，不静默切换身份。不把宿主凭据复制进沙箱 |
| SA02：首个业务输入前，同 app-server / 权限 / 根目录的确定性 `command/exec` 预检 | 继续是不调用模型的准备门槛。主动环境检查在该门槛之后增加 Agent 实测；不把额外模型检查插入每次普通业务启动 |
| SA04/SA08：workspace-write/MXC/never、零提权/审批/bypass | 保留为 sandbox 分支的规则与验收口径。显式 full_access 是新分支，不作为 sandbox 失败回退，不把其结果记为 SA04/SA08 通过 |
| SA05：只读预检不证明 push/PR | 保持；本功能以本次专属引用/PR 实写补充能力证据，不借用历史业务发布通过 |
| SA07：远端必须有执行主机认证及同权限确定性预检，缺能力为 unsupported | 指定执行主机 helper 承载原认证 owner、同权限预检及有限环境检查会话；调用方只交换原操作的绑定证据。未配置 helper 仍 unsupported。Agent 模型回复或普通 shell 成功均不能解除 SA07；full_access 也不能绕过远端身份/认证来源/实际工具上下文的证据要求 |

本规格不能单方面把在途 SA 文档变成已完成前置。实施前先在其最终版本明确“沙箱专项仅约束 sandbox 分支”；若仍声明全产品禁止 full_access，则冲突尚未解决，不能合入本功能的双模式启动变更。认证机制仅依赖本功能必需的准备能力，不以整套 SA 业务闭环全部完成作为 UI/契约设计前置。

### 2.1 前置的执行时点

| 时点 / 目标 | 必须满足的前置 | 未满足时 |
| --- | --- | --- |
| 规格、公开 fixture、确定性/Controlled 开发 | 固定拟议契约版本及 SA 接口假设，记录最终整合依赖 | 可做隔离开发，不记真实支持；不从在途工作树拷贝秘密或宣称已合入 |
| 任一模式的真实发布/SteerLine 集成 | 所复用 SA 认证/准备最终提交已合入，其自身交付门槛已满足；固定 Runner 构建、策略/schema/resource 摘要及支持组合 | 不能以本功能局部测试代替 SA 的交付要求或将 dirty EXE 作为已发布前置 |
| 本机 sandbox 首个 Agent 检查输入前 | 同一实际进程/账户/根目录/权限的 SA01–SA04 认证及无模型确定性预检有效，过期/撤销规则适用 | 接受后的检查保存明确准备失败，零模型输入；历史 ready 或无效 token 负例不是有效认证正例 |
| 本机 full_access 首个 Agent 检查输入前 | 复用已发布的 SA01/SA03 认证 owner；由本功能新增与实际 full_access 匹配的无模型确定性预检和权限证据 | 不要求伪造 MXC/AppContainer 事实，不把 SA02/SA04 的沙箱通过记到全访问；缺来源/同上下文事实仍阻断 |
| Orca 两模式首个 Agent 检查输入前 | 额外满足 SA07：执行主机解析认证、与实际模式一致的确定性预检及原生工具/操作关联能力 | 尚未实现原生端口时该组合 unsupported；已实现但现场凭据/资源缺失时是本次检查失败或未核实，不改写为“不支持” |

本次读取的 SA 专属验收仍记录：有效真实凭据正例未验、SA05 L2 smoke 因缺认证引用阻断，且要求有效凭据下 L2 smoke 通过后才交付其 PR。这是当前实际前置，不因本功能另建 Spec 而豁免；后续以其固定最终提交和验收为准。full_access 的 SA02 等价准备由本功能实现和验证，不是要求 SA 先新增全访问，从而避免循环前置。SA 的原 L2/L3/历史结果仍归原任务，本功能只登记引用及适用范围。

## 3. 权限模式

Runner 在公开 `execution.settings.access_mode` 声明 `sandbox` / `full_access`，缺省 sandbox。检查和普通执行复用同一配置编译、冻结、权限落实和恢复核验。Codex 两模式均为原生 never，不挂 auto_review，不允许运行中申请越界/提权；原始 args 中冲突覆盖在创建资源前拒绝。

- sandbox：使用 Worker 已准备的沙箱、工作区/Git 管理根、必要临时目录和网络权限。核实实际权限并通过真实工具执行；沙箱外规则放行的操作不能算沙箱内能力。必需操作需要审批即未通过，缺实际边界证据为未核实。
- full_access：只有输入明确指定才启用，从启动起允许在 Agent 沙箱外工作；它不授予额外 OS/资源权限。原生设置与实际工具均须核实；失败不切换账户、不提权，也不改原沙箱。

模式切换创建新检查；旧执行/检查冻结记录不改写。当前原生模式映射、支持版本和认证类型由 Runner schema/capabilities 声明，调用方不自行拼接权限 flags。不支持旧开发格式时按现有版本机制拒绝，不迁移、双写或回退。

沙箱外 canary 的正/负测试属于 Runner 定向原生验收，复用既有 SA 的有效证据；每次用户检查只验证本次必要操作和生效权限，不默认重复做沙箱安全认证。预期的边界拒写不是业务能力失败。

## 4. 最小公开接口

沿 `steerrunner-cli/v1` 请求信封增加三个命令，为本次新增接口，实际支持组合见 capabilities 和专属验收：

| 命令 | 语义 |
| --- | --- |
| `environment-check start` | `--input -` 消费 v1 写信封，固定 request_id 和不可变输入，异步接受并返回 check_id；通过既有显式服务启动能力准备服务，不等待模型/Git 完成才回执 |
| `environment-check inspect` | 恰选一个 `--check-id` 或 `--request-id`，返回下述 Inspection；只读，不启动服务/Agent、不清理、不恢复 |
| `environment-check stop` | `--input -` 消费 v1 写信封，input 含 check_id/expected_revision；停止新增操作并结算 owned 资源。资源待处理时，新授权可再次请求同一收尾流程；不能重启 Agent、重做创建或改写能力结果 |

首期无独立 events/cleanup/list 命令，不建立第二套事件游标；调用方维护自己的历史索引，通过 inspect 刷新。报告重启后可读，读取沿原服务与锁规则，不开放私有 DB 后门；服务不可用时返回不可用，调用方可展示已有缓存。

输入复用 Repository、ExecutionInput 和必要 Publication 值，增加执行/收尾期限；project ID 仅为不透明 correlation，不接受 Task/workflow/业务文档。Runner 先固定期望目标、模式、base 与清单；真实 runtime、进程/会话、账户、认证来源及生效权限按原回执追加绑定，不预填未知身份。冲突拒绝，不按现场改输入。

capabilities 只增加本检查的契约版本、输入/报告 schema 和已实现的目标/模式支持项，内容参与固定 resource_digest。当前普通 adapter 支持不等于检查支持。缺原生认证/取证机制是 unsupported；机制已实现但现场缺引用、凭据失效、权限拒绝或网络异常是本次环境不就绪，不能通过动态删 capability 隐藏。公开 schema、错误码和模式语义仅由本仓维护。

### 4.1 回执、查询与失败语义

新增 CheckReceipt 使用 command_id、check_id、receipt_status、action_status；外层沿现有 CLI v1 的 protocol/ok/request_id/data/error，写响应回显原 request_id，所有查询响应外层 request_id 为 null。接受只证明已持久化；输入校验、能力或服务启动在受理前明确失败时可以没有 check_id，不能伪造检查通过。有效且可识别的已提交写信封，其明确业务拒绝应绑定原请求可查询；无有效 ID/畸形 JSON 等协议拒绝只能按既有错误直接返回，不承诺创建请求记录。

Inspection 含 command（可空）和 check（可空）：按 check_id 查询返回 check 快照；按 request_id 查询返回原 start 或 stop 的命令身份、原 request_id/command_id/命令名/check_id/受理与动作状态，并带可取得的同一 check。stop 请求查询不得返回 start 回执冒充停止完成；原请求有明确拒绝而无 check 时仍须能取得拒绝事实。原请求不存在返回 request_not_found，它不授权换 ID 重发未知 start。报告缺失、错配或查询失败均不证明完成。

Check 快照至少包含 check_id、单调 revision、Runner 生成的 input_digest/resource_digest、冻结请求上下文及实际上下文、源 updated_at、必需项清单及分项证据/结论、原资源/PR 身份与结算、允许动作。input_digest 由 Runner 定义并返回；调用方保存首个已核实值、校验后续不变，不用自己的 JSON 编码重算替代。资源后续绑定可以补充实际身份，但不能改请求模式/目标/base。Check 的 input_digest 不等于 stop 请求输入的摘要。

错误沿既有退出码：无效信封/参数 2；unsupported_capability/platform/schema 3；service_unavailable 等 4；request_conflict/request_not_found/check_not_found/stale_revision/operation_unverified 等 5；request_outcome_unknown 6。新增 check_not_found 纳入 CLI/caller 同步映射。受理后准备或探针失败是可成功查询的 Check 结果（CLI ok=true、exit 0），不是把 inspect 本身写成传输失败；401/权限拒绝记 failed，无法确定的超时记 unverified，均保留具体阶段。

同一 check 较低 revision 不能覆盖较新快照；相同 revision 却内容不同是冲突。原 start 结果未核实、尚无可信 check_id/revision 时，界面只允许查原请求；stop 不接受空目标或猜测身份，后续取得可信绑定后需用户明确提交停止。开始前展示检查分支命名空间与本次操作清单，Runner 分配 check_id 后才返回精确引用。

## 5. 检查执行与副作用边界

Runner 单写服务保存独立检查记录，复用现有原请求、Operation、Attempt、资源 owner、准备/输入/停止机制。不伪造 development Run，不增加另一套调度器或通用工作流；检查完成不产生 pr_ready、TaskDelivery 等业务事实。活动检查及其晚到资源必须纳入 service stop / verify-stopped。

Runner 准备独立工作区与会话、随机 nonce 和本次操作清单，Agent 用实际工具完成文件/临时/Git 管理目录读写、远端读取/fetch、commit/push 和必需 PR 操作。确定性预检继续先行；其结果可复用为准备证据，但不是“模型已调用工具”的证据。不固定只能有一个模型回合，不把整个回合当成一个原子外部操作。

每个 push/PR 创建等外部写操作，Runner 必须先保存原意图、唯一目标和操作身份，再允许相应探针执行，执行后核验原对象；不能只依靠 start 请求去重或提示词要求模型不要重试。最小落地是在现有原生输入/工具和发布对账路径中接入固定内建探针，按外部写动作结算后再允许后继；不建设任意远程 shell 网关。若某 adapter 不能提供该边界，该写项 unverified，不宣称自动安全重试。单轮中重复/迟到工具请求、丢回执必须有受控反例，未落实前不得把该路径标为支持。

Git 引用为本次唯一 `steerline-check/<check_id>/base` 和 `head`：Agent 工具先在远端无覆盖创建 base，指向冻结 base commit，再创建只含本次探针文件的 head 提交并无覆盖推送。已存在/被更新的同名对象拒绝，不 force 更新业务分支。Runner 独立查询原远端核实仓库、引用及 SHA；push 未知只查原目标。

输入含 Publication 时，该组 PR 创建/查询/关闭是必需检查项；调用方须从项目实际发布配置装配，不能在缺认证时省略以取得“通过”。Runner 独立调用不带 Publication 时，该组明确 not_applicable 且报告标注未包含 PR，不能宣称发布就绪；不要求 Runner 读取调用方 workflow。Publication 的项目 base 只用于核对来源，测试 PR 的目标是本次生成的检查 base，不能沿用真实业务 base。

Agent 在原检查 base/head 间创建、查询并关闭测试 PR；Runner 核实 repository/base/head/head SHA/URL/编号。创建未知时只对账原 PR，不另建；不合并。读取成功不证明写权限，专属引用成功不证明受保护业务分支可直接推送。检查前调用方展示这些操作；本功能不新增每步 SteerLine 审批。

正常结束先处理外部对象，再停止并核实原写者，最后删除 owned 本地资源；终止/重启时沿原停止规则，不能为清理启动替代模型会话。确定性 owner 可以关闭原检查 PR、按 SHA 删除原引用，但须有该执行身份的已授权能力；没有则保留资源待处理，不调用中心宽权限凭据补做。确定性清理成功不能补记 Agent 对应能力通过。PR 历史保留；对象身份或 SHA 改变时不删。

可信证据为当前 check/session/nonce/命令对应的原生权限事实、工具调用/返回及独立 Git/provider 核验。模型自述、截图、单独成功 JSON/收据均不足；真实工具完整但原轮次未结算时保存分项，整体未核实。秘密不进入 argv、报告、持久输入和普通事件；仅保存非秘密认证引用及来源。

## 6. 故障、报告与适用范围

复用共同 W02–W14/W18、E01–E07、V01–V08；不运行 development W15–W17。原 request_id 幂等、不同输入冲突、接受与完成分离、未知只查原请求、迟到事实绑定原代次。浏览器退出不取消；服务重启只对账，不自动重发输入/创建资源。新一轮检查只在原写者及未知操作核实后以新 ID 开始，不拼接旧分项凑通过。

执行预算缺省 10 分钟，收尾另留 2 分钟；单次 I/O 沿既有 30 秒上限并受剩余预算约束。接受时固定期限，重启不重置；到期禁止新增操作，停止/清理未核实保留事实。

原 actor 的发送回执丢失后，只在原期限内读取同一 request_id 的缓存响应，并再次核实原 runtime/terminal/incarnation；不重发请求。暂时的 CLI/排队读取失败可只读重查，身份错配、部分写入或期限耗尽仍未核实。只有 helper 明确证明未调用 turn/start、且没有 turn 身份时，启动失败才记为 rejected；缺标记的旧 helper 或不确定 RPC 继续记 unknown。工具操作和工具回复分别判断：回复未知不重做已核实的原操作，报告保留有限原因，不保存原始错误中的秘密。

报告字段沿 §4.1；分项为 passed/failed/unverified/not_applicable，缺失/零值不是通过，空清单也不能通过。因上游阻断未执行为 unverified；只有固定清单中适用必需项全部通过才判本期能力通过，认证/资源缺失不能改 not_applicable。清理另判，残留不显示全部完成。明确未检查构建、测试、业务资源。

报告只证明当时上下文及该检查工作区。普通执行仍核实自己的 W03/W07；不新增“业务启动必须持有历史报告”门槛。账户/目标/模式等变化需重验；无法实时感知的凭据变化不保证持续有效。

## 7. 交付边界

计划覆盖本机 Codex 和 Orca 远端 Codex 的两模式。按目标/模式分别公布支持与验收：本机可先交付；远端在 SA07 等原生前置缺失时保留目标和缺口，明确 unsupported，不能以模型检查放行，也不要求 SteerLine 扩展 Orca 原生协议。远端补齐后分别实测再声明支持。本机通过不代表远端，全访问通过不代表沙箱。

本规格与独立 [SREC 验收](../acceptance/environment-check.md)归属 Runner 前置任务；SteerLine 自有接入规格/验收单独维护。Runner 契约及必要目标实现合入并发布固定 version/SHA/resource_digest 后，才作为 SteerLine 集成前置；UI 可提前消费公开 fixture 开发。本段原调查时未实现；本次用户已授权实施与定向验证，结果单独登记专属验收。

交接必须附固定提交上的 CLI/schema 和本仓 Controlled 测试实际校验的公开请求/响应样例（含 start/stop 回执、两种 inspect 选择、拒绝无 check、准备失败、未知/陈旧 revision），调用方 fixture 消费同版样例。样例不是另一状态权威，不携带秘密；样例或 schema 变更须使契约/resource 摘要变化。SREC 验证执行事实，SLEC 验证消费/项目隔离，二者不替代 SA 前置验收。
