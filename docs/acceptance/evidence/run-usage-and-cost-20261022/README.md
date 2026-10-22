# Run 用量与费用：2026-10-22 脱敏验收汇总

[final-summary.json](final-summary.json) 保留 PR 提交前 native-usage/3 的四个真实 Run 资格；[pr47-review.json](pr47-review.json) 记录合并前复审修复后的 native-usage/4 资格，包含新的源码/EXE 摘要、最终整包/集成回归、四组合复验及撤权事实。完整过程、初次失败、前后批次和未交付边界见[专项验收](../../run-usage-and-cost.md)。

不含日志正文、提示词、API 凭据、机器/SID 原值、原状态库、绝对用户路径或测试二进制。原始私有现场在本机保留；JSON 只保存已核验的计量及身份摘要。费用是明确测试价表的 API 等价 token 算术，非供应商现价或实际账单。

源码摘要按 git 跟踪的 `*.go`、`go.mod`、`go.sum` 与 `internal/resources/assets/*` 文件生成相对路径→文件 SHA-256 映射，再对按键排序、紧凑 JSON 的 UTF-8 字节取 SHA-256；不包含 Git 提交/构建元数据。提交前 final-summary 的完整 internal/集成批次在该轮最后两个局部校验修复前编译，随后另重跑 13 包和四组合现场；合并前 pr47-review 则在本轮两项修复后重新执行完整 internal、四项集成与四组合公开 EXE，不混用批次。
