# 候选必读输入文件

开发和单次执行的 `result_file.required_input: true` 将 `initial_content_base64` 声明为不可更改的本轮输入。能力标记为 `result_file.required_input = verify_exact_before_dispatch/v1`。Runner 沿用原 Run 的工作区文件绑定与初始化，在依赖此文件的实施、审计和单次执行派发前读取原位置并比较已接受字节。失败阻断派发。恢复使用原请求和原文件，`result-file` 查询将此模式报告为 `disabled`。

- 受控派发覆盖任务 101 五项原始字节、文件缺失、初始化后变更。匹配时发送一个业务回合，缺失或变更时发送零回合。
- Windows 本机实际 Runner EXE 配合原生协议夹具，分别通过任务 101 必读输入与原结果输出两种公开模式验证。工作区中的安排原始字节一致，任务和服务停止已核验。
- 验证使用 SteerLine 隔离现场。证据位于 `C:/SteerlineDebug/native-101-frozen-input-20261104/diagnostics/`。覆盖范围为受控本机和文件派发门禁，Orca 真机留待后续验证。

## 审计修复与复验

- 必读安排和进度文件各有 4 秒初始化窗口。任务 101 的五项安排与对应五个进度字段分别模拟 2.5 秒、2 秒初始化，两份文件都就绪后核对派发。普通结果输出和进度继续共用原 4 秒窗口。
- 审计前必读文件读取超时或字节不符进入 `required_input_unverified`，保留已验收的实施结果与原审计指令。显式恢复后从原位置重新读取并比对请求字节，只有匹配才派发一次原审计。首次初始化回读暂时不可用时也允许在原绑定上重新读验。
- 本轮验证使用 `C:/SteerlineDebug/native-101-runner99-audit-fix-20261104/diagnostics/`。任务 101 两条新回归通过，见 `runner-native101-focused.log`。公开 EXE 复验使用本机受控原生协议夹具，任务 101 的原顺序文件、普通结果输出和停止结算均通过，见 `runner-public-101.log`。被测 Runner SHA-256 为 `eb6688629830a5d66ec06d21ea69eae17b84a54e5eeb977e26b6e39db99a2007`。
- Runner 既有文件预算、停止、结果封存、审计和发布门禁定向回归通过，见 `runner-native101-final.log`。contracts 和 resources 包测试通过，见 `runner-contract-resources.log`。
