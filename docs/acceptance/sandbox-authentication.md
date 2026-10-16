# 沙箱认证专属验收

归属 [Spec](../spec/sandbox-authentication.spec.md) / [Plan](../spec/sandbox-authentication.plan.md)。
开始日期：2026-10-16。旧 L2/L3 的成功记录保持，含沙箱外审批的结果不证明本次新要求。

2026-10-16 后续修订：按用户授权，以执行主机已有 gh 登录的单仓库代理作为默认方案，
App 为可选来源。下表及此前批次段落记录早期 App 方案的历史结果，不能用于新代理版本通过。
代理版本 `codex-policy-v9` 的新结果单列文末。用户最新要求先提交 PR，再查看第二轮结果；
因此交付 PR 创建时明确保留 L2 pending，不预报通过。

| 项目 | 当前状态 | 证据/限制 |
| --- | --- | --- |
| SA01 受限认证准备 | 本机接线、受控通过；真实有效凭据未验 | App JWT/单仓库权限/期限/401/失效/目标不符已测；当前执行主机未配置 App 引用 |
| SA02 确定性沙箱门槛 | 受控及实际 MXC 负例通过；有效认证正例未验 | native command/exec 实测 AppContainer、环境、Git 目录、网络通过；无效 canary 在 GitHub 身份处拒绝，零 thread/input |
| SA03 秘密与资源清理 | 受控通过 | argv/环境隔离、有限诊断、期限、撤销、原进程退出已测；真实有效 token 的完整业务历史未验 |
| SA04/08 权限及审批审计 | 策略/受控诊断通过；真实业务全程未验 | never/MXC/固定 roots、关闭 shell snapshots；原生 history 审计拒绝提权/审批/bypass，实际业务尚未执行 |
| SA05 L2 完整闭环 | **失败，准备前置阻断** | `C:/srt/02ba5748823f`：缺少认证引用、零业务输入，无业务 PR，Run/服务均停止；最终冻结源复验另记下文 |
| SA06 测试材料清理 | 通过 | Spec/Plan、生成器、发布覆盖已清理；源 AGENTS 为产物/事实规则，无认证修复指令；旧冻结现场不修改 |
| SA07 L3 与稳定性 | **未验，能力缺口** | 现有 Orca terminal 无可信执行主机认证准备/同沙箱确定性命令能力；本次只实现了 fail-closed 阻断，远端闭环尚未实现，不计 L3 通过 |

## 首轮方案的机制诊断与实现边界（历史）

新现场 `C:/srt/authdiag-3971120298` 使用同版本 Codex 的 app-server `command/exec`，
固定 workspace-write/MXC/network enabled/never。Win32 `TokenIsAppContainer=1`，四个
Git 管理目录均读写成功，GitHub `/meta` 返回 200；同沙箱 `gh auth status` 报 token invalid、
`gh api user` 返回 401，而宿主 gh 是正常钥匙串 OAuth 登录。
Git remote 调用的 helper 在 MSYS `sh` 的 `NtCreateDirectoryObject` 上报 `0xC0000022`；
禁用 helper 后变为缺少用户名认证，不再触发 MSYS 启动错误。该仓库需要认证，不能把
匿名查询失败记为网络失败。

非秘密 `*TOKEN` canary 在该现场能够继承，因此**未证实环境过滤是本次 401 根因**。
已证实的是 Git 文件权限正常、网络正常、helper 启动边界失败和沙箱可用 GitHub 凭据缺失；
随后在新场地 `C:/srt/authdiag-2631366071` 用同一固定沙箱直接调用只读 Win32 CredRead：
精确引用 `gh:github.com:erranliu` 在宿主成功（error 0），MXC 内失败（error 5 / ACCESS_DENIED）。
探针没有读取、复制或输出 credential blob，仅保存引用、成功标记及错误码，立即 CredFree。
因此钥匙串访问边界已有直接证据，不再仅是推测。该场地进程已退出，零业务输入/远端变更。

首轮本机采用可信 runtime 签发的单仓库 App installation token，Git 使用目标 URL 范围内
的进程环境 HTTP header，gh 使用同一 GH_TOKEN；均不经过 MSYS helper/宿主钥匙串。
App 私钥不进入 worker，token/header 不进入 argv/config set、材料、库或 manifest。
模型生成代码能读取 token 环境，这是明确的信任边界，由 GitHub 单仓库权限及期限限制。
正常发布权限固定 never，不再包含自动审核后备路径；真实发布成功尚未证明。

受控 issuer 覆盖缺失/无效 key、401、无效响应、到期、错误仓库、多仓库、额外权限、
异常期限、仅签发一次及撤销一次；原生失败输出只存有限 stage，不复制 stderr/环境。
实际 MXC 负例的最新场地 `C:/srt/auth-boundary-2007950559` 使用生产 PrepareSession 路径：
环境/Git/网络检查后，受控无效 token 在 `github_identity` 处阻断，未创建 thread；
返回原 owned handle 供既有 W18 结算，最终 process absent、受控撤销确认。
这不是有效真实凭据或发布权限通过。

早期 `auth-boundary-3768020956` 的 Go 测试曾 exit 0，但 cleanup.json 的撤销为 false
（测试 HTTP server 的 defer 顺序错误）；不计完整清理通过。修复后新场地
`auth-boundary-1628824273` 以及上述最新场地均确认清理。三个场地及原始结果均保留。
最初 `authdiag-402847155` 的 Git 写探针有语法错误，后续新场地修正；不改写旧结果。

## 首轮正式入口与构建身份

执行 `./scripts/test.ps1 -Level L2 -Config .evidence/sandbox-auth/live.json -OutDir .evidence/sandbox-auth/formal-l2`。
L1 exit 0 / 197 顶层测试，6 个登记的显式 skip；driver 编译/零匹配守卫通过。
Controlled exit 0 / 12 场景。**该前置 EXE 为 `e1ca3c3f...`，在 W18 进程归属修复前编译**，
保留为首轮结果，不拿它拼接成最终源完整验收；最终源定向组件及 native 负例另行通过，
并为最终固定源启动新的正式复验。

首轮 live Run `ebe48b721394cd36ff285e8cb3f0d984`，base
`codex/srtest/02ba5748823f/base`，head
`codex/runner-ebe48b721394cd36ff285e8cb3f0d984`。
Runner `e04d07fde31710b5b93d3186010074927a44effc-dirty`，EXE SHA256
`3e0b41d189ee31c5ea42766527a62496d6c073ce89074e023a0ff467015e65ae`；
driver SHA256 `c158b3832ae520d39839d9aab2851ff2563f9dd324507471f80de0a885004785`。
该轮取证时间点重建的 Runner/driver 与 live 二进制完全相同。Codex 保持 0.159.2；未启动模型，
未改变模型或后端。

live exit 1 / 1 场景，诊断 `authentication_reference_missing`；prepare_session rejected，
零 thread、零 turn-start、零原生工具 journal；远端专属 head 不存在、PR 查询 `[]`。
只有驱动准备 base 的提交/推送，不计 worker 业务发布。Run stopped、输入撤销、服务停止。
没有实际业务 worker，不能把空工具记录写成“完整发布全过程权限审计通过”。

用户/机器环境变量**名称**检查没有发现已配置的 App key 引用；宿主 OAuth 调用
`user/installations` 返回 403，要求 App 授权 token，所以不能断言账号没有 App。
确切 blocker 是当前可信执行主机没有配置可解析的单仓库 App 引用/私钥能力。
只需提供 App ID、installation ID 和执行服务环境中的私钥变量名；不要把秘密发进聊天或材料。

证据导出见 [evidence README](evidence/sandbox-authentication-20261016/README.md)。
旧 L2/L3 的审批发布历史结果完全保留，不作为本次通过。日常安装及其既有任务未使用或修改。
在有效凭据下 L2Smoke 实际通过前不提交交付 PR。

## 固定最终源的第二轮与入口期限修复

`.evidence/sandbox-auth/formal-final/`：L1 exit 0 / 198 顶层测试，driver 编译/零匹配守卫通过。
Controlled EXE 已与最终 `3e0b41d1...` 相同，子进程实际 exit 0 / 12 场景，结束于
12:28:00（上海时间）。外层 6 分钟期限已在 12:27:07 把正式批次记录为 timeout/failed；
该轮未启动真实 L2，不把迟到子结果改写成正式通过。子进程随后退出，原始记录均保留。

查明 `test-foundation.ps1` 原有子进程期限为 8 分钟，外层却仅等 6 分钟。
`test.ps1` 现在等待 8 分钟加 30 秒结果落盘余量；不改变子测试、每调用、worker 准备
或原生业务期限，更不修改权限。修复后的新正式批次为
`.evidence/sandbox-auth/formal-deadline-fixed/`，失败结果及收尾在下方登记。

## 第三轮与最终交付边界

修复外层期限后的第三轮仍 failed：L1 exit 0 / 198；Controlled 日志仅有前 11 项完整 PASS，
最后 `TestSRRestartRequiresResume` 开始后子进程退出，没有 result.json，外层在 8m30s
后报告超时。因此该轮 Controlled 未通过、没有启动 L2，不伪造 exit_code 或完成事实，
也不继续放宽原有子进程期限。证据为 `deadline-fixed-summary.json` 和
`deadline-fixed-controlled.log`。其 Runner 仍为 `3e0b41d1...`。

超时遗留的专属测试服务 `C:/Users/lmas1/AppData/Local/Temp/sr-1442161568/runner`，
经原 state-dir、实例 `2e37edec3a643bf17004b3478dea334c`、EXE SHA 核验后通过公开 CLI 停止，
最终 `not_running`，同原 stop request 的 `last_shutdown.resources_settled=true`。
没有删除失败材料，未使用或停止日常安装/其他聊天资源。

最后对准备分支做三项定向收口：未取得任何凭据的失败不永久关闭冻结引用（是否能继续仍由
原显式 resume、原输入资格和原期限决定）；固定 GH_HOST 为 github.com，防止继承其他实例；
Git 写探针改用 Git 忽略的 `.lock` 文件名。这些变更晚于第三轮前置编译，**不拿该轮前置
代表最新源码整体通过**。最新三个相关包完整组件回归 exit 0，实际 MXC 负例在新场地
`C:/srt/auth-boundary-1736698634` exit 0：受控无效 token 在 github_identity 处拒绝、零 thread/input，
原进程退出、受控撤销确认。对应导出为 `delivery-components.log`、`delivery-native-*`。

最终源码构建 EXE SHA256 为
`caa16596e67a3771ab139ed1d538405cc3f1a58bf070623812b9dc18221f48a1`，
source 为 `e04d07fde31710b5b93d3186010074927a44effc-dirty`；逐文件摘要见
`delivery-source.json`，构建信息见 `delivery-build.json`。最新源码没有真实 L2 发布通过记录，
没有将 `3e0b...` 的旧前置或负例与新构建拼为通过。

**上述 App 方案结束时状态：实现部分完成、受控/实际沙箱负例通过；完整 L2Smoke 未通过；
L3/稳定性未验且远端准备能力尚未实现；未创建交付 PR。**
继续真实验收所缺的是执行主机上可解析的专用 GitHub App 引用/私钥能力，以及 Orca 的可信
执行主机准备/同沙箱确定性协议；正式 Controlled 前置也须在其既定期限内完成。

## 默认宿主 gh 受限代理方案（当前）

用户授权后不再将 App 当作唯一前提。默认方案从执行主机 gh 登录解析凭据，仅在可信
进程内存保留。子进程获得随机 256 位 capability，一小时有效；代理在每个请求上校验
capability、固定仓库和操作。GitHub OAuth、App 私钥及代理 TLS 私钥不进入模型环境、
argv、配置、数据库、manifest、历史或验收导出。模型代码可以读取 capability，但它不能
用于直连 GitHub，也不能通过代理访问其它仓库、管理 API、任意 GraphQL 或重定向。

Git 继续在 MXC 内运行，以 URL 范围内的进程 HTTP header 发送 capability，绕过不能
启动的 MSYS helper。其 HTTPS 代理使用当前 worker 的公开 CA 文件，不安装系统信任。
Windows gh 的进程级 CA 文件未生效，因此使用该版本已有的 `http_unix_socket` 接口；
公开配置和 socket 位于原 Git 写入根。原 gh 查询/创建请求直接由沙箱工具生成，代理
只认证转发 HTTPS，不调用宿主 Git 或 gh 发布命令；原 remote 和 PR URL 保持 github.com。
GraphQL 按 AST 和变量解析仓库/节点身份并逐字段允许；支持 PR list/create 所需子集，
不支持的命令明确拒绝。当前默认代理对 fork 仓库明确 unsupported，不扩大父仓库范围。

退出关闭两个 listener、全部连接及在途请求，清空主凭据缓冲和 capability；保留公开
证书/非秘密配置及审计计数。不撤销或修改宿主 gh 日常登录。Runner 崩溃时代理随进程退出，
capability 不能再使用。重启/恢复仍受原会话、准备期限和显式恢复授权约束。

定向组件 `host-broker-components.log` 三个相关包 exit 0，覆盖缺登录/401/错误目标/fork、
capability、期限、跨仓库、管理接口、GraphQL alias/fragment/变量越界、秘密隔离和关闭。
实际 MXC 只读正例：`C:/srt/auth-boundary-3273304199`，Codex 0.159.2、
`codex-policy-v9:0.159.2:mxc:explicit-roots:repository-auth-broker:never`。
Git 管理目录、身份/目标仓库、Git remote 和 gh GraphQL 查询通过，零业务输入、零发布。
同一个原生进程最终 process absent，代理 closed=true，非秘密计数为 Git upload-pack 1、
PR query 1、PR create 0、receive-pack 0。证据为 `host-broker-native-*` 和
`host-broker-transport-cleanup.json`；这仍不能证明远端写入和 PR 创建权限。

全部代理诊断使用独立新场地，早期失败 `native-host-broker-r1`–`r8` 保留原结果，未启动
业务。分别暴露工具环境过滤、Windows gh CA 信任、Git TLS EOF；r9 修复 TLS close_notify
后通过，r10 在最终 v9 源上复验通过。没有改写任何失败场地，也没有回退审批路径。

正式批次 `.evidence/sandbox-auth/formal-host-broker-1` 已启动，源码摘要为
`host-broker-source.json`。完整 Controlled 总预算改为 15m（外层另 30s），因为既有成功
顺序场景耗时合计超过原 8m；所有 CLI/准备/故障/场景轮询期限保持。最终构建、真实 L2
结果及原生权限/审批审计在批次完成后补录；当前尚不计 L2Smoke 通过、不创建交付 PR。
L3/稳定性仍未验，远端可信准备能力尚未实现，不能用本机结果替代。

### 代理首轮真实发布失败及兼容修复

`formal-host-broker-1`：L1 200 顶层测试、6 个登记 skip、完整 Controlled 12 场景均通过。
Runner SHA256 `6e006f109869d924e94ba8eeff7a7cac5d51204c60dae49c04baba9eb15f4e3a`，
driver `7caf5949d939d86e44d6b75db6b9aab44e9996700be3b923e9adb06fc84e9c95`。
新现场 `C:/srt/b71076c814f1`，base `codex/srtest/b71076c814f1/base`，revision
`1d9c66aae71915376eb91ba53e84dc5c9af4a932`。Run `3e5fb02cd39adb442a6c4f637c0c85b3`，
head `codex/runner-3e5fb02cd39adb442a6c4f637c0c85b3`，session
`01a1433a-c857-7fc0-bf8c-02a784ad118e`，原进程 PID 34540。

worker 通过实际沙箱认证门槛，实施/验证/提交后在普通沙箱成功 push。创建前的 gh 仓库
查询被代理过窄的只读字段白名单拒绝，HTTP 403。模型保持原结果未知并阻断，没有请求
审批或绕过；没有重发旧业务。代理关闭计数为 receive-pack 1、PR create 0、denied 1；
对原 base/head 的独立只读 PR 查询返回 `[]`。Run、原进程和服务停止，代理 closed=true。
**本轮 L2 失败保持失败，不以 Git push 成功计完整发布通过。**

原生 rollout 首次用旧审计器读取时因未支持 code-mode custom_tool_call，返回 tools=0 /
evidence incomplete，并未当作零审批通过。补齐格式后对同一 SHA256 的原始文件重新只读
审计：3 contexts、7 native tool calls、escalations/approvals/bypasses 均 0。对自由代码形式
的 exec 采取保守规则：出现提权 capability 即拒绝；未知 custom tool 格式也拒绝。
实际 OS argv 另证 MXC、never，无 token/header config set。见 `broker-first-real-*`；
原失败审计及原始 rollout 均保留，不改动历史业务结果。

兼容修复仅增加 gh 所需的仓库基础元数据和 PR 关联用户 display name；仍按 AST 和变量
约束根仓库、固定 repo node ID、允许操作和非 fork 边界，不允许任意节点/其它仓库查询。
新增单元回归和真实 MXC 显式 head 的 create dry-run（不会 push 或创建 PR）。
`C:/srt/auth-boundary-4148347561` 通过，PR query 3、upload-pack 1、receive-pack/create 均 0，
原进程退出、代理关闭。native r11 的查询失败保留；r12 是修复后的新现场通过。
证据为 `broker-compatible-*`。只读 dry-run 仍不是 PR 创建权限通过。

最终新批次 `.evidence/sandbox-auth/formal-host-broker-2` 正在执行；源码独立摘要见
`broker-compatible-source.json`。不复用首轮 PR/head，不拿首轮构建拼接最终通过。
