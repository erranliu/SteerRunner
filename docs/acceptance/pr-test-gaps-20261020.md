# 近期 PR 测试缺口与补测（2026-10-20）

范围：按实际合并顺序检查 #35（服务换代）、#34（R2）、#33（内容流程）、#28（个人多机认证）、#32（查询重试）。基线为 `bf3ec581cb0b23a7358d95e5efa8ff5f607c2f55`；本轮为该提交上的未提交工作树，不部署用户安装环境。执行平台 Windows/amd64、Go 1.26.4、Codex 0.159.2。

## 实现与补充覆盖

| 范围 | 确认的问题 / 补充测试 |
| --- | --- |
| #35 + #28 | 修复本机 Check 接管预检遗漏退役认证和未知 operation 版本的校验；覆盖 Codex/Orca local、null/对象及合法配置，拒绝时原事实不变，v3 数据库格式/原 Check 字节不变、零升级备份 |
| #35 + #33 | 修复内容 Run 兼容性预检误要求未使用的 parser 配置；生成/对话均覆盖真实持久记录的空配置，冻结事实不重写 |
| #35 parser | 修复 parser `Reconcile` 落入普通业务回合历史查询；使用原 owner 的只读 receipt 路径，测试完成、pending、未派发三分支，零替代 owner/业务观察 |
| #28 真机追补 | 实际 L2 暴露 PR 正文临时文件占用认证 socket 目录；补充运行时临时文件位置/同调用清理指引。新增 socket 外来文件回归：撤销能力并删除原 socket，但保留外来文件、清理状态维持未确认；没有放宽目录删除边界 |
| #34 | 补单文件 1 MiB、64 条、总量 16 MiB、发现 256 项的包含式临界值；SQLite 真事务失败回滚、关闭并重开数据库的去重/不可变读取、冲突请求拒绝；确定性文件增长、同字节替换及读取失败；受控 CLI 加入源文件删除后跨服务重放 |
| #33 + #35 | 新增 `TestSRContentGenerationRecovery`：活动生成正常停服、活动对话强杀、原问题跨代回答、启动 stop；核验原进程/session/turn/历史及输入计数 |
| #32 | 补服务观察取消时的只读准备查询退出、锁/取消注册复位、授权/记录不变及后继查询；保留已有次数、期限、错误分类、停止竞争和零重复创建/发送断言 |
| 测试设施 | 失败现场保留原日志和资源身份，防止递归删除破坏未结算 owner 的证据；新增定向恢复入口和 focused native 注册；L2 真实开发交付后运行原工作区 checker 并验证 R2 原字节/重放/Git 观察 |
| 完整目录维护 | 将工作流发现的旧“两项”断言更新为四个明确 ID/版本及未知选择零输入；完整 Controlled 聚合改为 45 分钟，child/outer 预算同步，单独 L4 沿 registry 超时，不改变业务/原生调用期限 |
| 原生换代驱动 | 更新远端活动回合、初始化窗口、通用终端空闲换代的旧“强制 resume”断言及证据校验，改验兼容换代保留原授权/身份、原输入一次、自动推进；显式 stop 的撤销断言保留。新增固定 full_access 的正常/强杀两个独立 case，原 sandbox case 保留；当前目录为 28 场景、38 case |
| 脚本退出码 | 修复成功路径继承预期失败探针退出码的问题；完整 Go L1 的 480 项结果本身通过，旧调用方退出码记录保留，修复后另以真实脚本入口复验 |

## 可重复入口

```powershell
./scripts/test.ps1 -Level Controlled -OutDir <新证据目录>
./scripts/test-generation-recovery.ps1 -OutDir <新证据目录>
./scripts/test-generation-recovery.ps1 -NativeExecutable <现有 Codex 0.159.2 EXE> -Model <可用模型> -OutDir <新证据目录>
```

恢复入口先运行定向规则和四个受控顶层场景。指定原生 EXE 才执行两个 focused native 场景；真实 parser 场景的业务执行/provider 仍是受控夹具，不计为真实开发交付资格。

## 验证登记

| 验证 | 结果 |
| --- | --- |
| Go L1 | 核心构建 479 项通过；追补临时文件指引后的完整复验 480 项通过。8 项按各自 native/helper 条件跳过，不计为原生验收 |
| 测试框架 / 扩展 | 最新脚本框架 19 组、扩展 29 项通过；native driver 编译及零匹配拒绝检查通过 |
| Controlled | 两个全新、不重不漏的分组覆盖目录全部 21 个顶层场景，无跳过；两个分组的 Runner 和 integration fixture SHA 均一致 |
| focused native | 原 Codex 0.159.2 / gpt-6.1-sol：内容 4 个窗口、parser 3 个窗口全部通过；内容窗口核验实际 OS shell 身份，parser 核验原 owner receipt 和原期限 |
| 认证边界 | 真实沙箱 gh broker/socket 工具边界通过 |
| 追补实际 L2 | 完整 `local-codex.result-acquisition` 通过：真实默认 parser、parser 边界、真实开发交付/合入、实际程序报告的 R2、认证撤销和资源清理均通过 |
| 静态检查 | `go vet ./...`、`go vet -tags live ./tests/integration ./tests/native`、`git diff --check` 通过 |
| L4 | 四个场景以相同追补 Runner/fixture 独立通过；资源压力完成 20 轮，handles 增长 53，低于固定上限 128 |
| 其它远端真机 | 完整权限 smoke、同服务两 Run 隔离、原生活动停止、首次初始化停止独立通过；追补后的完整权限正常/强杀换代、初始化跨代停止三项通过 |
| 脚本成功退出 | 实际脚本入口使用定向 L1 包，并执行完整框架、编译、零匹配拒绝和扩展检查，最终退出码 0；此项不另算一次完整 L1 |

Controlled 的通过记录是同构建分组结果，不宣称原先单进程全量批次通过。原始本地证据统一位于本工作树 `.evidence/pr-gap-*`；各独立 batch 的 `source-identity.json`、Runner SHA、日志和 `result.json` 绑定实际构建，不能混写旧批次为通过。核心修复和 focused native 对应 Runner SHA256 `B77E2188E8E5FAA6AC663F6C29D3D174361B968A473E24341EF44F07A7069836`；随后仅补充临时文件运行时指引，追补构建 SHA256 为 `653F316936A9A42A6A4B71757C69ACA38BF2BE2118E9B16C3EA9E1C159FD5D6A`，其完整 L1 和实际 L2 结果另行登记。

已保留的失败包括：Check/内容预检缺陷的修复前反例；初次启动 stop 测试使用停服前旧 revision（改用原离线数据库中的 revision）；初次原生 parser 等待窗口不足；一次原生 parser stop 在准备会话阶段结果未知、未命中目标窗口；全量 L1 的 12 分钟包级超时和首批完整 Controlled 聚合超时。它们不因后续复验通过而变更结果。

原 14-case 编排批次仍为 failed：既含实际失败，也含追补构建后被调度器拒绝的构建变化记录。L4、原生停止/隔离等独立通过项按其原 `result.json`、源摘要、实际 driver EXE 和零跳过逐项核验，见 `l4-independent.json`、`native-independent.json`；不将它们改写为原编排批次整体通过。

最终汇总见 [report.json](evidence/pr-test-gaps-20261020/report.json)，变更源摘要见 [changed-source.json](evidence/pr-test-gaps-20261020/changed-source.json)。完整 L1 后的测试驱动、目录和脚本改动已经 `livetest` 全包、live driver 编译、19 组框架和真实脚本入口复验。实际 L2 测试 PR #628，以及完整权限换代 PR #631/#632 已合入各自独立测试 base；多 Run 测试 PR #629/#630 按既有策略保留未合入，原 Run/终端及服务均已停止。

收尾检查清理了早先认证目录失败现场的精确 owner：核对原 PID、创建时间、EXE SHA、原 owner 目录和原生写者退出事实后，仅结束该 owner；原测试及清理回执的失败结论保持。最终本轮现场没有残留本地 Runner/测试进程，远端停止保证仍限原终端回执。完整远端 Check、远端 R2、跨平台和远端 OS 全树停止不在本轮通过范围内。

## SteerRunner 创建恢复追补（2026-10-20）

已在 SteerRunner 内实现有限独立创建执行器，使用现有 Orca `worktree create` / `terminal create`，不依赖修改 Orca。调用前固化原 Run/Attempt/operation/generation/配置、路由、命令及期限，独占登记原执行器；服务退出只取消等待，原执行器继续接收并可靠保存原始回执。新服务只读原日志，校验绑定、内容摘要和原 runtime/worktree/terminal/incarnation，再结算原意图。覆盖工作区、初始终端及 native-control 创建；启动前已撤销的任务也可补读原身份以精确停止，不恢复业务授权。

没有回执、CLI 未返回原资源身份、执行器也崩溃、回执损坏/异源、旧记录没有原创建日志时，保持 unknown。恢复不再次启动创建执行器、不重发 create、不按名称查找认领、不重置原期限；无原资源证据时不能宣称 stopped。

追补 Runner SHA256 为 `A415D3A11D327E89913AC666C7EF62F2BC9744285501BA27EEEB343826738FB2`，独立于上文历史构建。新增五组单元测试，含本机/远端路由、原调用方退出、初始终端、control、损坏/丢失/异源回执、写入失败，以及意图未完整落盘/启动失败/启动回执丢失后重复 Execute 的零重复创建。受控进程测试验证正常停服、强杀服务、回执落盘但未提交 DB 三个找回正例，以及原 CLI 响应丢失、执行器强杀两个 unknown 反例；全部固定一次 worktree create、零替换 terminal create、零业务发送。

真实 Orca 1.4.224 的 `orca-remote.initialization-stop/restart-before-readiness` 使用同一构建通过：原终端/授权跨代保持、准备阶段零业务输入、显式停止及离线原终端 scope 收尾均成立。这是新创建路径的现有后端兼容性验证；创建响应精确故障屏障使用受控公开协议夹具，不冒充真实后端任意丢包或多组件故障下的找回资格。

当前证据、各批构建摘要、失败历史及复验见 [creation-recovery/report.json](evidence/pr-test-gaps-20261020/creation-recovery/report.json)。最初夹具缺少 terminal list、裸终端身份错误、共享 CLI 锁干扰、unknown 收尾错误要求 stopped，以及只读 SQLite 取证时机问题的失败均保留。最终夹具隔离缓存/锁，通过公开 inspect 固定故障前 revision，强杀窗口确认原 owner/CLI 退出，unknown 用例仅收尾本现场服务/执行器。另修复回归发现的发布前证据门旧步骤名，保留其 L1 反例及修复后复验。本次未修改 SteerLine 或 Orca 源码；本节恢复能力无需用户管理员操作。

追补 L1 初始全量为 21 包通过、livetest 包失败（旧步骤名两项反例）；修复后完整 livetest 包通过，并追加创建独占登记测试通过。按包与新增项合计覆盖 485 个顶层通过、8 个条件/辅助跳过，不将最初全量 exit 1 改写为 exit 0。19 组框架检查通过。定向回归的内容恢复、授权撤销、原生 unknown/stop、正常/强杀及兼容新 EXE 接管四个顶层测试通过；该五项聚合批次因旧夹具的 owner-crash 取证失败仍为 failed。随后五个创建窗口在两次独立同 Runner 构建批次通过，最终批次使用公开 inspect 取证且零跳过。默认 Controlled 已注册 22 项，本次没有重新执行完整 22 项资格。只核对本节独立测试现场，残留所属进程为 0；未操作其它任务或安装服务。

## 管理员故障补验（2026-10-20）

用户启动独立管理员宿主后，普通用户主测试已执行全部八项正式 sandbox 故障 case：停止撤销的 `boundary`、`unreachable` 两项通过；初次批次其余六项失败，批次结果保留为 failed。`unreachable` 实际维持 240 秒断网并重启服务，验证撤销跨代保持、当前反馈和旧 revision 写入被拒绝、零新增业务输入。

| 用例 | 实际结果 |
| --- | --- |
| connection-reconnect / active-disconnect | 断网、原只读观察重试、身份/授权保持及恢复断言通过；最终发布被未提交工作阻断 |
| connection-reconnect / pre-send-disconnect | 初次因夹具使用旧 `publish` 步骤名未命中屏障；改用 `workflow.StepPublish`，新增旧名/报告/实现步骤拒绝反例，全新现场重跑已命中 Runner-only 断网、原操作连接重试和授权保持，最终发布仍被 Git 权限阻断 |
| service-restart / graceful-boundary、kill-boundary | 均命中断网和服务换代两个故障，自动延续断言通过；最终发布被未提交工作阻断 |
| stop-revocation / boundary、unreachable | 两项正式 case 通过，原终端、服务和网络清理通过 |
| publication-unknown / completion-gap、query-unavailable | 发布前即因 Git 权限失败，未命中真实发布成功后的目标窗口；不能据网络预检宣称该恢复路径通过 |

初次五项业务失败及修复后的 pre-send 重跑均明确遇到 `Publication requires committed work`，原现场存在 `index.lock` Permission denied 证据。以上共六个 sandbox case 仍需在具备受信任 Git roots 准备/授权能力的远端环境复验；没有扩大 sandbox 权限来取得通过结果。

本节实际 Runner 固定为 `653F316936A9A42A6A4B71757C69ACA38BF2BE2118E9B16C3EA9E1C159FD5D6A`，与上一节创建恢复的 A415 构建分别登记。各 case 的真实 native driver/source 摘要逐项保留，不声称整个初次批次使用同一 native driver 构建。步骤名修复后完整 `livetest` 包通过、相关 `go vet` 及 live native 编译通过，不另算一次完整 L1。

九次执行的原终端和服务均已确认停止、对应防火墙规则均已恢复。初次 pre-send 的原生停止最初未确认，后以原 runtime/terminal/incarnation/workspace 精确关闭并由原服务补读停止事实，离线 `resources_settled=true`；原测试失败与原 cleanup 字段未改写。管理员宿主共处理 39 个网络动作，零失败、零待处理请求，原 host_id 的 finished 回执和原 PID 退出均已确认。本批九个现场的本机可执行文件根下残留进程为零；远端停止证明限原 Orca terminal scope。

汇总及有限证据见 [admin-faults/report.json](evidence/pr-test-gaps-20261020/admin-faults/report.json)，原始大日志保留在各现场及 `.evidence/pr-gap-admin-*`。没有复制私有测试配置或 owner 认证数据。

## 合并最新主分支后的复验

已合入 `c122d641677593d4b694e2eda7502001510d4e8b`（#36），保留其执行边界重构；R2 冲突同时保留源文件删除后的回执重放及跨构建/重复重启断言。实测代码提交 `f5bd14b31ceb5215d3c03f911806f46fcbf641bf`，Runner SHA256 `A06094E7DB71B856F29767809CEE485ED195911AF2ADE7920A4B6A5AD7254380`。构建 dirty=true，包含当时尚未提交的独立管理员证据，未将后续证据提交号伪写为实测构建来源。

全部 internal 包编译通过；11 个定向包共 59 个 Go 顶层测试通过、零跳过。同一新构建的内容恢复、创建恢复、R2 三个受控顶层测试通过；R2 另以 A415 旧构建接受原任务，再由 A060 接管并重复重启通过。五个创建窗口各保持一次创建、零业务重发。当前新构建未重跑完整 22 项 Controlled 或真实原生矩阵，旧原生结果仍绑定原构建；详见 [pr-merge/report.json](evidence/pr-test-gaps-20261020/pr-merge/report.json)。

## 需要人工或环境处理的项目

1. SteerLine 同现场 UpdateRunner 界面联调：保持活动任务或等待回答，点击更新 Runner；核验界面仍绑定原任务、回答与报告，原会话继续。该仓库自动测试覆盖 Runner 公开契约，不替代消费者 UI 联调。
2. 远端 sandbox Git 写入阻塞：原 smoke、调用方响应丢失、正常/强杀活动换代四项，以及本节六个管理员 case，合计十个独立 case 尚未通过。需要执行主机具备受信任的精确 Git roots 准备/授权能力后复验；其中发布 unknown 两项必须实际进入发布成功后的故障窗口。完整权限有独立 case 和证据，原 sandbox 权限要求保留。管理员八项已执行，宿主已退出，当前无需再启动管理员窗口。
