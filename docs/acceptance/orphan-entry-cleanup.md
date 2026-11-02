# 零消费者生产入口与孤立夹具清尾：专项验收

2026-11-02 后续[实现再审计](orphan-entry-reaudit.md)完成：无产品缺陷，修复证据目录Git换行保护遗漏；消费者/共享夹具与持久格式复核、25个定向顶层通过，按新增授权提交提PR。下文保留初轮实施验收及当时交付状态。

日期：2026-11-02。结论：按 [Spec](../spec/orphan-entry-cleanup.spec.md) / [Plan](../spec/orphan-entry-cleanup.plan.md) 先设计审计，再实施 A/B，OEC-00–04完成。基线为含 #84 的 main `111bcd35bdfd7969d4c4575b5fe62204b41486fb` 加本批未提交差异；源码/产物以[逐文件身份](evidence/orphan-entry-cleanup-20261102/source-identity.json.gz)为准。未提交、推送、提PR、部署或发布；本记录只授予本批专项资格。

## 1. 删除范围与收益

| 增量 | 新增 / 删除 / 净减少 | 实施结果 |
| --- | --- | --- |
| A 生产入口 | 0 / 201 / 201行 | 删除17个零消费者函数、独占commitHash/ErrRepoUnavailable及导入；包括旧操作校验、未接入Workflow事实规则、Codex辅助函数、Orca旧查询/展示包装和两个未用路径/核验包装 |
| B 测试支撑 | 0 / 333 / 333行 | 删除孤立terminal.go（292行）和两个无调用平台锁文件（41行）；不计作生产收益 |

共13个Go文件发生纯删除，没有新增生产声明、适配层或恢复框架；未修改任何实际测试文件或断言。不包含建议C、持久动作退役、双操作表示合并或底层事件标签清理。[逐项统计](evidence/orphan-entry-cleanup-20261102/production-diff.json)。收益为误导入口、死规则及夹具维护面减少，未测同条件耗时，不承诺执行提速。

## 2. 消费者、合法入口和共享夹具

[设计审计](evidence/orphan-entry-cleanup-20261102/design-audit.json)完成时生产/测试差异为空；保存Spec/Plan摘要、774个源码/配置/脚本文件内33个候选符号的[原始引用](evidence/orphan-entry-cleanup-20261102/consumers-before.json)及790项源文件基线。A引用仅在可删除闭包内，B无文件外消费者。候选为内部Go函数，没有当前CLI、Lifecycle或helper合法入口进入这些函数，不以仅默认构建成功判断无消费者。

保留的真实路径：

- StableExecutionID、LifecycleRequest.Validate、Receipt.ValidateFor、当前严格decoder，实际命令/问答/回执和原资源绑定不变。另一个publication包的commitHash有provider消费者，按包作用域区分并保持原字节。
- application.acceptEvent、Workflow实际转换、单Persisted确认、owner原生journal、未知操作只读对账、有界恢复、预算、停止撤销和精确停止范围不变。
- Orca真实repository解析、DecodeProfile/Profile.Target、原工作区创建/身份核验、InspectLifecycleAuditWorkspace、validateBoundTerminal与close回执核验不变。目录包装删除后，verifyFileDirectories仍由真实progress/result读写调用。
- MockAppServerStdio/Main仍由mock_helper_cd67_test调用；MockAppServerConnection/AppServerState仍由reattach_receipt_cd67_test调用。appserver.go及这些测试完全未改。
- StagedBusiness仍由tests/helpers/native/main.go及staged_test调用；staged.go/staged_test.go、正式--orca-create-fixture/orca_creation.go、historyFixtureBinding及其恢复测试、native/integration正式探针完全未改。

[源文件比较](evidence/orphan-entry-cleanup-20261102/source-comparison.json)确认只有13个授权文件变化，其余777项已跟踪源/配置字节一致，包含全部原测试、共享夹具、脚本、客户端和依赖文件。没有通过删除有效消费者或断言取得通过。

## 3. 持久格式和保留实现

state/v8、SQLite user_version=8、CLI/Workflow/profile/owner/native协议、全部持久字段/动作/decoder保持；Intents和Snapshot.Operations及各自更新语义保持。被删类型只有无消费者测试夹具TerminalFixtureState，不属于产品保存根。

前后选定持久根类型、代表性Run/Snapshot/Request/Receipt/Operation/HistoryBinding和公开能力/资源摘要共**145116字节完全相同**，SHA256 `47d983e8958718ee57ba1a13f00e3504c51ff6268e964062f2ad6ef35c471367`。[编码比较](evidence/orphan-entry-cleanup-20261102/wire-comparison.json)。同源编码工具及前后原字节随证据保存，不只比较零值或JSON标签。

[AST比较](evidence/orphan-entry-cleanup-20261102/declarations.json)包含全部13个变更Go文件：100个保留声明（含12个类型）完全相同，无新增或修改保留声明；删除33个不同包符号，平台锁的两个定义是同一个包符号。其他源字节核对、纯删除差异和实际回归共同支持PC-R01–R06/R08/R09保持，未增加入口或耦合。

## 4. 最小验收与构建身份

| 项目 | 结果 |
| --- | --- |
| OEC-00 | 设计先行；候选闭包、真实入口、同名不同包变量和共享夹具复核通过 |
| OEC-01 | 前后编码相同；保留AST与其余源文件/测试逐字相同；无持久模型修改 |
| OEC-02组件 | execution/...、runstate/contracts/resources/storage/testkit/protocol：exit0，447顶层、1343子用例通过；8个顶层skip及3个无测试包单列 |
| OEC-02 application | 原操作/保存失败/原资源、单确认、预算、停止竞争/进度、人工恢复、普通重启授权等定向：exit0，49顶层、86子用例通过，0skip |
| OEC-03正式消费者 | native/integration各自live编译通过；正式native helper和scripts/build.ps1最终Runner构建通过 |
| OEC-03公开链路 | 同一最终EXE的NativeUnknownAndStop、RestartPreservesAuthorization、ServiceOwnership三场景3/3、0skip，包含隔离夹具及资源清理 |
| OEC-04 | A/B分项统计、精确命令/退出码、源/产物身份、失败与skip、长期说明及索引完成 |

最终组件加application共**496个顶层通过**，不把重复执行或skip计入。[精确命令和权限环境](evidence/orphan-entry-cleanup-20261102/commands.json)、[测试统计/名称/每包结果](evidence/orphan-entry-cleanup-20261102/test-summary.json)、[公开场景结果](evidence/orphan-entry-cleanup-20261102/foundation/result.json)。

最终Runner SHA256 `2606bfdafa88228761ee363d942f2850344b93945593fdaafbe32e6eba1555f1`；source_digest `ea6c2908ede069baee09f0a14b4738e12e03a728dfc0ed1eb4da0b4b80508257`。[构建记录](evidence/orphan-entry-cleanup-20261102/runner-build.json)和公开场景使用的产物SHA匹配；构建后重新计算源摘要相同。源码冻结后未再编辑生产或测试。

## 5. 首轮失败、skip和资格限制

首轮使用workspace-write沙箱：组件记录399顶层通过、46失败、8skip，另有超时未完成项；application为45通过、4失败。原始JSONL及统计保留。错误包含临时Git仓库.git/config写入Permission denied、loopback socket被拒绝、Node原生rename EPERM及后续超时；不能把这些首轮运行记为通过。在具备所需测试权限的进程中按同一命令复验，最终全部通过；两轮之间没有产品或测试源码修改。正式公开脚本按原CIM独立harness运行，启动成功与最终result.json分别记录。

8个组件skip为既有helper/opt-in项（完整名称见统计）；evidence、nativeaudit、runstate三个包无测试。没有新增真实模型/helper或完整L1/L2/L3资格，不把受控场景推广到全部后端组合。

审计工具初轮另有两项纠正：原始名称扫描把publication.commitHash误当作backend同变量；声明计数预期误写34，实际33个不同包符号（平台锁有两个定义）。均修正审计作用域/期望，不修改产品实现或原断言，详见[工具说明](evidence/orphan-entry-cleanup-20261102/audit-tool-notes.txt)。历史验收原字节未改，未操作用户已有Run或Orca工作区。
