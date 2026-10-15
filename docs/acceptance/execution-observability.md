# 执行可见性验收

范围及命令见 [Plan](../spec/execution-observability.plan.md)。2026-10-15，Windows / Go 1.26.4；独立临时目录和受控原生协议，无真实模型、GitHub PR 或安装环境变更。

| 项 | 验收要求 | 结果 |
| --- | --- | --- |
| ROV-A01 | 真实流程/状态/诊断变化原子入库；错误回滚，历史不可重画 | 通过；最终定向测试及提交前复验通过 |
| ROV-A02 | 仅公开摘要，身份与乱序守卫、脱敏有界；查询时间不冒充活动时间，零业务状态写入；观察问答/终态继续 | 通过；最终定向测试及提交前复验通过 |
| ROV-A03 | 真实 EXE/服务 CLI 可读摘要与变化；零额外 native input；原反馈流程保持 | 通过；TestSRRunWorkflow 实际 EXE/常驻服务受控验证 |

## 最终自动结果

2026-10-15 完成 Plan 定向命令：storage/application/Codex adapter/CLI 合计 19 个测试及子用例 pass，0 fail/skip；host 包完成编译。随后自查补充引号凭据脱敏和测试错误核验，再复验 LastMessage/WorkflowTransitions 两项通过。Go 输出位于 `.evidence/observability/go-final.jsonl` 与 `final-review.jsonl`。

真实 EXE/服务受控 `TestSRRunWorkflow` 通过，场景耗时 20.04 秒（含构建总计 29.428 秒），结果 scenario_count=1、exit_code=0。覆盖公开 CLI 两次读取同一 observed_at、Run/Attempt/Session 正确、读取后 revision 与 native turn/start 次数不变、CLI workflow_changed 真实历史，以及后续反馈仍使用原 session/PR。证据为 `.evidence/observability/integration/integration.log`、`result.json`、`caller-commands.jsonl`；均为隔离夹具，不含用户认证或真实任务数据。

`go build -trimpath -o .evidence/observability/steerrunner.exe ./cmd/steerrunner` 和 `git diff --check` 通过。原始产物忽略入库，本文保留复现命令与最小摘要。


限制：摘要是进程内最佳努力缓存，服务重启后为空，只在收到新通知后更新；不补抓所有历史，不包含 reasoning/工具全文。Orca 未实现该摘要且本轮不验收。历史 event 缺少 transition 时保持原始事实，不能用当前投影补画。
