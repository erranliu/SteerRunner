# Orca 双端执行合同

当前支持声明（2026-11-03）：Codex / development/2 / full_access 的本机与明确 environment 使用同一原生 Orca 执行合同。正式入口、冻结、完整原输入与结果、业务反馈、发布核验和停止均已在 B5 接通；B6 已同步公开合同，并通过最终集成构建双端开发及活动停止四场、原资源离线与独立审计。实际资格见 [B5 验收](acceptance/orca-shared-native-runner.md)、[B6 验收](acceptance/orca-native-rollout.md)及 [B6 Spec/Plan](spec/orca-native-rollout.spec.md)。不修改已安装服务或用户原配置。

## 选择双端原生合同

本机 execution 示例：

```json
{
  "adapter": "orca-terminal",
  "target": {"kind": "local"},
  "settings": {
    "agent": "codex",
    "access_mode": "full_access",
    "transport": {"executable": "C:/path/to/orca.exe"},
    "agent_launch": {"executable": "codex"}
  }
}
```

Workflow 使用 `{"id":"development","version":2}`。远端仅把 target 改为 `{"kind":"orca_environment","environment":"明确注册的环境名"}`；调用端仍使用同一 Orca CLI，执行主机须有匹配注册仓库、Codex、Git/gh 原生登录。连接与仓库 selector 按原目标解析，不从其他主机代执行。Spec/Plan 路径在固定 base 的原工作区读取。

此组合不配置 `agent_launch.runtime_executable/runtime_sha256`，不启 helper/control/history/终端扩展；未配置模型时沿宿主默认，配置模型/effort 也只是启动偏好。没有额外开关、第二 adapter 或迁移包装。`capabilities.orca_execution_contracts` 描述此选择和保留例外；实际准入仍由原配置 schema 与生产 gate 决定。

## 用户可依赖的边界

- `stop_evidence_scope=orca_terminal` 在受理时冻结，两端相同：按原 runtime/worktree/local host 完整清单验证精确 handle/incarnation 退出或缺席。它不证明全部子进程退出，也不撤销主机凭据。
- accepted 不代表 Started。活动 Started 来自独立原输入及 working 事实；该查询不授予流覆盖，完整 final 仍需独立原输入/结束证明。模型自述和终端画面不替代这些事实。
- 实际模型未独立核验，工作区审计来源 unreported；本合同不支持 development-audited。真实 Git/PR 仓库、base/branch/head 核验仍执行。
- needs_input 后在允许的 implementation 输入阶段用正式 reply(kind=feedback) 继续原会话；新输入仍须原会话 idle。pr_ready 没有反馈边，PR 由用户决定是否合并。
- Run stop、service stop 与资源结算分别报告。先正式 stop 原 Run，再停止服务退出 writer，最后用原 EXE/目录执行 `service verify-stopped`。只停服务不证明原业务终端已经停止。
- close 可能已执行而回执未知时，只查原完整清单，不重发、不建替代终端；明确在 CLI FIFO 未派发的取消才保留原操作首次资格。

## 保留例外及真实消费者

| 组合 | 保留原因和实现 | 本次原生资格 |
| --- | --- | --- |
| 本机 Codex 显式非空 helper，包括 audited | control_terminal / local_codex_history / 原模型与 workspace 来源；local_codex_stop 管登记业务/控制终端及 helper、app-server 根，scope=orca_local_codex_resources | 不继承；继续原显式 managed 合同，不能静默移除 helper 或改 scope |
| 本机 Pi/DSH managed | TerminalConfig/terminal_receipt 的工具准备和原证据仍有消费者 | 不继承；补原生 agent 证据另批验证 |
| 其他 Agent/Workflow/旧 private codec | 原 admission、证据、usage、只读对账及排空消费者仍存在；旧远端原生合同保留 | 不因首轮 Codex/development 通过而新增资格 |

这些例外不是新建的两套链路抽象：它们消费现有 Orca 能力，同时保留各自必要的执行证明。已统一组合只留目标连接/仓库解析差异。没有无消费者的整段 helper/control/history 可删；本轮只删除 PreparationFacts 中被紧邻 agentExecutionSupport 完全覆盖的重复拒绝分支。后续实际模型、workspace audit 或其他 agent 能力补齐时，再逐组合取得资格并删除最后消费者；不能先降保证再宣称统一。

## 升级与原记录

外层 state/v8、user_version 和既有路由摘要不变；原生 profile/state codec 9 与旧4/6、7/8是私有合同区别，不代表目录自动迁移到 state/v9。旧 Run 不回填、不重释，不添加旧活动任务接管机制。

本批启用流程使用原 EXE 先排空 Run 与所有未决操作、正式停止和真实只读核验，再以新 EXE 和 fresh 目录开始新 Run。兼容客户端能够选择原协议服务实例，不表示替换原 writer 或活动升级已发生；既有兼容性规则保持，不把 EXE 文件名变化当作接管授权。不自动更改用户已有服务、目录或配置，也不部署本次构建。
