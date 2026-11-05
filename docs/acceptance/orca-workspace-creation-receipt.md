# Orca 工作区创建回执收敛验收

2026-11-05，主线基线 `9c36f366`，候选源码 `6869b678a84f8fce634f2e711eb0030c1d429fac`。写入和冷读 EXE 均由 `scripts/build.ps1` 从同一干净源码构建，源码摘要为 `644688b85d2903eae2beb32d41f1d4e936f55517077a007f65538b4972c21c4a`。

| EXE | SHA256 |
| --- | --- |
| 原写入 EXE | `0d8bcb25c44135c6a15e9e10592fa4f870c83f63b4c821882ba3cebbc3314e1f` |
| 冷读 EXE，`-Version 0.2.0-workspace-receipt-reader` | `ea08b28bde26f499395b8e25dcd445507e4d31bd0e56c2147a2b99d72332c08d` |

## 受控恢复

Windows 执行 `go test ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/orca -count=1`，两包通过。

- 双端 adapter 夹具在 owner 输出可读、原工作区查询失败时重建生命周期，从原 owner 输出恢复同一份 checkpoint。只派发一次创建，恢复阶段无仓库发现、额外创建或业务输入。
- 历史回执夹具覆盖本机缺少可选 resolved runtime、已有扩展字段、配对远端原 ID、外层 operation 归属和原有 checkpoint 重新编码。旧回执读取沿用现有语义。
- 公开 CLI 定向集成的 `wire-loss` 与 `owner-crash` 原用例通过，确认服务重启后原创建仍未知、无后继授权、没有再次创建或接管工作区。
- 主线原 `receipt-before-commit` 用例未到达故障窗口，旧夹具等待 `terminal.show`。以临时测试 overlay 将同一故障窗口放在结构化路径实际使用的 `worktree.show`，并按原结构化工作区和会话未创建时的停止合同断言。使用固定候选 EXE，`graceful` 和 `receipt-before-commit` 两场均通过，确认原工作区 operation、一次创建、零终端创建和零业务发送。overlay 未进入交付 diff。

## 双端真机

| 目标 | 原 Run | 原工作区 operation | 原 selector |
| --- | --- | --- | --- |
| 本机 Codex `execute/1` | `b67fbad1f2851944285712e0ee6ed87b` | `0520fe9cdf30d2227bad436ced3b2cd1` | `path:` |
| 配对远端 Codex `execute/1` | `b84fb2a1e380cc4e9b8eb179c3cf7ee4` | `5d1cd1323c15f2d0f5e814dc5c7afe95` | `id:` |

两场串行使用同一写入 EXE，每场取得一套原创建计划、派发和输出。以摘要核对计划、派发、输出与原 Run/input/operation 的归属，并核对响应 runtime、worktree ID、instance、固定 base 和结构化会话绑定。两场均取得预期三行可信结果，关闭原会话，撤销恢复授权，封存版本 1 的 `steerrunner-result-file/v1`。文件验证使用预置内容及其摘要。

两场正常停服后，使用 SteerLine `e010373e` 的生产 `VerifyUpgradeDrained` 和两份固定 EXE 冷读各自原目录。两端排空均通过，停止范围为 `orca_agent_session`，冷读前后数据库和停机证明摘要一致。原始 CLI 回执、创建绑定证明、临时 overlay 和冷读日志位于本机 `.evidence/orca-workspace-receipt-live/`。本记录覆盖受影响组件、定向受控恢复及专项真机。

## PR #110 合入前审计

首次创建和原计划恢复仅共用 CLI 结果解码与绑定。恢复进入共用函数前仍要求原计划的 runtime 非空及原 selector 通过校验。CLI 外层 runtime 优先，原 selector 提供远端仓库 ID。已保存的 operation 回执保持原读取分支。主线 `9c36f366` 用相同历史回执夹具复验通过，与候选的接受及重新编码行为一致。

复验核对两份候选 EXE 的实际摘要及源码身份，重新读取两场真机的原创建计划、派发和结果摘要，并再次通过双 EXE 原目录排空。另用 #109 原写入 EXE 与 #110 冷读 EXE 验证 #109 的双端 `execute/1` 原目录，两个原目录均通过 SteerLine 生产升级排空。四个原目录冷读前后的数据库和停机证明摘要一致。

定向重跑原 owner 恢复、仓库及 runtime 错配、未知结果和旧回执用例均通过。用同一固定写入 EXE 复验四个重启集成子例，原 `wire-loss`、`owner-crash` 及结构化夹具的 `graceful`、`receipt-before-commit` 均通过。每例只观察到一次原创建，恢复未派发第二次创建或业务输入。
