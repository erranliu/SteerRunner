# Orca 配对公钥主机指纹收敛 Plan

已按下列步骤实施并完成受影响包等价验收及修改审计。 [Spec](orca-pairing-host-id.spec.md) · [验收记录](../acceptance/orca-pairing-host-id.md)

1. 实现前刷新主线，在独立分支对照四处代码。记录解码器、摘要格式、错误类型及各入口检查顺序，特别核对 usage 的空 runtime 检查位置、文件通道的错误映射及一次解析边界。以调用方已取得的 `statePairing` 作为共享方法接收值。
2. 在现有 Orca 协议包增加私有 `statePairing.hostID()`，替换 `NewRemoteAgentSessionClient`、`openWorkspaceFiles`、`openUsageFiles` 和 `UsageHostID` 的重复算法。移除因提取而失去用途的 imports。审查 diff，确认网络、能力声明、配对解析、主机比较和错误处理分支均保留原位置。无需新接口或重命名公开 API。
3. 使用少量独立固定向量验证 Base64 公钥对应的完整 `sha256:` 字符串。覆盖带 CRLF 的可解码输入、非法字符和填充错误。解码错误核对原类型和值，预期指纹不通过新方法生成。公钥长度等解析器既有规则保持，复用原验证用例。
4. 在 Windows 执行 `go test ./internal/execution/adapters/protocol/orca -count=1`。重点核对 `TestRemoteAgentSessionWireAndDispatch`、`TestRemoteAgentSendWireAndDispatch`、`TestStructuredUsageChannelDeclaresAuthAndRequestCapabilities`、`TestWorkspaceFilesUseTheVerifiedPairingSnapshot` 和 `TestOrcaScopedFileCapability`。复用既有 usage 原身份和文件权限用例，只补尚未覆盖的行为差异。确认本机不进入配对解析，远端错 host 沿各自错误分类拒绝，声明能力及写入次数与基线一致。
5. 用静态 diff 和现有配对替换夹具检查 #113 的单次解析、同配置拨号和替代端点零连接。若发现必须改连接时序、增加共享状态或调整调用方错误才能完成提取，先修正提取边界并重新核对等价性。
6. 在独立 `docs/acceptance/` 记录基线、实际 diff、命令、返回值及错误等价结果。本次属于纯算法提取，验收以受影响协议包和现有受控夹具为主。出现行为差异时按具体影响补验。完整发布资格沿 [测试指南](../testing.md) 的既有门禁，历史真机证据仅作参考。

2026-11-05：基线仍为 `f6a389cf`。生产修改仅新增私有方法、替换四处算法及清理失效 imports。新增固定向量及普通/structured usage 原身份拒绝用例；受影响包通过，审计后收紧原解码错误直接比较并定向复验通过。未执行完整 L1/Controlled、L2/L3 或新增真机验收，不据此声明发布资格。
