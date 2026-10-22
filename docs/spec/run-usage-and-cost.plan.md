# Plan：Run Token 用量能力与独立费用估算

日期：2026-10-21；实现一致性复审更新：2026-10-22。用户已授权实施、现场验收及复审修复后提交 PR；核心实现已接入，精确通过/未交付范围见[实施验收](../acceptance/run-usage-and-cost.md)。对应 [Spec](run-usage-and-cost.spec.md)。不修改 Orca、不部署 helper、不部署或更换生产服务；计量/估价自身不新增模型调用。2026-10-21 本机 Codex 补样按授权创建最小原执行 Run，后续复用其日志；其他现场复用既有原 Run。仅临时登记必要日志叶目录，测后撤销。

## 1. 范围、依赖与完成口径

实现两个单向依赖模块：基础用量能力，以及只消费公开用量快照的费用估算模块。基础能力覆盖 Orca 本机/远端 Codex、Pi、DSH 六种组合的适用性、状态及事件契约；不增加执行后端，不更改原 Run 控制语义。远端只用既有 Orca 文件接口，所有解析与计价留在本机。

按 `P0 → P1 → P2/P3 → P4 → P6` 建设基础能力；`P1 → P5 → P6` 建设费用模块，费用配置不成为 P2/P3/P4 前置条件。P7 登记生产矩阵及缺口。P5 可以独立使用用量契约夹具验证，不等待远端现场或模型调用。

本机三 agent 和远端 Codex 是现有生产执行组合；远端 Pi/DSH 当前生产 gate 不支持。后两者的原版 CLI 调查只作为格式/传输证据。本文不以 headless、导入临时 Run、remote helper 或套用本机扩展替代其生产接入；可靠生产绑定未由独立执行接入交付时，这两项 G01 不得标通过，capability 必须如实声明不支持。

## 2. 预期改动位置与约束

下表保留原计划职责；对应生产包已创建并接线，未通过项仍按验收文档登记。

| 位置 | 后续职责 |
| --- | --- |
| `internal/usage/api` | 无 I/O 的用量 DTO、状态/原因、只读 Reader 接口和版本；不依赖价格、Orca 或 agent parser |
| `internal/usage` | 绑定、归一化/归属、幂等入账、覆盖/快照和受控采集；不持有业务状态推进权 |
| `internal/usage/parsers` | 三 agent 已验格式的本机解析、完整记录边界及字段语义；显式格式版本 |
| 现有 Orca protocol 包 | 复用连接、身份校验和文件/目录元数据 RPC；只提供必要的类型化访问端口，不上传 reader/helper |
| `internal/storage` | 独立计量事务、唯一约束、快照/水位及授权清理记录，沿原单写者/目录 ownership |
| `internal/application` / `internal/host` | 正常生命周期通知、用量只读服务、采集预算和重启/停止回收；不改变执行 gate |
| `internal/cost` | 只依赖 `usage/api` 的确定性估价及独立价目表；不读取采集存储/日志 |
| `internal/cli` / `internal/contracts` / `internal/resources` | 公共查询、信封、内嵌 schema、宿主策略与能力目录；复用 DTO 定义，不再维护另一套 token 语义 |

禁止用量模块导入 cost，禁止 cost 导入 Orca transport/parser/collector 或直接访问 usage 私有表。宿主是装配入口，可以分别启用两者。现有 local 执行 helper/扩展不修改、不重部署；不向 Run profile、冻结输入或模型提示词注入价格信息。

## 3. 实施步骤

### P0：冻结契约及调查证据

- [ ] 将 Spec §4 的 DTO、状态、nullable/零值、分页和 query-only 语义落实为精确 schema/类型；明确计量快照 revision 与业务 Run revision 分离。
- [ ] 按 Spec §2 固定 Windows/amd64 六组合的 collection/production_run_binding 声明；核对真实生产 gate，禁止从测试脚本的配置接受推导支持。不存在的 Run 保持协议错误，不为状态演示构造虚假 Run。
- [ ] 从已有 2026-10-21 调查产物提取脱敏、最小必要格式夹具，保留来源版本/原文件摘要和提取说明。默认不提交完整 transcript、环境配置、凭据、绝对用户路径或临时二进制。
- [ ] 梳理现有 Run/attempt/generation、NativeProof、Pi/DSH config/readiness/command 回执可提供的身份/边界。列出 Codex 初始化、恢复、DSH 双 session、子会话与未知情形。
- [ ] 按 Spec §7.2 在现有 service start stdin input 中增量定义 usage_policy 的精确 schema/默认值、有效策略查询及实例 revision；省略保留原运行策略，显式不同返回 usage_policy_conflict。更改经原 service stop/start，不建设热更新/配置平台。核验既有配对可提供的稳定 host_id；没有可靠身份不得登记。费用策略不混入 usage_policy。
- [ ] 固定每源预算、末次读取期限、状态保留/清理与游标过期行为。核对现有宿主配置入口后发布 schema，不能临时塞入 execution.settings 或悄悄改变旧请求校验。

完成证据：类型/schema 对照、来源映射及六组合缺口表；只读调查不计生产实现通过。

### P1：先交付不依赖价格的用量接口和存储

- [ ] 建立 `RunUsageReader`、GetRunUsage(runID, revision?)/ListRunUsageEvents 及 `run-usage/v1`。首次没有数据按确定性规则区分 pending/unavailable，不填零；重试性不是 pending 的判断依据，Run 不存在等使用原 CLI 错误。
- [ ] 新增 bindings、events/归属/冲突、checkpoints/snapshots、access leases/意图的最小存储表/约束，不修改原 Record 或 RunRequest 的序列化摘要。
- [ ] 为 event identity、单事件单 Run 归属和 source checkpoint 建立唯一约束；计量入账与水位同事务。稳定主机/source namespace 与连接 runtime 分开，避免 Orca 重启重复计量。
- [ ] 固定历史汇总/状态/覆盖/元数据和事件页的同版本读取、过期/不存在错误及并发行为；裁定、元数据或清理事实变化时新建 revision，不使旧页混入新聚合。保留全部引用明细或明确 snapshot_expired，不只有历史总数。计量事务不写业务 revision/event。
- [ ] 公开 CLI 查询只读已提交投影，不创建服务、目录、终端或模型请求。能力和 schema 在无模型/无价目表环境仍可查询。
- [ ] 验证关闭/缺失费用模块、坏价表/加载故障不影响宿主与上述全部行为；无用量源时也能先交付明确结果。按 issue.category 隔离 token、来源元数据与清理问题。

完成条件：U01/U05 的受控用例通过；基础包不导入 cost 或价目表。

### P2：本机格式适配、归属与去重

- [ ] Codex 使用原 session/header/turn/input 关联；逐 response_id 读 token_usage_record.usage，累计字段仅交叉核对。未验旧格式不自动降级为累计求和。
- [ ] Pi v3 校验 session header、原输入全文摘要和回执边界；按消息关系归属工具回合中的所有 assistant usage，正确处理 input/cacheRead 含义。
- [ ] DSH v4 校验 native session 与 seq 区间；流式解压串接 zstd frames，只计最终 assistant/message 的 usage，拒绝把嵌套 stream 和 step 投影再次累加。
- [ ] 在 Runner 进程内使用固定、经核验的 Go zstd 解码依赖并约束内存/解压上限；不把 Python、Node、系统 zstd 命令或外部解压进程变成产品运行依赖。
- [ ] 统一 token 字段、null/零、reasoning 子集与整型溢出校验；缓存写入非零/TTL 缺证据时保留未知细分，不使用零值掩盖。
- [ ] 相同 token 记录幂等；token 冲突隔离，最后一条可信事件消失时转 unavailable。仅元数据冲突保留可信 token，null→确证值合法补齐并新建版本。文件移动、runtime 重启、同 session 跨 generation 不产生新事件；真正重试的新响应保留。
- [ ] 实现恢复区间、初始化 phase、人工输入歧义及明确父子/fork 水位边界。区分 fork 继承与原会话显示分支切换：前者不重复计费，后者已发生调用不能因不在最终祖先链而丢弃。无法证明的覆盖保持不完整，不从 cwd 或 prompt 相同推断归属。

完成条件：U02/U03/U04/U08 的夹具及存储故障窗口通过；不读取真实用户日志来补齐单元测试。

### P3：已有 Orca 文件通道及可恢复授权回收

- [ ] 将已实测方法封装为本机类型化端口：projectGroup/folderWorkspace 的 list/create/delete，files 的 stat/readDir/readChunk。连接继续固定原 environment/runtime，原始 bytes 在本机处理。
- [ ] 只接受受信绑定中的来源及相对路径；验证 source namespace、最小日志叶目录、路径语义和已知链接边界。授权配置不接受模型提供的任意目录。
- [ ] 创建意图先持久化，资源响应后保存精确 ID；实现创建/删除未知响应只读对账和归属歧义处理，不盲目重发。原创建仍未结算时“列表为空”只能保持 pending；迟到创建在原目标完成清理，不再重新授权读取。
- [ ] 持久消费者记录/锁/epoch 管理同一主机/目录的自建登记；releasing 阶段禁止新 acquire，旧 epoch/旧实例回执不能触及新登记。重启不重复增加引用；不接管用户已有 folder/group，不在计量登记下建业务终端。
- [ ] 对删除的潜在终端副作用做独立验证：发现登记被外部复用或无法证明删除安全时保留 cleanup_pending，不关闭无关终端。
- [ ] 取消、失败、权限撤回、预算超限、服务正常停止及启动/重连执行有界回收；使用独立清理上下文，不因父取消立即跳过。group 必须自建且为空才删除；保留失联/复用 pending，不实现虚构远端 TTL。
- [ ] 关闭采集/撤回先持久化本机许可 revision/instance fence，禁止新操作，结果提交重验；在途操作不能宣称已回滚。撤权必须由原主机认证查询证实 ID 消失和 selector_not_found，且没有未结算创建；网络/认证/文件 ENOENT 反例不许转 revoked。外部修改无法安全核实时 pending，不把非原子的空列表检查当成 compare-and-delete；现有用户访问仍保留。

完成条件：U06/U07 的受控故障矩阵通过，且实际 RPC 路径无 helper/日志 shell reader。远端故障不能回退到本机或另一个 environment。

### P4：自动采集与原生命周期的最小接线

- [ ] 从原执行准备/会话绑定、提交接受、已观察结束/停止接收只读通知，推进计量水位；未知投递仅观察原请求，不发送补报/模型输入。
- [ ] 本机直接读已绑定文件，远端按 P3 取 bytes；固定长度快照、有界完整记录和前缀验证。DSH 首期重读/本机解压去重，不预建远端增量协议。
- [ ] 运行中有界采集；结束后最多 30 秒末次读取。未落盘/损坏/断线/超限时，有可信数据保留 partial，无数据按 Spec §4.3 返回 pending/unavailable，及时释放登记。
- [ ] 停服不停止远端原业务，计量回收也不关闭原业务终端；重启先对账旧计量资源，不恢复 agent、不重放输入。再次获授权的业务输入建立新水位/快照。
- [ ] 将 access_cleanup_status 独立投影，分开报告用量可用性、覆盖完整性、采集收敛及清理结果。各预期源（含未绑定）的状态/水位都进入同一快照，不能移除失败源或缩小 scope 来报告 available。已 finalized 历史不因后来离线/关闭采集降级；关闭采集仍保留只读历史查询和遗留清理。
- [ ] 不等待计量成功才执行原日志/工作区清理；源提前消失明确记缺口，不阻断业务、不重建日志。原文精简时保留可重采范围的事件 ID/归属裁定；去重/明细过期保存 tombstone，返回 RETENTION_EXPIRED/snapshot_expired，不重开空账本。

完成条件：U04/U05/U07/U08/G02 受控验证通过；原执行/停止/发布回归无语义变化。

### P5：独立费用模块

- [ ] 在纯模块内定义价格策略、来源/有效期/摘要、模型与计费上下文匹配、十进制金额及 schema；无文件/Orca/执行依赖。
- [ ] 仅通过 Reader 先取指定 revision 的汇总，再读同版本事件页；缺版本/过期不改算 latest，不触发补采。逐事件计算并汇总，用量 revision + 价格摘要固定结果。价格更新不重写 token 或旧估价。
- [ ] 实现混合模型、时段/日历、档位/上下文、缓存类别和显式假设；缺模型/价格/时间/档位时按 Spec 返回未计价，不从采集时钟或原生 cost 猜单价。
- [ ] 实现 estimated/partial/unavailable：非 finalized 或任一源缺口只能给已覆盖金额；没有可计价事件时金额 null，已验证 finalized 零调用才允许零总额。收敛只读 usage coverage，不另耦合业务 Run.status。基础来源元数据冲突/缺失只影响必要的计价，不能删除 token；可选费用策略初始化失败不妨碍基础服务启动。
- [ ] 增加 `estimate-run-cost` 公开只读查询和独立能力声明；关闭费用模块不改变 GetRunUsage、采集或撤权。

完成条件：C01/C02/U01 通过；费用测试全部可用冻结契约夹具离线复算，不要求真实模型或在线价格查询。estimated 必须同时满足 available、finalized、完整覆盖及全部计价；模型元数据冲突和坏价表均不改变基础 token 的可用性。

### P6：公共接口、故障和回归验证

先完成下节的确定性测试，再进行有授权的最小真实用例。每次现场绑定选定 Runner 构建、Orca/客户端版本、配置摘要、Run/session、快照、source hash 与清理结果。所有实际凭据留在原执行环境，不回显或写证据。

有价/无价、模块关闭、来源不可得和部分结果都经公开 CLI 验证，不能只调用私有 parser 后宣称接口通过。查询零副作用通过隔离后台 tick 或按调用因果计数验证，不把正常后台采集的 RPC 误归于查询。真实模型只验证统计原调用，不为取得 token 另发追问、token 查询提示或重复业务输入。

### P7：矩阵登记及交付

- [ ] 本机 Codex/Pi/DSH 和远端 Codex 使用真实生产 Run 验证公开接口；覆盖初始化和至少一个 command，核对规范化事件与日志已知范围，不把累计快照/子会话重复求和。
- [ ] 远端 Pi/DSH 的 reader/parser 用受控 fixture 和已有真实 CLI 证据验证；生产自动统计仍单列待依赖。只有另行具备可靠原生生产 Run 绑定且符合 TUI/停止契约后，才补齐其 G01 现场验收与 capability。
- [ ] 更新公开 schema、CLI 示例、能力、限制、原配置/日志保留关系及 `docs/testing.md`/`docs/test-catalog.md` 中实际新增测试；不宣称当前未实现的命令存在。
- [ ] 实施时新建 `docs/acceptance/run-usage-and-cost.md`，逐条登记 U/C/G 结果、构建及证据。保留未通过/未执行项，禁止以调查或纯 mock 覆盖生产缺口。

## 4. 测试计划

下表保留设计阶段的完整测试矩阵，不代表逐项已通过。新增生产包和实际测试已落地，执行与缺口见实施验收；测试匹配数必须非零，不能把 `[no tests to run]` 计为通过。确定性测试只用独立 state-dir/临时仓库。显式现场测试可以只读原生产状态库/日志并查询已安装 Orca，将冻结证据复制至无业务授权的隔离服务；不写原库、不替换安装、不恢复原业务。

### 2026-10-21 实现一致性复审增补

此表登记本次修复及其定向回归，不把部分实现的整条 P 步骤勾为完成；T01–T17、G01 的剩余资格继续按实施验收保留。

| 对应步骤 | 本次落实 | 定向回归 |
| --- | --- | --- |
| P0/P5 | capability 分离 run_usage/run_cost，六行分别声明 collection/production binding，target 使用执行契约值 | TestUsageCapabilitiesBoundariesAndSeparation |
| P1/P2 | 无显式正常等待时 unavailable；缺核心计数 partial；元数据冲突保持未知且不删除 token；校验已知缓存关系及溢出 | TestUsageIncompleteFieldsAndNoImplicitWaiting、TestUsageMetadataConflictRemainsUnknownAfterReplay、TestUsagePartialFieldsStillValidateKnownRelations |
| P1 | 首次观察前只发布 unavailable/COLLECTION_NOT_OBSERVED，不假定采集已启用；重复发布不刷新快照 | TestUsageInitialProjectionDoesNotInventEnabledWait |
| P2/P4 | 稳定机器/账号来源摘要、checkpoint 绑定；新输入/回执/profile 改变使收敛缓存失效并重置观察预算 | TestUsageCollectorPermitAndSourceIdentity、TestUsageCorruptCheckpointCannotRebind、TestUsageNewAuthorizedInputInvalidatesClosedCoverage、TestUsageChangedCurrentProfileCannotReuseClosedBinding |
| P3/P4 | stop 回执事务内撤销许可，观察入账重验实例/策略；撤销后禁止读取与新建，但允许原资源清理 | TestUsageStopFencesLateObservation、TestUsageRevokedPermitStillAllowsCleanup、TestUsageRevocationBetweenCreatesStopsAuthorization |
| P3 | 已知迟到结果在有界独立上下文持久化；子 group/repo 和删除后的变化阻止级联；清理状态按 Run/epoch 保留 | TestUsageKnownOutcomeSurvivesCancellation、TestUsageGroupCascadeReferences、TestUsageGroupReferenceProof、TestUsageIncompleteInventoriesFailClosed、TestUsageCleanupStatusIsRunScoped |
| P4 | 结束绑定等待耗尽后明确 unavailable/partial，不永久 pending | TestUsageEndedBindingWaitCannotStayPending |
| P2/P5 | DSH 毫秒时间戳及响应 seq 下界；缺 tier/context 不选通用价格，unknown 时间不能套 completion 假设 | TestUsageDSHNativeTimeAndSequence、TestCostUnknownDiscriminantCannotSelectFallback |

公共接口和撤权仍使用真实 EXE 的 TestRunUsageRemotePublicLive 复跑已有完成 Run；只读源库、独立 state-dir、合成价表、原登记基线及 selector 撤销断言均保留。此次没有增加模型调用或改变远端 Pi/DSH 执行 gate。

| ID | 场景与通过条件 | 计划入口 / 证据 |
| --- | --- | --- |
| T01 / U01,U05 | 基础包无价格依赖；无价表/费用关闭时用量完整可用；query 不产生网络/RPC/目录/输入操作；业务 revision/event 不变 | `go test ./internal/usage/... ./internal/application ./internal/cli ./internal/contracts ./internal/resources -run 'Test(RunUsageContract\|RunUsageQuery\|UsageWithoutCost)' -count=1 -v`；`usage-contract.log`、公开 JSON |
| T02 / U02,U03 | Codex 逐响应与累计快照、Pi 缓存/工具多响应、DSH 串接帧及嵌套重复；未知/零/非法/溢出、非零 cacheWrite；原生 reason/output 不重复 | `go test ./internal/usage/... -run 'Test(UsageParser\|UsageNormalization\|UsageDedup)' -count=1 -v`；来源 manifest、归一化明细 |
| T03 / U03,U04 | 同 prompt 不同响应、同事件多路径/多 generation、Orca 重启、双 session、恢复边界、fork 前缀/子会话缺口、人工插入与冲突，零串账 | `go test ./internal/usage/... -run 'Test(UsageBinding\|UsageOwnership\|UsageLineage)' -count=1 -v`；绑定反例及事件归属表 |
| T04 / U03,U05 | 事件写入前后/水位前后的故障、事务回滚、重复重放、冲突裁定、并发分页、游标错 Run/过期，快照不混版且不影响业务 CAS | `go test ./internal/storage -run 'TestUsage' -count=1 -v`；事务故障矩阵/快照 JSON |
| T05 / U06,U07 | 精确目录、路径穿越/链接、不配置授权、拒绝读取、授权读取、撤权失效；预有用户登记不变；私有目录不承载业务终端 | `go test ./internal/usage/... ./internal/execution/adapters/protocol/orca -run 'Test(UsageAccess\|UsageFileRPC)' -count=1 -v`；RPC 方法/路径白名单及资源前后清单 |
| T06 / U07,G02 | 并发引用、最后释放、创建/删除响应丢失、部分创建、断线、崩溃、重连/停服、权限撤回、外部复用，重试不产生重复登记/误删/业务输入 | `go test ./internal/usage/... ./internal/application ./internal/host -run 'Test(UsageLease\|UsageCleanup\|UsageRecovery)' -count=1 -v`；意图/journal、pending→revoked 回执 |
| T07 / U08 | 活跃追加、半行/缺帧、损坏、截断、header/前缀变化、64/256/4 MiB 边界、30 秒预算/取消，已核实部分保留，不能伪造完整 EOF | `go test ./internal/usage/... -run 'Test(UsageSnapshot\|UsageLimits\|UsageFinalization)' -count=1 -v`；水位/状态及资源上限 |
| T08 / C01,C02 | 多模型/时段/档位/缓存、时间边界、历史价格缺失、requested 元数据、partial、零/未知、原生 cost 失配、精度舍入、费用关闭；相同快照/价表确定性 | `go test ./internal/cost/... -count=1 -v`；冻结价表、用量输入及期望金额/原因 |
| T09 / G01,G02 | 四个现有生产执行组合经真正 EXE/服务与公开接口获得用量； repeated query 零新增模型/RPC；重启无业务输入；业务结束/停止及清理仍符合原契约 | 现有 integration/原生 live 入口增加定向 usage 场景，P0 固定精确名称后执行；每组合单独 Run/session、版本、CLI 结果和清理证据 |
| T10 / G01 | 远端 Pi/DSH 保持 unsupported 的公开反例；后续生产绑定交付后才能增加真实正例，临时 headless/TUI shell 不能替代 | capability/gate 一致性测试；独立依赖/未验收清单；未来真实 Run 证据 |
| T11 / G02 | 受影响原有提交、输入对账、反馈、恢复、停止和发布回归不变；统计/价格故障不触发 agent、SDK、补报、helper、诊断终端或业务状态变化 | 对实际修改包的既有测试和受控集成闭包；零外部调用计数、原状态对照 |
| T12 / U05,C01 | V1 汇总/事件读取期间产生 V2 元数据或 token 裁定，仍只按 V1 估价；显式读旧汇总一致；不存在/过期退出码 5；不偷换 latest、不补采 | `go test ./internal/usage/... ./internal/cost/... ./internal/storage -run TestUsageSnapshotRevision -count=1 -v`；同版本头/页/费用及故障窗口 |
| T13 / U01,U03,C02 | 同响应 token 相同但 model/时间/档位冲突仍计 token，仅费用不可得；null 补齐不重复；坏价表/加载故障不阻断宿主/用量启动；token 冲突仍隔离 | `go test ./internal/usage/... ./internal/cost/... ./internal/host -run TestUsageIsolation -count=1 -v`；token/metadata/费用三组独立断言 |
| T14 / U04,U05,C02 | 无数据正常等待/离线失败、全部冲突、可信零、历史 finalized 后离线/关闭/待撤权、两源仅一源成功；scope 不缩水；运行中完整前缀只能给覆盖金额 | `go test ./internal/usage/... ./internal/cost/... -run TestUsageAvailability -count=1 -v`；状态×数据×覆盖表及金额 null/零区别 |
| T15 / U03,U04,U08,G02 | fork 继承不计新费用，显示分支变更不丢原有调用；原工作区提前清理不被计量阻塞；明细/去重到期后原文件重现也不重复入账 | `go test ./internal/usage/... ./internal/storage ./internal/application -run TestUsageRetention -count=1 -v`；谱系、水位、tombstone 和原清理结果 |
| T16 / U06,U07,G02 | service start 省略/同策略/显式冲突、stop/start 撤回许可、在途读取/迟到创建、旧实例/epoch 与新 acquire 竞态、未知创建列表空；旧许可不能恢复采集或删新租用 | `go test ./internal/usage/... ./internal/host ./internal/cli -run TestUsageAuthorizationFence -count=1 -v`；policy revision/epoch/意图与清理回执 |
| T17 / U06,U07 | 原主机认证成功且 ID 消失/明确 selector_not_found/无未结算创建才撤权通过；网络/认证/ENOENT/主机变化为反例；共享活跃与旧 selector revoked 区分；外部复用/非原子删除不能伪造安全 | `go test ./internal/usage/... ./internal/execution/adapters/protocol/orca -run TestUsageRevocationProof -count=1 -v`；逐租用证明及聚合状态表 |

未来真实验证先使用已有完成 Run 做读取；新增真实 Run 必须有实施/现场测试授权及相应生产支持。远端只通过原 Orca 连接定位/读取/撤权，不能以 SSH 或远端 shell 计算结果。现场临时目录/凭据的创建与删除不是生产统计必须步骤；采集器自身只撤销自建登记，不删除原会话日志。

## 5. 文档阶段检查与风险登记

原文档审计阶段允许的检查是 `git diff --check`、Markdown 相对链接、Spec/Plan 条款与测试映射、既有执行/CLI 契约的只读核对和文档范围检查；不运行生产测试或真实远端探针。原审计只修改本组两份文档；工作区已有的原两份 Orca Spec 交叉引用保留，没有 `internal/`、`cmd/`、依赖或配置实现变更。Spec §11 的 A01–A10 均映射到实施步骤和 T01–T17；这些是后续测试计划，不记为已测试通过。

实施前须保留以下明确限制：

1. 远端 Pi/DSH 真实日志可统计不等于生产 Run 已支持；该依赖未完成必须保留原因和未验收项。
2. Orca folder 登记不是只读 ACL，也没有实测可用的服务端租约 TTL；离线撤销只能报告 pending，不能承诺即时回收。
3. 某些请求、标题或子代理用量可能没有进入已覆盖日志；范围/缺口必须对调用方可见，不能把日志合计当供应商总账。
4. 原生格式升级、非零缓存写入、路径/链接解析限制及预算超限都可能导致部分或不可得；不可偷偷降级解析或填零。
5. 旧 Run 缺可靠绑定、旧快照过期或价格历史缺失时保留明确结果，不迁移伪造事实；费用失败只影响费用输出。

实施阶段原授权为先不提交；2026-10-22 用户已更新为复审修复并回归后可以提交 PR，包含必要的本地提交和分支推送，不包含合并或部署。复用原日志的只读验证和隔离测试不改变生产实例；计量动作不新增模型调用，远端自动登记回收的现场资格仍单列。

## 6. 本次实施记录

生产代码已接入 P0/P1 的契约、快照、查询和策略；P2 的三格式与可靠回执区间；P3 的文件通道、持久化意图、引用/epoch 和保守回收；P4 独立采集；P5 纯估价模块；P6 的规则/公开 EXE 验证。2026-10-21 增补远端原 Codex Run 的公开 EXE 现场：自动登记/读取/撤销、初始化 phase、精确历史 TUI 换行规则及旧 pending 登记恢复通过。后续补齐本机 Orca Codex 主会话的真实原 Run/原日志、公开 EXE、独立合成价格、停止/重启和强制重读去重；精确范围与初次失败修复见[验收边界](../acceptance/run-usage-and-cost.md)。此记录不将原复选框或 T01–T17 整组自动标通过：fork/子会话完整性、其它包装和全部恢复竞态仍未取得资格。P7 的远端 Pi/DSH 生产 gate 继续不支持。

本机定向入口 `TestRunUsageLocalCodexPublicLive` 默认 skip，必须显式指定 `SR_USAGE_LOCAL_LIVE=1`、`SR_USAGE_LOCAL_STATE`、`SR_USAGE_LOCAL_RUN` 以及全新的绝对 `SR_USAGE_LOCAL_EVIDENCE`。它只读现存、当前契约版本的已停止 Run，在独立目录保留完整冻结身份和已撤销的授权，沿真实服务作原日志自动发现、逐响应独立控制和、分页、坏/关闭价表、停启及强制重读。不得为重读修改原业务 Run 或原日志，只在停止的验收副本中清除观察完成缓存，保留 prefix/binding 和账本；必须证明新实例确实重读且 revision/事件/金额不变。`TestUsageLocalCodexHomeRequiresKnownRoot`、`TestUsagePreparedLocalCodexInitialization`、`TestUsageRecordedCodexIDsUseOriginalTurnAndWireDigest` 覆盖现场修复的正反例。


2026-10-21 本机 Pi/DSH 现场增补（T02/T03/T04/T08/T09/T13/T16 的具体子集）：`TestRunUsageLocalPiPublicLive`、`TestRunUsageLocalDSHPublicLive` 默认 skip，复用原 pr_ready Run，历史字段只读投影到无业务授权/待执行意图的隔离服务。与 Codex 共用公开 EXE 汇总、分页、独立估价、坏/关闭价表、正常停启、强制来源重读及历史断言；独立原文件控制程序核对 Pi parent 链和 DSH 原生 session/seq 区间，逐响应 command/attempt/generation 归属与未知字段。最终分别 18 响应/200,432、21 响应/318,722 token；合成价表 0.04292800/0.07843400 USD。lease=0、permit inactive、目录基线/原证据不变。仅隔离采集服务停止重启，未重启原业务或新增模型调用，不扩大为全部 G01/G02、fork/子会话或远端 Pi/DSH 资格。入口变量复用上述 `SR_USAGE_LOCAL_*`，精确选择 agent 对应测试；完整边界见验收文档。


### 2026-10-22 提交前一致性复审

- P0/P7：Spec 当前六行矩阵与 capability 对照；旧调查时缺口保留历史标签。本机三组合、远端 Codex 的主会话资格单列；远端 Pi/DSH 生产 gate 保持不支持。P0–P7 原复选框包含未交付子项，不能整体勾选为完整交付。
- P2/P4：`TestUsageFinalReplyBelongsToCommand` 先复现错误，再修复 Pi/DSH 最终 reply 的 command 归属校验。parser 推进至 native-usage/3，使旧观察缓存按版本重新核验；已证实归属的 token 保留，回执矛盾只使覆盖 partial/未 finalized。
- P1：`TestUsageConflictEvidenceIsAtomicAndDurable` 验证隔离事件与双方规范化来源证据同事务、失败全回滚、重放不重复 journal/快照、旧页不改写。
- P3：`TestUsageLeaseReferencesAndEpoch` 增加 epoch 独立 marker 和空资源 ID 的创建意图断言；`TestUsageIncompleteInventoriesFailClosed` 覆盖 group/folder/repo/session 的分页/数量矛盾信号。旧 ID、epoch 和未知结算仍不得重用。
- P5：`TestCostTokenOnlyScopeAndCompletionAreExplicit` 验证 API 等价 token 范围、订阅/工具等排除项及 completion time 假设同时公开；基础用量 DTO/价格依赖仍不混合。

本轮实际回归、现场复验和撤销证明见验收记录；未实现的 fork/子会话、零调用证明生产源和过期/tombstone 机制保持缺口，不以 PR 交付掩盖。


提交前已整合 main `4d11148` 的 14 个上游提交。三个新增查询加入完整 ServiceBindingCommands/CLI，沿原 RWMutex 准入围栏；`TestSRFullBindingCLI`、`TestSRFullBindingRejectsBeforeEffects` 及带完整绑定的 `TestRunUsagePublicExecutable` 验证与新契约一致。保留原子 observe、命名和材料语义。整合前的通过批次不代替整合后 internal 全包、专项集成及四组合现场结果。


末次归一化复核补齐 nullable 输入分类约束：即使部分分类未知，已知 uncached/cacheRead/cacheWrite 合计也不能超过 input_total，且已知合计自身不能溢出；保留 null 而非填零。`TestUsagePartialFieldsStillValidateKnownRelations` 增加 cacheWrite/uncached 单项、两个已知缓存项合计越界和部分计数溢出反例。此修改后相关 13 包与四组合现场另行复验，应用层/执行链整组不冒充在最后归一化补充之后重新完整运行。


L1 入口同时拒绝既有日志 opt-in `SR_USAGE_EXISTING_STATE` 与远端 Run/evidence 残留；子进程负例确认 guard 在读取日志前拒绝，测试脚本语法通过。普通 Go 测试默认仍跳过现场，不能将 skip 当现场资格。


`TestCostPolicyOptionalNullCannotErasePredicates` 补齐价表读取与 schema 对齐：tier/min/max context 的显式 null 不得被 Go 当作省略而消除计价条件；省略与显式零正例保持可用。修复只在费用装配入口，最后 13 包及四组合现场按冻结代码再次复验。

### 2026-10-22 PR #47 合并前复审

用户本轮授权复审、修复、回归后合并 PR #47；仍不修改 Orca、不部署 helper、不部署分支，不向原 agent 新增模型输入。此前“未授权合并”是提交前阶段的历史记录。

- P1/T03/T13：`TestUsageCrossRunConflictSurvivesCachedObservation` 先复现后认领 Run 在无事件重读时丢失冲突、恢复 available/finalized 的错误；计量事务现保留该 Run 已持久化的 USAGE_CONFLICT，并使用认领方 source_ref。测试同时断言幂等 revision、旧版本不变、独立费用只能 partial/covered，不能给总费用。
- P2/T02：`TestUsagePartialFieldsStillValidateKnownRelations` 补齐已知输入/输出（含 reasoning 下界）不能超过 total、跨输入输出的下界合计不能溢出；字段未知仍保留 null，不推算值。parser 推进至 native-usage/4，旧收敛缓存重新核验。
- 本轮最终代码的整包回归、四组合公开 EXE 和授权撤销结果单列于验收增补，不复用前次源码/EXE 摘要冒充新资格。
