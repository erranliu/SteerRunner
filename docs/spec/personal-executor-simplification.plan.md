# Plan：个人多机执行器的执行链路简化

日期：2026-11-01。状态：P1 已独立验收；P2 已实施并独立验收，Orca local 与完整 L1 补验通过；P3 已按 PES-A03/A04/A05/A08 独立验收通过，state/v7；P4 本项（原生问答输入）已按 PES-A06/A08 独立验收通过；其他候选不展开。依据 [Spec](personal-executor-simplification.spec.md)，调查基线 `e2729563562314659a799a5f82debe427fcd7c24`；实际本批为该基线加未提交变更，见[P1 验收](../acceptance/personal-executor-p1.md)。

## 1. 批次与交付规则

一个 Spec + Plan，推荐顺序为 P0 按批基线 → P1 快速验证 → P2 观测解耦 → P3 恢复/兼容删除 → P4 有收益的局部职责收口。P5 只汇总各批结果和更新总状态，不是前批交付门禁。各批可独立提交、独立验收并形成独立 PR；合并、安装和发布仍按实际授权执行。

| 批次 | 独立交付所需 | 不等待 |
| --- | --- | --- |
| P1 快速验证 | PES-A07/A08 及 Spec §8 对应收益；生产行为/state/v6 保持 | 通道、恢复、v7 或 adapter 改造 |
| P2 观测解耦 | PES-A01/A02/A08 及对应收益；仍用当前 v6 | 恢复删除或全部职责收口 |
| P3 恢复/兼容删除 | PES-A03/A04/A05/A08、三类规则删除及生产实现净减少 | 所有后端知识统一搬迁 |
| P4 局部职责收口 | 每项 PES-A06/A08、实际变化面减少 | 所有具体 import 消失或其他收口项完成 |

P1 可供后续批次使用；若某批在此前实施，也可用现有组件/受控入口独立验证，不使新验证框架成为业务修复的硬前置。后批失败、环境不足或暂停，不撤销前批已完成结果；缺口不得冒充本批通过。全部完成才标记总计划完成。

用户已追加授权按批实施并独立交付 P1、P2；本批不包含安装、生产任务排空、旧数据迁移/删除、产品合并发布或外部业务仓库修改，不另建产品重定义、逐 Run 构建权限或通用 Runtime 项目。原 L2 需要的专用测试仓库/测试 base 分支和测试 PR 仍按原 case 执行。

## 2. P0：按批记录最小基线与收益

1. 开始当前批次时记录实际 HEAD/dirty、支持路径和用户改动，只盘点本批消费者；不先运行全后端或完成后续所有清单才允许第一批开始。
2. P1 记录默认 test.ps1 的实际启动阶段、选定 case、构建/预检、fixture、case/清理及总耗时。优先使用同主机、同目标/同 case 的近期可比完整报告；没有时明确缺少耗时对照，不为凑数据额外跑完整矩阵。
3. P2 盘点 progress/result 的路径、环境、初始化尝试、Initialized 门禁、保存合并、采样及收尾，保存正常闭环和通道故障阻塞的最小受控反例。
4. P3 盘点 verify-takeover、RecoverySupport、restoreOriginalAttempt/BindRecovery、retiredDevelopment/LegacyRemoteProfile、旧停止 adapter 和历史 decoder。区分当前 full_access、内容 read_only、停止证明与真正的历史兼容；记录至少三类拟退役规则、原执行重启和原创建回执样本。
5. P3 同时盘点 resume 的 handler unknown 拒绝、Controls.CanResume、控制命令生命周期和原 Reconcile/观察入口，确认无 observer 时也有公开人工对账路径。盘点 state/v6、旧命令及恢复能力的 schema、CLI、skill/helper、fixture 和外部 SteerLine 消费者。
6. P4 每次只选一个实际重复解释或跨层修改的后端能力；记录当前私有知识所有者、实际调用入口及一个具体协议字段调整原本会修改哪些文件/职责。
7. 每批附简短收益记录：基线、预期删/增规则、对应 Spec §8 门槛和本批必需检查。历史约 33 分钟前置或约 4 分钟 L2 只能作背景，不作为本次实测收益。

出口：当前批的变更/保留清单及收益可核对。P3/P4 若没有净删除或变化面收缩的可行方案，先缩小该项，不以扩大框架兑现计划。

## 3. P1：先交付快速定向验证

1. 保留现有 `go test <受影响包> -run <明确模式>` 和 `test-foundation.ps1 -Pattern`，明确它们不需要完整 test.ps1 前置。
2. test.ps1 增加 Spec PES-04 的 `-Focused`：单级 L2/L3、明确 Scenario、Config、固定 RunnerExe、新 OutDir；拒绝 PrerequisitesFrom、ListAvailability、全目录、混合级别和自定义 Catalog。
3. 复用原 case、配置评估、fixture、scheduler/lease、native driver 和正式停止清理，只改变阶段选择/资格汇总。Focused 不启动完整 L1/Controlled，仍本轮编译 driver 并检查零匹配；RunnerExe 使用现有匹配生产输入的构建来源回执，不新增证明协议。
4. 报告增加 validation_scope=focused；只表达选定 case 的结果，完整级别、baseline verified 和交付资格始终 false。更新本仓汇总消费者，失败/未收尾仍失败。
5. framework 合同覆盖参数拒绝、实际阶段调用计数、非零精确 case、清理失败和完整资格始终 false；复验默认入口及现有 L2 前置复用的相关合同。不复制真实测试实现或缩减其断言/期限。
6. 运行至少一个受支持目标的正式 focused case，记录实际 Runner/driver 来源及阶段耗时。其他目标的可用性/真实结果按实际情况列项，不将该场景称为完整 L2/L3 资格。

独立出口：PES-A07/A08。完整 L1/Controlled 启动次数各为 0，选定 case 实际执行并收尾；有对照则列省掉的前置耗时，没有则只声明已证实的阶段省略和本轮耗时。P1 可以在生产代码仍完全不变时交付，不等待 P2–P4。

实施结果：`scripts/test.ps1 -Focused` 已接入原配置、case/scheduler 和清理，预检 driver 传递扩展至 L3；26 项 focused 合同、原 75 项前置复用合同、配置与调度相邻回归通过。正式本机 L2 `local-codex.smoke/default` 1/1、185.85 秒，L1/Controlled 启动均为 0，原 Run/服务已停止；完整 L2 资格保持 false。未取得本批真实 L3/Orca 资格，没有同条件完整批次耗时对照，详情及原始报告见[P1 验收](../acceptance/personal-executor-p1.md)。

## 4. P2：通道故障不再阻塞业务

1. 修改 worker.advance、prepareRuntimeFiles、prepareProgress/prepareResult：首次业务输入前只做一次尽力准备，两通道共享最多 4 秒；失败、超时或未确认不再成为业务推进门禁，也不在后续轮询/阶段重复准备。
2. 复用现有初始化尝试事实，覆盖尚未建立远端绑定的失败；记录失败时不发写 RPC。下一业务事务仍按自身持久化规则执行，不能把通道容错扩大为忽略业务意图提交失败。
3. 文件外部 I/O 移到全局锁和 dispatch gate 外；在短临界区核对原绑定/授权、记录初始化派发开始，该界限之前 stop 已提交则不发。跨过界限的文件动作作为在途动作有界结束，返回后复核并只保存合法文件事实，不据此派发业务后继。
4. 保持 O_EXCL、不覆盖、固定原主机/工作区路径和环境注入；业务开始后不再等待文件等于初始种子。progress 原绑定可读时继续采样，失败保留最后快照，不补写种子。
5. result 在原结束资格后仅按原绑定和稳定字节采集，不要求种子确认；保留一次封存、version=0/1、两秒预算、不刷新及失败不阻止收尾。文件/业务/另一通道保存互不覆盖，不增加业务 revision。
6. 受控反例覆盖四种启用组合、初始化失败/超时/响应丢失/保存失败、agent 已改写种子、重复唤醒/普通重启和 stop 竞争。断言实际业务输入/PR、初始化写次数、重复准备等待和快照结果。
7. 对实际触及的本机及 Orca local/environment 文件接线做最小真实验证，复用一个 Run 覆盖多个结论；未执行路径列为本批缺口。无需用模型重复所有文件错误组合，不等待恢复或边界重构。

独立出口：PES-A01/A02/A08。指定故障下通道造成的业务停滞为 0；每通道初始化写 RPC 最多 1 次、首次总窗口最多 4 秒、后续准备等待为 0；记录到下一业务输入及 stop 受理的实测耗时。保留当前 v6，P3/P4 未完成不阻止本批交付。

P2 实施记录：删除 Initialized 业务门禁与重复准备；原 Next 提供首次输入候选后，两通道一次持久化尝试并在锁外共用 4 秒窗口。progress 可只读接续未确认初始化，result 结束时直接核验实际字节；停止采集也释放 dispatch gate。沿用同一 Record/SaveProgress，未增加状态格式、数据库或队列。受控四组合 PR/停止闭环及 34 个顶层定向组件测试通过，真实路径与补验结果见[P2 独立验收](../acceptance/personal-executor-p2.md)。

## 5. P3：删除恢复/兼容并补齐人工出口

### 5.1 升级与一次性格式切断

1. 仅在本批启用 storage.Format/state/v7、user_version=7、host 默认目录及客户端发现。v6 在 worker/owner/远端调用前非破坏拒绝；不写迁移器、不处理旧现场。
2. 不增加 execution_build_sha256 或逐 Run 的构建分类，不增加异构建只读投影、按构建拒绝 stop、混构建资源结算等分支。既有 EXE SHA 的来源回执、实例绑定及精确资源用途保持。
3. 升级说明采用原版本先排空，再启用新版本；排空包括原生资源及未完成操作/控制命令，pr_ready 不等于排空。不新增自动排空或逐 Run 构建识别器，也不支持同目录混合构建活动任务。
4. 以后格式/执行契约兼容且已排空时允许继续原目录、查询历史；仅因实际不兼容才切格式/目录。保留当前解码，不维护旧私有版本矩阵，不因 EXE 字节变化增加业务门禁。
5. 更新本仓 CLI/schema/capabilities、skill/helper、fixture、打包检查和当前说明，显示新目录，不暗改显式 state-dir。外部 SteerLine 只登记集成变化，不改外仓或声称已联验。

### 5.2 恢复删除与公开人工对账

1. 保留 shutdown detach、独立 owner、原创建执行器回执、原执行版本重启重连及离线 final 补收。删除 restoreOriginalAttempt、BindRecovery、资源重建字段/资格和专属成功测试；正常首次资源创建及当前停止证明保持。
2. 资源丢失或自动未知窗口耗尽，持久化 blocked/manual_intervention_required、ResumeRequired=true 并暂停新业务输入。保留原操作及结果；业务等待、stop/close 和里程碑不自动解除。
3. 按 Spec §4.3 保留一次自动有界查询窗口，窗口结束不无限重开。accepted/ResourcePending、活动长回合、未知停止分开处理，不用统一 30 秒规则推断它们失败或已停止。
4. 修改 resume handler/Controls.CanResume，使技术待人工且仍 unknown 的 Run 也可受理显式 resume；保留 request_id/expected_revision、无待停止及未关闭等准入。reply/message 不能借此绕过 unknown。
5. 受理后先执行一次最多 30 秒的原操作只读对账/观察，只用原 ID/输入/资源身份，不 Execute、不创建 Attempt/owner/session/终端。复用原命令/事务，不另建查询任务账本。
6. 保存真实回执/final 后，按最新绑定、停止/关闭、源和业务资格复核：未知已由原事实解决且原资源有效，才在事务中解除技术暂停；活动回合只接回观察，idle 才继续未派发的唯一后继。仍未知/超时/冲突/资源丢失则本次 action_status=failed、继续待人工并给具体诊断。
7. 同 request_id 返回原结果；新 request_id/当前 revision 可以人工再查。服务中断可重复只读原事实，不重做已成功控制或后继。迟到事实不自动恢复授权，stop/close 先提交则本次不得继续。
8. 公开 CLI 必须证明：没有存活 observer、原输入已接受而回执晚到，自动超时后 resume 可对账并继续；仍未知后新请求可再查；重复请求、提交失败、控制中断和 stop 竞争全程零输入重发/零资源重建。
9. 删除 verify-takeover 路由、候选执行、兼容评估和客户端调用；旧命令走普通未知命令拒绝，不保留假 compatible 壳。

### 5.3 退役兼容与净删除核对

| 位置 | 删除 | 必须保留 |
| --- | --- | --- |
| application/access_mode、port、takeover/verify_stopped | 退役权限分类、旧停止实现选择、候选兼容评估 | 当前请求校验、原资源停止及证据范围 |
| adapter profile/state/retired_stop/recovery_compatibility | 旧开发权限和历史 decoder/版本列表、资源重建绑定 | 当前 full_access/read_only、owner/model bridge/checkpoint/退出机制 |
| policy/permissions/workspace | 仅为旧授权保留的 --add-dir/恢复授权编码 | 直接路径身份值、worktree/common-dir 核对 |
| storage/legacy_parser、旧 parser 元数据 | v6 原生 parser 识别及无消费者字段 | 独立 textanalysis、当前业务分析结果 |
| 测试/case 注册 | 跨构建接管、资源重建、退役权限兼容的成功场景 | 普通重启、格式拒绝、丢失转人工、人工对账和零重发反例 |

逐项搜索消费者，保留当前 read_only、共享准备门禁、退出签名、日志访问租约；不能按 sandbox/legacy 名称整块删除。记录三类退役规则、删除/新增入口和状态分支。生产实现扣除搬迁后删除行数须大于新增行数，测试和文档另列；实际规则删除是主证据，行数辅助核对，不靠压缩格式凑净减少。

独立出口：PES-A03/A04/A05/A08 和净删除门槛。本批涉及共享格式/控制，需本批最终代码一次完整 L1+Controlled，以及受影响本机/Orca 最小真实接线；不等待 P4，不为无关代码变更重复全部前置。

## 6. P4：按能力局部收口，不搬通用准备逻辑

1. 从私有 profile/helper 构造、权限资源、问答解码、checkpoint 或文件源中选择一项实际有重复解释/跨层修改的能力。登记前后私有知识所有者及真实调用方，不要求整批全搬。
2. 复用现有 LifecycleResolver/窄端口，按能力收口并删除原重复分支。允许 host/cmd 或 application 内一个明确登记的集中组装文件 import 具体 adapter；不把 application 零具体 import 作为目标，不新增仅为隐藏 import 的多层 facade。
3. 只传实际必要纯值，不传整个 Record/Engine/Store/提交回调，不将私有字段全部挪进新公共包。业务来源、审计核验、模型角色和下一步骤仍由业务所有者负责。
4. 保留 reliable.Next 的通用 workspace/resource/session 顺序、与 QualifyAction 共用的门禁及发送前复核。adapter 继续提供具体准备事实/机制；不复制共享判断，不把创建/发送隐藏到查询中。
5. P3 删除 BindRecovery 后，只读原资源资格端口可收缩后继续存在，不强制删除整个 LifecycleRecoveryPort 或改名。共享准备 capability 与旧接管 codec/版本清单分别处理。
6. 调整依赖守卫保护私有知识和禁止反向依赖；集中组装只放行明确登记文件，不误禁所有具体组装，也不把业务代码塞入例外。
7. 对本能力的正常路径、unknown、提交失败、必要只读/问答/停止/审计来源做定向回归；只有触及真实协议、原生启动/回执/停止机制时补对应最小真机接线。无协议变化不因文件移动要求全后端重新资格。

每项独立出口：PES-A06/A08，至少消除一个重复私有解释入口，或用同一协议字段调整前后对照证明无需再改 application 业务协调代码（集中组装接线可以改变）。只改文件位置/import、仍保留同样知识和新增纯转发层，不算达到收益，应缩小或取消该项，不阻塞前批或已完成能力。

### 6.1 本轮选定能力与收益基线

本轮 P4 只收口原生问答的输入片段编译。P3 验收工作副本中，application/control.go 与 codex/lifecycle.go 各自解码/解释问答 Delivery，再选择结构化 AnswerParts 或原始 caller.answer；一次编码调整需要同步两处输入选择分支。

收口后由 Codex adapter 的纯值能力 PrepareLifecycleAnswer 校验原 route 并提供模型输入片段，原生发送前使用同一私有 answerInputParts。application 继续校验答案、冻结通用预算、保存意图及调用原 Execute/Reconcile；不解释 Delivery 或私有 codec。能力为可选窄端口，不扩大所有 backend 必须实现的 LifecyclePort，不携带 Record/Engine/Store/提交回调，不创建包装 resolver 或新组装层。

已验收收益（见 [P4 报告](../acceptance/personal-executor-p4.md)）：问答输入表示的私有选择入口由 2 处减为 1 处，application 私有问答解码/Delivery 解释由有变无；以不属于 Codex 的 opaque route/自定义片段验证业务协调无需随编码变化。权限资源、profile/helper、checkpoint、文件源本轮不继续展开；未证明具体收益的搬迁取消。本轮不改 native 协议/启动/回执/停止或数据格式，采用组件及公开受控问答/只读/unknown/停止回归，不重跑全后端或整套前置。

## 7. P5：按批验收记录与总状态汇总

P5 在每批交付时增量更新记录；不设置一轮覆盖所有阶段的最终验证来阻止前批交付。

| 批次 | 本批必需证据 | 真实范围处理 |
| --- | --- | --- |
| P1 | 参数/阶段/资格/清理合同，原默认/复用相关合同，正式 focused case 与耗时 | 至少一条实际支持路径；其余未验不冒充全级别资格 |
| P2 | 四组合及故障不阻塞、写次数/时长、字节快照/重启/stop 竞争 | 受改动的本机、Orca local/environment 最小文件接线分别列结论 |
| P3 | 退役规则和净删除、格式拒绝、无 observer 人工对账、重启及停止竞态；本批一次完整受控 | 受影响本机 L2 与 Orca 路径最小接线，按新契约验证，不借旧构建证据 |
| P4 每项 | 所有者/调用变化、重复入口删除、共享门禁未复制及受影响回归 | 机制未变采用受控；机制变化补对应真实探针 |

真实目标缺失则记录本批相应路径缺口和部分完成；不能把该路径标为已验，也不以此撤销其他批独立结果。Pi/DSH 仅在实际改动其机制时补受影响探针；不要求纯搬迁扩大全 Agent 资格矩阵。默认全量及交付包入口本身的完整资格规则不被 focused 替代。

各批记录实际 commit/dirty、EXE/driver 来源、命令、场景、耗时、收益门槛、外部资源收尾及缺口；旧 evidence 保留原路径与字节。当前文档、capabilities/目录和客户端说明随所属批次更新，前两批不提前宣布 v7 或删除恢复已完成。

更新总状态时分别列 P1/P2/P3/P4 各项已完成/部分完成/待调整及收益实测。未达到某项收益时不标为完整交付；全部验收满足才标记总计划完成。部署、原版本排空、旧目录处理和发布另行安排。

## 8. 本轮文档修订检查

- [x] 取消逐 Run SHA 构建门禁、混构建活动任务/差异投影；保留既有 SHA 来源/实例/资源身份用途。
- [x] v7 只归恢复删除批次，升级先排空，格式不兼容才再切目录；前两批当前 v6 可独立交付。
- [x] 保留 Next/QualifyAction 共享准备门禁，允许集中组装，取消全域零 import 目标。
- [x] 显式 resume 可在 unknown 待人工时受理，先有界只读原操作对账，再依真实事实和当前控制资格续行。
- [x] 四批独立验收交付，收益指标、净删除和变化面门槛明确，P5 不阻塞前批。
- [x] P1 已按 PES-A07/A08 独立验收：26 项 focused 合同、75 项前置复用合同、配置/调度/读取消费者回归及正式本机定向 case；不授予完整 L2/L3 或发布资格。
- [x] P2 已实施并形成独立交付；组件/公开入口、本机与 Orca environment 健康/故障探针通过。本轮补齐 Orca local 最终构建健康/故障 2/2、37 包/725 个 Go 测试的完整 L1、26 组 framework 与 29 项扩展，PES-A01/A02/A08 独立验收通过；原失败轮次保留，不扩大为完整 L2/L3 或发布资格。
- [x] P3 已独立验收：state/v7，三类退役规则删除，生产净减少 558 行；完整 L1 37 包/720 测试、Controlled 26/26 及同一生产 EXE 的三条真实最小链路通过，详见 [P3 验收](../acceptance/personal-executor-p3.md)。
- [x] P4 原生问答输入职责收口已按 PES-A06/A08 独立验收：私有输入选择入口 2→1，application 不再解码 route/解释 Delivery；145 项组件、25 项 application 定向及 3 项公开受控通过，详见 [P4 验收](../acceptance/personal-executor-p4.md)。
- [x] P5 已增量汇总 P1/P2/P3/P4 本轮选定交付结果；其他职责候选不扩大实施，部署/发布与外仓资格另计。
