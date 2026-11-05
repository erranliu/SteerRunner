# Plan：离线 inspect 与执行生命周期解耦（B）

日期：2026-11-05。状态：P0–P3完成，见[验收](../acceptance/content-inspection-boundaries.md)。依据 [Spec](offline-inspection-boundary.spec.md)，基线 `f6a389c`。

1. **P0 设计审计**：核对 CLI/host 三 selectors、原错误顺序、Runs 全量严格解码、Command 的 null/错误、resident 锁范围和真实 New/nil launcher 消费者。与 A 文档先一起审计；A 验证完成后实施 B。
2. **P1 实施 B**：新增 application 持久读取函数，复用窄查询/投影；resident 原锁内调用共享逻辑。host 只替换构造/关闭片段，保留所有入口和读取护栏。
3. **P2 专项验证**：新独立 host/application 测试覆盖三 selectors、command-only/dangling Run、not_found、坏无关 Run、DB 查询故障、读取副作用，比较 resident/offline 响应；保持原测试/共享夹具字节。
4. **P3 收尾**：运行 host/cli/storage/contracts 及 A/B 受影响定向回归，比较持久声明/代表编码；正式构建及公开受控 ownership、原未知/stop、原授权重启。记录具体资格，不扩为完整 L1/L2/L3。更新长期实现说明及独立 acceptance，复核 A/B diff 和未授权改动。

实际结果：新增B三顶层通过，原host覆盖实现也通过同组契约断言；最终四包70顶层通过、4个原生现场opt-in跳过，A/B执行/结果/停止16定向通过。148565字节代表编码一致，351个既有内部测试字节未改。共享StagedBusiness只追加主线新提示词识别，独立新测试先复现失败，再与新旧夹具4顶层通过；SQLite临时协调文件及初轮失败边界见验收。

正式构建后，同一最终EXE的3项公开受控通过，重启全部4case通过。前两轮旧夹具失败/中止不计通过，相关原Run均经公开stop及service verify-stopped核实结算。未运行完整L1/L2/L3或真实模型；改动保留在工作分支供审阅。

后续用户授权再审计及PR：未确认生产缺陷，读取/错误语义与共享夹具、持久格式保持；74个组件顶层及46个应用定向通过。三项受控沿已确认相同Runner/driver/fixture的初轮证据，不虚记重跑。见[再审计](../acceptance/content-inspection-reaudit.md)；按本次授权提交提PR。
