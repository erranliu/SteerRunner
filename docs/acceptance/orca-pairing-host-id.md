# Orca 配对公钥主机指纹收敛验收

2026-11-05，合并前验证。基线 `f6a389cfd4ae7827d7c722555a85e94f1e1ba708`（#114，含 #113），隔离分支 `codex/orca-pairing-host-id`。依据 [Spec](../spec/orca-pairing-host-id.spec.md) 和 [Plan](../spec/orca-pairing-host-id.plan.md)。宿主为 Windows amd64，Go `go1.26.4`。

生产 diff 仅涉及协议包四个文件：在 `state_stream.go` 增加私有 `statePairing.hostID()`，由 `NewRemoteAgentSessionClient`、`openWorkspaceFiles`、`openUsageFiles`、`UsageHostID` 调用，清理失效 imports。未改 Orca、公开 API、持久格式、能力清单、配对解析器或拨号参数。

## 执行结果

```powershell
go test ./internal/execution/adapters/protocol/orca -count=1 -json
go test ./internal/execution/adapters/protocol/orca -run '^TestStatePairingHostID$' -count=1 -json
git diff --check
```

| 执行 | 结果 | 范围 |
| --- | --- | --- |
| 原主线，沙箱内首次执行 | 失败，22.104 秒 | 本机测试 WebSocket 报 transport unavailable，另有子进程 mutex 用例 EOF；尚未修改代码。 |
| 原主线，相同命令在沙箱外重跑 | 通过，23.845 秒 | 55 个顶层测试通过，312 项通过事件（含子测试），4 项显式 opt-in 跳过。 |
| 候选，相同命令在沙箱外执行 | 通过，24.376 秒 | 57 个顶层测试通过，326 项通过事件（含子测试），同样 4 项跳过。 |
| 审计收紧测试断言后，固定向量定向复验 | 通过，2.648 秒 | 1 个顶层测试、10 个子测试；生产代码与上一行相同。 |
| 格式与差异检查 | 通过 | 修改的 Go 文件已 gofmt；`git diff --check` 无错误。 |

4 项跳过为 `TestOrcaLocalPipeNativeEvidence`、`TestOrcaLocalIPCNativeProbe`、`TestOrcaTerminalQueryLive`、`TestUsageLocalAcceptanceRepoCleanupLive`，均未提供其显式原生前置。不将跳过视为真机通过。

本地原始 JSONL 位于隔离 worktree 的 `.evidence/pairing-host-id/`：`baseline.jsonl`、`baseline-unrestricted.jsonl`、`candidate.jsonl`、`audit-recheck.jsonl`，未纳入仓库。上表保留实际结果及计数。

## 等价核对及审计

| 边界 | 证据及结果 |
| --- | --- |
| 指纹字节、格式和解码语义 | 新增 `TestStatePairingHostID`：两个独立 32 字节固定向量、CRLF、短字节、空字节和非零填充位均得到固定预期字符串；非法字符、错误填充及空格按原解码器返回空结果和精确 `base64.CorruptInputError` 值。方法不新增 key 长度或连接字段检查，解析器的原 32 字节验证未动。 |
| 会话身份、能力及未知回执 | 原 `TestRemoteAgentSessionWireAndDispatch` 和 `TestRemoteAgentSendWireAndDispatch` 全部通过，覆盖错 host/runtime/request、能力缺失、拒绝认证、取消、派发授权失败及丢失回执不补发。原 host 比较及 `stateIdentityError` 保留在调用方。 |
| usage 检查顺序及拒绝前零连接 | 新增 `TestUsageFilesRejectOriginalIdentityBeforeDial`，普通及 structured 两入口均覆盖：空 runtime 返回 `IDENTITY_UNVERIFIED`、错 host 返回 `IDENTITY_CHANGED`、同时缺 runtime/错 host 优先返回前者、配对不存在优先返回原解析错误；累计连接和文件请求均为 0。 |
| structured usage 能力 | 原 `TestStructuredUsageChannelDeclaresAuthAndRequestCapabilities` 通过，握手及请求保留原能力，不消耗 mutation gate。普通 usage 的 `capabilities == nil` 分支及原拨号未改。 |
| #113 单次解析与同配置拨号 | 原 `TestWorkspaceFilesUseTheVerifiedPairingSnapshot` 六个场景全过：normal、replaced-before-open、replaced-after-resolve、original-unavailable、wrong-runtime、wrong-instance。每次解析计数 1，替代端点连接/文件请求均 0；公钥和 token 不同且 runtime 相同。成功时原端点连接 1 次、文件请求 2 次；原错误分类保留。 |
| 本机原 runtime 与文件绑定 | 原 `TestOrcaScopedFileCapability` 及包内 Windows IPC 用例通过，本机分支未改；错 host/runtime、scope、预算及文件字节边界沿原断言。 |

逐行审计生产 diff，确认共享方法只解码接收值公钥并计算指纹，无配置读取、解析、主机比较、网络或共享状态。各入口的原检查次序、错误映射、能力声明、同一配对值拨号及原未知回执行为保留。未发现需要改动运行逻辑的问题。

审计修复一处测试断言不足：原 `errors.As` 比较会允许解码错误被包装；已改为直接比较原错误值，防止原错误类型或返回形态变化，定向复验通过。该修复不改变生产代码。

本次为纯算法提取，以上是受影响协议包及本机 IPC/加密 WebSocket 受控等价验证；未执行完整 L1/Controlled、完整 L2/L3、新双端真机或原目录升级排空，历史 #113 证据不追认为本候选资格。发布门禁继续遵循 [测试指南](../testing.md)。
