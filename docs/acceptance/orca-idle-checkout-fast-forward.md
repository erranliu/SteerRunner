# Orca 空闲终端与远端 checkout 快进

2026-11-16 在已合入 #166 的 `4b357b38` 上构建独立 Runner，在「台式机64G」的配对 Orca 使用临时 Git 仓库。验证仓库有独立 bare origin、checkout 和检测入口。受测 EXE SHA-256 为 `0c23e2637212915b190275aa39a0b887dde4f491a90d81d8d7dd9e07bf9cfbce`，生产和测试源码摘要为 `42dd3c574f5545e89b949e3af2b49268be680dd4e6420490b51e1df153f46996`。

首轮只检查前台进程和子进程。Run `c5bac199667bff1df6fcbf9b86aada89` 在子 PowerShell 运行时错误放行。真机查询确认 Orca 在子 PowerShell 和 ping 运行时均可能报告 `no-children`。复验构建对 Windows PowerShell/CMD 进一步读取原终端的当前屏幕，要求可识别的默认空闲提示符。自定义提示符、状态不明或身份变化仍会阻断。现有结构化会话标签保护继续适用。

| 真机步骤 | Run | 结果 |
| --- | --- | --- |
| 快进前核验材料 | `ab35aa927200091cdbc87e7a86eb2ce7` | 封存 `matches: false` |
| 保持子 PowerShell 运行 | `1bcf4993673ee264b8b93284121bd4b5` | 进入失败终态，零快进终端派发 |
| 保持 PowerShell 内部命令运行 | `8129f63e684d88fdbc8e14b522e13b1e` | 进入失败终态，零快进终端派发 |
| 保持空闲 PowerShell 打开 | `2e9cb62c1bf55d08adefb572f8a39aaa` | 从 `162d4da9` 快进到 `e3e83387` 并封存结果 |
| 快进后核验材料 | `6601056128327415578b2276608b8556` | 封存 `matches: true` |
| 执行更新后的项目检测入口 | `3a280a7de7242c1e8a29d9dde80da1cd` | 封存 `{"live":"after-v2"}` |

Windows `go test ./internal/execution/adapters/protocol/orca ./internal/execution/environmentcheck -count=1` 通过。独立工作树中的 `.evidence/orca-fast-forward-live-v2/` 保留构建回执、请求、Run、封存结果和快进生命周期事件。308 正式 checkout 仅仅读取过 Orca 状态，验收检测在临时仓库执行。

两轮独立 Runner 服务均核实为停止。远端测试终端已关闭，临时仓库已注销，专用目录已删除。
