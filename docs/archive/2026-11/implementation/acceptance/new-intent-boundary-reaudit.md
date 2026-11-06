# 新意图写入边界与展示清尾：实现再审计

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。结论：对照[Spec](../spec/new-intent-boundary-cleanup.spec.md)/[Plan](../spec/new-intent-boundary-cleanup.plan.md)重新审计，未发现需要修复的产品缺陷。限制仍只在新意图构造侧，历史严格解码、原操作只读对账、共享夹具、持久格式及停止承诺保持。按新增用户授权提交并提PR，不自动合并或部署。初轮实施资格及两次新增夹具失败保留在[原验收](new-intent-boundary-cleanup.md)。

## 1. 最新主线与实际入口

审计开始时main由98309f8推进到 `66cc0acf50b675c75458d9d6281139e16158a3a3`。以ff-only快进，原任务修改和新增回归保留。上游变化为10个测试/文档文件，其中8项属于源身份清单；没有internal/cmd或Runner构建脚本变化。不能把这8项上游差异当成“全部源码仍同初轮”，故按新基线冻结身份，重新编译live夹具、构建Runner和运行三个公开场景。[基线审计](../../../../acceptance/evidence/new-intent-boundary-reaudit-20261102/audit.json)

重新扫描实际调用及候选生产者：InputValue的唯一Go调用仍在operation.New，生产入口是saveIntent；四类无生产候选动作只在此拒绝。New在生成/持久保存意图前返回错误，saveIntent/SQLite回归确认Run和events原字节/数量不变。原Validate/Clone/枚举/decoder仍接受合法保存形状，未因新写入拒绝而删字段或增加旧记录读取门禁。[消费者](../../../../acceptance/evidence/new-intent-boundary-reaudit-20261102/consumers.json)

CandidateForOperation仍由planner及owner.validateOwnerRequest读取原payload，后者继续核验原摘要和Input；它们均不调用InputValue。worker未知原操作先选择原Intent进入Invoke/Reconcile，不重建New；原保存/回执/SettleStop、预算、原结果和stop控制优先级保持。submit_input及两类stop新构造保留。空PreparationFacts.Cleanup只移除无生产赋值的瞬时步骤，不改变LifecycleInput.Cleanup或真实资源清理。

## 2. 共享夹具、持久格式与观察/停止

按最新main重新比较全部5个生产变更文件：与授权精确变换一致；90个保留声明（37个类型）相同，仅原5项声明修改及Display删除，无新增生产声明。Observe整个文件只删除Summary/Title及赋值，原replay、事件选择、checkpoint、journal.Write/Sync、拒绝/转发、确认等待、取消和完成逻辑相同。[全文变换](../../../../acceptance/evidence/new-intent-boundary-reaudit-20261102/transforms.json)/[AST](../../../../acceptance/evidence/new-intent-boundary-reaudit-20261102/declarations.json)

21个关键原消费者文件及共享夹具逐字相同：含worker/manual/control/verify-stopped、存储、Invoke/Settle、owner、实际Codex/Orca Lifecycle及精确本机停止，MockAppServer、StagedBusiness、historyFixtureBinding、异步问答Kind/Status原断言和正式native创建helper。当前任务没有修改已有测试或共享夹具；新增new_intent_boundary_test也与初轮最终资格相同。上游两份integration测试变化已由快进采用，不由本任务改写。

当前重新捕获的持久根类型、代表Run/Snapshot/Request/Receipt/HistoryBinding、四类原保存输入及公开能力/资源摘要，与初轮实施前后均为相同145263字节，SHA256 `a5ec2f5e94487c7b4cea1711a50b23a423bc41aaf0c2d5444e0b9ec5284c7c16`。[编码](../../../../acceptance/evidence/new-intent-boundary-reaudit-20261102/wire-comparison.json)。state/v8、SQLite user_version=8、动作/字段/严格decoder、双操作表示和owner/native协议不变；PreparationFacts和ObservationEvent仍只作瞬时授权变化。停止撤销、原操作未知不重发、原范围有证据才stopped均无新门禁或放宽。

## 3. 本轮验证、失败及资格

| 项目 | 结果 |
| --- | --- |
| application最终定向 | exit0，17顶层/25子用例通过，0skip；包含四类新写入拒绝/原保存对账、8类保留构造及原操作/预算/停止/代次授权 |
| 原消费者定向 | exit0，10顶层/18子用例通过，0skip；历史恢复、原请求/回执、问答Kind/Status、appserver/staged及单确认 |
| 当前主线正式消费者 | native/integration各live编译、正式helper及最终Runner构建通过 |
| 当前主线公开场景 | 同最终EXE的NativeUnknownAndStop、RestartPreservesAuthorization、ServiceOwnership，3/3、0skip，含隔离资源清理 |
| 证据复核 | 初轮完整SHA清单有效；初轮498个组件/application及三个公开场景保留原资格，不重复计入本轮27顶层/43子用例 |

本轮application首轮16顶层通过、1失败：既有TestFrozenBudgetMissingDoesNotGateOriginalResults的3个execute子用例在resultFixture初始化时read_failed，尚未进入其缺预算断言。该测试、fixture、runtime_files/result_file及workspace读写正文均未改。隔离复验该顶层及9子用例通过，随后同一完整定向选集17顶层再次全部通过；没有修改生产、夹具、超时或断言。原因未确证，不把首轮失败改判或无依据归因为并发，保留[核对说明](../../../../acceptance/evidence/new-intent-boundary-reaudit-20261102/file-fixture-review.json)及原始日志。[统计](../../../../acceptance/evidence/new-intent-boundary-reaudit-20261102/test-summary.json)/[精确命令](../../../../acceptance/evidence/new-intent-boundary-reaudit-20261102/commands.json)

最终Runner SHA256 `c0387affcfee319a2efd12a7ac2410e05312f3a6dda9299f2f9b3d128196355d`，构建base为66cc0ac；Runner源码摘要仍为 `b800a590e06af4faf25e92e8e4bd81c283489e4e06438ab84d76b0b9e3b16a87`。新主线完整源与live/helper/EXE身份见[源身份](../../../../acceptance/evidence/new-intent-boundary-reaudit-20261102/source-identity.json.gz)/[构建](../../../../acceptance/evidence/new-intent-boundary-reaudit-20261102/runner-build.json)。三个新公开场景使用此EXE，不以初轮EXE代替最新测试入口验收。

按当前主线导入这三个场景的计时，status=passed、qualification_eligible=false；不授予完整Controlled或L1/L2/L3、真实模型或新后端组合资格。初轮编码/测试/失败证据不回写。新证据目录按仓库惯例补Git原字节保护；暂存时逐字核验两个目录的blob及SHA清单，不把换行归一化后的字节当作原证据。
