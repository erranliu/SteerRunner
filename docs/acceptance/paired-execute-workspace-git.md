# 配对 Orca 的 execute/1 工作区 Git 证据

2026-11-16。开发基线 `a6e731cc8504bfc4a994daebae2c3badec35596b`。公开入口和不确定状态见 [R2 工作区证据](../run-evidence-r2.md#当前-git-与诊断)。

- 台式机 64G 的只读原生探针确认 `git.status` 在一条回复中提供完整 HEAD、分支和状态条目。该探针使用已登记的 SteerlineTest checkout，未创建工作区、终端或模型会话。
- 定向 Go 测试通过。覆盖同一实例的干净和未跟踪文件样本、HEAD 或条目变化、分支错误、缺失或截断的状态、工作区替换、工作区已转交，以及原本机证据与 Orca 复用行为。
- 验收构建为 `.evidence/remote-workspace-git/steerrunner.exe`，SHA-256 `18abcf83a17c4102618e0fa2bb4b502c57bffa0c80c1222561bc00f2679f8a1c`。`capabilities --offline --json` 公布 `run_evidence.git.targets` 中的远端组合。
