# SteerRunner 独立三级测试

测试源码、夹具、现场生成、执行和结果判定全部归属本仓；不要求 SteerLine
checkout、EXE、数据库、配置或测试调度器。运行目标为 Windows/amd64。

| 层级 | 范围 | 外部副作用 |
| --- | --- | --- |
| L1 | 本仓规则、组件、协议、存储及依赖边界 | 临时目录、受控进程，不启动真实模型或创建 PR |
| Controlled | 真实 EXE/CLI/IPC/SQLite，隔离 Git 和原生协议/provider 夹具；为 L2/L3 前置 | 本地隔离 Git push，不创建真实 PR |
| L2 | 本机真实 Codex + SteerlineTest 完整 smoke | 消耗模型 token，真实提交、push、创建并合入专属测试 PR |
| L3 | 持续扩展的真机真实环境场景集；首个场景为远端 Orca environment smoke | 每个场景按其 driver 执行真实副作用，记录独立证据 |

受控前置目前有 12 个场景，覆盖独立包、服务归属、材料冻结、权限资格、完整工作流、
存储/日志/事件、PR 身份、工作流完成前的发布事实、原生未知结果与停止、重启后显式恢复。
另增加丢响应找回和停止授权跨重启保持。L2 保留一个 development smoke；L3 已注册
原 smoke 和五个执行稳定性场景，共十二 case。新增真机验收进度见
[稳定性验收](acceptance/l3-execution-stability.md)，注册齐全不代表真机已通过。

## 入口

在仓库根目录使用 PowerShell 7：

```powershell
.\scripts\test.ps1 -Level L1
.\scripts\test.ps1 -Level Controlled
# 将 example 复制到本地非秘密配置，填写原生可执行文件与已配对 environment。
New-Item -ItemType Directory -Force .evidence | Out-Null
Copy-Item tests/live-config.example.json .evidence/live-config.json
.\scripts\test.ps1 -Level L2 -Config .evidence/live-config.json
.\scripts\test.ps1 -Level L3 -Config .evidence/live-config.json
.\scripts\test.ps1 -Level L2,L3 -Config .evidence/live-config.json
.\scripts\test.ps1 -Level L3 -Scenario orca-remote.smoke -Config .evidence/live-config.json
```

L2/L3 自动先运行 L1 和完整受控前置。任何前置失败都不会启动模型。
`-Scenario` 为精确场景 ID，未匹配、重复或跨级选择失败。每次生成新现场，
不复用上次 Run、请求授权或已经执行过的 manifest。脚本等待完成后返回退出码；
不能以后台 PID 或启动成功作为通过。

L2 当前原生资格固定 Codex 0.159.2；配置 `codex_executable` 为实际 native EXE。
本机 `model` 留空使用原生默认模型；可显式固定模型和 reasoning_effort。远端可单独设置
`remote_model`、`remote_reasoning_effort`、`remote_args`；`remote_model` 显式留空表示采用
远端 Codex 的原生默认模型和 effort，与手动运行 `codex` 一致。省略该字段才继承共用 model。
L3 远端 Orca 使用指定 CLI、已配对 environment 和远端已有的同名 SteerlineTest Git
项目。两端分别在实际执行主机准备认证，不从调用方复制认证，也不修改全局权限配置。
本机 GitHub worker 默认使用执行主机已有 gh 登录的受限认证代理。OAuth 只在可信
runtime 内存中，worker 只继承随机单仓库 capability，一小时到期、退出即失效。
Git smart HTTP 经进程级代理及公开 CA，gh 经独立公开配置的 AF_UNIX 接口；原工具
仍在 MXC 内执行，原 GitHub URL 保持。socket 位于已有 Git 写入根，证书不安装到系统。
代理仅允许目标仓库的 Git 上传/下载、身份/仓库只读查询和 PR 查询/创建；禁止其它
仓库、管理接口、未知 GraphQL 及重定向。可选 `execution.settings.github_auth` 使用
App 的非秘密 `app_id`、`installation_id`、`private_key_env`，不要求 App 才能测试。
宿主登录成功不能证明就绪；实际沙箱门槛失败仍零业务输入，不回退审批发布。
参见[本次认证验收](acceptance/sandbox-authentication.md)。

省略 `remote_args` 或设为空时，使用远端原生权限配置。如果明确选择一次测试的原生参数，
在配置中填写 `remote_args`，这些参数会冻结到 manifest，不能把不同配置的运行合并为
同一通过结果。已验证支持 MXC 的 Windows 主机可显式配置
`["--no-daemon","--strict-config","-c","windows.sandbox=mxc","-c","approval_policy=never","-c","approvals_reviewer=user"]`：
使用独立原生进程和原生沙箱，拒绝沙箱外审批，不写全局配置。
示例配置采用此组合。当前 Orca terminal 适配器尚无可信执行主机认证准备及同沙箱
确定性命令能力，GitHub worker 在任何业务输入前明确 unsupported；配置这些原生参数
不等于补齐该能力，远端真机及稳定性本次尚未验收。先按
[官方兼容性探针](https://learn.chatgpt.com/docs/windows/windows-sandbox)验证主机支持。
采用原生默认模型时可使用这些参数；指定 model 时，启动编译器拒绝额外 `-c` 覆盖。
出现交互权限确认、原生拒绝或 shell 初始化失败时保留失败；手动诊断中的单次确认
必须另记介入证据，不能记作无人值守通过。

## 完整 smoke 契约

固定任务改编自 SteerlineTest 的 native-101 Spec/Plan，源 commit 为
`cebdd5ab552150c49743e40593c836689ecf789b`。自有版本位于
`tests/fixtures/evidence-smoke/`，只更新 `evidence/native-101.md`，实际读取 UTC，
运行检查并提交。执行步骤：

1. 生成 `C:/srt/<随机ID>` 短路径现场（可用 `-FixtureParent` 指定父目录），构建 Runner，
   冻结 EXE/Spec/Plan 摘要、配置、来源与专属 `codex/srtest/<ID>/base`。
2. 克隆固定 SteerlineTest 源提交，提交本轮 checker/publisher 和材料作为测试 base，
   推送这个新 base。远端通过独立准备终端 fetch/cat-file，记录固定 runtime 并确认终端关闭。
3. 通过真实 Runner CLI 提交冻结 Spec/Plan，由 Runner 实施、检查、提交、报告、发布、
   核验 PR，到达 `pr_ready`。宿主不代替模型提交或发布业务改动。
4. Driver 从 GitHub fetch 实际 PR Git objects，校验原生 final、PR 身份、仅目标文件变更、
   精确格式和新鲜时间戳。远端同样核验实际 Git 内容和固定 runtime 的原会话。
5. 发送一次真实反馈，生成更新的 UTC 记录，检查新 commit、原 session/attempt/branch/PR 复用。
6. 刷新原 PR，精确绑定 head，driver 以 `--match-head-commit` 合入本轮专属测试 base。
   Runner 通过 `inspect --refresh-pr` 保存真实 merged、merge commit 和时间。
7. Fetch 合入后的 base，验证 merge commit、原 head 的祖先关系和文件内容，并在实际合入
   checkout 再运行 checker。
8. 显式 stop 原 Run，确认 `new_inputs_allowed=false` 和 `stop_status=stopped`；再停止服务并确认
   `not_running`。保留 Git 工作区、分支、PR 和原始现场供审计。

Runner 生产契约的交付里程碑仍为 `pr_ready`。合并属于测试 driver，结束判定包含 Runner
观察 merged 后的显式停止；不引入 SteerLine TaskDelivery，也不新增生产自动合并行为。

## 证据与失败

批次 `.evidence/tests-*/summary.json` 包含来源、每级结果及现场路径。
现场 manifest 记录冻结输入；`evidence/` 保存 `result.json`、`scenario.json`、CLI journal、
driver 的 Git/GitHub/checker 调用、原始/反馈 PR、合并与停止快照。Runner 自身业务数据和
原生协议日志在该现场 `s/` 中，模型 token 信息以原生记录可提供的真实数据为准。

smoke 通过要求：非零且精确的场景数、无意外 skip、真实产物正确、检查通过、PR 合入、merged
事实核验、原资源和服务均停止。L1 编译 live driver 并验证零场景失败门槛，同时运行入口
的零匹配/缺配置/重复场景反例、Windows
PowerShell checker 和发布夹具的受控反例，覆盖空 PR 列表、复用原 PR、冲突和查询失败
禁止重复创建。L1 只允许明确列出的两个 subprocess helper、三个显式 Orca 取证入口及一个
显式 MXC 工具环境诊断 skip；这些 skip 不计真机通过。后者用
`go test ./internal/execution/adapters/codex -run '^TestAuthenticationNativeToolBoundary$' -v -count=1 -args -sandbox-auth-native=<Codex绝对路径>`
单独执行，使用无效受控 canary，不运行模型、不证明真实认证/发布。
同一命令增加 `-sandbox-auth-host-broker=true` 可在全新诊断现场使用宿主登录的受限代理，
验证真实 MXC 的 Git/gh 只读能力；仍零业务输入、不证明 push/PR 创建。此入口保留原进程
身份、准备事实和关闭计数，不导出宿主 token。

未满足冻结 case 预期的失败、超时、unknown、unsupported、缺 final 均不通过；不可达停止
的 `revoked-unverified` 按其故障窗口契约独立判定，不冒充停止确认或业务交付。清理未确认
一律不通过。保留原现场，不自动
重发原生输入或替换会话。`scenario.json` 在外部动作前保存阶段和已知 Run/PR 身份；若
进程异常退出没有结果，批次失败，须用原 state-dir、request_id/Run ID 对账及停止。
修复后创建新现场；旧失败证据不改写。

这里的 unknown 指操作、结果来源或副作用无法对账。原生主代理已经核实当前回合 done、
原正文和来源完整时，可省略的 native outcome 仍按 unreported/unknown 原样保留，不补成
success；它与“原操作结果未知”分别判断，沿用既有 Orca 完成契约。

准备期限只覆盖当前阶段的准备。一个已完成的长业务回合切换报告、发布或反馈阶段时，
新输入获得自己的有限准备预算；未决原操作的对账仍沿用原期限，不因重试获得新预算。

单步诊断入口为 `new-test-fixture.ps1 -Scenario ... -Config ...` 和
`test-native.ps1 -Manifest <绝对路径>`。它们供已有前置通过证据的开发诊断使用；正式分级
验收使用 `test.ps1`，确保前置门槛和批次汇总完整。

审计 workflow 的两模型定向诊断复用同一个 native driver。配置中先显式指定执行 `model`，
再用下列命令选择不同的审计模型；省略审计 effort 时继承执行 effort。此诊断核验两项审计、
执行模型回切及实际 PR 后停止原 Run/服务，保留测试 PR 未合并，不计作普通 L2 完整 smoke。
它不改变默认 L2/L3 场景选择或生产默认发布提示词；该新 Run 冻结测试用发布提示词覆盖。

```powershell
$auditManifest = ./scripts/new-test-fixture.ps1 -Scenario local-codex.smoke -Config .evidence/live-config.json -AuditModel '<审计模型>' -AuditReasoningEffort high
./scripts/test-native.ps1 -Manifest $auditManifest
```

## 扩展 L3

场景注册在 `tests/catalog.json`。L3 是环境场景集合，增加场景无需新增测试级别。
同一流程的新真实目标可增加独立 ID、target 和超时；全跑 L3 自动包含所有注册场景，
也可用 `-Scenario` 选择子集。报告始终记录实际选择，不将子集通过表述为全 L3 通过。

新的流程在 `tests/native/run_test.go` 的 `drivers` 注册表增加 driver，并在
`new-test-fixture.ps1` 增加对应现场构造。Driver 使用公开 Runner CLI，定义可独立核验的
产物和完整停止判据。增加错误 PR/错误 runtime/陈旧产物等受控反例后，再加入真机目录。
每个 case 都要有自己的现场、Run、base/head 和结果；交付 case 必须核验本轮真实 PR，
停止 case 保存专属分支的无 PR 证据。禁止用模拟目标替代真实环境。

L3 执行稳定性契约见 [Spec](spec/l3-execution-stability.spec.md)、
[Plan](spec/l3-execution-stability.plan.md) 和[实际验收登记](acceptance/l3-execution-stability.md)。
选择场景即执行其全部变体，每个 case/repeat 独立建现场。稳定性通过还要求故障命中、
原生调用审计、原身份和完整收尾；预期停止/撤销未知不要求 PR 合入，也不能记成业务交付。

完整稳定性套件需要管理员测试宿主、Node.js、已安装的 Windows Orca CLI 公开运行文件及
已配对的真实 environment。使用 `tests/live-stability-config.example.json` 创建本地非秘密
v2 配置；保持原生自身认证。完整资格入口：

主测试保持普通用户身份，管理员终端单独运行
`./scripts/stability-admin-host.ps1 -Directory C:/srt-admin`，并将 v2 配置中的
`admin_host` 设置为该目录的绝对路径。该宿主只接受固定现场的网络 block/restore，
核验宿主 PID/创建时间与原 manifest；每次操作有独立回执和恢复 watchdog。
宿主最多运行十二小时，可在该目录写入 `stop` 文件结束；先确认本轮故障设施均已恢复。
重复验收请使用新的空目录，避免上次的 `stop` 文件使宿主立即结束。

```powershell
.\scripts\test.ps1 -Level L3 -Repeat 3 -Config .evidence/live-stability-config.json
```

每轮六场景合计十二 case，三轮共三十六次；L1/Controlled 每批完整执行一次。
`-Repeat` 默认 1，范围 1–10。`-SkipAdminRequired` 仅供用户明确选择排除管理员场景时使用，
报告保存所有排除项及 `passed_with_exclusions`，不能声明完整 L3 通过。当前最终验收要求全部执行。
