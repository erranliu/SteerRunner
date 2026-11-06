# Plan B：旧 Orca 执行支线退役

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-05。状态：P0–P4 完成；A 后实施，已整合 main `a78d87da`。依据 [Spec](orca-legacy-execution-retirement.spec.md)，实际资格与基线失败见[验收](../acceptance/orca-reader-execution-boundary.md)。

1. P0：在 A 验证点固定生产调用图；生成旧 writer 根、独占 helper、reader/当前 workspace/usage/files 保留项及顶层测试/共享夹具清单。确认新请求和重启两个入口均已关闭旧链。
2. P1：工厂只装配当前结构化执行，删除各旧执行分支；按清单删独占 helper/进程内字段。保留 A reader、持久类型和公共 workspace 创建/原 receipt 对账。
3. P2：逐项退役旧写行为专属 case/opt-in，保留混合文件中的 shared fixture 和 reader/current 反例；旧记录 fixture 改为直接构造冻结事实，不重建第二旧执行器。编译所有保留包和 live probes，处理实际消费者，不以 skip/弱断言补洞。
4. P3：当前 Orca、历史读取、原操作恢复/停止及相关 application/host/contracts 回归；原编码/声明比较，正式 Runner/driver 构建和公开受控。需要真实环境的资格如实单列，不动用户已有执行现场。
5. P4：审计 diff/消费者/共享夹具/持久原字节，更新 Spec/Plan、长期实现说明和独立验收；A/B 分别记账，留工作树供后续审阅，不自动提 PR/部署。

P2 入口补充：注销旧 `orca-remote.native-input` 专项，原脚本仅作明确拒绝入口；不改变通用场景、共享模型桥或当前结构化专项。保留原终端只读方法与并发测试，fixture 不再调用旧执行工厂。

共享公开夹具审计补充：`TestSRResourceCreationRecovery` 和 CLI 排队案例共同使用 creation wire/IPC fixture，必须保留。前者五个工作区 fault window 保留；只退役依赖旧 initial-terminal 计划的两个子例。已恢复工作区但尚未派发 Agent create 的停止，核对原 worktree/runtime 与 `session_not_created`，不再要求旧 `orca-terminal:*` session。receipt-before-commit 门移到结构化链真实读取的 worktree.show；共享 fixture 对 terminal send 从仅允许旧 bootstrap 收紧为零输入，不能实现第二旧执行器。

收尾主线整合：在独立工作树解决三处真实内容冲突，保留 #117 新增的 `worktree-receipt-without-terminal`、`missing-creation-plan` 两个当前反例及 RPC/shell 身份审计，最终创建恢复为七个窗口。Orca 类型与相同目标 main 编码一致；adapter 72 顶层通过，最终六个公开受控场景零失败/零跳过。扩展测试的两项提示基线失败均在独立 main 复现，未冒充完整 L1/Controlled 或真机资格。A/B 实施点 `0a374215`，主线整合点 `68c3057a`，无历史重放或生产部署。
