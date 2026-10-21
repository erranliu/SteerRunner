# 可靠性交付受理与观察接口验收

2026-10-21，依据[实施 Plan](../spec/reliability-simplification.plan.md)。当前工作区包括其他未提交改动；结果只证明本批公开接口及列明边界，不是干净发布、完整原生执行或安装资格。

| 项目 | 结果与证据 |
| --- | --- |
| 实际受理前完整 binding、所有覆盖命令缺字段/错实例/目录/EXE/version/resource/protocol/ready | TestSRFullBindingRejectsBeforeEffects 通过，handler 不可进入 |
| 停止与新受理顺序 | TestSRFullBindingStopAdmissionOrder 通过，原已受理 handler 返回前 stop 不越过；已接受 stop 后新 submit 拒绝 |
| CLI 完整绑定解析、冲突与零隐式服务启动 | TestSRFullBindingCLI 及既有 TestSRExpectedInstanceDoesNotStartService 通过 |
| 同读事务分页、空页后新增、并发提交与无丢事件 | TestSRObservationSnapshotPaging / TestSRObservationConcurrentCommits 通过；原 TestSRRunAtomicCommit 通过 |
| 原实例及 writer 退出证明 | TestSRExpectedInstanceFencesReplacement / TestSRShutdownProofRequiresReleasedWriter 通过 |
| SteerLine 实际公开 CLI 联验 | 固定新 EXE，真实临时空闲服务；正确 binding、错 binding 明确拒绝、observe 不存在 Run 返回 run_not_found、原 stop 和资源结算通过；没有业务 Run/原生模型 |

本仓原始日志在 `.evidence/reliability-simplification/`，SteerLine 的 `.evidence/reliability-simplification/phase-b/` / `phase-f/` 保存两仓联验和调用方证据。最终公开 CLI 构建 SHA256 为 `4df6d5a73e184b138eafd6c727becf5b65bd3dd0187cae2acd12015dbb85ca0f`，resource digest 为 `2bcf605cf715bf1e7c9456198fe935fd897222dfe19edd6a8749512a50baa37e`。

修订中曾误用既有 observation.go 文件名导致编译失败，已恢复原文件并将新接口移至 run_observation.go。公开 CLI 首次 capability 检查失败，修正从 version 移到 capabilities 后重新构建联验，未继承旧二进制结果。失败与最终日志保留，不计失败为通过。

未实现安装租约、完整 caller census、owned writer 上下文或安装续装。read_only、未知操作、原生输入授权与停止事实不因新读取接口提升；未运行全量、真实模型或安装现场。
