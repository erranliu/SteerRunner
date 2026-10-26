# Orca 完整元数据下的原输入补证

当 working 通知未被接收、原生已 done 且 prompt 预览为空时，现有独立原输入查询只对 thin 元数据完成记录启用。若 session.tabs 已提供完整且与 worktree.ps 同根的主记录及 hook 元数据，反而停在缺少原输入归属证明，不能推进发布。

只扩展现有独立查询的准入，不添加状态、协议字段、codec 或重放路径：

- 保留原 thin 资格。新资格要求两个公开投影的同一 Codex pane/provider/主 done 记录、完整可信 hook 元数据、正的 turn marker；部分或冲突元数据不能获得新资格。
- 以原持久化 baseline 查询其后唯一、未截断的原生 user 输入，全文匹配已接受的原输入。初始化 READY 不得被当作业务输入，公开创建时间不能冒充持久化 baseline。
- 独立查询前后围栏核实相同 runtime、terminal、incarnation、worktree、真实路径和 branch，provider/transcript locator 不变。完成正文仍来自原主 agent，transcript 只补输入归属。
- 查询前后主记录、正文、marker、hook authority/incarnation/revision/观察时间完全一致；比原 baseline 陈旧的 revision 拒绝，明确 authority 冲突保留。已有 outcome、截断、重复输入、会话边界等守卫不变。
- 查询资格仅存在进程内，不能从 wire 或序列化记录制造，不能借给其它完成记录；不伪造 working 观察，不发送第二次输入、reply 或 resume。
- 原生发送前把 CRLF 编译为 LF 并移除 TUI 本来会移除的结尾换行，与现有原输入匹配契约使用同一函数；保留用户原本的空行及冻结逻辑输入/digest。不得在事后折叠 transcript 空白来放宽全文匹配，已被 paste 改写的旧输入不能补记成功。

已运行的 f3 现场不修改 home/pin 或私有状态。新增 EXE 只作为未来新轮次的可审查候选，真实资格及使用授权单独记录。

见 [Plan](orca-full-metadata-completion.plan.md) / [验收](../acceptance/orca-full-metadata-completion.md)。
