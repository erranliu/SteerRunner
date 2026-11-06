# Plan：Orca 双端原生 Runner 候选 B5

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：已实施，OSR-01–05 全部通过；最终 ac361dd 同构建四场真实资格及实施后独立审计均已闭环。依据 [Spec](orca-shared-native-runner.spec.md) 和[长期路线](../../../../spec/orca-capability-layer.spec.md)。本批包含候选实现与真实资格两个依赖阶段，不能只完成前者便签收 B5。

## P0 候选范围与固定格式

冻结 Codex/development/2/full_access/native 配置、双端目标、`orca_terminal` scope 和未核验实际模型/工作区审计边界。原 helper/其他 workflow 仍沿现行合同，不记本批资格；不引入公共 debug flag、测试 adapter 或直接 Lifecycle driver。

## P1 生产接线

1. application/freeze.go 为选定无 helper 新 Run 冻结 NativeProfileVersion=9、native_terminal 标记和 scope；contracts 准入明确候选/不兼容 audited/显式 helper 冲突。启动命令只含原生 Codex/权限/偏好/初始化，无 helper/control/history 编码。
2. orcatransport 的 profile_decode/stateVersion/UsageBinding 统一识别新合同，旧 codec 不接受新标记，9 拒绝任意 helper 字段/非 Codex。原 Target hash 算法不改，旧 Run/摘要不迁移；新双端 state codec=9。
3. 逐项审查 lifecycle.go 的准备/状态/输入/停止/Shutdown、native_observation/native_state、verify_stopped 的 environment 分支；只把合同判断换为明确的原生合同选择，保留连接、主机、selector、local authentication 差别。共用 composer/send/原输入与 final 证明，移除候选对 local managed readiness/control 的调用。
4. 新本机 scope 接入原 remote-style 单次 close/完整清单/原 stop progress；命名改为通用 native 职责。application 的冻结 stop scope、投影和离线 verify-stopped 消费相同合同，不以旧 stopped 标签绕过核验。其他 scope 和持久 receipt 格式保持。
5. 同步候选能力报告/文档；所有新格式消费者可读，旧配置/格式严格隔离，原生能力包依赖守卫保持。

## P2 受控验证

新增双端候选真实构造/codec/准入/冻结测试，不手工省略 profile 或 scope；验证新标记旧 codec 拒绝、helper 残留、状态/usage/只读映射和原请求证明。复用原 NativePrompt/OriginalInput/NativeProof/跟随回合、预算和 CRLF 回归。

在 application 的正式冻结/Engine/原 operation/存储路径覆盖候选双端活动撤销与禁止新 send、未知 close 一次、提交失败和重入、替换 incarnation/host/count/trunc 及排队未派发。使用屏障/故障点与实际调用计数，不把旧 LocalCodexStop 测试更名当新覆盖。补 working Started→done/同前缀第二 user/替换 provider/incarnation/partial 或截断 final/后续 working 通知反例，严格隔离活动输入证明与完成及流资格；Blocked 且输入已禁用的 Run 仍保留合法首次 stop，已尝试/未知 stop 只查询。准确补跑 TestSRServiceOwnership 的 offline_verification/remote_offline_scope 并新增本机候选正式 CLI 子例。

必要两包/application/contracts/resources/workflow 定向，生产与 live-tag 编译、依赖守卫及持久格式 diff。记录实际根/子节点/叶例、fail/skip，不用零匹配签收。

## P3 正式 Runner 资格驱动

新增独立 native-capability-runner driver 与注册的本机/远端案例，沿现有 tests/native/Manifest/CLI fixture 体系；原 developmentSmoke 会合并 PR，不能复用其成功条件。新 driver 的成功要求测试 PR 程序核验且不合并、同会话 pre-pr_ready feedback、正式 Run stop/service stop/offline 核验和零旧链。

脚本只准备隔离 SteerlineTest base/材料、冻结 EXE/CLI/config，注册 native 候选允许无 helper（只限对应案例，旧 fixture 约束保持）。本机已有 `C:/Projects/SteerlineTest`、远端明确 `台式机64G` 中对应注册仓库先只读验证；实际环境/版本变化重新冻结，不换环境续接未知 Run。准备任何资源先登记精确原目标，响应未知只原计划查询，不重复创建。

第一业务输入保持 implementation 未完成，正式 feedback 后由原主机模型完成最小测试修改和 commit/push/PR。driver 通过正式 CLI/API 提交/回复/读取，独立 GH/原主机只读事实核对 repo/base/branch/SHA/PR。额外诊断终端只读、登记/关闭且不替代业务证据；不生成 Snapshot/Run、补写状态或伪造发布。

同最终源码/构建双端成功案例各一。再分别 fresh 活动停止 Run（至少新增本机，远端无等价证据则补），由原请求 Started 与原生活动状态触发正式 stop。失败、重试新场次和原清理分别登记，不把清理成功改判业务通过。保留正式受理/Run/原资源、配置/codec/scope、调用日志、PR核验、在线停止/持久读取及真实离线结果；归档不含凭据。

## P4 收口

按 OSR-01–05 写验收、源/构建/目标/原始证据，更新长期状态；实现提交后独立审计，确认修复独立 commit。任何实现修复影响资格，用最终构建复验对应场景；未通过时修复/续验，不提前标完成或开始 B6。全部批次完成后才统一提 SteerRunner PR，测试仓库 PR 单列且不合并。
