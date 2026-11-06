# 新意图写入边界与展示残留清尾：专项验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-02 后续[实现再审计](new-intent-boundary-reaudit.md)完成：未发现产品缺陷，历史/原对账/共享夹具/格式/停止保持；快进最新main66cc0ac后27个定向及三个最新公开场景通过，原夹具初始化首轮失败单列。按新增授权提交提PR；下文保留初轮98309f8的证据及交付状态。

日期：2026-11-02。结论：按[Spec](../spec/new-intent-boundary-cleanup.spec.md)/[Plan](../spec/new-intent-boundary-cleanup.plan.md)设计审计后实施A/B，NIB-00–05完成。基线main `98309f835012c5ea43cad2245a11341a0d9fc040`加本批差异，源与产物见[逐文件身份](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/source-identity.json.gz)。尚未提交、推送、提PR、合并、部署或发布。

## 1. 范围与收益

| 增量 | 生产新增/删除/净减少 | 结果 |
| --- | --- | --- |
| A新写入侧 | 2/13/11行 | InputValue移除四类无生产候选的新构造case；删除PreparationFacts.Cleanup瞬时字段和Next空分支；两行注释说明新意图与原请求解码责任 |
| B展示残留 | 9/24/15行 | 删除无调用Config.Display、ObservationEvent.Summary/Title及对应赋值；Kind/Status与真实原生观察行为保留 |

生产合计净减26行，收益为新写入范围明确及少量维护面减少；新回归不计生产收益，没有同条件性能测量，不宣称任务提速或显著缩短验证链。[分项统计](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/production-diff.json)。

## 2. 新写入与原保存路径

[设计审计](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/design-audit.json)完成时生产/测试差异为空，保存787项源/配置、文档摘要和[原引用](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/consumers-before.json)。InputValue唯一Go调用来自operation.New，生产经saveIntent进入。四类动作没有当前生产候选；Cleanup唯一候选生成点的PreparationFacts字段也无赋值。

只有新意图构造拒绝report_completion、acknowledge_delivery、cleanup_resources、host_action。LifecycleInput.Validate/Clone、Candidate/Request/Receipt校验、CandidateForOperation、严格持久decoder和全部旧动作字符串/字段保持。submit_input的构造及Orca执行/对账仍保留，stop_observation/stop_execution构造和原控制路径保持。

新增TestSRNewIntentBoundaryRejectsNewButReconcilesSaved覆盖四类合法旧输入：

- 经真实saveIntent拒绝后，内存Run、SQLite runs.body原字节、events数量、Revision和双操作列表不变，原生execute/reconcile/stop计数为零。
- 手工按原字节形状编码输入、冻结原操作身份/摘要/时间并保存；没有调用已收窄的New重建旧操作。CandidateForOperation、原Request/Receipt校验通过，尾随JSON仍拒绝。
- 经SQLite重载后由正常advance入口只调用原Reconcile一次，零execute/stop；原操作ID、输入及结果回执归属保持。fake原操作端口只证明恢复入口没有被新写入门禁挡住，不授予真实适配器未支持旧动作的新执行能力。

TestSRNewIntentBoundaryKeepsSupportedPayloads覆盖八类保留动作，含submit_input和两类stop；实际saveIntent落盘后的原输入字节、摘要和双操作关联保持。两项新回归共12子用例通过。

## 3. 共享夹具、格式与保留行为

只有5个生产文件授权变化；[全部源比较](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/source-comparison.json)确认其余782项已有源/配置、全部原测试/断言和共享helper保持原字节。新增测试只放在new_intent_boundary_test.go，不删除或改写旧断言。

保留MockAppServer、StagedBusiness、historyFixtureBinding、正式--orca-create-fixture及native/integration探针。异步问答仍实际读取ObservationEvent.Kind/Status；Summary/Title无读取，journal保存的仍是原Envelope。Observe整个文件与基线进行精确的标签删除变换比较，原事件选择、replay、checkpoint、journal.Write/Sync、验证/拒绝、deliver调用及顺序、取消和完成判据不变。

[AST](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/declarations.json)中90个保留声明（37个类型）相同，只有5个授权声明修改及Display方法删除，无新增生产声明。历史decoder、owner.validateOwnerRequest、operation.New/Invoke/Reconcile、Settle/SettleStop、保存、单确认、原预算/结果及精确停止正文保持。未引入新耦合或新恢复框架。

持久根类型、代表Run/Snapshot/Request/Receipt/HistoryBinding、四类保存旧输入字节和公开能力/资源摘要前后**145263字节完全相同**，SHA256 `a5ec2f5e94487c7b4cea1711a50b23a423bc41aaf0c2d5444e0b9ec5284c7c16`。[编码比较](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/wire-comparison.json)。PreparationFacts和ObservationEvent为授权变化的瞬时类型，前者不再混入持久根声明捕获；没有改写state/v8、SQLite user_version=8、CLI/Workflow/profile/owner/native协议或任何持久字段/动作/decoder。

## 4. 最小验收与构建

| 验收 | 实际结果 |
| --- | --- |
| NIB-00 | 设计先行；写入/读取/owner/恢复与共享夹具消费者复核完成 |
| NIB-01/02 | 两项新边界回归2顶层/12子用例通过，已计入application；原操作、晚到结果、缺预算、人工恢复和stop等定向通过 |
| NIB-03 | 持久编码、保留声明及原测试/共享夹具保持；5个生产文件精确授权变换成立 |
| NIB-04组件 | execution/...及runstate/contracts/resources/storage/testkit/protocol：exit0，447顶层/1343子用例通过；8个既有skip、3个无测试包单列 |
| NIB-04 application | exit0，51顶层/98子用例通过，0skip，含新增两项；组件加application共498顶层通过，重复运行不累加 |
| NIB-04正式消费者 | native/integration各自live编译通过；正式native helper和scripts/build.ps1最终Runner构建通过 |
| NIB-04公开链路 | 同最终EXE的NativeUnknownAndStop、RestartPreservesAuthorization、ServiceOwnership三场景3/3、0skip，含隔离夹具及原资源清理 |
| NIB-05 | A/B记账、源码/产物、命令/退出码/失败/skip、Git证据原字节保护及文档索引完成 |

[精确命令](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/commands.json)/[测试名称与统计](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/test-summary.json)/[公开结果](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/foundation/result.json)。最终Runner SHA256 `770ee58826baafb10dc66dd5db10a754bfaab9186aacc4a84e9ea8a973847401`，source_digest `b800a590e06af4faf25e92e8e4bd81c283489e4e06438ab84d76b0b9e3b16a87`。[构建](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/runner-build.json)绑定同最终EXE；构建后复核源摘要和788项冻结源身份相同，未边编译边编辑。

按最新主线用test-foundation精确选择三个场景，再由test-controlled -FromEvidence导入计时。[计时摘要](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/timings/summary.json)为mode=timing-import、status=passed、qualification_eligible=false，不表示完整Controlled或L1/L2/L3通过；没有扩大组选集或修改既有测试脚本/目录清单。

## 5. 首轮失败与限制

新增边界测试首次直接调用private invoke(reconcile=true)，遗漏worker gate约定而触发unlock of unlocked mutex，原日志保留。改用正常advance后，手工旧请求未带StartedAt/UpdatedAt，原有界对账预算正确阻止恢复，四个旧动作子用例失败；补齐首次冻结时间后，两项测试及12子用例通过。只修正新增夹具，没有改变生产锁、期限、恢复或原断言，详见[失败说明](../../../../acceptance/evidence/new-intent-boundary-cleanup-20261102/first-failures.txt)及原始JSONL。

8个skip为原有helper/opt-in项，完整名称见统计；evidence、nativeaudit、runstate三个包无测试，不计通过资格。设计工具初轮误把PowerShell的InputValue参数及livetest.Cleanup当作同名Go消费者，按作用域纠正并保留这些真实脚本/夹具，未改产品或删除原测试。

本批只收窄新构造与无消费者展示，不退役持久模型、不合并双操作记录。历史原字节仍可读取，不承诺重建丢失资源、刷新原预算、跨构建接管或支持旧动作在真实适配器执行。未运行真实模型或取得新组合、完整L1/L2/L3资格，未操作用户已有Run或Orca工作区。未提交、推送、提PR或部署。
