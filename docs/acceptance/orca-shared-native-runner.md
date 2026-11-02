# Orca 双端原生 Runner 候选 B5 验收进展

日期：2026-11-02。状态：进行中，未取得真实 Runner 资格，不开始 B6。

候选完整接线到公开 submit/freeze、profile/state codec 9、同一原生 Lifecycle、正式 Engine 停止与持久重入、在线/离线 orca_terminal scope。旧本机 managed 合同及其他组合仍保留。

受控六包回归：456 个根、1565 个子节点、1789 个叶例，2013 pass、7 skip、1 fail；失败为旧远端无冻结 scope 时工作区来源错误回退到 helper owner。已恢复旧远端目标判断，并补本机候选同类拒绝；定向复验 2 根、6 子节点、6 叶例全通过。原始失败保留，未改记为全套通过。

双端停止矩阵在真实构造器、CLI FIFO、正式冻结/Engine/存储路径覆盖未知关闭、活动资源、替换 incarnation、缺 host、错 runtime、截断、查询失败、回执提交失败及原请求重入。上述六包运行中矩阵全通过。排队取消零派发、后续首次关闭通过，额外加了测试异常退出释放屏障。

受控正式离线 CLI 回归：1 根、30 子节点、26 叶例全通过（旧本机/远端及新双端候选）；测试配置/Manifest 包 41 根、191 子节点、215 叶例全通过。live-tag 编译、场景目录脚本校验通过；编译和受控 fixture 不计真实资格。

新增注册 driver 只通过正式 CLI 提交/反馈/停止，不构造 Run/Snapshot，不代模型发布、不合并测试 PR；记录精确 CLI/转发器/Runner 身份，验证同原 terminal/provider 的 pre-pr_ready 反馈、测试 PR 仓库/base/head、原清单、Run stop/service stop/真实离线复核。四个真实场次尚未执行。OSR-03/04 真实部分及 OSR-05 审计待闭环。

当前原始记录在 .evidence/orca-b5；完成时按已有可复现 UTF-8 文件与 SHA256 格式归档，含初始失败。

后续修订：77c7ad8 修复测试 Manifest 对 Windows 正斜杠 CLI 路径的误拒绝；独立审计复核通过。1334745 补双端正式 Engine 的 totalCount/rows 不符反例，两个叶例全通过，不重发关闭、不结算。排队释放屏障后的正常主机权限复验通过；沙箱 Git 临时仓库写权限失败另保留。

真实场次 5a64408f87c1 在 service start 前因测试路径比较失败；准备终端已确认关闭，无 Run/PR。场次 3e4b30af354f 正式 Run 03dabd99220327b14179c99ad70c0058 已验证原请求 Started、完整 needs_input final，业务反馈被 Engine 的旧 gate 拒绝：workflow.FeedbackStep 明确支持 input.Continue，Engine 只认 ManualImplementation。本次 Run stop、service stop、真实 offline 验证均通过，关闭一次、零旧链，但业务场次仍为 failed、没有 PR，不能计资格。

针对上述入口不一致，允许 waiting_input 的冻结业务 input Continue/ManualImplementation，活动 native question 的 execute 状态和 milestone 继续拒绝；原 qualifyContinuation 的 session/resource/idle 校验不改。新增双端正式 Engine 的 idle/active 四例验证：先保存显式 feedback，只有原会话 idle 才冻结一个后继输入，非 idle 零输入/零派发；活动 execute 和 milestone 入口均拒绝。四例通过。准备未执行的远端 674b961e26f4 不记资格，保留其已关闭准备终端与 base。
