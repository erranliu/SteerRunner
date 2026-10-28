# PR67 必要修复整合到 PR68

日期：2026-10-28。基于 PR68 的 `2dd3129bf60849bf838f012158c47c962cfedb54`，选择性迁入 PR67 固定 head `49722a8053dfa21d449693c8c4c5c5cfa5c57db5` 的原文不变草稿折行修复。PR67 的原分支、提交和历史验收记录保留；由 PR68 承接整合后的代码与验收，不整包合并旧分支。

## 迁入与保留边界

- 迁入 `native_paste.go` 的一次重绘判定、精确稳定读取和自身视口恢复；`terminal_viewport.go` 使用现有原生 API，绑定原 runtime/terminal/incarnation/worktree 与唯一 viewport owner。只对 Runner 新建并拥有的远端 Codex 终端启用。已有外部显示策略不接管，真实空格/换行/字节变化不等价化。
- 迁入视口拥有权/恢复、833 字符冻结草稿和后续输入反例。后续输入用例缩小到新预算以内；833 字符旧样本为 1083 UTF-8 字节，仅作受控 transport fixture，不把其中业务材料路径或可选写文件说明发给真实模型。新增超限时零重绘/零投递、真实末尾空白丢失拒绝。
- 保留 PR68 的字节预算、完整零值/缺失语义、共享校验及原输入/完成证明；这些相关生产文件相对 `2dd3129` 字节未变。唯一原输入、进程围栏、baseline、稳定 canonical final、未知不重发规则不变。
- 不迁入重复零值实现、`omit_publish_source_facts` 实验开关、通用 `DiagnosticPath`/进程诊断层和长短实验驱动。视口原始回执留在原 paste receipt，真实探针通过已有 passive trace 记录视口 RPC，不增加模型可见文字或新运行时协议文件。

实现规则见[原文不变的草稿核验](../native-composer-reflow.md)。历史 3715B 长输入样本不满足当前 3000B 硬上限，不作为本次资格。

## 受控验证

原协议与 adapter 完整受控包均通过，真实开关关闭：25.269s / 99.533s。迁入的 26 个子场景各重复 10 轮，共 260 次通过。补充末尾空白丢失反例及修正固定探针说明后，最终 27 个定向子场景再次通过；这两处后续源码变化均只在测试文件，完整包验证过的生产内容未变。

反例包括真实/插入换行、改字节、丢失 fit 回执、替换 provider/terminal/incarnation/worktree、外部 viewport、fit/restore 尺寸错误、恢复失败、旧 final、变更输入、未知 paste/submit 后零重发，以及调用方 2001B / Runner 1001B 在重绘或发送前拒绝。可复算的最终 frozen 输入在重绘中不改字节，不借预算。

迁移中的一次普通沙箱 Go 临时目录拒绝及一次移除旧诊断层后遗留的未使用变量/import 编译失败留在本地失败日志；修正后以上验证正常 exit 0，未将失败批次计为通过。

## 最小真实验证及失败保留

同一原配置的 Orca 1.4.242 runtime、`gpt-6-luna/xhigh`、账号摘要和 SDK 环境 launcher，关闭 setup，仅使用新的独立现场。复用 `short-sequence`，首任务仅一次 `Write-Output` 并返回唯一标记，后续发布式短输入只返回第二个唯一标记，不创建 PR、不执行项目业务测试。真实变体不引用旧 Spec/Plan 或结果文件。

| 证据根 | 实况与计数 |
| --- | --- |
| probe01 | 首次 979B 输入粘贴一次成功，未发送 Enter，没有任务接受回执；固定 inert notes 末尾带一空格，screen 丢弃该空格并有显示折行。生产严格比较正确拒绝，未进入 fit；最终一次读取遇整体期限到达及不完整 JSON。原 operation 仍为未知，未重发；保留失败。 |
| probe02 | 新现场的 fallback shell 尚未核实关闭，测试在完整零终端前置检查处退出，只有一次 terminal list；未创建模型、无任务输入。原记录 `cleaned_up=false` 不改写；独立核实空闲 shell 后关闭，以新证据根启动该仍未执行过模型的现场。 |
| probe03 | 仅在固定探针生成时去掉末尾空格，再冻结全文；不是生产剪裁或修改调用方正文。除新会话/唯一 nonce 外，首任务规范化内容相对 probe01 仅差该末尾空格。978B 首任务与 144B 后续输入均完整接受并确认 final，51.28s（包 54.025s）。 |

probe03 **实际触发重绘路径**：trace 19/20 为一次 `mobile-fit` 请求/回执，原 177×83 改为 1024×80；27/28 为同一 owner 的一次 restore，请求完成后恢复为原 177×83。其余原终端检查、精确两次稳定草稿、原生产 Enter 及原输入/完成核验照常执行。C/R/T 分别为 978/0/978 与 144/0/144；初始化沿原 118B Runner 输入。两条各一次 paste 和一次 bare Enter，没有人工补 Enter、未知输入重发或模型业务操作。

419 个执行侧 Go 文件与实际 EXE 摘要已冻结，真实探针之后源码核对无变化。两个新建 worktree 的终端已确认退出、完整 scoped census 为零；核对精确非主路径后回收，后续查询均 `selector_not_found`。未修改或恢复原 Run、原 publication operation 或旧失败现场。

公开验证集见 [pr67-integration](evidence/native-completion-20261028/pr67-integration/)，含[摘要](evidence/native-completion-20261028/pr67-integration/verification-summary.json)、受控通过日志、无副作用 frozen 输入和源码摘要。完整 CLI、observer、账号/session、失败及清理捕获保留本地 `.evidence/pr67-integration` 原路径/原字节，不随 Git 发布；摘要列其原始 SHA。

结论仅为所选折行修复已与 PR68 预算/证明链整合并通过本次受控及预算内真实路径。PR67 旧长输入记录不替代新预算验收，原历史失败不追认为已修复，未重跑 MagicPotion、Unity 或 SteerLine 业务链。
