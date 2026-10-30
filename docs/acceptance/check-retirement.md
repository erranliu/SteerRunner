# 旧 environment-check 退役

调查基线：`af29ee471587d7eef41a5eb9ff7d52aefb815183`，2026-10-29。
实现与验证在独立工作区进行；不部署、不访问安装现场及 SteerLine 状态。
本工作区初始无自有旧测试现场，因此没有需要用原版本收尾的 Check。
Runner 只负责普通 Run 和通用 result 字节通道；检测意图、版本化材料/脚本及结果含义由项目与 SteerLine 持有，Runner 不解释 ready。

## 删除前盘点

| 调用链 | 旧业务及资源 | 处理 |
| --- | --- | --- |
| CLI / host | environment-check start/inspect/stop、check-id、绑定命令、服务分发 | 删除 |
| contracts / resources | Check 请求、回执、报告、停止核验字段、schema、fixtures、能力矩阵、catalog | 删除 |
| application / runstate / storage | Check actor、worker、nonce、操作记录、原请求索引、checks 表、commands.check_id | 删除 |
| 服务生命周期 | 启动恢复 Check、Shutdown 结算、verify-stopped、takeover Check 投影 | 删除；Run 结算保留 |
| Codex | EnvironmentSession、固定 probe 工具、提示词、checkWriteGate、remote Check actor | 删除 |
| Orca / agent helper | LocalTerminalEnvironmentSession、--environment-session | 删除 |
| 客户端 / skill | Check action、原请求查询重定向、公开调用说明 | 删除 |
| 测试框架 | Check 专用组件/集成/真实测试、case 注册、配置校验、结果判定与只读重验入口 | 删除 |

## 共享代码的生产消费者

| 保留内容 | 真实消费者 |
| --- | --- |
| authentication runtime / broker / transport | 普通 Codex Run、受管 Orca 本机执行的 Git 与发布授权 |
| 有限远端诊断（从旧 Check 文件移出） | steerrunner-agent --configuration 普通远端 Run 失败处理 |
| 原生 turn 完成校验（移为 validateTurnCompletion） | 普通远端 Run bootstrap 和原生模型验证；不保留 EnvironmentCall |
| backend HostEnvironmentPort / reliable environment 准备 | 普通 Run 的工作区、权限、原生输入准备 |
| Target.Environment、Orca terminal/control helper | 普通 Orca Run 路由、原终端观察、停止和结算 |
| 输入预算及来源绑定 | 普通 Run、内容流程、parser、原生输入与原请求对账 |
| tests/fixtures/evidence-*/check.ps1 | 普通开发/稳定性测试的项目证据校验脚本 |
| progress-file / result-file / artifacts | 普通 Run 的现有字节通道和通用证据，不扩展能力 |

## 收口约束

新状态格式整体拒绝旧格式；无迁移、兼容读取、双写或资源停止假设。
SteerLine 必须同步移除旧 Check 启动绑定、占用检查、接管/清理、HTTP/UI 消费者。
旧客户端不能直接连接新版本；由原版本先结算旧现场，再使用新独立状态目录。
本变更不解决本机 Orca 普通 Run 的既有阻断。

## 公开契约、状态与打包

- 产品版本为 `0.2.0-dev.4`，资源版本为 3；资源摘要随资产、模型输入契约和能力目录改变。
- CLI 信封仍为 `steerrunner-cli/v1`，普通 Run/operation/owner/result 契约未改。它不是旧 Check 客户端的兼容保证。
- `environment-check start/inspect/stop`、`--check-id`、`validate --kind environment-check` 均在创建状态前拒绝。CLI/host 分发、完整服务绑定命令集合和客户端 `Check` action 全部移除。
- `environment_check`、`environment_check_schema`、`environment_check_fixtures`、对应 validation kind/combination、固定工具/提示词和 `environment.task` 输入来源均消失。
- `verify-stopped` 只输出 Run 证明，不再包含 `checks`；takeover 只投影支持的 Run 和原 submit 请求。Run 的原身份、parser、owner、Orca scope、授权与资源结算核验完整保留。
- SQLite 仅新建/接受 `steerrunner-state/v5`、`user_version=5`；删除 `checks` 表和 `commands.check_id`，删除旧升级器及兼容 reader。旧 v3/v4 整体返回 `unsupported_schema`，不能得出资源已停止或可接管。
- 包清单结构保持 version 2，客户端契约版本升为 2；配置文件与 Viewer 快照格式未改。`assets/*` 内嵌清单随三份专属 JSON 删除自动收缩，客户端/skill 按清理后的源码打包；不携带另一个替代检测资产。
- 受控目录从 26 减为 25 个顶层测试；四个旧 Check L3 case 退出，当前 L3 为 25 场景/35 case，全通用目录 30 场景/40 case。其余 Run case、真实 fixture checker 与严格计数/非零/清理断言保留。

## SteerLine 必须同步移除

本轮未读取或修改 SteerLine 源码/安装；以下是按 Runner 删除契约给出的配套清单，而非 SteerLine 已完成的声明。

1. 旧 Check 启动绑定、原请求落盘/对账路由、实例绑定中的三个命令；不得改成另一种检测专用 Run。
2. start/inspect/stop 的 CLI/HTTP 调用及 check_id selector；旧拒绝回执和 `data.check` 报告消费者。
3. 能力读取中的三个 `environment_check*` 键、旧 validation kind、workflow=null 的 Check 组合及固定十二项报告解释。
4. 占用检查、活动 Check actor/状态、服务重启接管与清理职责，以及 `verify-stopped.data.checks` / takeover `kind=check` 消费者。
5. HTTP/UI 的旧检查按钮、状态、报告、资源残留及清理展示；旧安装 pin/状态目录切换必须遵守 v5 边界。

原版本仍负责既有现场的正常收尾；新版本不提供旧命令回退或旧记录兼容读取。项目版本化文档/脚本、SteerLine 固定提示词提交和 ready 判定不在本次变更内。

## 真实失败与测试框架同步

所有首轮失败均保留，未改写为通过：初始沙箱组件执行受 socket 权限/长临时路径影响；删除后的编译及固定验证找出了残留 reader 名称、计数和非法 submit 测试输入，随后修正并使用新目录重跑。

完整 Controlled 又暴露调查基线自带的 fixture 漂移：生产 `prompts.json` 的六个当前前缀不被旧替身识别，真实 `turn/start` 返回 `fixture received an unknown stage`。只同步自有 `StagedBusiness` 和 native fixture 的固定前缀，真实 Git 动作、结果与所有断言均未改；未知提示仍拒绝。新增窄回归实际验证 commit、push 原 HEAD、原 PR URL 报告及审计 report 零修复动作。生产提示词未修改。

固定 L1 的另一基线遗漏是 `TestShortInputProbeInspect`：它是原有显式现场诊断，默认跳过却未登记。按既有同类规则增加精确名称，并在执行前禁止其现场环境变量；sentinel 反例证明默认 L1 不触达现场。跳过单列，不计为真实后端资格。独立复审认可这两项测试框架同步。

中间独立批次也出现过 `planned-recovery` 夹具的 context deadline 和 application 包固定 12 分钟总超时。原报告及未完成项保留；未删除对应断言、缩减选择或延长固定测试预算。sentinel 自身最初的期望消息文本错误也保留了失败报告，修正为原有完整精确消息后在新目录重跑。

旧失败 Controlled 批次在产生原失败断言后中止。仅依据该批 source-identity 与 caller journal 停止自有 harness，再正常 stop 原 Run、stop 原服务，离线 `verify-stopped` 确认两个原目录资源结算；保留原状态、请求、日志及独立收尾证明。未删除失败现场以制造通过。

## 剩余引用与独立复审

当前生产源码中没有旧 Check 标识、专用 actor/执行器/helper 或存储双轨。仍使用 environment 的 Orca 路由和普通 Run 准备端口在上表有实际消费者。`CloneIsolated` 原先仅由 Check 使用，已连同专属测试删除；旧诊断枚举也按生产来源逐项裁剪。

能力目录的旧 Check `nil/nil` workflow 成功分支亦已删除。所有组合必须携带有效 Run workflow，缺 workflow 的查询直接拒绝；其它 fixture/config 的可选 workflow 保留真实的缺配置诊断消费者。

剩余旧业务名称仅用于明确拒绝旧契约的负例、本退役说明，及带日期/历史边界的混合验收和冻结证据。历史证据不会被当前测试、运行时或打包消费，也不代表当前能力。[逐文件索引](evidence/check-retirement-20261029/remaining-references.md)列出 232 个文件、4089 个协议标识匹配行及解释；[JSON 索引](evidence/check-retirement-20261029/remaining-references.json)保存全部行号和 SHA-256。索引不自扫描本次退役证据包。

独立只读复审发现并复核了共享 bootstrap 校验、失去消费者的 clone helper、诊断枚举、状态拒绝和现行文档矛盾；修复后未发现旧 Check 后台实现、契约空壳或普通 Run 消费者误删。复审单独核实了夹具同步未放宽断言，未把零匹配、skip 或改报告当作通过。最终测试结论以原始结果为准。

## 最终验证证据

最终 Runner SHA-256：`7b57b06472ae7c36befa71222cdc6e61f7bc2fda410adee6d6aaf32e58fdc2b5`。
基于上述调查提交的未提交工作区修改独立构建，未部署或安装。

| 验证 | 原始结果及范围 |
| --- | --- |
| 固定 L1：`scripts/test.ps1 -Level L1`，新目录 `C:/srt/cr29e/fixed` | [passed](evidence/check-retirement-20261029/l1-summary.json)：712 个顶层测试；[框架 24 项](evidence/check-retirement-20261029/framework-result.json)；终端扩展 29 项；native driver 编译通过；零匹配拒绝守卫通过 |
| 完整 Controlled：`scripts/test-foundation.ps1`，按当前 catalog 的全部 25 个精确名称选择，新目录 `C:/srt/cr29e/controlled` | [exit 0 / 25 项](evidence/check-retirement-20261029/controlled-result.json)，0 skip；[原日志](evidence/check-retirement-20261029/controlled.log)；[精确 pattern、源码/EXE/fixture 身份](evidence/check-retirement-20261029/controlled-source-identity.json) |
| 客户端：`scripts/test-client.ps1`，新目录 `C:/srt/cr29e/client` | [32 项受控断言](evidence/check-retirement-20261029/client-result.json)，绑定最终 EXE；live_execution=false |
| 公开契约与旧状态拒绝 | [三项公开查询及四项旧格式负例](evidence/check-retirement-20261029/public-contract-result.json)；v3/v4 各自 verify-stopped/takeover 均 exit 3，原库摘要保持；[当前能力原件](evidence/check-retirement-20261029/capabilities.json)无旧能力 |
| 固定提示词夹具回归 | [原始 Go 结果](evidence/check-retirement-20261029/fixture-regression.jsonl)：真实临时 Git 提交、push 原 HEAD、原 URL 报告、未知输入拒绝及 audit report 零修复 |
| 独立 Runner/Viewer 构建与打包 | [构建回执](evidence/check-retirement-20261029/runner-build.json)、[包清单](evidence/check-retirement-20261029/package-manifest.json)、[16 文件完整性与摘要](evidence/check-retirement-20261029/package-integrity.json)；未运行安装器 |
| 原失败保留与收尾 | [原夹具失败](evidence/check-retirement-20261029/baseline-fixture-failure.log)、[原 L1 失败](evidence/check-retirement-20261029/initial-fixed-failure.json)、[未登记 skip](evidence/check-retirement-20261029/unexpected-optin-skip-failure.json)、[中间超时](evidence/check-retirement-20261029/intermediate-timeout-failure.json)及[两个原 Run 的结算](evidence/check-retirement-20261029/failed-batch-settlement.json)；未修改原报告 |

完整 [汇总](evidence/check-retirement-20261029/verification-summary.json)、[证据清单](evidence/check-retirement-20261029/evidence-manifest.json)及[独立复审](evidence/check-retirement-20261029/independent-review.md)随本记录保存。大体积 JSONL/原始 caller journal 保留在清单给出的本轮私有目录，并绑定大小和摘要。

Controlled 覆盖普通 Run 提交/观察、原请求与输入归属、progress/result 写入与封存读取、停止、正常/强杀重启、不同资源摘要接管、原创建回执与 pending stop，以及权限/认证/Orca 受控协议和通用证据通道。17 项显式现场 opt-in 默认 skip 单列，不计为通过或原生资格。未运行真实 L2/L3，不授予新的本机/远端 Orca 资格；本机 Orca 通用 Run 的既有阻断没有因本轮清理被宣布解决。
