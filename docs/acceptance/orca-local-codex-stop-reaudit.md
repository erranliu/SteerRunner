# 本机 Orca Codex 停止实现再审计与 PR 验证

日期：2026-11-01。结论：发现并修复六处遗漏；停止条件已在本路径收敛，未留下已确认且未处理的阻断问题。初轮“文件门禁删除”的验证不足以覆盖业务状态读取和 live 测试消费者，本报告补正其完成结论。原[验收](orca-local-codex-stop-simplification.md)及证据保留原事实，不追认为最终构建资格。

范围依据[Spec](../spec/orca-local-codex-stop-simplification.spec.md)、[Plan](../spec/orca-local-codex-stop-simplification.plan.md)和[长期承诺](../spec/steerrunner.spec.md#长期产品承诺)。已整合 main `36de6ae56324b36d89adf240fc1fd26f595f0829` 的 execute/1、离线 inspect 和原 stop 保护；未部署、迁移旧数据、操作用户 Run、合并或发布。

## 发现与修复

| 严重度 | 再审计发现 | 最终处理及反例 |
| --- | --- | --- |
| P1 | stop、scope 读取和 ShutdownLifecycle 仍经过完整 state()，历史 reader 证明、业务冲突或 control READY 可间接阻止资源结算 | 提取严格解码、envelope/原资源/启动身份绑定的 resourceState；正常 state() 在其上继续完整业务校验。scoped stop/verify 和 scope 读取使用资源层，删除 ShutdownLifecycle 重复的业务读取。reader-evidence/control-not-ready/business-conflict 三个反例均能停止，但正常业务读取仍拒绝，零新控制输入 |
| P1 | 迟到 control 创建回执只更新停止快照的 Implementation，Operations 仍是原 unknown，可能永远无法结算 | scoped 原 stop 只读导入最新已提交操作事实；原意图、正文、目标和身份不改写。原创建回执确认后可结算；异源资源仍拒绝；不再 close |
| P1 | 无外部资源记录时，已保存 stopped 可掩盖尚未核实的创建 | scoped 离线核验同时检查原未知操作；真正未创建且已停止可成功，新增未知创建后必须 false，旧 stopped 不能覆盖 |
| P2 | adapter 虽限制 15 秒，上层仍给本路径 60 秒，后续采集可超出合同预算 | 上层 scoped stop 同样为 15 秒，取消贯穿原操作；其他后端的 60/90 秒预算保留。受控检查进入 port 的实际 deadline，并验证取消后保持 unknown/授权撤销 |
| P1 | 带 live 标签的 local cleanup 仍引用已删除的 ReadControlClosureEvidence，并要求签名/Closed，验证链路继续耦合旧停止条件 | 改用同一登记资源 verifier，核实冻结 scope，归档实际 helper/root/control 身份；Pi/DSH 实际退出证明保留。删除无消费者的 SignedNativeExitEvidence/ControlClosure 测试包装，签名 writer 及直接 Codex 验签未删除；native/integration 正式编译通过 |
| P2 | 本机工作区路径改为大小写/分隔符精确字符串比较，偏离原 Windows 路径等价规则，可拒绝合法停止 | 恢复本机路径归一化后的 EqualFold；worktree/runtime/terminal/incarnation 和 branch 核验保持。等价路径停止通过，异源身份反例继续拒绝 |

修复前定向反例复现失败，见 repro.jsonl.gz、offline-repro.jsonl.gz；修复后对应测试通过。跨 Run 资源及原记录保持、事务失败零派发、停止后零新业务输入、未知关闭响应只读对账等既有断言仍保留。

## 收敛与耦合检查

当前 scoped 结算只依赖原业务/control 终端、helper/app-server 根身份，以及是否存在未知创建。退出签名文件、control Closed、pending reader 响应和历史业务回执均不再成为这组资源结束的门禁；同一 verifier 服务显式 stop、普通关闭中的核验、离线核验和 live 清理测试。实际 helper 请求、Job 清理和根身份探测继续执行，不能用资源 scope 宣称全部后代退出。

没有新增清理账本、持久阶段、资源重建或恢复队列。资源解码及身份规则共用，业务采纳建立在其上；避免另复制一套完整状态 decoder。两条测试注入边界仍直接默认到现有 OS probe/Orca 文件通道，删除了只包装这两个赋值的 localStopDefaults 工厂。application 原本已依赖 Orca adapter，本次仅按冻结 scope 选择本路径合同，未增加包依赖方向或循环。

与 main 的全后端 pending-stop 保护合并为同一个入口：本 scope 返回约定的 operation_unverified，其他后端保持 main 的 invalid_state，不叠加第二套停止受理逻辑。execute/1 的迟到 final 不替换原 stop，离线 inspect 不启动执行，均有最终回归。state/v8 的格式隔离保留，不为 v7 或更旧 Run 重新解释承诺。

## 最终构建与验证

| 项目 | 固定事实 |
| --- | --- |
| 测试源码 | 合并提交 `756b064bc445418266bb94c5aafe01406d1f889f` 加最后三处工作副本修正；build 明确 source_dirty=true，最终 PR 提交整理不改写该历史 |
| Runner SHA256 | `a2936f13bfe6241036c18adb4dce751970e6f914c9f3e466dd00590843b59a24`，Windows/amd64，0.2.0-dev.4 |
| 源码摘要 | `fdc4ef0c4947d28f0fa028431d5a671aa99c717a2e6699862e5497bb9f163e81`；归档前用正式 Get-SRSourceIdentity 核对一致 |
| 真实 helper SHA256 | `22e7f73791ae3e27764725573c94221c44e1ba37d67686e1be2f6ff1acf221c6`，从当前源码独立构建 |
| 真实模型 | 本机已安装 Codex，gpt-6.1-sol，已有登录；仅 READY bootstrap，业务输入 0，无任务实施或 PR |

原始记录见[构建](evidence/orca-local-codex-stop-reaudit-20261101/runner-build.json)、[源码增量](evidence/orca-local-codex-stop-reaudit-20261101/source-delta.zip)、[机器汇总](evidence/orca-local-codex-stop-reaudit-20261101/test-summary.json)。顶层测试数不累加重跑或子用例：

| 验证 | 结果与实际范围 |
| --- | --- |
| 完整 Orca adapter / protocol | 161 / 41 通过，各 3 个既有显式 helper/live skip；86.850 / 23.731 秒。发生在整合 main 前；包内随后只删除冗余赋值工厂，最终真实探针与 live 编译使用该最终接线 |
| 整合 main 后 application 定向 | 38 通过，253.681 秒，覆盖 freeze、普通恢复、单次执行及相关合同；同次选择器在 host/operation 零匹配，不计这两包覆盖 |
| 最终 application 停止/可靠性定向 | 24 通过，13.541 秒，覆盖六处修复涉及的 scoped 路径、未知创建、原意图、事务/竞态、人工对账和 execute/1。与上一行有重叠，不相加为独立覆盖 |
| 最终完整小包 | host 13 通过/1 个显式 native skip；contracts 17、resources 18、storage 18、operation 8、livetest 40 通过。离线 inspect、只读锁、v1–v7 拒绝及新库 v8 保留 |
| 带 live 标签编译 | `go test -c -tags live` 分别构建 tests/native 和 tests/integration 成功；未执行任何 live 场景，见 compile-only.json |
| 最终 EXE 的正式公开受控 | ServiceOwnership、NativeUnknownAndStop、RestartPreservesAuthorization 3/3，exit_code=0；后者含 graceful/kill/creation_receipt/pending_stop，见 controlled-result.json |
| 最终 EXE 真实停止探针 | 1/1，88.687 秒。公开 stop → 停服 → 离线核验 → 删退出文件/journal → 再离线核验成功；活动期 false、停止后 true，前后 DB/原 shutdown 字节不变，零隐式启动。正常 ShutdownLifecycle 同 scope 通过 |

真实探针使用隔离注入的当前 Run 绑定真实创建回执/readiness/原根身份，不是公开 submit 到任务/PR 完成的整轮。原业务/control 终端与 helper/app-server 根实际结束，Job 工具后代终止仍由原真实 Job 夹具证明；本报告不将根缺席推广到所有后代。原生探针资源关闭并清理后，新鲜清单确认自有 worktree 缺席，见[清理确认](evidence/orca-local-codex-stop-reaudit-20261101/cleanup-confirmation.json)。

## 保留承诺与实际成本

PC-R01 的原子受理/提交、R02 的可能外发后零重发、R03 的业务/审计来源、R04/R05 的原资源普通重启及人工恢复、R06 的输入撤销和有界真实停止、R09 的逐 Run 身份均有对应回归或原机制保留。作用于这一 scope 的 stopped 不生成 WritersAbsent，不授权恢复业务，也不改变 PR、审计或 Workflow 结论。直接 Codex、原生远端、Pi/DSH 和独立 parser 未改用新范围。

相对目标 main 的生产 Go/JSON/client **+294/−272，净 +22 行**；其中 Orca adapter **+243/−253，净 −10 行**。不含 internal/testkit、测试、文档和 skill 文本，见[逐文件计数](evidence/orca-local-codex-stop-reaudit-20261101/production-delta.json)。收益仍是两类证明规则及隐含业务门禁删除；未测得等条件时间加速，不宣称全局净删或故障率改善。

## 未通过的扩展尝试与限制

- 扩跑的 application 全包在 8 分钟累计预算用尽；已结束的 137 个顶层测试通过，但 package 为 fail，不能记作全包资格。未据超时推断停止挂死；改以受影响的完整 adapter/小包及最终 application 定向完成本批要求，不提高生产等待预算。
- `go test -tags live ./... -run '^$'` 不是合法的整仓纯编译入口：native/integration 的 TestMain 拒绝零场景。另有主线 experiments 引用已退役 LinkedWorktreeArgs，造成编译错误；这两个源码在本 PR 相对 main 无变化。最终使用 `-c` 分别编译受影响的正式 native/integration，未修复无关实验，也不宣称全仓 live 编译通过。
- 初次编译尝试还保留了清理旧文件与编译并发的 file-missing、删除工厂后的 unused import；修正并在稳定源码上重跑，不把这些失败改判为通过。
- 没有完整 L1/Controlled、L2/L3、全 Agent 真机或外仓资格。原 PowerShell 客户端只保留本批已验收的 v8 默认目录/BOM 改动，本轮无新增客户端派发逻辑；未重复无变化的客户端模型矩阵。

原始失败、最终通过、公开调用和必要身份均归档于[证据目录](evidence/orca-local-codex-stop-reaudit-20261101/)，完整性见 [SHA256](evidence/orca-local-codex-stop-reaudit-20261101/sha256.json)。未归档私有 endpoint 凭据或状态数据库。
