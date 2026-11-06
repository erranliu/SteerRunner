# Plan B：旧证明计算器删除闭包

日期：2026-11-06。状态：P0–P3 已实施并完成受影响受控检查；A 验证点 `ceca3316`，A 收尾审计修正另存 `d31b7e7c`；依据 [Spec](orca-legacy-proof-calculator.spec.md)。

1. P0 在 A 通过点固定调用/声明/共享夹具清单和代表编码原字节；再次核对 query 唯一外部测试消费者及不可达 hook preflight。
2. P1 先将 codec 的 Conflict/Waiting 保留断言改为直接读写证明；原 roundtrip、usage 及身份检查保留。按精确闭包删计算方法和独占 transient fixture，不删历史类型或协议共享能力。
3. P2 运行 adapter、历史 decode/usage/files/stop 与 host/contracts/storage 定向，比较全部生产持久类型及代表编码；编译 A live/helper，核对 A 未新增旧计算器依赖。
4. P3 正式构建及公开受控原操作恢复/停止，审计最终 diff 与共享消费者，记录 A/B 独立验收和实际资格；更新状态，不自动部署或提 PR。

实际闭包：删除 23 个生产计算/旧 preflight 函数、7 个专属 feed 夹具函数及 7 个进程内类型；原 73 个 adapter 顶层测试全部保留，另加三 codec 的 Waiting/unknown 原字节回归。NativeProof 与其他保留的 30 个 Orca 类型声明完全一致；旧 State/Profile/NativeProof 代表编码在独立 A 源归档中与 B 逐字节一致，通用持久 wire/能力摘要也一致。具体构建、各级范围、三项 opt-in skip 与真实 L3 缺口见独立验收。
