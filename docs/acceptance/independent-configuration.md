# 独立配置与测试配置验收

历史构建验收记录：以下 Check、旧状态升级和已删除源码的描述仅说明当时构建，不是当前入口。2026-10-29 已退役旧业务，当前边界见[退役记录](check-retirement.md)；原始失败、资源与结算证据保持。

> Workflow/2 组合增量：保留本页原构建事实；当前基线、manifest 与严格公开校验使用 development/2，移除已废弃的反馈资格参数，配置读取显式使用 UTF-8。最终配置/CLI/注册需求与框架、原生配置冻结接线的复验见[组合验收](evidence/rebase-control-20261019/README.md)。

日期：2026-10-19。[Spec](../spec/independent-configuration.spec.md) / [Plan](../spec/independent-configuration.plan.md)。

以下保存首次实施验收；实现审计后的修复和最新主线构建复测另见末节，原构建结果不覆盖为新构建通过。

本轮按用户授权实施 Runner 仓库，保持生产 CLI v1、写信封、原生产 gate、存储与冻结恢复路径。
Runner 发布静态能力／schema 并校验输入；SteerLine 管理实际环境、配置方案和选择；测试框架
独立读取新测试文件、按唯一 registry 装配与调度。未修改 SteerLine，不能代记其集成验收。

## 来源与范围

- 源码基线 `59c7597ac8056fc5c29ad622034e10e74e4395de`，本工作区 dirty；验收针对本次构建，不继承历史结果。
- 原始证据根：`.evidence/ic-20261019-014416/`。正式真机批次：`acceptance-r2/`，完整日志 `acceptance-r2.log`；最终 L1 单独位于 `l1-final/`。
- Windows/amd64、Go 1.26.4；本机原生 Codex 0.159.2，配置执行模型 `gpt-6-luna`、effort medium、sandbox。
- 只配置 `local-codex`，固定 development/1，显式绝对 native 路径，无提示词覆盖；使用专属 `erranliu/SteerlineTest`。
- 本次请求完整 L2、L3，共 30 case。配置齐备：L2 1、L3 3；L3 另 26 项缺可选配置，须记 unavailable。

## 实施与兼容

新增 `capabilities --offline`、三个裸对象 validate kind、`run_input` schema、有限能力目录和来源身份。
validate 的 gate 判定／fault／退出码沿既有生产契约，schema 只补诊断；保留旧字段／schema 键、
原 submit／检查 start 信封和 request_id。能力目录不作为旧请求的新白名单。
修正旧发布 schema 对 `access_mode:""`、`github_auth:null` 的额外限制，使之匹配原 gate；没有收紧旧准入。
在线来源由实际响应服务附带，离线 SHA 来自当前 EXE；生产资源摘要包含新静态声明，不含测试或实例身份。

新测试格式为 `steerrunner-test-config/v1`。原 v1／v2 扁平测试配置明确拒绝，替换为新 example／外层 schema；
原冻结现场仍由原构建结算。证据输出复用 `-OutDir`，没有再增配置管理器或环境就绪状态机。
最终补充了批次摘要校验和报告集合／失败优先级回归。测试夹具仅支持本仓既有 GitHub command
发布取证；其他产品合法 provider 明确记为 case 配置不符，生成器保留认证引用并拒绝自动换 provider。
这些补充属于测试消费／报告，Runner 交付 EXE 与受控／真机待测 EXE 的 SHA 一致。

## 验证结果

| 项目 | 结果与证据 |
| --- | --- |
| 定向 Go 契约／CLI／资源／testkit | 通过；包含原 gate 对照、未知扩展标识、fault 优先级、裸对象／误传信封、UTF-8／重复键／大小／I/O、schema 可解和来源静态摘要 |
| 配置与框架回归 | 通过；静态不探测工具、旧格式拒绝、全缺配置 incomplete／零派发、独立已配项执行、非完整覆盖判定 |
| 完整 L1 | 最终回归通过：307 个 Go 顶层测试、28 个 Node 扩展测试、framework、live driver 编译及零匹配门；7 个明确 opt-in／subprocess helper skip 不计真机通过。原始报告为 `l1-final/summary.json` |
| Controlled 16 场景 | 全部通过、无 skip；包括旧写接口／冻结／实例绑定、审计、多 Run、环境检查及新增在线来源断言 |
| L2 本机基线 | 1/1 通过，生产默认提示词、真实 Git/gh、反馈更新、PR #564 合入及完整停止／认证收尾；同构建本机基线成立 |
| 已配置 L3 | 3/3 通过：本机 feedback（PR #561 合入）、multirun（独立 PR／会话、停止与反馈隔离）、sandbox 环境检查（PR #556 关闭及完整收尾） |
| 完整 L3 | 请求 29，实际通过 3、不可用 26、实际失败 0、显式排除 0；complete_level_passed=false。L2+L3 批次为 incomplete／exit 1，原因是缺配置 |
| SteerLine 消费 | 本仓未运行；由调用方独立验收，不能宣称该客户端已通过 |

IC-01／02／04／05／06／10 的静态反例由新 Go／framework 测试覆盖；IC-03／07 沿完整
Controlled、冻结配置摘要及原生 manifest／停止断言；IC-08／09 必须引用下面实际结果，不能用静态检查代替。

| 验收项 | 本仓结论 |
| --- | --- |
| IC-01 | 通过：目录／gate／就绪职责分开，保留旧扩展输入，schema 与 gate 对照通过 |
| IC-02 | 通过：三种裸对象、诊断与无服务副作用；Controlled 和实际运行服务均证明在线／离线静态契约与 EXE 摘要一致，在线含真实 instance_id |
| IC-03 | 通过：生产依赖不含 testkit；原冻结／幂等／恢复回归通过；最终 helper 仍接受真机批次同一冻结配置 |
| IC-04 | 通过：配置约束、原生 Git 前置及同构建本机 L2；基线资格独立于完整 L3 |
| IC-05 | 通过：唯一 registry、完整角色／模式、检查不强带 workflow、基线不成为其他链路硬依赖 |
| IC-06 | 通过：零派发、独立派发、失败优先、全缺项、显式排除／零执行及逐级集合覆盖回归 |
| IC-07 | 通过：原可靠执行断言、同批配置／构建／需求摘要及改动快照拒绝；原始报告保留 |
| IC-08 | 通过：默认提示词与真实 Git/gh 发布、反馈、PR #564 合入、Run／服务停止及认证回收 |
| IC-09 | 本机已配置 L3 3/3 通过；完整 L3 缺 26 项配置，不授予完整资格 |
| IC-10 | Runner 侧通过：原写形状／默认／主 fault／实例绑定及旧字段兼容回归；SteerLine 客户端自有消费验收未运行 |

便携证据在 [evidence/independent-configuration-20261019](evidence/independent-configuration-20261019/artifacts.json)：
[能力与在线来源](evidence/independent-configuration-20261019/capability-summary.json)、
[最终 L1](evidence/independent-configuration-20261019/l1-summary.json)、
[Controlled](evidence/independent-configuration-20261019/controlled-result.json)、
[框架](evidence/independent-configuration-20261019/configuration-verified.json)、
[环境检查](evidence/independent-configuration-20261019/environment-check-scenario.json)、
[多 Run](evidence/independent-configuration-20261019/multirun-scenario.json)、
[反馈](evidence/independent-configuration-20261019/feedback-scenario.json)、
[L2](evidence/independent-configuration-20261019/l2-scenario.json)、
[完整真机清单与不可用原因](evidence/independent-configuration-20261019/live-summary.json)。原生二进制、完整日志与冻结现场保留于原路径。

真机批次实际执行并通过 4/4 case；L2 的 complete_level_passed=true、baseline.l2_verified=true。
便携 live-summary 保留原结果及原报告 SHA，仅按最终报告器从既有 case 记录补充 requested／excluded／
unavailable／executed ID 集合，未重分类结果。原批次前置 L1 为 305 项，最终源码的 L1 回归为 307 项；两份原报告均保留。

缺配置的 26 项按现有 registry 分组为：Orca 本机 8、Orca 远端 16、本机审计 1、本机 full_access
环境检查 1。没有猜测或自动发现部署、替换目标／模型或缩小原请求范围。本轮只证明本机基线及上述三个本机 L3。
环境检查 PR #556 关闭；feedback PR #561 与 L2 PR #564 合入；multirun 的两个 PR 按原 case 预期保留未合入，两个 Run／服务均已停止。

## 失败与修正留证

`acceptance/` 首次完整入口发现 PowerShell 有序字典被参数类型转换成副本，覆盖结果未写回；
已改用 IDictionary。`qualification/` 的合成调度 worker 缺 driver_sha256，在继承 StrictMode
时触发既有构建一致性检查；已补齐真实要求的合成身份字段。两次均在框架阶段结束，未派发真机。

`final/` 在原生派发前主动中止，用于修正上述旧 schema 与 gate 差异；保留 interrupted 状态。
这些批次均不计通过。`framework-initial/` 是早期定向检查，正式资格仍取本次完整批次。

## 实现审计与主线复测

用户授权再次审计并提交 PR；本轮人工核对 Spec／Plan、生产增量与内部测试调用路径。
分支同步到主线 `a3dc15d`（包含 PR #27 的停服取证），本轮证据根为
`.evidence/ic-audit-20261019/`。生产写 gate、信封、默认值、冻结／存储／恢复路径没有因配置管理而改变。

| 确认问题 | 修复与回归 |
| --- | --- |
| worker 预检与生成器重读之间，配置可能偏离批次原摘要；旧 helper 可能使用不同需求 | 生成器在实际解析后对照批次配置／Runner／helper／需求身份；校验 helper 内嵌 registry；冻结 catalog；错误摘要和旧 helper 反例在准备前拒绝 |
| 前置失败／调度 checkpoint 会漏掉未派发请求，queued 被计作执行 | 合并完整计划与实际记录，保留 unavailable／excluded 原因；queued 不计执行或通过，失败仍优先；真实入口前置失败与结果真值表覆盖 |
| L2 专属 Git 前置放在批次层，会阻断独立 L3 | 移入 L2 worker 的准备步骤，保留每 case Git 边界日志；实际 L2 准备失败／独立 L3 调度反例覆盖，不改变原检查内容 |
| 检查附带合法审计 workflow 时可能误选审计 driver | driver 由注册确定，检查 manifest 只投影 execution；配置携带的 workflow 仍走原公共校验 |
| 静态可用性与已有现场约束不一致 | 提前报告已知不支持的检查 App 认证、非规范 GitHub fixture URL、准备要求的绝对路径及混合角色 helper 不一致；Run 支持不受检查限制影响，静态不探测工具 |
| Orca manifest 将原公开的空模式／省略模式拒绝为非 sandbox | 仅在只读匹配时采用原 sandbox 默认，不改写冻结 execution；补 manifest 默认值反例 |
| 主线严格空清单契约与旧夹具不一致，新增 native opt-in 未纳入允许 skip 集 | 将测试空清单从 null 修为 []，保持生产严格性；显式登记新探针的默认 skip，仍禁止 L1 开启 native opt-in |
| 指南仍含已删除的扁平配置字段及旧 case 数量 | 改为统一公开字段路径，并与唯一 registry 当前 22 个 L3 场景／29 case 对齐 |

上述修复沿现有接口、case 注册、queued 状态及报告器完成，没有增加生产 profile／环境库、就绪缓存或另一套调度状态机。

| 最新复测 | 结果 |
| --- | --- |
| 完整 L1 | 318 个 Go 顶层测试、28 个 Node 测试通过；live driver 编译与零匹配门通过。8 个已登记 opt-in／subprocess skip 不计真机通过 |
| 最终定向与框架 | 资源／CLI／contracts／testkit 最新反例通过；完整 framework 通过，包括真实 L2 准备失败与独立受控 L3 继续调度 |
| Controlled | 16/16，无 skip；与真机使用的 Runner EXE SHA 相同 |
| L2 | 1/1；worker 内原生 Git loose/packed／跨仓库边界通过；默认生产提示词、反馈更新、PR #577 合入与完整停止／认证收尾通过 |
| 已配置 L3 | 3/3：环境检查 PR #573 关闭且 owned 对象收尾；反馈 PR #576 合入；多 Run PR #574／#575 按原 case 要求保留未合入，两 Run／服务均停止 |
| 完整范围 | L2 请求 1、通过 1；L3 请求 29、通过 3、unavailable 26；实际 failed=0、excluded=0。批次 incomplete／exit 1；baseline.l2_verified=true，L3.complete_level_passed=false |

正式批次为 `acceptance-r2/`，MaxParallel=2，四个实际 case 的 Runner／native driver／冻结配置身份一致。
Runner SHA 为 `70bddb0197bbad4270b81d458636c36772119a8bf1275741e1d2c495a753bb30`，
构建源为 `3c7df3d` 加本轮工作树改动；精确身份及源码摘要见便携证据。
缺项仍为 Orca 本机 8、Orca 远端 16、本机审计 1、full_access 检查 1；不能授予完整 L3 资格。
SteerLine 自有客户端端到端未在本仓执行，不能代记通过。

最新便携证据：[构建身份](evidence/independent-configuration-audit-20261019/artifacts.json)、
[L1](evidence/independent-configuration-audit-20261019/l1-result.json)、
[Controlled](evidence/independent-configuration-audit-20261019/controlled-result.json)、
[框架](evidence/independent-configuration-audit-20261019/framework-result.json)、
[L2 失败隔离反例](evidence/independent-configuration-audit-20261019/dependency-isolation.json)、
[完整真机报告](evidence/independent-configuration-audit-20261019/live-summary.json)、
[L2 原生 Git](evidence/independent-configuration-audit-20261019/l2-git-boundary.log)。
文本副本统一为 UTF-8／LF；原报告 hash 与便携副本 hash 分别登记，不改写结果或重新分类。

本轮留证与适用范围：

- `acceptance/` 在模型派发前因主线严格空清单与旧 null 夹具不符而失败；修复为 [] 后复跑。该失败报告仍保留 30 个请求、26 unavailable、0 executed，基线 false。
- L1 后补充的可选配置反例以最终定向 Go／framework 复测覆盖。批次冻结的 helper 未被替换；最终 helper 对同一冻结配置复评仍恰为同四项 available，见[身份与选择对照](evidence/independent-configuration-audit-20261019/final-helper.json)。
- 正式批次的父脚本启动时仍含原批次层 L2 Git 前置；其已通过且原日志保留。L2 worker 实际另通过移入后的同一检查；最终父入口已删除公共 L2 门，失败隔离由最终完整 framework 的反例验证，没有用旧批次层通过代替新 worker 取证。
- `boundary-final/` 是一次以相对 Evidence 调用框架子脚本的手动准备失败，未派发模型，不计通过；随后从正式 framework 入口使用绝对证据目录重跑，`framework-after-boundary/` 全部通过。
