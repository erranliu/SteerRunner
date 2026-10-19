# SteerRunner

SteerRunner 是个人多机执行器：在本人管理和信任、预先配置好的执行主机上，接收冻结的 Spec / Plan，执行开发 workflow、处理反馈并产出可核验 PR。支持同一服务多个独立 Run；调用方显式选择本机或 Orca environment。SteerLine 保留任务、DAG、机器选择、候选关系和 TaskDelivery，通过 CLI 使用它。

用户负责每台执行机的工具安装和 gh 登录。Runner 负责 Run 隔离、执行与发布对账、显式恢复、停止及清理；本机和 Orca local 受管执行另提供环境检查与有限期单仓库授权。当前不提供团队/多租户账号管理、GitHub App 签发、集中密钥托管、自动跨机迁移或额外远端 daemon。各 agent、权限模式和平台的实际支持以能力声明及验收为准。

当前已接通独立 Run 执行：一个 Windows/amd64 EXE 同时提供常驻服务和 CLI；固定 Spec/Plan、按需报告→发布→核验 PR、原会话人工续执行、显式恢复、停止授权及事件查询均由 Runner 持久化。本仓自有 L1、受控集成前置、L2 本机 Codex 真仓 smoke 和可扩展 L3 真机环境测试；现场生成与验收不依赖 SteerLine 项目。各层实际通过情况及失败现场见验收登记。

- [执行可见性：状态变化与最近消息](docs/spec/execution-observability.plan.md) / [验收](docs/acceptance/execution-observability.md)
- [Orca 本机/远程连接 Spec](docs/spec/orca-local-remote.spec.md) / [Plan](docs/spec/orca-local-remote.plan.md) / [当前验收与原生能力缺口](docs/acceptance/orca-local-remote.md)
- [远端仅通过 Orca 执行 Spec](docs/spec/orca-remote-native.spec.md) / [Plan](docs/spec/orca-remote-native.plan.md) / [验收](docs/acceptance/orca-remote-native.md)（2026-10-19 原生链路修订；新远端 Run 不部署运行 SteerRunner helper）
- [通用 Orca Agent 接入 Spec](docs/spec/orca-generic-agent.spec.md) / [Plan](docs/spec/orca-generic-agent.plan.md)（不修改 Orca；本机 Pi/DSH 原 TUI 已接通，支持受管工具与实际权限；[当前构建验收](docs/acceptance/orca-generic-agent.md)）
- [首版产品与 CLI 契约](docs/spec/steerrunner.spec.md)
- [独立调用与内部测试配置 Spec](docs/spec/independent-configuration.spec.md) / [Plan](docs/spec/independent-configuration.plan.md) / [验收](docs/acceptance/independent-configuration.md)（调用方管理选择，Runner 发布契约；内部测试配置独立）
- [跨仓拆分实施计划与最小测试安排](docs/spec/steerrunner.plan.md)
- [内置 Workflow 选择与审计模型 Spec](docs/spec/workflow-selection-audit.spec.md) / [Plan 与验证记录](docs/spec/workflow-selection-audit.plan.md)
- [可靠执行中间层模块化重构 Spec](docs/spec/reliable-modularization.spec.md)与 [Plan](docs/spec/reliable-modularization.plan.md)（operation 及 RM-P2 职责整理已完成，收尾验收通过）
- [专属验收要求与结果登记](docs/acceptance/steerrunner.md)
- [来源清单及 P2 交接门槛](docs/extraction/source-map.md)

首版衔接 SteerLine 的执行三层重构，提取 Workflow、ReliableExecution、ExecutionAdapter 及独立宿主。SteerLine 保留任务规划、DAG、候选关系和最终 TaskDelivery，通过 CLI 消费 SteerRunner 的运行事实。

独立项目默认资源内嵌。任务材料、仓库、认证及具体执行后端需要的 Git / 代理工具是执行输入与环境依赖。OpenHands、SWE-ReX 或持久化 workflow 引擎的集成可以后续改变内部实现，不改变本版对外职责。

## 构建和启动

开发构建需要 Go 1.24.2 或更新版本、Windows/amd64 和 Git（本次验证使用 Go 1.26.4）。在仓库根目录用 PowerShell 执行：

```powershell
.\scripts\build.ps1
.\bin\steerrunner.exe version --json
.\bin\steerrunner.exe service start --state-dir C:\Temp\SteerRunnerDemo --json
.\bin\steerrunner.exe service status --state-dir C:\Temp\SteerRunnerDemo --json
.\bin\steerrunner.exe capabilities --state-dir C:\Temp\SteerRunnerDemo --json
```

构建输出 `bin/steerrunner.exe` 及构建来源/摘要记录。运行只需 EXE；内置流程、内置步骤提示词及 schema 已内嵌，不需要 SteerLine、模板目录、Python 或 Node。CLI 启动成功后退出，常驻服务继续运行。CLI 与服务使用同一个 EXE 文件，但各自是独立进程。

Runner 不维护部署清单或命名 profile。SteerLine／独立调用方选择后端、管理默认方案并提交完整请求；Runner 应用公开默认值、校验和冻结输入。离线能力查询与纯校验不启动服务、不检查工具或读取内部测试配置：

```powershell
.\bin\steerrunner.exe capabilities --offline --json
Get-Content -Raw -Encoding utf8 .\run-configuration.json | .\bin\steerrunner.exe validate --kind run-configuration --input - --json
Get-Content -Raw -Encoding utf8 .\run-input.json | .\bin\steerrunner.exe validate --kind run --input - --json
Get-Content -Raw -Encoding utf8 .\check-input.json | .\bin\steerrunner.exe validate --kind environment-check --input - --json
```

三个文件直接保存对应对象，不含写信封。`validate` 返回 `request_id=null` 和字段诊断；成功只说明生产纯 gate 接受，不证明环境就绪。实际 `submit`／检查 start 仍使用原 v1 写信封和 request_id。能力目录列出实现组合、schema 引用和限制；离线来源标记 EXE，在线来源由响应服务附带其 instance_id。旧接口不要求先调用 validate。

Windows 自动启动先验证挂起子进程的 Job 隔离；原生 breakaway 被明确禁止或创建后仍有 Job 关联时，使用系统自带 Windows PowerShell/CIM 挂起创建独立进程，核验用户、权限与 EXE 后才恢复执行，不请求提升权限。该系统能力不可用时返回明确错误。`startup_unverified` 表示原启动结果未核清，须核实 `service/bootstrap.json` 对应的原进程，不会自动重建另一份启动。`serve --state-dir <绝对路径>` 可从已有独立宿主前台运行，生命周期由该宿主负责。

## 提交与查看执行

本机执行当前固定验证原生 Codex `0.159.2`，其他版本会在权限资格核验时拒绝。
本机及 Orca local 受管 GitHub worker 使用执行主机已有 gh 登录的单仓库受限代理，宿主 OAuth 不进入 worker。
Runner 在业务前准备各 Run 独立的有限期 capability，并执行与实际权限模式一致的原生工具检查。
认证秘密不写进请求、持久化配置或材料；停止关闭该 Run 的 broker，不退出宿主 gh 登录，不影响其他 Run。
GitHub App 支持已移除，旧 `execution.settings.github_auth`（包括 null）明确拒绝，不能静默改用个人身份。
升级前须用旧版本停止、核实并收尾已有 App Run/Check；新版本不迁移其身份或接管旧 App 凭据。
固定 approval never，失败不会回退自动审核发布。
Orca local 受管执行使用可信 helper、同权限准备与认证撤销；新远端 Run 只使用已有 Orca/Codex/Git/gh，不部署 helper，不承诺 Runner capability、同工具预检或登录撤销。远端配置中的 runtime_executable/runtime_sha256 键（含空值）拒绝。
个人多机边界、App 移除和本次验证见[专项记录](docs/acceptance/personal-runner-gh-only-20261019.md)。
同服务多 Run 保留原工作区、会话、事件、PR 与停止隔离。原生远端不支持有来源证明的双模型审计或完整环境检查；原 helper 构建的历史验证范围见
[Orca 扩展验收](docs/acceptance/orca-remote-extensions-20261018.md)。
设置、外部前置及受控/真实验收边界见[测试指南](docs/testing.md)和[专属记录](docs/acceptance/sandbox-authentication.md)。

把[契约示例](docs/spec/steerrunner.spec.md)中的仓库、完整 commit、材料、adapter 和 publication 改为自己的环境，保存为 UTF-8 `request.json`：

```powershell
$OutputEncoding = [Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
Get-Content -Raw -Encoding utf8 .\request.json | .\bin\steerrunner.exe submit --state-dir C:\Temp\SteerRunnerDemo --input - --json
.\bin\steerrunner.exe inspect --state-dir C:\Temp\SteerRunnerDemo --request-id my-submit-001 --json
.\bin\steerrunner.exe inspect --state-dir C:\Temp\SteerRunnerDemo --run-id <run_id> --json
.\bin\steerrunner.exe events --state-dir C:\Temp\SteerRunnerDemo --run-id <run_id> --json
.\bin\steerrunner.exe last-message --state-dir C:\Temp\SteerRunnerDemo --run-id <run_id> --json
```

accepted 表示材料及原请求已提交，执行结果以 Run 和命令 action_status 为准。丢失响应时查询原 request_id；同 ID 的重试必须保留原正文。服务重启只读取原执行事实；继续执行需要当前 revision 的新 resume，旧答案和反馈不会随重启自动发送。

同一服务内，Orca 下一轮发送前的状态连接失败会在冻结的 `transport.wait_timeout` 内退避重试，每次 WebSocket 建连最多等待 5 秒。恢复后重新核验原终端、工作区和 provider session，再发送一次原输入；认证拒绝、身份变化和等待/阻塞状态不重试。超过期限仍明确阻断。已经尝试发送但结果未知的操作继续只对账原请求，不自动重发。

本地 Codex 读取本 Run 的冻结文件。Orca 在实施输入内读取附带的冻结文档；本机和远端都不读取调用方的移动文件。当前 Orca CLI 通过参数传文本，材料 JSON 与四份正文总计上限为 12000 UTF-16 单元，超过在接受前拒绝；远端真机闭环及执行稳定性的已验收范围见[专属记录](docs/acceptance/l3-execution-stability.md)。Orca 的 local_path 指向已注册项目，environment 必须预先配置；执行主机的同名 Git 仓库须已获取固定 base commit，创建工作区时核验实际 base/head。Runner 不复制调用方的认证或安装配置。

## 主动环境检查

`environment-check` 独立于 development Run，不接收 Task、Spec/Plan 或 workflow。
检查和普通执行均通过 `execution.settings.access_mode` 选择 `sandbox`（默认）或显式
`full_access`，两者始终 `never`，不会从沙箱失败自动切换模式。全访问不增加 OS 权限。

开始前应展示 `steerline-check/<check_id>/base|head` 命名空间及检查清单：原生权限、工作区/临时/Git
管理目录读写、远端读取/fetch、提交与专属引用 push；传入 Publication 时还必须创建、查询并关闭
专属 base/head 间的测试 PR。PR 不合并，清理按原 SHA 删除 owned 引用。无 Publication 时 PR 项为
`not_applicable`，报告不证明发布就绪；不检查构建、测试及业务资源。

本机 codex-app-server 及 Orca local 受管路径已实现 Windows Codex 0.159.2 + HTTPS GitHub `.git` 仓库 + 执行主机 gh 登录 broker 的两种模式。
上述本机路径的普通执行与环境检查统一使用执行主机 gh 单仓库 broker；原生远端完整环境检查明确 unsupported，普通远端 Run 使用已有 Orca 原生认证。不支持 GitHub App。旧 App 配置拒绝测试不计为 App 真机验收。
已实现组合缺登录、权限或网络时，受理后的报告记录失败/未核实。真实验收和发布状态分别见
[环境检查记录](docs/acceptance/environment-check.md)和[Orca 扩展记录](docs/acceptance/orca-remote-extensions-20261018.md)。

`check-request.json` 使用 v1 信封，input 含 `repository`、`execution`，可含 `correlation`、
`publication`、`execution_timeout`（默认 `10m`）和 `cleanup_timeout`（默认 `2m`）。Repository 和
Publication 形状沿普通执行；项目 base 只核对来源，测试 PR 使用 Runner 分配的专属 base。
capabilities 返回内嵌检查 schema、操作清单、支持矩阵和经过生产查询路径校验的公开样例。

```powershell
Get-Content -Raw -Encoding utf8 .\check-request.json | .\bin\steerrunner.exe environment-check start --state-dir C:\Temp\SteerRunnerCheck --input - --json
.\bin\steerrunner.exe environment-check inspect --state-dir C:\Temp\SteerRunnerCheck --request-id check-start-001 --json
.\bin\steerrunner.exe environment-check inspect --state-dir C:\Temp\SteerRunnerCheck --check-id <check_id> --json
```

start 异步回执只证明持久化。stop 的 v1 input 为 `{"check_id":"原ID","expected_revision":当前revision}`，
同样使用 `environment-check stop --input -`；按 stop request_id 查询返回原停止命令。
查询不启动服务、恢复 Agent 或清理资源；服务重启只对账原对象，不重放模型输入/写操作。
同一写操作的重复工具请求不会再次转发；未知结果只查原对象。能力结果与清理结算分别判断，
残留对象不显示全部完成。公开契约及样例随 `resource_digest` 固定。

## 选择 workflow 与审计模型

当前开发格式为 state/v3、资源版本 2；workflow/1 和 state/v2 明确拒绝，不迁移旧运行。请使用新的空 state-dir。SteerLine 接入限制见[结果获取 Spec §8.1](docs/spec/result-acquisition.spec.md#81-steerline-不改的核查与具体限制)。

独立解析采用显式开启的 Windows / Codex 0.159.2 受限 exec：原生只读权限、独立会话和工具事件事后拒收；不承诺空工具面或零隐式上下文。支持程序提取与一次原 agent 报告；审计可选独立解析，明确获取失败且资源结算后转一次原审计补报。可选 Runner 配置在 `<state-dir>/service/result-acquisition.json`，不增加 SteerLine 配置；启用配置也不会绕过原生门槛。[实施与验收记录](docs/acceptance/result-acquisition.md)。

服务启动后，`capabilities` 的 `workflows` 返回可选流程、允许覆盖的提示词步骤及模型要求。submit 继续通过 RunRequest 的 `workflow.id/version` 选择：

- `development/2`：实施→发布→pr_ready；可信成功行或完整机器结果可跳过补报，每个原源最多补报一次。未完成的实施等待输入。
- `development-audited/2`：实施→一次代码审计及 P0–P2 修复→发布；审计记录修复数量、内容及提交前后来源。支持本机 `codex-app-server` 与配置可信 helper 的 Orca local Codex；实际模型回执和执行主机 Git 来源均须核实。原生远端在接受前返回 unsupported_capability。

带审计请求的相关字段如下，其余 RunRequest 字段仍必填。模型标识请替换为账号实际可用型号：

```json
{
  "workflow": {
    "id": "development-audited",
    "version": 2,
    "audit_model": {"model": "review-model-id", "reasoning_effort": "high"},
    "prompt_overrides": {"audit.code": "审计代码、修复 P0–P2 并完成必要验证，提交改动并报告实际修复。"}
  },
  "execution": {
    "adapter": "codex-app-server",
    "target": {"kind": "local"},
    "settings": {"model": "execution-model-id", "reasoning_effort": "medium"}
  }
}
```

审计与实施使用同一 agent、thread、工作区和分支。审计流程要求显式填写执行模型与审计模型，实施和发布使用执行模型，审计及同轮修复使用审计模型；省略审计 effort 时继承规范化执行 effort。模型不可用时明确阻断，不回退或换会话。

`inspect.audit` 显示当前实施结果、原审计来源、实际修复数量与逐项内容，`execution.requested_model/requested_reasoning_effort` 表示冻结的轮次请求参数。仍有 P0–P2 未解决或验证未完成时不发布；P3 建议不计修复数。补报最多一次，坏 JSON 不当作通过。提示词和模型接受后冻结，修改需新 Run。暂不支持外部 workflow 装载。

存储使用 v3；旧目录、旧流程版本明确拒绝，保留原目录由原 EXE 收尾，不建立兼容和迁移路径。

## 停服及回执

PowerShell 7 示例（stdin 为 UTF-8 JSON）：

```powershell
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
$runnerExe = '.\bin\steerrunner.exe'
$runnerState = 'C:\Temp\SteerRunnerDemo'
$runnerStatus = (& $runnerExe service status --state-dir $runnerState --json | ConvertFrom-Json).data
$runnerRequest = @{
    protocol = 'steerrunner-cli/v1'
    request_id = 'demo-stop-001'
    input = @{ expected_instance_id = $runnerStatus.instance_id }
} | ConvertTo-Json -Compress
$runnerRequest | & $runnerExe service stop --state-dir $runnerState --input - --json
```

exit 0 表示命令已持久接受。停服后再次启动服务，可用 `inspect --request-id demo-stop-001` 查询原回执。同 ID 同正文重试返回原命令，换正文返回冲突；旧实例的停服重试不会停止新实例。只读查询不会自动启动服务。

## 状态目录

调用方使用绝对 `--state-dir`；省略时为 `%LOCALAPPDATA%/SteerRunner`。目录下有 `runner.db`、`service/`、`inputs/`、`r/`、`workspaces/`、`logs/launch.log` 和 `logs/service.log`。由 SteerLine 使用时，应指定其项目运行目录下的 `runner/`，使数据和日志归属于该项目。CLI 不读取 SQLite，同目录只有服务持锁写入。

本机新 Run 从宿主维护的仓库缓存创建独立、无 hardlink/alternates 的 Git 副本。
worker 的 common Git 目录只属于该 Run。sandbox 模式只为这个私有目录增加写根，
允许 Git 创建 `packed-refs.lock` 等根级锁，不授权共享缓存、其它 Run 或整个状态目录。
代价是每个 Run 多一份本机仓库副本。
已有 Run 的共享缓存和冻结权限保持原样，不在恢复时扩大授权；该修复通过新 Run 生效。

本机 GitHub 认证 broker 的 socket 使用 `a/<26位资源键>/s`，避免把 Git 管理目录的深度叠加到 AF_UNIX 地址上。资源键保留 128 位摘要，绑定项目、执行、attempt、资源 generation、配置摘要、工作区和 remote；权限资源 codec v4 在会话创建前固定该目录。原生启动、就绪检查、正式回合和环境检查只增加本执行的目录访问，不授权共享的 `a/` 或整个状态目录。地址仍按 106 字节预检，超限时明确阻断。

认证关闭、进程退出或准备失败时，仅删除原 socket 和已核验的空目录；目录重定向、身份替换、外来文件或崩溃遗留目录不会被递归删除或自动接管，未完成清理会如实报告。旧 codec v3 权限资源仍可读取，但不能静默增加认证目录授权；需要使用新资源创建执行。

不兼容旧 SteerLine 数据或其他数据库，不提供旧格式迁移。替换构建前先停服；构建脚本不会结束或替换正在运行的服务。

## 定向验证

L1–L4 定义、维护规则和执行入口见[测试指南](docs/testing.md)，具体内容见
[测试清单](docs/test-catalog.md)：L1 为快速单元/集成测试，
L2 为最小核心 smoke，L3 为尽量正交、可并行的不同流程及核心可靠性测试，L4 为更复杂
边界或确实无法并行的测试。调用方退出、断连、服务换代、停止撤销和发布对账保留在 L3。
当前已注册 L2 两 case、L3 二十二场景／三十 case（含远端可靠性、本机反馈／审计／多 Run、四种本机环境检查及 Pi／DSH）、
L4 四个受控边界场景。受控前置已纳入审计及多 Run，支持有界并行与 L4 独占；L2 最小
driver 需要同构建反馈通过资格。真实后端并行资格及远端原生范围分别登记，逐项内容见清单。
历史实际结果见[稳定性验收](docs/acceptance/l3-execution-stability.md)，
该受测套件完整资格为同构建连续三轮。
历史[主线复测](docs/acceptance/l3-rebase-20261016.md)及[不可达撤销修复复测](docs/acceptance/l3-revocation-fix-20261016.md)保留当时构建结果与认证阻断；
原构建 Orca 十项通过记录见[扩展验收](docs/acceptance/orca-remote-extensions-20261018.md)，
当前清单移除 App 专项后仍有八项管理员故障未验收，见[变更记录](docs/acceptance/orca-remote-app-case-removal-20261018.md)。
统一运行 `./scripts/test.ps1 -Level L1`，真机级别需显式配置，详见测试说明。

接受前仓库分支刷新回归使用真实隔离 Git，不调用模型或真实 PR：

```powershell
go test ./internal/application -run '^(TestSRFreezeBaseBranchRefresh|TestSRRepositoryCacheWithLongBranchPath|TestSRCompactCacheCanPublishLinkedWorktree)$' -count=1 -timeout=4m -v
```

场景、故障反例及结果见[分支刷新测试计划](docs/spec/repository-base-refresh.plan.md)和[验收记录](docs/acceptance/repository-base-refresh.md)。

```powershell
go test ./internal/contracts ./internal/cli ./internal/storage -run '^(TestSRCLIContract|TestSRShutdownReceipt|TestSRUnsupportedStorage)$' -count=1 -timeout=2m -v
.\scripts\test-foundation.ps1
```

进程测试脚本构建真实 EXE，创建独立测试宿主后返回证据路径。等待该路径的 `result.json`，其中 `exit_code: 0` 且 `scenario_count: 2` 才表示两组测试通过；脚本启动成功不等于测试成功。详细断言在 `integration.log`，调用输出在 `caller-commands.jsonl`。测试使用本次临时目录，不启动真实代理或 PR，也不使用 SteerLine 安装目录。

程序清单资源已生成并纳入源码；只在修改 `app.manifest` 后运行 `go generate ./cmd/steerrunner` 重新生成。项目采用 [Apache-2.0](LICENSE)，来源说明见 [NOTICE](NOTICE)。第三方运行依赖和 Go runtime 的许可声明见 [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt)。正式发布须完成对应验收。

当前测试建设范围包含 Windows/amd64 本机 Codex L2/L3、远端 Orca L3 和 L4 受控边界。历史取消及失败记录保留；
支持声明以对应版本的实际测试结果为准，分层规范见[测试指南](docs/testing.md)，
当前覆盖与缺口见[测试清单](docs/test-catalog.md)。

## 环境检查规划入口

2026-10-16 仅规划、先不实现：[无人执行权限与 Git 环境检查 Spec](docs/spec/environment-check.spec.md) · [Plan](docs/spec/environment-check.plan.md) · [专属验收](docs/acceptance/environment-check.md)。本仓维护 Runner 公开契约与执行事实；SteerLine 独立维护调用与展示。沙箱认证工作保持原作用域，整合冲突及远端前置见 Spec。

## 结构化结果获取与 Workflow 定义

2026-10-17 已开始实施：[结构化结果获取 Spec](docs/spec/result-acquisition.spec.md) · [Plan](docs/spec/result-acquisition.plan.md) · [专属验收](docs/acceptance/result-acquisition.md) · [源码与 Codex 能力调查](docs/acceptance/evidence/result-acquisition-design-20261016.md)。按步骤组合程序提取、独立解析和原 agent 报告；保留业务顺序与 PR 核验，Runner 独立配置，SteerLine 不改。三种方式已按确认的最小 parser 边界实现，原生权限保证、行为约定和事后检测分别记录；workflow/state 版本影响见 Spec。当前验证与未通过项见专属验收；未部署，不启动 roadmap 调度。

Workflow 能力/状态机定义：[统一能力与脚本边界](docs/workflows/README.md) · [普通开发流程](docs/workflows/development.md) · [带审计开发流程](docs/workflows/development-audited.md)。每份定义列出状态、转移核验、提示词和预算；执行 agent 切阶段统一执行定义提示词，定制脚本仅用于结果核验。两份内置定义共用状态与结果机制；development 不调用 parser，审计流程仅审计结果可解析。完整验证及未完项以专属验收记录为准。

绑定某次服务实例时，对业务/查询追加 `--expected-instance-id <instance_id>`；Runner 在实际请求受理前拒绝替代实例，且不隐式启动新服务。支持标志为 `capabilities.instance_binding.expected_instance_id: true`。`service status` 仍只供诊断，`service stop` 使用其输入正文的原 expected_instance_id。详见 [CLI 契约](docs/spec/steerrunner.spec.md)。
