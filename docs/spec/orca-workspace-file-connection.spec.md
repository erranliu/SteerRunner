# Orca progress / result-file 连接绑定 Spec

以 #112 合入后的 `main`（`e7876c0d`）为基线。`OpenWorkspaceFiles` 已共用双端 worktree 校验和文件 RPC。远端现先通过 `UsageHostID` 解析配对并校验公钥，再由 `openStateConnection` 重新读取配对后拨号。两次读取之间若配置被替换，拨号可能使用未经前一次检查的配置。本 PR 只收敛文件通道的连接取得过程。

## 连接与错误合同

| 阶段 | 要求 |
| --- | --- |
| 本机 | 继续要求原绑定的 host 为 `local`。复用本机 named pipe、原认证、启动元数据 runtime 校验，以及 `status.get` 和后续回执的原 runtime/request ID 校验。本机分支不读取远端配对。 |
| 配对远端 | 每次打开文件通道只解析一次显式环境的配对。将该值的公钥摘要与原绑定中的 host 摘要核对，再把**同一个配对值**的 endpoint、公钥和设备认证用于拨号。沿原加密握手、能力声明和 runtime/request ID 校验。身份不符即拒绝。配置更新发生在校验后时，本次只能使用已校验的配置，拨号失败仍按原错误处理。 |
| 共用文件入口 | 成功连接后继续按原 worktree ID、路径、已绑定时的 instance、文件范围和字节上限检查。只有这些检查通过才返回文件句柄。失败时按原行为关闭已打开连接。保留 `files.readChunk` 和 `files.writeBase64` 的参数，包括初始化请求原有的 `expectedExecutionHostId`。 |

保持现有错误分类。非法绑定、无法解析原配对或公钥不符返回 `binding_invalid`。本机连接或 runtime 校验失败、远端已校验配置拨号或握手失败，以及 `worktree.show` 的 RPC 失败，沿原 `transport_unavailable`。已收到的 worktree 事实不符仍为 `binding_invalid`。文件读写和边界错误沿原实现。

在现有 Orca 协议包内优先用一个私有连接入口，直接复用原本机打开与远端 socket 函数。复用现有冻结绑定、配对解析器和 socket 类型。无需新持久状态、连接池、跨通道接口或目标抽象。不改 Orca、`agentSession`、usage、其他状态连接调用方、旧记录准入及公开 wire。进度、结果文件初始化与封存、停止证明和升级排空合同保持原样。已使用单次快照的路径继续沿其原合同。

## 验收

- 受控夹具覆盖连接前配对已替换、一次解析后配置被替换、本机 runtime 错配及 worktree/instance 错配。替代远端使用不同公钥和可用 endpoint，但故意通告同一 runtime。验证每次打开只解析一次，替代端点零拨号和零文件 RPC。原端点正常时只使用原绑定，原端点失败时返回原错误。保留正常双端文件读写和原错误分类。
- 同一候选在本机和真实配对远端各跑一次默认 Orca Codex `execute/1`。真实执行期间用 `progress-file` 读取原工作区新写入的进度字节。用 `result-file` 验证停止前仍待封存，取得原会话关闭证明后才封存实际结果字节。校验原 Run/input、runtime/worktree/instance、版本、摘要及重复读取稳定性。
- 用原写入 EXE 正常停服，两个实际原目录分别通过 SteerLine 原 `VerifyUpgradeDrained` 双 EXE 冷读，保留原停止范围和无未决控制证据。受控配对替换使用隔离夹具，真实配对保持原配置。
