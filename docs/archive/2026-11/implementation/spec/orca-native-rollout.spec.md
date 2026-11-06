# Spec：Orca 原生合同正式启用与旧消费者收口 B6

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-03。状态：已实施，ONR-01–05 及最终同构建四场真实资格、实施后独立审计全部通过。依据[长期设计](../../../../spec/orca-capability-layer.spec.md) §6.2–6.3；前置 B5 已由 `0f22212` 签收，最终 `ac361dd` 同构建双端开发与活动停止、原资源/真实离线及独立审计全部通过。

## 1. 正式支持及例外

正式声明 B5 已接通并取得资格的 Codex/development/2/full_access：本机与明确 environment 均使用既有公开配置，不提供 runtime_executable/runtime_sha256，冻同 profile/state codec 9 与 orca_terminal scope。接线、可靠性、完整输入/结果、反馈及停止机制使用 B5 原实现，不新增启用标志或测试后端。正式启用指把已可受理的合同同步为公开产品承诺和 capabilities；不部署/安装或改写用户现有配置。

公开说明精确区分：声明模型只是启动偏好、实际模型未独立核验，工作区审计 unreported；query-only working Started 不授予流覆盖；orca_terminal 核实原 runtime/worktree/local host 的原终端，不证明全部后代退出或撤销凭据。

保留且逐项列明真实消费者：显式本机 Codex helper（含 development-audited 的原模型与工作区审计、登记根停止合同）、本机 Pi/DSH 的受管工具/证据、其他 workflow 与旧 profile/state 的原读取/对账/排空。它们不继承 B5 原生资格，不自动移除 helper、不降 scope、不作为两端已统一的组合。后续补齐实际模型/工作区审计或其他 agent 原生证据须另批取得资格；不能为本批净删指标删仍有消费者的旧链。

## 2. 最小生产收口

Capabilities 增加机器可读的 Orca 合同选择/目标/资格边界，补足本机与远端原生 orca_terminal 描述，并明确显式 managed 与其他组合例外。既有 capability catalog、配置 schema 和实际 admission 保持；不以描述性 metadata 扩大组合，不触及原生直接 Codex 路径。

PreparationFacts 在 agentExecutionSupport 已成功之后再次检查 Pi/DSH Terminal/Environment/RuntimeSHA 的分支不可达：前一函数对非 Codex 已要求这些条件及 Terminal.Agent 精确匹配。删除这段重复 guard，保留前一门禁、原错误和零副作用。其余 helper/control/history/owner/usage 消费者逐项登记，不假称无消费者而删除。选定原生组合已在 B5 不再消费这些旧实现，B6 不复制一套 native 流程。

同步 README、当前产品承诺、长期设计与主要 Orca 历史设计的当前边界链接；新增当前合同说明，给出仅 target/连接不同的双端配置及保留例外。不改历史验收事实，不把旧 PR/源码资格平移到最终构建。

## 3. 升级与验证

state/v8、user_version、profile/state codecs 4/6/7/8/9、路由摘要、原 receipt/stop progress 均不变。原 EXE 先排空 Run 与未决 operation，service stop 后由原 EXE 真实 verify-stopped；本批新 EXE 使用 fresh 目录；兼容客户端可选择原服务实例，但不代表替换旧 writer、不迁移摘要或历史证明、不为本批添加旧任务接管器。

B6 改变公开能力输出及准备检查源码，因此冻结最终 Runner/driver/CLI，执行双端 fresh 正式开发与独立活动停止四场；沿 B5 正式 driver 受理/冻结/原会话反馈/真实 commit-push-PR（不合并）/Run stop/service stop/真实 offline，重复 §6.3 受控停止故障矩阵与排队/blocked 首次停止。B5 结果只是前置，不能代替最终构建。

| ID | 验收 |
| --- | --- |
| ONR-01 | 公开 native 选择、scope、实际模型/审计/流边界与正式冻结一致，配置 schema/catalog 不新增资格 |
| ONR-02 | 删除不可达 guard，仍有消费者的旧链逐项保留，明确未统一组合及其必要性；原 agent 拒绝与零副作用回归通过 |
| ONR-03 | 最终同构建四场真实资格、零旧链、原资源/停止离线闭环通过，测试 PR 未合并 |
| ONR-04 | 受控未知 close/DB 回执失败/替换/完整性/排队/blocked 及正式 CLI different_release、离线和查询不启动通过；格式与摘要不迁移 |
| ONR-05 | 源码/构建/范围/原证据及实施后独立审计闭环，确认修复独立提交；全部批次完成后统一提 SteerRunner PR，不合并 |
