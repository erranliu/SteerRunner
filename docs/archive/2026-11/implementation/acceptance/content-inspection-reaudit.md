# 内容结果锁与离线 inspect 实现再审计

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-05。基线及目标 main：`f6a389cfd4ae7827d7c722555a85e94f1e1ba708`；分支 `codex/content-inspection-boundaries`。依据 [A Spec](../spec/content-result-lock-boundary.spec.md)、[B Spec](../spec/offline-inspection-boundary.spec.md)及[初轮验收](content-inspection-boundaries.md)。本次用户授权复审、修复确认问题并提交 PR。

## 审计结论

未确认需要修改的生产缺陷，保持初轮生产实现。补充真实 `Handle(stop)` 入口的并发回归，覆盖前轮只通过 TryLock/直接事务验证、未直接覆盖的请求排队及 revision 校验时机。没有为审计新增配置、可选端口、持久票据或框架。

| 审计项 | 实际消费者和合法入口 | 结论 |
| --- | --- | --- |
| 材料采样/同 Run 排序 | worker.invoke 原 receipt、attachObservation 原结果、resumeRun 原结果均传原 worker.gate；finish 的 nil gate 为测试包装 | 只释放 Engine.mu，不释放 gate。final 的原生结果/来源和原 revision 检查仍先执行；只对原成功且未超限的内容结果采样，错误优先级保持 |
| 采样后复核及提交 | 原 Record.Clone 深复制；提交的输入、来源、授权、控制和游标变更经 commitLocked 增加 revision；独立 progress/result-file 投影沿原合并规则 | 重新加锁后必须匹配原 revision，陈旧采样不写业务结果、不 signal、不返回消费成功。finishContent 只改原克隆；Store.Commit 成功后才发布内存、signal/返回 true。失败仍可读原事实，不扩大为新输入或资源重建 |
| 停止和晚到原结果 | Engine.Handle 控制先 gate 后 mu；finish 保留原停止后的结果收录路径 | 不添加 Allowed 门，不恢复已撤销授权，不替换原 PendingStop。新测试经真实 stop 入口验证结果先提交，随后 stop 按提交后的 revision 接受或返回 stale_revision；新 revision 重试后原结果/撤销同在持久库 |
| 离线读取及错误 | CLI offline → host.InspectOffline → InspectPersisted；resident inspect 共用窄查询/投影 | host 的参数、deadline、writer lease、bootstrap、OpenReader 及关闭顺序保持。先严格解码全部 Runs，再查 selector，坏无关 Run 仍先失败；Run 解码错误保持 host service_unavailable，Command SQL 错误保持 application internal_error。三个 not_found、command-only/dangling Run 的 null、原 Go 值和 JSON 形状保持 |
| 构造和只读副作用 | 删除的仅是 offline New/Shutdown，resident New 及真实 nil launcher owner 消费者未改 | 原 Engine 未 Start 的 Shutdown 只取消/等待，不有另一次 DB 提交。新入口不查身份、构造 worker 或调用 port；Store 原 mode=ro/query_only 及临时 WAL/SHM 行为不变，未迁移或写业务数据 |
| 共享夹具 | contentEngine/repositoryFixture/auditEngine、原生 AppServer、R2/运行时文件均保留 | 351 个既有 internal 测试原字节未改。唯一共享支撑改动是 StagedBusiness 追加主线新提示词前缀，全部旧识别/真实 Git 动作/原断言保持；其原始失败及新旧消费者通过证据沿初轮保留 |
| 持久格式与承诺 | storage/runstate/contracts/backend/workflow/adapters 的原源码未改；无新增持久声明 | state/v8、原编码/解码、事件和 command 提交保持。执行、原资源恢复、有界预算、未知零重发、冻结停止 scope 均未修改；PC-R01–R06/R08/R09 保持 |

采样仍是对原文件的即时读取，既有全局锁也不能禁止宿主外部随后改文件；不新增文件系统/数据库跨系统原子性承诺。B 取消的是执行构造前置，application 的包级编译依赖仍在。

## 本次新增回归与验证

`TestContentMaterialReadSerializesPublicStopAtOriginalCommit` 阻塞实际 finish 的原材料校验器，同时通过真实 Handle(stop) 提交控制。两种 case 分别验证结果提交后的 revision 可受理、旧 revision 仍被拒绝；停止响应后持久 Result/History/撤销/原 PendingStop 一致，且无新增 StageInput/Intent。此测试不代替初轮材料漂移、陈旧采样、事务失败和不同 Run 并发回归。

| 命令 | 本次结果 |
| --- | --- |
| `go test ./internal/application -run '^TestContentMaterialReadSerializesPublicStopAtOriginalCommit$' -count=1 -timeout=2m -json` | 1 顶层、2 case 通过，0失败/跳过 |
| `go test ./internal/application -run '^Test(Content\|RecoveryFrozenMaterials\|LocalCodex\|NativeCompletionProof\|SingleExecution\|R2\|SRStopSerializes\|FrozenBudgetUnknown\|SRExecutionWaitRecovery)' -count=1 -timeout=8m -json` | 46 顶层通过，0失败/跳过，211.533秒；表中反斜杠为 Markdown 竖线转义 |
| `go test ./internal/host ./internal/cli ./internal/storage ./internal/contracts ./internal/testkit/protocol ./tests/helpers/native -count=1 -timeout=5m -json` | 74 顶层通过，0失败；原4个现场 opt-in 跳过，名称见证据统计，非真机通过 |
| 正式 build.ps1、integration `go test -c`、native fixture `go build` | 均 exit 0；Runner/driver/fixture 分别与初轮 controlled-qualified 产物逐字节 SHA 相同 |
| 代表编码与原源码摘要 | 148565字节编码 SHA `e11066197caa8eed82b90b01618e241a0a083715ffa1f1be571446e51b4dce3a` 与原基线相同；351个原内部测试字节及持久声明/编解码保持 |

上列测试集合有重叠，不累计为唯一用例数量。完整 application/L1/L2/L3 与真模型未运行。

## 构建与公开受控证据

本次正式构建仍为基线 f6a389c dirty，source_digest `29633212ec3362ab1cf2f6710bc4436510ca8741c50417efeb3a6608bfd71041`；变化仅为新增内部回归。Runner SHA256 仍为 `e40b074f24b9b736cd3857b9d15be2b6457ac72833a6e31163696a31e86e1a7d`。重新编译驱动和原生夹具后也与初轮 SHA 相同，身份比对见 audit.json。

因此保留初轮同 Runner/driver/fixture 的三项公开受控及重启四 case 资格，不将它们虚记为本次重跑：ServiceOwnership、NativeUnknownAndStop、RestartPreservesAuthorization（graceful/kill/creation_receipt/pending_stop）初轮均 exit 0。当前定向回归单独记录。初轮错误断言、主线共享夹具失败和原现场停止/独立核验均保持历史记录。

原字节日志、源码/格式/产物身份及统计见[再审计证据](../../../../acceptance/evidence/content-inspection-reaudit-20261105/README.md)。按本次用户授权提交、推送并创建 PR；不自动合并或部署。交付前刷新目标分支并以 GitHub 实际文件合并状态为准。
