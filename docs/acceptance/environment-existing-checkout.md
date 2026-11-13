# 环境检测复用现有 checkout

2026-11-13。Runner 在独立 worktree 实现 checkout_probe/v1。调用方冻结 manifest 机器段、入口和声明依赖的摘要。执行机复用已登记的 checkout，并在调用前后核对仓库与材料。直接 Windows 执行使用原生 Job 持有进程树，Orca 使用原运行时的独立终端。项目报告和临时文件在仓库外，退出码与结果分别核验并封存。

- Runner 定向回归覆盖 CRLF checkout、不同 HEAD、Git URL 可选 .git 后缀和材料漂移拒绝。
- 隔离 Windows 仓库的本机进程及 Orca 原生终端均创建 Run、完成检测并封存原结果，未创建额外 Git worktree。
- 已验证真实配对机能发现 308 checkout 并在仓库身份不符时阻断项目入口。真实项目最终 ready 状态见配套 PR 的验证说明。
