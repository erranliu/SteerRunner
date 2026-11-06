# Spec：离线 inspect 与执行生命周期解耦（B）

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-05。状态：设计审计后独立实施，B01–04及联合受控验收完成，见[记录](../acceptance/content-inspection-boundaries.md)。基线 main `f6a389cfd4ae7827d7c722555a85e94f1e1ba708`。依据[长期产品承诺](../../../../spec/steerrunner.spec.md#长期产品承诺)，配套 [Plan](offline-inspection-boundary.plan.md)。与[A](content-result-lock-boundary.spec.md)独立实施及验收。

## 1. 消费者和收益

CLI `inspect --offline` → host.InspectOffline。基线 host 取得原 writer lease、检查 bootstrap、OpenReader 后，构造完整 application.Engine，再 Handle inspect、Shutdown。虽然从未 Start、没有执行动作，构造仍依赖服务进程身份/epoch、worker/wake channel、context cancel 和关闭生命周期。B 将运行时依赖缩为持久查询及公共投影，未来 Engine 初始化变更无需纳入离线读取的搜索空间；包级编译依赖 application 保留，不另拆通用查询框架。不删除 Engine.New 的驻留服务/测试消费者，也不删除 nil launchOwner 的真实原生 owner 路径。

## 2. 读取及错误边界

host 保留 selectors 恰选一个、command=inspect、ExpectedBinding/ExpectedInstanceID 拒绝及 ctx.Err 检查；保留原 lease、busy→directory_conflict、bootstrap→startup_unverified、OpenReader 当前格式和原关闭顺序。

持久读取函数只依赖 Store 和 selectors：先执行 Store.Runs 严格解码**全部** Run，再按 Run.ID 建映射，随后读取原 selector。不能因目标 Run 正常而跳过无关 Run 的坏编码，不能改为局部宽松解析、忽略字段或重建记录。Store.Runs 失败仍由 host 包装为原 `service_unavailable / Original persisted projection is unavailable.`（已有 Fault 保留）。删除的仅是 CurrentIdentity 等执行构造故障，此后进程身份探测不再是离线 inspect 前置。

| 查询 | 原语义必须保持 |
| --- | --- |
| run_id | 缺失→exit 5/run_not_found；成功 `{command:null,run:RunView}` |
| request_id | Store.Command(request_id)，缺失→exit 5/request_not_found |
| command_id | Store.Command(command_id)，缺失→exit 5/command_not_found |
| command 对应 Run 缺失或 RunID=nil | command 成功，run:null，不变为 run_not_found |
| command 查询故障 | 沿原 application fault 映射（通常 exit 1/internal_error），不统一包装为 host service_unavailable |

复用驻留 inspect 的查询/投影实现。驻留 command 查询及关联 RunView 仍在 Engine.mu 同一临界区；驻留 run 查询仍基于原克隆，不改其它 Handle 命令。保留完整 null/字段/诊断，不刷新 provider，不探测原生执行，不写 DB/事件/命令/endpoint，不迁移格式、不授予授权。

## 3. 删除与保留

删除 InspectOffline 内 Engine.New/Shutdown 与 discard logger，新增窄持久读取函数和共享投影；不新增通用 read-service、接口注册表或另一本 RunView。

保留 host/inspect_offline_test、startup_state_test、application 的各类 Engine/contentEngine/auditEngine 和 native helpers；不删除或重构共享夹具，唯一追加适配见下段。state/v8、Store.Runs/Command、持久声明/解码、执行/恢复/停止及 PC-R01–R06/R08/R09 不变。

公开验收发现的主线提示词/原生替身不匹配单列适配：仅向 StagedBusiness 追加当前真实渲染前缀，保留全部旧分支和测试；不删除共享夹具或修改原断言，不改变 B 的读取协议。

## 4. 最小验收

| 编号 | 验收 |
| --- | --- |
| B01 | 三 selector 与驻留 inspect 的响应逐字节相等，覆盖命令有 Run/无 Run/关联 Run 缺失及三类 not_found |
| B02 | 无关坏 Run 仍先于 selector 错误失败；命令表读取故障保持 internal_error，全部 Run 解码失败保持 host service_unavailable |
| B03 | 原 offline invalid/deadline/lease/bootstrap/foreign-format 和启动状态测试通过；DB 原字节及持久/执行文件清单不变；原 SQLite 只读路径允许临时 WAL/SHM（本夹具无 WAL 数据），不新增该行为 |
| B04 | 离线调用图不再经过 New/Shutdown、identity、worker、port；既有测试/共享夹具及持久声明/编码不变 |

收益为去除执行构造和读取的依赖，不声称查询只读目标一行、内存复杂度改善或真实模型资格。
