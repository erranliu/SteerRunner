# PR 合并后复验

隔离 worktree：`C:/Users/lmas1/.codex/worktrees/content-workflows-pr/SteerRunner`。
合入 main `7d830e7`；最终生产源码为 `46d66d7645f653277dd4b8617a5f632139ea3826`。
后续提交仅登记文档与证据，不改变受测生产源码。版本与摘要见 [build](build.json) 和 [version](version.json)。

- 规则测试：`scripts/test-content-workflows.ps1` 的最小 owner/应用测试通过，见 [content-rules](content-rules.log)。
- 完整选定 owner 包：contracts、resources、cli、storage、workflow、reliable、operation、codex、resultacquisition、workspace。首次 resources 检查发现退役认证字段仍出现在 schema；修正后 resources/contracts 全包复跑通过。首次结果与修正结果分别见 [owner-initial](owner-initial.log)、[owner-correction](owner-correction.log)，其余包首次均通过。
- 最终受控及开发回归：`scripts/test-foundation.ps1 -Pattern '^TestSR(ContentGenerate|ContentConversation|ContentQuestionAndUnknown|RunWorkflow|NativeUnknownAndStop|RestartRequiresResume)$' -OutDir .evidence/content-pr-controlled-final`。6 个场景全部通过，[日志](controlled-and-development.log) / [回执](controlled-and-development-result.json)。
- 真实 Codex：`scripts/test-content-workflows.ps1 -NativeExecutable <本机 codex.exe> -Model gpt-6.1-sol -OutDir .evidence/content-pr-validation`。实际 Codex 0.159.2，结构化生成与持续对话、中断后继续、空闲重启继续均通过，见 [日志](native-integration.log) / [回执](native-result.json)。原生会话为 `01a153ca-73ef-7921-94b3-87f62c988691`，从未创建替代会话冒充恢复。
- 最终受控与真实 Codex 使用相同 EXE SHA256：`5892bd49ebc37500486924f4f8d0f8e087a0aa4b791bdc1c6cf454c247e4b000`。公开 [verify-stopped](verify-stopped.json) 核实两个 Run 均已结算。

[capabilities](capabilities.json) 及同目录输入 schema 从该 EXE 离线导出；业务请求示例仍见首次验收目录与 workflow 文档。只构建隔离产物，未改安装环境。未跑 full L1–L4，也不将受控人工问答、坏 JSON、未知结果反例说成真实 Codex 资格；此前应用全包聚合超时记录保留在首次验收目录。
