# Plan：内容结果材料校验与提交锁边界（A）

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-05。状态：P0–P3 完成，见[验收](../acceptance/content-inspection-boundaries.md)。依据 [Spec](content-result-lock-boundary.spec.md)。基线 `f6a389c`。

1. **P0 设计审计**：复核三个生产 finishWithGate 入口的 gate 所有权、原校验优先级、commitLocked revision/事务发布、共享夹具。保存原源码/既有测试摘要与代表编码；先修正文档中的不成立项，再实施。
2. **P1 实施 A**：只迁移 final 材料采样出 Engine.mu，保留 worker.gate；重锁复核 revision；finishContent 接收本次错误。复用大小判定，不引入第二套策略或持久缓存。
3. **P2 专项验证**：新增独立测试文件，复用 contentEngine；确定性阻塞读取验证无关 Run inspect/stop、同 Run gate、原 revision/来源变化、持久失败后重读、晚到 final，以及错误优先级。原共享夹具和测试保持字节不变。
4. **P3 审计收尾**：运行受影响内容/恢复/停止/R2 定向测试及 storage/host 回归，比较编码和未改源；独立记录 A 的结果和限制。A 通过后才开始 B 的代码，不将 B 依赖为 A 的验收条件。

执行后在本 Plan 记录实际命令、数量及失败，证据入口放独立 acceptance。禁止将未执行的完整测试或真机资格记为通过。

实际结果：新增4个顶层通过；内容/材料/原完成证明/停止/R2等40个顶层通过（含新增），storage/host35个顶层通过、1个原生opt-in跳过。103.132秒定向不代表完整application；351个既有内部测试文件未改，148565字节代表编码相同。B代码尚未修改时完成这些核验。

用户追加再审计及PR授权：生产实现保持，补充真实Handle(stop)排队/原提交边界回归；46项应用定向及74项组件通过，原编码、共享夹具及同EXE受控资格保持。具体范围和未重跑项见[再审计](../acceptance/content-inspection-reaudit.md)，按授权提交提PR。
