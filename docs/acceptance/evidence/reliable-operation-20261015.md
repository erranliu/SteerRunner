# SteerRunner operation 首轮提取证据

日期：2026-10-15。范围见 [Spec](../../spec/reliable-modularization.spec.md) 和 [Plan](../../spec/reliable-modularization.plan.md)。状态：首轮代码及单元/兼容/最终受控回归完成。本次不运行真实模型/真实 PR，不声明五个模块全部完成。

本文保留首次 operation 阶段的构建和结果；下述“当前 EXE/代码清单”仅指该阶段。后续 RM-P2 整理及新构建完整复验见[收尾证据](reliable-modularization-20261015.md)，不能把本文旧 EXE 的通过沿用为后续构建通过。

## 1. 来源与变更

- 基线源码：`af545563c58171c584bd35d098166cf39012e699`；基线受控构建前只有文档在途修改。
- 实施分支：`codex/reliable-operation`，尚未提交；最终实现是该基线加当前工作树变更。
- 代码清单 SHA256：`30bd53057e39d1664ff6ea8cdfbc1fc1435d43ef9dcb6d10667637790ef359fa`。24 个变更/新增/删除的 Go/PowerShell 路径及各自 SHA 记录在本地 `.evidence/foundation-20261015-090102/source-changes.json`；不包含后续证据文档修订。
- Go：`go1.26.4 windows/amd64`；Windows：`10.0.26200.0`。
- 基线 EXE SHA256：`9A215F39112E8BFC5807ED2C923DF7481DAAA2A339E1FA6F6EF67ECB651A815E`。
- 当前 EXE SHA256：`5FCE931B0BA995E5C86274B65393A1F53336B7FFF68219F3DE1C2BD9FC174A7C`；兼容测试与最终受控组的 source-identity.json 相同。

提取 operation/input/receipt/uncertainty 规则；Intent 的 Go 类型从 runstate 移入 operation，原 JSON 编码不变。构造与正常结算/停止结算分别通过 New/Settle/SettleStop，宿主保留提交和资源生命周期。原操作测试迁入新包，Turn 测试留在 reliable。共享 Digest 留在 reliable，operation 单向依赖 reliable 复用发送前资格校验；没有新增准备框架。

未修改 CLI/公共 DTO、Store/SQL、资源/schema、adapter、原生协议/权限、gate/锁或服务退出代码。Run stop 中仅替换操作结果结算；其后端停止选择与特殊回执表示均保留。

## 2. 单元和依赖验证

基线与当前均运行：

```powershell
go test ./internal/execution/reliable/... ./internal/execution/workflow ./internal/application ./internal/storage -count=1 -timeout=3m
```

当前全部通过；详细输出保存在本地 `.evidence/foundation-20261015-090102/unit-operation.log`。新增 TestSROperationIntentCommitFailureDoesNotDispatch、TestSROperationReceiptCommitFailureReconcilesOriginal 使用测试内 SQLite trigger 触发真实事务回滚，无生产 fault 开关或新持久化接口。前者零原生调用；后者 execute=1、reconcile=1，原 ID/输入/摘要/绑定保持。

TestStopSettlementPreservesExistingSnapshotRepresentation 保护 stop 的 journal/快照幂等键差异、原回执身份和原时间/输入。现有依赖守卫递归覆盖新包，拒绝可靠层反向导入宿主/存储/业务/adapter，且 Workflow 不导入操作子包。实际 Go 包依赖无环。

适配器按实际风险定向运行，未改动实现：

```powershell
go test ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport -run '^(Test.*(Lifecycle|Recovery|Preparation|Policy|Workspace|Binding|Outcome|Receipt).*)$' -count=1 -timeout=3m
go vet ./internal/execution/reliable/... ./internal/application ./internal/runstate
```

适配器定向测试及 vet 通过。原生 live 门控场景不计为真机验收。

## 3. 真实 EXE 受控证据

本地证据目录位于仓库 `.evidence/`，未纳入源码；保留的 result/source-identity/integration/caller-commands 及兼容数据库属于本次隔离现场。进程均由测试拥有并精确清理，不使用用户运行目录。

| 证据目录 | 内容 | 结果 |
| --- | --- | --- |
| `foundation-20261015-085321` | 基线 EXE 的权限、workflow/反馈、日志事件、PR 归属、发布事实、未知操作停止、重启恢复七场景 | exit_code=0，scenario_count=7，无失败/skip |
| `foundation-20261015-090102` | 当前 EXE 的 TestSROperationV2Compatibility；使用上行真实基线 EXE | exit_code=0，scenario_count=1，pr_ready/stopped/unknown 三子项全部通过 |
| `foundation-20261015-090314` | 当前同一 EXE SHA 的最终七场景 | exit_code=0，scenario_count=7，七项全部通过，无失败/skip |

兼容场景经公开 CLI 让旧 EXE 创建正常完成、已停止、未知结果三类 Run；旧服务退出后保存基线数据库，在同一隔离 state-dir 切换新 EXE，两次启动/写回/停服。验证材料/摘要、原会话/Attempt/代次/工作区、操作、final/PR、原请求幂等回执和事件首条/游标。turn/start、thread/start 数不增加。

对保存的数据库只读复核：格式均为 `steerrunner-state/v2`；三类样本的整个 Intents 与 Snapshot 在新构建写回后均相同，已停止样本旧 stop 幂等键差异也保留。摘要记录在本地 `foundation-20261015-090102/v2-compatibility-summary.json`，三对数据库为 `*-baseline-v2.db` / `*-current-v2.db`。不推断所有历史数据均已覆盖。

## 4. 验收结论与边界

RM-P0、RM-P1、RM-P3 完成；RM-A01–A08 针对首轮范围通过。RM-A01 由依赖守卫和调用点审计覆盖；RM-A02 由迁移后的原操作测试及新增事务失败反例覆盖；RM-A03/04/06 复用现有身份/结果/资格规则测试，相关规则未重写；RM-A05/07 由最终停止/重启/正常闭环及反馈/PR 场景覆盖；RM-A08 由三类真实基线 v2 样本及公开事实核对覆盖。30 个顶层单元测试通过，未出现失败；实际代码清单与最终测试时一致。

首次 operation 阶段未执行 RM-P2。后续已补做剩余职责的逐项核对和最小整理，当前结论以收尾证据为准，本文不再代表整个重构的结束点。

RM-A09 未触发：未改变原生协议/参数/权限/生命周期；没有运行真实 Codex、真实 PR 或 Orca 真机。不合并、不发布、不更换用户运行中的 EXE。源码与证据均保留在本工作树供审阅。
