# 取消调用方正文预算验收

日期：2026-10-28。基线：`6d5db39f761f886b29f49f8a2f6d62d4580a5cdc`，独立分支 `codex/remove-caller-input-limit`。

本次取消 Runner 对 SteerLine 等调用方的 2000B 正文额度，同时取消 3000B 总正文额度，避免总量间接限制调用方。仅保留 Runner 生成文字 ≤1000 UTF-8 字节；调用方、Runner、总量及全部片段来源仍计量、冻结并核对。公开能力 caller/total 为 null，明确表示无正文额度，Runner 为 1000。

直接正文、prompt_overrides、inline 材料/schema、反馈及回答沿共同规则受理，不截断、摘要或拆分。未修改完成确认、合法零值处理、草稿匹配、权限、未知结果对账或补发规则。现有 API、材料配置、WebSocket 封装、Orca CLI/Windows 命令行等独立容量保护保持；无正文额度不代表传输无限大。

受控验证命令：

```powershell
$env:GOTMPDIR='C:\sr-bgt'
go test ./internal/execution/model ./internal/resources ./internal/application ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport ./internal/execution/adapters/protocol/codexapp ./internal/execution/adapters/protocol/orca -count=1 -timeout=5m
```

验证点：

- C=1999/2000/2001、T=2999/3000/3001 及 10552B 调用方文本通过；1MiB 调用方搭配 Runner 1000B 在公共计量层通过，未声明该大小的原生传输资格。
- Runner=999/1000 通过，1001 在短/长调用方两种情况下均拒绝；保持总量等式、Unicode/换行归属、冻结后追加/变更来源反例。
- 工作流长覆盖加反馈、app-server 最终参数、长中文/emoji 回答和异步原轮回答、Orca 原生准备入口均覆盖；Runner 超限仍在粘贴/重绘前拒绝。
- 原受控包覆盖完成确认、零值、重绘和传输边界；真实模型 opt-in 未开启，未运行 SteerLine、Unity 或其他业务链，也未恢复旧现场。

第一轮七包执行：model 1.124s、resources 2.038s、codex 58.253s、orcatransport 101.902s、protocol/codexapp 0.523s、protocol/orca 23.629s 均通过。application 全包达到 5 分钟总时限（303.671s 退出失败，当时 TestRAParserAcquisition/failed 运行 3s）；没有断言失败输出，但全包不记通过。随后单独执行 `go test ./internal/application -run '^TestBudget' -count=1 -timeout=4m -v` 验证本次影响，5 项顶层测试全部通过，39.518s，退出 0（日志 `.evidence/remove-caller-limit/application-budget.log`）。原始日志：本工作区 `.evidence/remove-caller-limit/controlled-packages.log`（本地忽略，不提交原始现场）。

历史预算和 PR67 整合报告只证明原冻结版本。本次不把历史未知投递、原发布草稿或超时重新判定为成功，也不声称取消预算已经解决原投递故障。当前规则见 [Spec](../spec/workflow-input-budget.spec.md) 和 [Plan](../spec/workflow-input-budget.plan.md)。
