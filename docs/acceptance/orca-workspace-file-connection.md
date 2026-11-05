# Orca 文件通道连接绑定验收

2026-11-05，基线 `e7876c0d`，候选源码 `ec83f90410b8d0b90a028a37ea49fa3345742e1e`。写入和冷读 EXE 的源码摘要均为 `5fd160e8ec35885305517becd17f7eacf48fbe1fb5bfa0558aebcf1072e10bc3`。

| EXE | SHA256 |
| --- | --- |
| 双端共用写入 EXE | `992b029de3d5601d9fd793e72f8cfc868086ebf44204e379c5bda8a76f442a8d` |
| 独立冷读 EXE | `dcde06e4d3fee6c25977babfa8622e5e6e276ebb19f598992665bcb47d21be3a` |

## 受控验证与审计

- 现有本机 IPC 和新增加密 WebSocket 夹具通过。替代端点使用不同公钥并通告相同 runtime，覆盖打开前替换、一次解析后替换、原端点不可用、runtime 不符及 instance 不符。每次打开只解析一次配对，替代端点零拨号、零文件操作。本机原 host/runtime 错配保持原错误分类。
- 临时 Go overlay 将拨号改回重新解析配置，替换后和原端点不可用两个反例均失败。正式实现同组通过，确认回归断言可发现原双次解析问题。测试只操作隔离配对文件。
- `go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -count=1 -timeout=6m` 通过。应用层进度、结果文件及单次执行封存定向回归通过。
- diff 审计确认生产变化限于文件连接取得。连接后的 worktree/instance 校验、文件参数、范围和字节限制沿用原实现。会话、usage、通用状态连接、封存及停止代码保持原样。

## 同候选双端真机

原 Spec 将进度场景写为 `execute/1`，现有生产准入明确拒绝该 Workflow 的 `progress_file`。两端首次测试提交均在受理前返回 `invalid_input`，原服务已停止，未创建 Run。Spec 和 Plan 据此改用已有支持的 `development/2`。同一 EXE 在新目录完成以下两场，生产准入和预算保持。

| 目标 | 原 Run | 原 turn |
| --- | --- | --- |
| 本机 Codex `development/2` | `f1fd6ebf7629d888d29fd812afe4e8f3` | `01a1aa0a-ab53-7c01-a428-7ce6ddf8543f` |
| 配对远端 Codex `development/2` | `28586075f0ad3ba56e15802645f8b537` | `01a1aa0c-245e-7e22-a9bf-d2bc9eb465c0` |

两场的原执行回合实际写入进度和结果文件，内容不同于初始化字节。活动窗口内，在读取文件前后核对原 Run 仍运行且 turn 不变，`progress-file` 返回预期新字节，`result-file` 仍为 `pending`。可信结果进入等待输入后显式停止，原会话关闭且恢复授权撤销，结果按 `steerrunner-result-file/v1` 封存版本 1，重复读取的字节、观测时间和摘要保持。进度版本 2 保留预期字节，并在停止后关闭。

| 目标 | 进度 SHA256 | 结果 SHA256 |
| --- | --- | --- |
| 本机 | `0b6cd255ace38fb1004f9aa4663ea2cb82fe46ecbda85fdc89fe6cc031604d42` | `eb147986f4752c36f68a6ec6bcc2f74516ded15516883f22a5e4c3694671285c` |
| 配对远端 | `672f0799c8f1affd53800e3a28a63817cd52169e8141df751f8f671eb8b3adcd` | `f22f45d2c8bc7af18bb8ae9deacac7b1e8d8ed4ada9d2a77ae98a9b96c9bbe28` |

审计冷读持久记录，核对冻结正文摘要、原消息/turn、文件绑定的 host、worktree 和 instance。两个实际原目录通过 SteerLine `57e3c52` 生产 `VerifyUpgradeDrained` 的双 EXE 冷读，原服务均 `not_running`，停止范围为 `orca_agent_session`，数据库与停机证明摘要不变。原始 CLI、活动窗口证明、临时变异测试和冷读工具位于本机 `.evidence/orca-workspace-file-live/`。本记录覆盖受影响组件、定向应用回归和专项真机。
