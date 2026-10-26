# Orca 完整元数据原输入补证 Plan

关联 [Spec](orca-full-metadata-completion.spec.md)。

1. 复用现有 completion input query 私有证明，对完整、稳定、同根元数据启用同一 terminal 围栏及唯一原输入读取；保留 thin 路径及部分元数据拒绝。
2. 原生 lifecycle 在缺少 working 时可进入该严格查询，不建立新状态或伪造 working；authority/revision 和旧冲突守卫继续约束最终结果。
3. 用真实 IPC fixture 复现原 f3 拒绝同形状；补完整元数据正例、观察器重建、重复输入及元数据/资源读取竞争反例。
4. 定向 owner、正式 L1、构建字节身份分别存档。候选二进制及变更可审查后，按原固定 Runner 验收约束取得未来新轮 pin 授权；不得热替换原 Run。
5. 真实候选又暴露 Windows CRLF paste 多出空行：按公开冻结提示、材料引用及源码固定后缀逐字节重建，验证原 native 输入正好为 CR→LF 的 paste 结果。发送前复用已有 LF 形式编译，保留原逻辑输入/digest；用真实原发送捕获、ConPTY 行变换和 IPC 完整 lifecycle 回归，覆盖 observed/missed/restored，以及正文、额外空行和重复输入拒绝。新 EXE 另登记，不替换此前获准的旧候选。

| 验证 | 入口与通过条件 |
| --- | --- |
| 旧 f3 复现 | 独立普通 clone、仅加入同一测试文件；missed/missed-restored 均因原输入 preview 缺失而无最终结果，零重发 |
| 协议 | `TestCorroboratedCompletionInputRequiresStableOriginalResources`；完整投影通过，资源/pane/provider/正文/marker/元数据变化及重复/截断输入拒绝，序列化及原地修改不能制造证明 |
| lifecycle | `TestCorroboratedOriginalQueryCompletesThroughNativeLifecycle`；missed/restored 输出原 canonical 正文及原 outcome，working 仍为 0，单次输入；冲突/陈旧/重复均拒绝 |
| 回归 | 两个 Orca owner 全包及正式 `scripts/test.ps1 -Level L1`，不把 skip 或零匹配算通过 |
| 真实验证 | 仅获准后使用新普通 clone/home/固定新 EXE，原业务合同和测试要求不变；原报告、进度、发布和清理全部核实后累计，两次连续通过才完成 |

命令、失败和边界见[验收](../acceptance/orca-full-metadata-completion.md)。
