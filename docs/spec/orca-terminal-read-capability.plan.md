# Plan：Orca 终端读取能力 B2c

日期：2026-11-02。状态：P0–P3 完成，独立审计未发现确认的 P0/P1/P2；见[验收](../acceptance/orca-terminal-read-capability.md)。依据 [Spec](orca-terminal-read-capability.spec.md) 和[长期设计](orca-capability-layer.spec.md)。B2b 独立审计通过，无确认问题。

## P0 固定行为和白名单

调查 control_terminal.go/controlLines 与 local_authentication.go/readLocalCodexReadiness 两处生产 read，保存原参数 cursor/limit=1000、默认 timeout、字段范围及错误分支。允许新协议能力及两处窄接线，不改控制分页算法、readiness 回执校验、身份查询实现、发送或停止。

## P1 单页能力和消费者

1. 新增 Orca TerminalRead(handle/cursor/limit) 与 TerminalReadPage，ReadTerminal 仅一次原 MetaCall。保持 --terminal/--cursor/--limit/--json 顺序和原始行，不增加独立 timeout；缺 handle 或非正 limit 调用前拒绝。
2. 集中解码共同基础页和完整 next/latest 的原两个字段范围，分别保留本地完整性。返回原 Result；原 runtime/handle 对应关系由自有匹配方法检查。原生字段或解析失败不升级为有效行。
3. controlLines 每次前查询→ReadTerminal→后查询的顺序保持，要求完整分页字段，再沿原循环处理数字、保留窗口、boundary、complete tail 和 trim；仅删除参数/结构和重复 runtime/handle 核对。
4. readLocalCodexReadiness 使用基础页、原 result/错误与从头完整性要求；继续忽略未消费的 next/latest 类型，全部前缀/唯一性/准备绑定代码保留。

## P2 确定性验证

新增 `TestOrcaTerminalReadCapability`：无 backend 原生值/真实 Client Output，精确默认 timeout/argv，特殊原始行/空行/CR 不变；保留 source、所有 cursor、bool 缺失/false/true、错误和不确定性；缺输入零调用。坏包、错 runtime/handle、坏 next/latest、重复坏类型、wire 解析标记分别断言，不自动追页。

在原 readiness/control 测试补同一坏 next/latest 的不同消费范围反例。复跑 TestLocalPreparationOriginalStream、RemoteBootstrap/Readiness、TestFixedControl*、TestControlTerminalRPC* 及原业务身份/取消定向。所有旧分页断言、原 send/create 计数保留；必要时用正常权限运行原进程夹具，不操作真实共享 Orca。

go test -json 记录实际 roots/子例/fail/skip；新功能先跑，受影响定向通过后不重复未触及模型现场。workflow 依赖守卫、git diff --check 和两处旧 read 调用零残留必须通过。

## P3 收口与审计

写 `docs/acceptance/orca-terminal-read-capability.md`，按 OTR-01–05 列原始证据、两个消费者、删除清单、剩余未迁模块及未验边界。同步文档状态/索引和长期路线，提交实现后独立只读审计一次；确认问题独立 commit 修复，不提阶段 PR。

所有直接验收、实际消费者接线/删除和独立审计完成后标 B2c 完成。
