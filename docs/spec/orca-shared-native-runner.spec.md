# Spec：Orca 双端原生 Runner 候选 B5

日期：2026-11-02。状态：待实施。依据[长期设计 §6.2–6.3](orca-capability-layer.spec.md#62-b5-的候选生产接线和真实-runner-验收)，对应 [Plan](orca-shared-native-runner.plan.md)。调查基线 `c9b9c4b`，B0–B4d 已完成；B4d 唯一审计 P2 已独立修复并关闭。

## 1. 候选支持与合同

首轮候选为 Codex、development/2、full_access、明确本机或配对 environment。两端同一个 Orca adapter 消费 B1–B4 的能力，均不依赖 managed Codex helper、control/history、终端扩展或调用端 transcript。连接、仓库 selector 和明确主机路由仍按原差别解析，业务输入/证明/停止流程不因位置分叉。

候选使用既有公开配置：Codex agent_launch 不提供 runtime_executable/runtime_sha256。不新增长期 debug 开关或测试后端；新候选构建在独立状态目录完整受理此配置。本批不修改已安装服务、旧状态目录或其他组合。显式 helper 配置的现有受管组合仍按旧合同，不能继承本批资格或记为已统一；B6 根据实际消费者决定启用/删除及明确保留例外，不能静默重写这些配置。

新原生 profile codec 定为 9，新增私有 `native_terminal:true` 表示这个冻结合同；两端 state codec 同为 9。旧 4/6 profile、7/8 state 不重释，旧 codec 出现新标记（包括 false/null）拒绝，9 必须明确 true、Codex 且无任何 helper 字段。Target 的原路由摘要算法保持，配置/profile/state 的新版本身份分别冻结并核验。

候选 stop scope 两端均为 `orca_terminal`：核实原 runtime/worktree/local host 的完整清单与原 handle/incarnation 退出/缺席。它不证明 Codex 全部后代退出，不撤销主机凭据，不沿用本机受管 helper/root scope。此收缩只适用于新明确原生合同，不能把原本机受管 Run 改记为新 scope。

声明模型为冻结启动偏好，实际模型未经独立核验；原生工作区审计来源为 unreported。audited 不准入候选，不平移旧 managed 模型/history/Git 审计资格。Pi/DSH、其他 workflow、显式 managed 组合未取得本批资格，现有支持不因本批扩大或删除。development 的正式 PR repository/base/head/身份校验仍保留。

## 2. 完整生产接线

正式 ValidateRunConfiguration/submit → freeze → 新 profile/StopEvidenceScope → Engine/reliable → 同一 Lifecycle → 原生能力 → operation/结果持久化 → 在线 Run stop → service stop → 离线 verify-stopped。禁止在 driver 中构造 Run/Snapshot/operation 代替任何环节。

freeze 在两端无 helper Codex 的选定组合上生成同一候选 profile/codec；明确拒绝不兼容组合，不从缺字段/证据失败退回旧链。启动、准备和原输入使用同一原生 proof/query/notification/session reader，原唯一输入、Started/accepted、完整 final、反证和恢复判据保持。调用位置选择不改变来源强度，本机候选也使用原生独立查询的严格连接/归属条件。

候选两端共用 composer wait、单次原 send、20 秒提交等待提示、CR 输入编译和原回合证明；准备/状态拒绝 helper 残留。public candidate/last-message 映射同样覆盖本机候选。Run 路径、预算、正文、授权、证明和持久化仍在执行层，不能回迁到 Orca 能力包。

stop 与 Shutdown/只读验证按冻结合同选择，不再以 environment 是否非空代替候选合同。原 operation 和 receipt/stop progress 格式保留；新 state codec 只区分合同，不建立第二账本。UsageBinding 等所有生产 profile/state 消费者识别新 codec，禁止借新版清单/旧标签授予资源结算。

## 3. 成功业务资格

两端各一例 fresh 正式 Runner Run，隔离 SteerlineTest 测试 base/workspace/state/evidence，冻结源码/EXE/CLI/目标/配置。原主机真实工具修改最小文件、运行测试、commit/push 并创建测试 PR；driver 独立核对 repo、原 base、branch、精确 SHA/PR 身份及公开结果。不能使用脚本伪造 worker 发布，不合并测试 PR。

同会话反馈安排在 implementation 允许输入、尚未 pr_ready 的阶段：第一原输入通过明确 needs_input/未完成结果保持该阶段，正式 reply(kind=feedback) 后仍使用原 terminal/incarnation/provider 完成实际修改；不重开发布后反馈边。完整 final、业务成功与 PR 程序核验分开记录。

成功后显式正式 Run stop，持久 stop_scope/status/新输入禁用成立，再正常 service stop 退出 writer；真实 service verify-stopped 同目录只读复核。保留完整原清单及整个调用日志，核对 helper/control/history/扩展调用均为零。测试准备/诊断设施须单独登记并关闭，不能混进原业务停止 scope 或靠清理掩盖失败。

## 4. 活动停止和故障闭环

| 场景 | 必须验证 |
| --- | --- |
| 本机新候选真实活动 Run stop | 原业务请求 Started 且原生回合仍活动时正式撤销；不依赖 sleep/模型自述；停止后无新业务 send/后继/答案/反馈，完整原资源清单及离线核验通过 |
| 双端候选 close 已生效但响应丢失 | 实际 close 一次；仅查询原完整清单对账，不新建/替换/重发或补造 close 回执；活动/不完整/冲突保持未核实 |
| 已在途停止回执提交失败/重入 | 沿原持久意图/operation 恢复，不以缺回执授权第二次关闭；同请求与新请求均不更换原 pending stop |
| incarnation/runtime/host 冲突、截断/缺 host/查询失败 | 不认领替换资源、不保存虚假 stopped/resources_settled，保存诊断；旧 stopped 标签不绕过新鲜清单 |
| 排队未实际派发 | 与丢响应明确区分，仅按原 operation 保留首次资格，不将任意 timeout 当作未派发 |

故障在受控夹具通过新 profile/codec、正式冻结/执行/原 stop operation/存储路径注入，至少覆盖本机与远端；旧 managed 或仅旧远端测试不直接计入。远端真实活动停止按受影响程度补验，缺同构建等价证据时执行独立 Run。活动停止与成功 PR 案例不合并计数。

## 5. 验收与推进

| ID | 条件 |
| --- | --- |
| OSR-01 | 新准入/冻结/profile/state/usage/stop scope/在线及离线消费者全部接通；无仅测试装配或 native→managed fallback |
| OSR-02 | 双端同一原生执行流程，旧位置分支只留连接/selector；原证明强度、输入/恢复和现有非候选回归保持 |
| OSR-03 | 同源码/构建双端正式开发/反馈/final/commit-push-PR/Run-stop/service-stop/offline 闭环通过，零旧链调用 |
| OSR-04 | 双端候选故障矩阵及真实活动停止闭环通过，清理/未知/skip 单列，不计通过 |
| OSR-05 | 直接证据、合同/索引、依赖/格式和实施后独立审计闭环，确认修复独立提交 |

B5 未满足全部条件不得签收或进入 B6 最终启用。B6 不首次实现本节已宣称验证的入口/停止；最终删除或配置变更影响接线须用最终构建复验。当前旧支持合同不会被本文的候选目标覆盖。
