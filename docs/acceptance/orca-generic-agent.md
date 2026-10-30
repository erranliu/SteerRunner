# 通用 Orca Agent 本机接入验收

历史构建验收记录：以下 Check、旧状态升级和已删除源码的描述仅说明当时构建，不是当前入口。2026-10-29 已退役旧业务，当前边界见[退役记录](check-retirement.md)；原始失败、资源与结算证据保持。

日期：2026-10-18；工作分支 `codex/orca-terminal-agents`，基线 `7c48969`（本机 Orca 接入合并后）。验收构建来自当时未提交的工作树，以本文登记的构建 SHA 绑定。

本次只改 Runner 及其配套进程内扩展。Orca 1.4.224 不改源码、安装包、命令或全局配置；没有 structured、SDK 会话驱动、RPC、ACP 或 headless 替代路径。执行仍是原 Orca terminal 内的原 agent TUI。

## 实现与适用范围

- Pi 0.87.1：一个受管扩展集合，通过正式事件核对输入、原 user/assistant entry、最终原因及 agent_settled；原生 read/write/edit/powershell 工具支持默认 sandbox 和显式 full_access。
- DSH 0.2.1-rc.2 + 社区 dsh-tui 0.16.1：原 TUI、Run 私有固定 profile、正式 Cordis 插件；只采信原 root session 的提交、消息、turn/end、whenIdle 和真实 flush。launchpad=false，原始 Windows 输入采用 bracketed paste/CR，提交后的 LF 文本必须通过原摘要。
- Windows 工具通过 native Codex 0.159.2 的 sandbox 子命令落实 AppContainer；这里没有 Codex 模型或会话。full_access 使用同一工具策略的明确另一分支，不升级审批。Node 实测 24.13.0，PowerShell 固定为 Windows PowerShell 5.1。
- 仓库认证复用既有短期单仓库 broker，provider 环境不进入工具。原 Job 绑定 owner/worker 的 PID、创建身份、handle；服务退出只释放观察引用，Run stop 撤销原 Job 并证明写者及 helper 根进程退出。
- helper metadata 冻结实际 EXE、扩展集合及原生来源 SHA；重启从原材料读取。业务生命周期、operation、事务及 codec 沿用原实现，没有新增资格数据库或第二套业务状态机。

工具当前每次最多 1 分钟、接收输出 16 MiB、文件读取 8 MiB；单次调用结束也回收其残留子进程。这里验证已适配版本的本机 development/1；不从结果推导 Pi/DSH 的远端或 environment-check 支持，也不承诺所有 Orca 可启动的 agent 自动可靠接入。

## 真机记录

首次五场景完整批次已通过，以下为历史构建；审计修补后的统一批次见下一节。

| 场景 | 现场 | 测试 PR | 结果 |
| --- | --- | --- | --- |
| `orca-local.pi` | `C:/srt/ca3892b064b4` | [514](https://github.com/erranliu/SteerlineTest/pull/514) | 通过，Run/服务停止 |
| `orca-local.pi-full-access` | `C:/srt/cd5a1729c59c` | [512](https://github.com/erranliu/SteerlineTest/pull/512) | 通过，Run/服务停止 |
| `orca-local.dsh` | `C:/srt/3b9d7d175a34` | [516](https://github.com/erranliu/SteerlineTest/pull/516) | 通过，Run/服务停止 |
| `orca-local.mixed` | `C:/srt/200062568f32` | [513](https://github.com/erranliu/SteerlineTest/pull/513) / [515](https://github.com/erranliu/SteerlineTest/pull/515) | 通过，Run/服务停止 |
| `orca-local.codex` | `C:/srt/e114252cbdc8` | [517](https://github.com/erranliu/SteerlineTest/pull/517) | 通过，Run/服务停止 |

本批 Runner SHA256 `ff5accb6b6ebf3751a4bcfba7657f38d78042e16a86ed9d110c95803aaadb621`，helper SHA256 `3289c0425453305d8c9916ca846b256ca487c5bef2653ffc0da0490e1e068fdc`。原生退出码最后兼容修补另有后续回归，原记录不覆盖或改写。各现场的 `manifest.json` 冻结 Runner/helper SHA、材料、配置、本机 runtime 和 repository；`evidence/scenario.json` 与 `result.json` 同时通过才算通过。注册、编译、无工具探针和失败现场不计入通过。


上一轮原生 stderr 兼容修补曾通过两权限实际回归，随后重新完整跑通；最终状态判定中的历史 LASTEXITCODE 问题在本轮审计修补：

| 场景 | 现场 | PR | 结果 |
| --- | --- | --- | --- |
| `orca-local.pi` | `C:/srt/2c97dd4eeed9` | [519](https://github.com/erranliu/SteerlineTest/pull/519) | 完整链路及 Run/服务停止通过 |
| `orca-local.dsh` | `C:/srt/0074c521b451` | [518](https://github.com/erranliu/SteerlineTest/pull/518) | 完整链路及 Run/服务停止通过 |

补跑 Runner SHA256 `1618eb23a3e4fc85ca8c710c422cc6a3c6713c622efbe2889b3a2879c89d54cc`，helper SHA256 `b770078fe85a83dbd8b92dde391a19d9038c64e61bf853590812e8d6c28dbbc3`；Pi full_access 及混合隔离的上一完整批次仍按原构建记录，最后修补另有实际 full_access 工具回归。

上一轮 L1：294 项 Go + 22 项 Node 扩展契约测试通过（`.evidence/orca-generic-local-footer-final/summary.json`）。完整 Controlled：16 场景通过（`.evidence/orca-generic-local-v14-regression/summary.json`）。当时工具修补后又通过原生工具、profile/来源/冻结/注册等定向回归；没有重跑未改动的整套远端 L3。历史聚合证据：`.evidence/orca-local-agents/summary.json`。

历史核心 case 验证 development、业务 needs_input/reply、原会话反馈、独立获取 PR head/内容、真实合入、原 request_id 查询/幂等重放、服务换代后显式 resume、实际启动父子写者后的活动 stop 和延迟写入消失。Pi/DSH 还额外核验父子写者属于原 Job；Codex 保留原停止证明并独立核验已启动 OS 进程退出。历史“丢 Runner submit 响应”只是舍弃正常返回变量，不能算真实故障注入；Codex 的旧零输入指纹也没有非零基线。这两项由本轮新增真实证据替代。

混合 case 在同一服务运行 Pi/DSH，核对各自输入、事件、原生会话/回执、workspace/branch/PR，停止第一个 Run 后要求另一个 Run 在原会话成功反馈并更新原 PR，最后停止两组资源与服务。

## 审计修补后的最终验收

最终统一入口 `.evidence/orca-review-fixes-qualification/summary.json`：**通过，5/5，零排除**。L1 通过 **300 项 Go + 28 项 Node**，Controlled **16 场景**通过；L1 的 7 个已有 opt-in/helper skip 不冒充真机执行。Windows 原生进程反例已单列 `_windows_test.go`，Linux 目标的 orcatransport 测试交叉编译通过。

| 场景 | 现场 | PR | 结果 |
| --- | --- | --- | --- |
| `orca-local.codex` | `C:/srt/11b2c3180aa8` | [529](https://github.com/erranliu/SteerlineTest/pull/529) | 完整链路、真实合入、Run/服务停止 |
| `orca-local.pi` | `C:/srt/a25644fe3332` | [528](https://github.com/erranliu/SteerlineTest/pull/528) | sandbox 完整链路、真实合入、Run/服务停止 |
| `orca-local.pi-full-access` | `C:/srt/0b108b75c815` | [527](https://github.com/erranliu/SteerlineTest/pull/527) | full_access 完整链路、真实合入、Run/服务停止 |
| `orca-local.dsh` | `C:/srt/48a32ebcf2b9` | [530](https://github.com/erranliu/SteerlineTest/pull/530) | sandbox 完整链路、真实合入、Run/服务停止 |
| `orca-local.mixed` | `C:/srt/fff7e1c540ea` | [531](https://github.com/erranliu/SteerlineTest/pull/531) / [532](https://github.com/erranliu/SteerlineTest/pull/532) | 停 Pi 后 DSH 原会话反馈更新原 PR；两 Run/服务停止，PR 按预期保留 pr_ready |

五场景均使用同一冻结构建：

- Runner SHA256 `c6abd04115c9195be60310223888faced5d1e23567310b077c9b9fe7cc9d15a1`。
- helper v22 SHA256 `9a52cb1c65256a3b27ea847b89e7c885af7ce52e630e285be47e22eeb0a6e67a`。
- native driver SHA256 `5bff3bba7a0b642e82a4919163e96cd1fe5e4eeb12ad89ba05c931248910ae63`。

聚合复核 `.evidence/orca-review-fixes-qualification/acceptance-summary.json` 绑定原报告、各 manifest/关键证据 SHA，复查同构建、四个真实不同反馈 head/merge、混合资源隔离以及五个服务 `not_running`。`owned-exit-audit.json` 独立复查本批 8 个受管实例：owner/worker OS PID 均不存在，原退出事实均证明 writers_absent 和 authentication_closed；Codex 另有实际父子写者退出证明。Orca app.asar SHA256 再次保持 `882fde748964d91737792c1e186673d7cbe5f7112aebd605f56b5228aa0f501d`。

本轮修复与证据：

- **启动失败和正常停止**：仅凭 owner 缺失不放行。同步准备失败留下绑定原 helper/Run/构建/配置的正向 startup-exit；实际 helper 退出后关闭原 PTY。正常路径先撤销原 Job 并证明零写者，等待原 helper 写出认证关闭事实并退出，再关闭 PTY。三条受管核心现场均有 `startup-failure.json`，证明未启动 agent、零业务输入、原终端关闭与 Run stopped。
- **实时权限和原生设置**：Pi 每次工具调用及输入/结算都核验实际 session/model/thinking/tools/cwd，正式选择事件立即撤权。DSH 通过既有 `llm.resolveCallConfig` 和原 root `agent/request` 取得并核验实际 effort，再核对持久 request header；没有读取不存在的 options 字段猜值，没有新建模型会话。新 helper metadata v3 冻结回执 v2；显式 effort 不匹配则阻断。接收端同时固定原实例、native session、实际 model/effort 和准备事实，缺失/超限/冲突/晚到及 malformed failure 均有真实通道反例。
- **真实丢 Orca 接受响应**：四个核心现场的 test-only CLI 转发器执行真实 Orca terminal send 后只扣留一次返回，不改 Orca。`lost-native-send-response.json`、`withheld-native-send.json` 证明恰好一次启动输入和一次业务输入，前后指纹相同，原 start_turn 保持 outcome_unknown、未晋升 started、无 PR，停止原 Run。三条受管现场还必须先等到真实成功 `late-native-final.json`，证明晚到完成仍不能补造 Orca 接受回执。正常 request_id 查询/幂等重放独立命名，不当作故障。
- **恢复与活动停止**：四个核心现场 `service-restart-explicit-resume.json` 均以真实原终端非零 send 基线证明换代零输入、显式 resume 和原 session/attempt/workspace/PR 保持；Codex 不再对空扩展目录取指纹。`active-writers-before/after-stop.json` 证明实际启动父子写者、原资源停止及等待延迟窗口后没有写入；Pi/DSH 同时核对原 Job。
- **PowerShell 和工具未知结果**：最终 cmdlet 状态不继承旧 LASTEXITCODE；PS5 成功 native stderr 的例外限定原脚本、最终命令行列及实际退出码。双权限实际工具探针 `.evidence/pi-tools/native-review-final-v21/summary.json` 验证 stderr/UTF-8、旧退出码、忽略的 cmdlet 失败及同位置外来脚本错误，并验证 AppContainer、工作区外写入差异、provider 环境过滤。v22 保持这些代码，另修正 deadline/cancel 保留 ctx 错误。`.evidence/pi-tools/native-deadline-v22/summary.json` 实际执行 60 秒截止，证明未知结果使后续工具撤权（host 只启动一次）、父子进程退出、等待 22 秒后无延迟写入；`independent-exit-audit.json` 再核对原 PID 均不存在。工具探针不计作 L3。
- **结构收敛**：共享原子写入器位于 receipt.mjs，DSH 不再依赖 Pi 模块；删除假想 mainAgent 通用途径的测试声称。保持一个 adapter、原业务状态机及单一有界事实通道，没有 Orca 补接口、structured、新服务、自动重发或资格数据库。

本轮失败记录保留，不能用最终成功覆盖：

| 现场/报告 | 结论与处理 |
| --- | --- |
| `.evidence/orca-review-fixes-batch1/summary.json`；`C:/srt/28b27bb4ab65` / `96d8d7621571` | 批次失败。Pi 通过并合入 [520](https://github.com/erranliu/SteerlineTest/pull/520)；DSH v19 实际 effort 读取错误，在业务前安全拒绝。修正为正式原生配置入口。失败 driver 原 stop 被服务关闭打断，已改为等原 stop 收尾后再停服务。 |
| `C:/srt/96d8d7621571` | 原失败 Run 的生产 stop 仍为 unknown，不能记成 stopped；原退出事实证明认证关闭/零写者，原 owner/worker PID 不存在，服务独立复核 not_running。见 `.evidence/pi-tools/review-dsh-v19-failure-cleanup.json`；没有替换 agent 或修改旧结果。 |
| `C:/srt/4764b7c35fbe` | DSH v20 定向完整通过并合入 [521](https://github.com/erranliu/SteerlineTest/pull/521)；旧 driver 未把晚到最终事实列为强制断言，另由 `.evidence/orca-review-fixes-final/dsh-late-native-final-audit.json` 核验，最终统一批次已直接强制验证。 |
| `.evidence/orca-review-fixes-final/summary.json` | 中间四场景批次失败。Codex/Pi/Pi full_access 各自通过，[524](https://github.com/erranliu/SteerlineTest/pull/524)/[523](https://github.com/erranliu/SteerlineTest/pull/523)/[522](https://github.com/erranliu/SteerlineTest/pull/522) 合入；混合原生 driver 通过，但运行中代码更新导致 Runner 构建变化，scheduler 正确拒绝资格。混合 PR [525](https://github.com/erranliu/SteerlineTest/pull/525)/[526](https://github.com/erranliu/SteerlineTest/pull/526) 保留，未将该批改写成通过。随后冻结代码重跑本节完整五场景。 |

中间七个现场的服务均独立复核 `not_running`，11 个受管实例的原 OS PID 不存在，原退出事实均证明零写者与认证关闭；分别见 `.evidence/pi-tools/review-intermediate-service-status.json`、`review-intermediate-owned-exit.json`。测试分支、工作区和所有原始通过/失败记录保留。最新通过范围仅为本节列出的版本、本机 development/1 与权限组合；未重跑未改动的整套远端 L3，也不推导 DSH full_access 支持。

## 失败与修复记录

保留失败现场供对账，均不计通过：

| 现场 | 发现与处理 |
| --- | --- |
| `C:/srt/8e678b3f530b`、`c98f5a7739e0`、`e266c5e0af86` | 旧 Orca agent 标签 / Codex pane 门槛；受管 Pi/DSH 改用实际原生身份及来源证明，保留原资源身份约束。前两个现场有独立 owned 清理记录，不能冒充生产 stop 通过。 |
| `C:/srt/143ba4a2a3da` | OpenRouter 上游 402；保留真实 provider failure，后续配置明确选择已有 DeepSeek 直连模型。 |
| `C:/srt/96708af84343`、`5f40d9a370e9` | answer_saved 的接受后状态及旧 Codex 空闲判定；分别修正 driver 等待及原生最新输入结算核验。 |
| `C:/srt/1b6db37387c2`、`46bf4df5a59f`、`213ecdc23226` | DSH 首屏/传输破坏换行，按摘要拒绝；改为原生 launchpad=false 和 bracketed paste，未放宽摘要。 |
| `C:/srt/f1d474cf7c58`、`326c3bbb20b5`、`4142be3c9bf7` | driver 在 resume 接受后过早读取；等待真实恢复授权，并立即更新 cleanup 的服务实例。原 Run 已停止，原遗漏的替换服务另有独立清理记录。 |
| `C:/srt/31dbb14d3053` | 混合 driver 用首个 agent 校验第二个；按各自冻结 execution 核验。两组资源、服务已停止。 |
| `C:/srt/7af0332c7f5a` | Windows PowerShell 5 的 Stop 把捕获的原生 stderr 当作异常，Git 已 push、PR 尚未创建；保留 unknown/block，不重发。工具宿主改用正常原生命令语义，受信任文件/准备脚本仍严格失败，补实际 stderr/UTF-8/退出码回归。 |
| `C:/srt/80f9f9d54ea9` | Codex 停止探针脚本在工作区外且未使用既有 Bypass 调用；原生权限拒绝，未启动写者，case 失败。修正为工作区内脚本及 checker 同样的启动方式，没有放宽 OS 权限。原 Run、服务已停止。 |

失败的测试 PR #502/#503/#504/#505/#507/#510 已在核对冻结 repository/base/head 后关闭；记录见 `.evidence/pi-tools/failed-pr-cleanup.json`。合入记录、失败材料、测试分支和工作区保留用于复核；不删除用户工作。Orca app.asar SHA256 复核仍为 `882fde748964d91737792c1e186673d7cbe5f7112aebd605f56b5228aa0f501d`。

运行配置和可重复命令见 [测试说明](../testing.md)，语义边界见 [Spec](../spec/orca-generic-agent.spec.md) 与 [Plan](../spec/orca-generic-agent.plan.md)。
