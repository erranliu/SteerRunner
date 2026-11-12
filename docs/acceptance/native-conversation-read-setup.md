# 原会话分页读取装配验收

2026-11-12，基于 `83e186c54af742b9d63c4572e1b2e27bddde5ba2` 的合并前定向验证。

`native-conversation-history` 从冻结快照和原 profile 装配读取，去掉临时 worker、执行 owner 回调及本机进程探针的装配依赖。Orca adapter 复用现有结构化 profile 解码和状态校验，再绑定原 runtime 和配对身份。冻结 `AgentLaunchCommand` 仍传入旧启动配置校验。分页和单条正文继续调用原 `agentSession.history` / `agentSession.item`。

Windows/amd64 定向测试通过。应用层用隔离命名管道读取首屏、较早页和完整正文。错 Attempt、非法 cursor 和旧启动配置均在 RPC 前拒绝，原查询在途换代时返回 `stale_revision` 且无旧页数据。原分页、单条正文和共享状态校验回归通过。宿主与 CLI 定向回归通过，包括分页命令的服务绑定拒绝分支。应用用例首轮夹具漏了历史响应的内层 `ok`，按原协议补齐后通过。

```powershell
go test ./internal/application -run '^TestReadNativeConversationUsesOriginalBinding$' -count=1 -timeout=2m -v
go test ./internal/execution/adapters/orcatransport -run '^(TestNativeConversationPagesOnlyReadOriginalPublicMessages|TestNativeConversationBodyReadsOnlyTheSelectedItemAndKeepsUTF8Boundaries|TestStructuredFilesWhileNextInputQueued)$' -count=1 -timeout=3m -v
go test ./internal/host ./internal/cli -run '^(TestSRFullBindingRejectsBeforeEffects|TestSRFullBindingCLI|TestSRCLIContract|TestNativeViewerDoesNotStartService)$' -count=1 -timeout=3m -v
./scripts/build.ps1 -OutFile .evidence/native-conversation-read-setup/final/steerrunner.exe
```

正式 EXE 另以本地临时测试 overlay 接入相同 Orca 夹具。公开 CLI 经隔离常驻服务成功读取三种结果，退出时确认该服务已停止。该次覆盖使用本地 `.evidence/native-conversation-read-setup/overlay.json` 及同目录夹具副本。验收范围为定向组件、隔离 CLI 冒烟及正式构建。

构建记录与通过点输入及 EXE 一致。source_dirty 为 true，source_digest 为 e76d76b19ac8d3096f2ce07e709ee8a0640ef21d668a5b9d28fd8d18f8a02a8a，EXE SHA256 为 b8eca340bcff96af80d884599b7472dc700d3da8505a4be568b03855567a18d7。
