# 沙箱认证实施 Plan

1. 分别诊断 MXC 的 Git 文件写入、helper/钥匙串、环境过滤及网络，保存脱敏原生证据。
2. 在现有 Codex runtime 中增加执行主机 gh 登录的受限认证代理作为默认方案。
   宿主 OAuth 只留在可信进程内存；worker 只继承随机、有限期、单仓库的代理 capability。
   代理仅接收 worker 发起的 Git smart HTTP、身份/目标仓库查询和 PR 查询/创建，
   拒绝其它仓库、管理接口、未知 GraphQL、重定向及越界请求；不运行宿主 Git/PR 发布命令。
   Git TLS 信任仅通过该 worker 的公开证书文件配置；Windows gh 使用原生已支持的
   `http_unix_socket`，socket 放在已有 Git objects 根，配置只包含公开路径。
   不安装全局证书，不扩展写入根，不修改 gh 或 Codex 版本。
   2026-10-19 移除 App 签发及私钥管理；严格拒绝旧配置，不静默切换身份。保留每 Run 独立 gh broker、到期与停止清理。
   在原 app-server 的准备阶段执行确定性沙箱探针，不增加业务输入或替代 session。
3. 固定 never 审批，删除恢复认证/提权提示。保留 W16 对账和 W18 原树清理；
   到期/环境失效阻断，停止撤销本 Run 的 capability 并关闭 broker，保留宿主登录及其他 Run。Orca 缺确定性能力时在业务前阻断并记录缺口。
4. 清理可执行夹具、生成器和覆盖提示。更新准备/异常/验证矩阵及正式 live 审计。
5. 定向测试后通过正式分级入口运行 L2；具备远端能力及认证后运行 L3 对应闭环。
   环境缺失保留失败现场，不回退审批发布；不将受控通过写作真实通过。

## 测试计划

所有结果登记 [专属 acceptance](../acceptance/sandbox-authentication.md)。
完整受控集十二个顺序场景的已有成功用时合计超过 8 分钟；将整体 harness 预算对齐
为 15 分钟、外层另留 30 秒记录延迟。CLI/准备/故障注入/场景轮询期限均保持。

| 验收项 | 级别/场景及命令 | 环境/时点 | 通过条件 | 证据 |
| --- | --- | --- | --- | --- |
| SA01/03 | `go test ./internal/execution/adapters/codex -run 'Authentication|SandboxReadiness|WorkerRuntime' -count=1` | 隔离 HTTP/进程夹具，修改后 | 缺失/失效/目标不符/权限超界拒绝；无秘密 argv/日志/文件；撤销及到期 | `.evidence/sandbox-auth/targeted.log` |
| SA01/03/05 | broker 定向用例及真实 MXC 只读探针 | 新现场，正式 L2 前 | 宿主 token 不继承；跨仓库/未知操作/错误 capability/过期拒绝；Git/gh 实际工具经代理访问原仓库；关闭代理后 capability 失效 | broker 组件结果及新 native 场地 |
| SA01/05/08 | 同 native 入口加 `-sandbox-auth-host-broker=true`；原生 JSONL 审计回归 | 新现场、真实业务前 | gh PR list 与显式 head 的 create dry-run 查询兼容，零 push/create；AST 仍拒跨仓库；审计包含 function_call 与 code-mode custom_tool_call，缺证据/未知格式不通过 | `native-host-broker-r12.log`、组件结果及现场只读/清理事实 |
| SA02/04/07 | lifecycle/controlled 准备反例，正式 L2 前置 | 新现场，业务前 | 环境过滤/沙箱失败零 turn/start；原身份/恢复/授权不变；远端无能力阻断 | 正式批次 L1/controlled |
| SA02/03/04 | `go test ./internal/execution/adapters/codex -run '^TestAuthenticationNativeToolBoundary$' -v -count=1 -args -sandbox-auth-native=<native.exe>` | 新 `C:/srt/auth-boundary-*`，定向后 | 实际 MXC/环境/Git/网络通过；受控无效 canary 在 GitHub 身份处拒绝；无业务输入、原进程退出及撤销回执 | 现场 evidence、`.evidence/auth-investigation/native-boundary*.log`；不计真实认证通过 |
| SA06 | framework/fixture 检查，同正式前置 | 生成器更新后 | 无审批/认证修补任务指令，保留 repo/base/head/产物/未知结果检查 | 正式批次 framework |
| SA01–06/08 | `./scripts/test.ps1 -Level L2 -Config .evidence/sandbox-auth/live.json`，local-codex.smoke | 本机 Codex 0.159.2，新专属 base/head；定向通过后 | 同构建实现→发布→反馈→合并→stop；原生 never/MXC、全程零越界审批 | 批次 summary、新 `C:/srt/<id>` manifest/native/audit/cleanup |
| SA07/08 | `./scripts/test.ps1 -Level L3 -Config <remote-config>` | 实际远端 Orca，各 case 独立现场；本机前置后 | 每个已注册稳定性 case 各自故障/身份/收尾契约；同权限审计 | L3 summary/原始 case；缺能力列未验 |

只读检查证明可见身份、仓库读取及本地 Git 文件能力，不能证明 PR 创建、远端写权限、
分支保护允许或完整业务成功；这些必须来自真实发布回归。无凭据时仍执行正式入口，
保留业务前 blocker 和清理证据，不能标记 L2 smoke 通过。
