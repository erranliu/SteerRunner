# 独立配置与测试配置验收

日期：2026-10-19。[Spec](../spec/independent-configuration.spec.md) / [Plan](../spec/independent-configuration.plan.md)。

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
