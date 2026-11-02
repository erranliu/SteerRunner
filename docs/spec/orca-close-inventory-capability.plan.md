# Plan：Orca 关闭与完整清单 B4d

日期：2026-11-02。状态：实现及受控验证通过，待实施后独立审计。依据 [Spec](orca-close-inventory-capability.spec.md) 和[长期设计](orca-capability-layer.spec.md)。B4c 确认问题先独立修复。

## P0 固定边界

固定 client.go/readonly_close.go/stopped.go、agent/initial-terminal 清单及所有调用点。保留旧 stop progress/receipt/State/Profile 等持久声明，记录本机 managed 与远端 terminal 两种现行 scope；本批不新增本机原生生产准入。

## P1 实施

1. Client.InspectTerminal 使用 TerminalIdentity；CloseTerminal 返回原 Result、TerminalClosure 和调用/原生身份错误，不返回 StopReceipt、不自动重试。移除未使用 Config.Target 和 backend 导入。
2. terminal_inventory.go 提供纯列表参数、TerminalInventory 的基础/完整两种解码事实及 CompleteWorktreeInventory，严格保留原完整原主机清单判据和原生字段。准备消费者使用对应解析范围，原重试/等待和选择保持。
3. 执行侧 terminal_resources.go/readonly_close.go 接管旧 InspectBoundTerminal、CloseTerminalChecked、VerifyTerminal(s)Stopped 和只读设施关闭策略，统一映射执行身份/错误/StopReceipt；原双端停止、verify-stopped、testkit/host/探针实际接入，删除旧协议执行流程。
4. 原策略测试移至执行侧并保留 case/断言；协议新增独立关闭/清单测试。增加能力包依赖守卫，禁止导入其他 internal 执行/应用所有者。

## P2 验证

新增 TestOrcaCloseInventoryCapability：原 argv、一次 close 响应丢失无重发、排队未调用、原 runtime/handle/ptyKilled、原完整清单、坏/缺字段和基础字段范围不受无关扩展影响。受控调用次数与原 consumer 回归共同验证。

复跑实际存在的 TestTerminalIdentity*、TestTerminalClose*、TestTerminalStopped*、TestReadonlyClose*、TestRemoteStop*、TestTerminalStop*、TestLocalCodexStop*/StoppedRoots*、TestFixedControl*、TestWorkspace*/CreationOwner*、application stop progress/派发顺序/丢回执/只读核验、integration verify-stopped 定向；先 rg 确认名称和非零命中。原现行 scope 的测试只作为结构等价，不计 B5 候选资格。

生产入口构建、`go test -c -tags live ./tests/native` 仅编译，受影响两包全集、依赖守卫、持久白名单与 diff 检查。需要进程/锁/IPC 的夹具用正常权限，不操作真实 Orca 或模型。记录所有 fail/skip/实际命令、源码身份及测试迁移数量。

## P3 收口

写验收和证据、同步路线/索引，提交实现后独立只读审计，确认修复独立 commit。B4d 完成后才按 B5 冻结候选完整生产接线和真实 Runner 验收计划；不提阶段 PR。

实际迁移/删除、全部消费者、持久边界和验证见[验收](../acceptance/orca-close-inventory-capability.md)。
