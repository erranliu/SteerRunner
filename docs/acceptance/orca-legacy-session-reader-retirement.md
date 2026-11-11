# Orca 旧会话分页器退役验收

2026-11-11，基于 `49ce31d2a4ef6d4e3721ca0d91a9d9ae5753cf43` 的合并前定向验收。

## 实际消费者与变更范围

`OpenSessionReader`、`SessionReader`、专属页类型、`ErrSessionReadBudget` 和 RPC `nativeChat.readSession` 只在实现与 `TestOrcaSessionReadCapability` 中有引用。删除 `session_read.go`（115 行）及专属 `session_read_windows_test.go`（185 行）。删除后生产、测试、工具和脚本扫描无旧入口引用。

当前会话历史和原轮次重连仍经 `AgentSessionClient.History → agentSession.history`。stability 的 `QueryOriginalAgent` 和 `SubscribeState`、共享连接及文件/用量入口、历史冻结记录和只读停止证明的文件保持原样。此变更仅退役旧分页器及测试。

## 实际执行

Windows/amd64，PowerShell 7，所有 Go 测试均为定向 `-count=1`，无失败或跳过。执行节点含顶层与子例。

| 阶段 | 顶层 | 节点 |
| --- | ---: | ---: |
| 删除前 Windows 分页器 | 1 | 14 |
| 删除后 Orca 协议 | 5 | 33 |
| 删除后 Orca adapter | 3 | 19 |
| 删除后应用层观察重连与停止 | 2 | 7 |

```powershell
go test ./internal/execution/adapters/protocol/orca -run '^(TestOrcaSessionReadCapability)$' -count=1 -timeout=5m -json
go test ./internal/execution/adapters/protocol/orca -run '^(TestAgentHistoryRequiresCompletePublicEnvelope|TestOrcaNativeStateStream|TestOrcaLocalOriginalQuery|TestOrcaScopedFileCapability|TestUsageFilesRejectOriginalIdentityBeforeDial)$' -count=1 -timeout=5m -json
go test ./internal/execution/adapters/orcatransport -run '^(TestStructuredObservationRecoversPagedCompletedTurn|TestStructuredObservationPreservesNativeIdentityMismatch|TestStructuredHistoryFailsClosed)$' -count=1 -timeout=5m -json
go test ./internal/application -run '^(TestSROrcaObservationReconnectsOnlyTheConfirmedTurn|TestSROrcaObservationStopCancelsReconnectAndRejectsLateReceipt)$' -count=1 -timeout=5m -json
./scripts/build.ps1 -OutFile .evidence/orca-legacy-session-reader-retirement/final/steerrunner.exe
```

正式 Windows 构建退出 0，构建记录与实际产物及通过点输入一致。构建时 `source_dirty=true`，`source_digest=502b8e232c9ad963ea0cbcdd3c62160f46daade395858dd11cfb53a1a23ec809`，EXE SHA256 为 `c04997c00ecfad0596c2282c73de24001f3e6e1cad8cc84b986603337e20114a`。原始输出保存在本工作副本的 `.evidence/orca-legacy-session-reader-retirement/`。

验收限于上述定向集合与正式构建。完整 L1–L4、Controlled 和真实 Orca/stability 场景未运行。

## 提 PR 前主线补验

主线合入 #145 后快进至 `6f2b78ce5fbb37177f17c58b5ef71691fda54f62`。两个退役文件的差异和首次验收相同，旧引用复查仍为空。重跑删除后的同一 10 项定向测试，顶层和子例计数相同，无失败或跳过。原基线保留原记录。

正式构建通过，产物和构建输入身份复核一致。通过点 `source_dirty=true`，`source_digest=ceece9aa580b018f73f1bc0af15bbcd914765e59d7e277e074f2c94f107c1077`，EXE SHA256 为 `a3440f9540115792d8819da5641cf159c63b328166edfdc1f0c54474c3236c5c`。本地补验输出位于 `.evidence/orca-legacy-session-reader-retirement/pr-refresh/`。
