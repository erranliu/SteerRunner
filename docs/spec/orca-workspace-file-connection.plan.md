# Orca progress / result-file 连接绑定 Plan

实施、审计和验收结果见独立记录。 [Spec](orca-workspace-file-connection.spec.md) · [验收记录](../acceptance/orca-workspace-file-connection.md)

## P0 核对现状

1. 刷新主线并建立独立候选分支。核对 `scoped_files.go`、`state_transport.go`、`state_stream.go`、`state_local.go` 和 `orcatransport/progress.go`。记录两次配对读取的调用顺序、现有错误映射、工作区验证及结构化 Run 的冻结 host 来源。配对读取次数的边界是一次 `OpenWorkspaceFiles`，不改历史记录的上游 host 取证。
2. 确认现有本机 IPC 夹具和加密 WebSocket 夹具可计数解析、拨号、`worktree.show` 和文件 RPC。确认有本机与真实配对远端 Codex、隔离仓库、可用的固定构建入口及 SteerLine 生产升级排空入口。

## P1 最小实现及受控回归

1. 仅改文件通道连接选择。本机沿原打开方式，远端对一次解析所得的配对值核对原公钥摘要并直接使用该值拨号。检查配对解析、公钥校验、连接错误与后续 worktree 错配仍由各自阶段分类。连接后沿用原文件代码。
2. 用现有加密夹具建立原端点 A、替代端点 B，两个端点使用不同公钥并通告相同 runtime。固定测试时序，使隔离配对在第一次解析取得 A 后、拨号前切到 B。核对拨号参数来自 A，且每次打开只解析一次。A 可用时原工作区读写通过，A 不可用时按原连接错误拒绝。再验证调用前已换成 B 时零拨号且返回 `binding_invalid`。按端点统计拨号、工作区查询和文件操作，B 的计数保持为零。复用已有夹具，若需要注入时序，只加私有且针对该测试的接缝。
3. 在 Windows 运行 `go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -count=1`，复用 `TestOrcaScopedFileCapability`、`TestScopedFileWireBytesAndBounds`、`TestResultFileRPCUsesOriginalSelectorAndIndependentPath`、`TestResultFileOriginalIPCBoundaries`。按影响复验应用层进度与结果文件，核对运行时读取、停止前不封存、原停止后一次封存和重复读取稳定性。确认生产 diff 仅涉及文件连接，其他通道和停止代码未改。

## P2 同一候选双端真机

1. 从同一干净源码用 `scripts/build.ps1` 构建固定写入 EXE，另用不同 `-Version` 标识构建冷读 EXE。记录源码摘要及两个 EXE 的 SHA256。用同一写入 EXE 在本机和配对远端串行各运行一次默认 Codex `development/2`，分别使用新 state-dir 与隔离工作区。核对现有准入，`execute/1` 不接受 `progress_file`。复用 #110 的 Runner CLI、原目录排空和已配对环境。 [双端验收参考](https://github.com/erranliu/SteerRunner/blob/e7876c0dcbfbbc87a92543a66fdf14fb1de84834/docs/acceptance/orca-workspace-creation-receipt.md)
2. 每端为同一 Run 配置进度和结果文件，使用现有绑定及精简测试提示词。让原执行回合写入两份不同于初始化内容的固定字节，并保留可观察的活动窗口。以原实例的 `progress-file` 读取预期新字节，同时用 `inspect` 和 `result-file` 证明原回合仍活动、结果仍待封存。取得可信未完成结果并进入等待输入后显式停止，取得原会话关闭证明，再核对 `steerrunner-result-file/v1` 的关闭状态、实际字节、版本和摘要，重复读取确认快照稳定。测试输入预算沿现行门禁。
3. 原写入 EXE 正常停服并确认 `not_running`。对本机和远端两份原目录分别调用 SteerLine 生产 `VerifyUpgradeDrained`，使用写入 EXE 与冷读 EXE。记录两 EXE 一致的原 Run/input 和停止证据范围 `orca_agent_session`，并核对无未决控制及冷读前后的数据库和停机证据摘要。
4. 将受控替换、双端进度及结果回执和双 EXE 排空写入独立 `docs/acceptance/` 记录。缺证据的场景记录为阻断，保留原发布门禁。 [测试指南](../testing.md)
