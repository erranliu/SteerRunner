# Plan：无人执行权限与 Git 环境检查能力

日期：2026-10-16。已实施；以下保留实施顺序和原前置，当前实际组合、构建及结果见专属验收，远端范围按 Spec 的后续修订。[Spec](environment-check.spec.md) · [本仓专属验收](../acceptance/environment-check.md)。本 Plan 只安排 Runner 改动；SteerLine UI/HTTP/caller 测试归其自身 Plan，不共享验收项。

## 1. 前置与整合

调查基线和 SA01–SA08 交接表见 Spec。实施时重新固定主干、沙箱认证最终提交、原生版本及实际构建；本次仅新增文档，不修改 `8876/SteerRunner` 的在途实现。不得以本规格的全访问分支反向改变沙箱专项验证口径。

先按 Spec §2.1 的时点表核对最终认证 owner、准备 API 和证据：sandbox 复用 SA01–SA04，full_access 复用 SA01/SA03 并由本功能实现其同权限无模型预检，不反向要求 SA 先实现全访问。远端另外要求 SA07。只使用执行主机 gh broker，缺登录阻断；移除 App 引用与签发，旧字段拒绝；普通执行不追加模型资格轮次。SA04/SA08 的作用域须与显式全访问统一，未统一前不能发布双模式。

设计时读取的 SA 有效凭据正例及 L2 smoke 尚未通过。实施时 owner 6818e71 与真实通过记录 40b4ccd 已随 177045c 合入；原无效 token 负例和本功能受控测试仍不能代替该正例。fixture/Controlled 开发可先行；真实发布/集成仍须所复用 SA 最终提交已合入并满足其自身交付门槛。登记引用与适用范围，不要求本任务重复执行未改动的完整 SA 业务流程，也不豁免上游自身门槛。

## 2. 实施步骤

### R1：同源权限与公开契约

修改 `internal/contracts/`、`resources/assets/settings.schema.json`、`resources/resources.go`、`cli/cli.go`、`host/server.go`：落实 Spec §4/§4.1 三命令的参数选择、CheckReceipt/Inspection/快照与现有 v1 信封/退出码。增加 check_not_found 映射、按 stop request_id 查询原停止命令的回执、明确拒绝无 Check 的查询、单调 revision/摘要不变守卫。查询不启动服务，不增加 events/cleanup 命令或通用 schema 加载机制。

支持能力只反映已实现机制；缺现场引用/凭据、401、权限拒绝或网络异常进入受理后报告的 failed/unverified，不能动态改为 unsupported。Publication 有无决定固定 PR 清单适用性，开发项目不得漏传已有 Publication 以跳过发布检查；不从 workflow 猜测。检查 base/head 由 Runner 生成，项目真实 base 不作测试 PR 目标。

在 Codex `policy.go`、`native_params.go`、配置/启动/连接/生命周期，以及 Orca 启动编译中统一 access_mode；thread、turn、command 与普通业务都使用同一冻结策略。sandbox/full_access 均 never、无 auto_review；冲突 raw args 拒绝。修改生效策略与冻结记录的格式/摘要核验，不迁移旧记录，不为旧执行保留自动审核启动回退。

### R2：独立检查用例复用原资源 owner

在 `internal/application/`、`runstate/`、`storage/` 增加最小 Check 聚合，复用现有 Operation/Attempt、单写提交、写锁、期限、原请求和停止机制。不要为省事构造空 Spec/Plan 的 development Run，也不先抽象整个执行引擎。仅当现有逻辑确实被 Run 字段绑定时，提取检查与业务两处都消费的窄准备/停止方法；不预建通用 job 框架。

同源确定性准备后，由实际 Agent 执行内建探针。将权限事实/工具来源归 adapter 核验；Git/provider 原对象核实归现有 owner。按外部写动作保存意图、授权探针、结算，再推进后继；对同轮重复工具动作/丢响应增加原生端口约束和受控用例，不能用一次 start 幂等或 prompt 代替。明确 base 创建、head commit/push、PR 创建/关闭和清理各自操作者及所有权。

运行成功、取消、超时都汇入同一收尾；stop 的新授权仅重试可核实的 owned 资源结算，不恢复模型或原创建操作。服务 stop / verify-stopped 遍历活动 Check 和晚到资源，清理失败与能力结论分开，查询只读。秘密由认证 owner 管理，不写输入/报告。

### R3：远端能力核对

核对 Orca 当前接口是否提供执行主机认证来源、同权限确定性预检、实际工具/权限/审批事实及写动作关联能力，逐项登记在验收。机制缺失返回 unsupported；机制已实现但现场不就绪使用环境失败/未核实报告。写明需要的原生端口能力，不得在本任务里暗含建设 Orca 新协议。若必须修改 Orca，先将其列为外部明确前置；本机交付不等待未支持的远端，远端声明支持仍必须先补齐并实测。

### R4：文档、驱动与交接

更新本仓产品/测试说明、示例与生成器的 access_mode，沿现有分级入口登记本功能场景。SA 文档实施时同步其最终版本的 sandbox 作用域和同源接口，不改历史成功/失败记录。

交接给 SteerLine 的内容是：已合入 Runner commit、固定 EXE version/SHA/resource_digest、capabilities/schema、实际支持组合、所引用 SA 的最终提交/适用证据，以及本仓 Controlled 测试已校验的同版公开请求/响应样例。样例覆盖 Spec §4.1 的原 start/stop 查询、无 Check 拒绝、准备失败、未知和 revision 冲突；SteerLine fixture 必须同版消费，不能各自手写不一致响应。没有真实构建前不得把拟议字段写成可用接口；无需先完成 SteerLine 安装或操作其数据库。

## 3. 测试计划

全部自动项链接[本仓 SREC-A01–A06](../acceptance/environment-check.md#自动项)。结果与原始证据位置只登记本仓 acceptance；原始日志放正式入口新临时现场。新测试/场景名为计划新增，实施须登记且验证非零匹配；skip/unsupported 不记对应原生组合通过。

| 级别 / 场景 | 自动项 | 环境、命令、时点 | 通过条件与证据 |
| --- | --- | --- | --- |
| 文档静态 | 文档整理 | 当前 Windows/Git/Python；交付前运行本次本地 Markdown 链接检查、`git diff --check` | 无新增缺链/尾空白，记文档记录，不替代产品项 |
| 定向 L1：SREC-RULES | [A01–A05](../acceptance/environment-check.md#自动项) | R1/R2 后；`go test ./internal/application ./internal/contracts ./internal/cli ./internal/storage ./internal/execution/... -run '^TestEnvironmentCheck' -count=1`；受控进程/Git/provider | 前置按模式/时点、SA 证据不可错用；信封/原 start-stop 查询/准备失败结果/非支持与缺凭据区分；固定清单及 PR 适用性、证据伪造/身份错配、写入未知/重复/清理守卫；A01–A05 |
| 权限与认证定向回归 | [A01/A02](../acceptance/environment-check.md#自动项) | R1 后；`go test ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport -run 'NativePermissions|ThreadStart|TurnStart|Authentication|SandboxReadiness|EnvironmentCheck' -count=1`；实施时按实际已有测试名补齐 thread/turn/command 与普通业务冻结用例 | 双模式无审核依赖；普通执行与检查一致，SA 认证/预检不退化，无旧策略回退；A01/A02 |
| Controlled：SREC-CLI | [A01–A05](../acceptance/environment-check.md#自动项) | R2 后扩展 `tests/integration`，`scripts/test.ps1 -Level Controlled`；实际 EXE/CLI/IPC/SQLite、隔离 Git及原生/provider fixture | 同模式普通启动/确定性准备回归、三命令回执/两种 inspect、拒绝无 Check、准备失败仍可查询，样例逐项校验、重启/迟到/服务结算与原请求故障非零覆盖；A01–A05 |
| L3 原生：SREC-NATIVE | [A06](../acceptance/environment-check.md#自动项)，并实测 A01–A04 的原生条款 | 新增 `environment-check.local-sandbox` / `local-full-access`；具备远端前置后新增 `orca-sandbox` / `orca-full-access`；各执行 `scripts/test.ps1 -Level L3 -Scenario <完整ID> -Config <本次配置>`，沿入口既有 L1/Controlled 前置 | 每个声明支持组合实际 Agent、权限、Git/PR、零审批和资源结算；A06 分组合登记，回填 A01–A04 对应实测证据；不凭受控回执签收真实能力 |

本机两模式必须分别通过才声明本机双模式支持。远端未支持时，确定性 unsupported 反例必须通过，真实远端保持待前置；目标仍保留，不能删除测试后宣称全部目标完成。定向原生 canary 仅在 Runner 准备/策略、实际权限根/账户、原生版本及目标均相符且未被本次改动影响时复用已核实 SA 证据，登记来源；仅“Codex 版本相同”不足。策略、认证或根目录变更须补对应最小正/负边界验证；不在每次用户检查里重复安全认证。全访问不借用 MXC 负例证明其权限能力。

原生测试用 Runner 自有隔离夹具和 `erranliu/SteerlineTest` 本次专属分支，只创建/关闭、不合并测试 PR。driver 可准备独立 fixture source/base、配置和只读取证，但不能代建或推送 `steerline-check/<check_id>/base|head`、代做本轮待验提交/PR；这些是实际 Worker 要完成的能力操作。driver 准备的 source/base 与检查 owned base/head 分别记录，防止夹具动作冒充通过。新现场冻结来源、模式、权限和目标，换模式新现场。缺 EXE/认证/远端能力记录阻断，不复制安装认证，不手动批准补跑通过。

不默认跑 Full、完整 development L2 或所有 L3 稳定性组：本功能不改开发阶段/交付，定向权限回归覆盖普通启动变更，受控与原生检查闭环验证新增行为。只有实际改动/失败显示缺口时，说明原因追加原有对应组。

## 4. 完成条件

本仓必要 SA 整合、公开契约、支持组合实现、定向/受控/对应原生验证及评审合入齐备，发布固定构建后，Runner 前置才完成。远端未支持单列，不阻断已支持本机的 SteerLine 接入，也不记远端通过。以上为完成门槛；本次实施、支持范围、原生结果及本地未发布状态见专属验收。
