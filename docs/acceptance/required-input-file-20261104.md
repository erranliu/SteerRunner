# 候选必读输入文件

开发和单次执行的 `result_file.required_input: true` 将 `initial_content_base64` 声明为不可更改的本轮输入。能力标记为 `result_file.required_input = verify_exact_before_dispatch/v1`。Runner 沿用原 Run 的工作区文件绑定与初始化，在依赖此文件的实施、审计和单次执行派发前读取原位置并比较已接受字节。失败阻断派发。恢复使用原请求和原文件，`result-file` 查询将此模式报告为 `disabled`。

- 受控派发覆盖任务 101 五项原始字节、文件缺失、初始化后变更。匹配时发送一个业务回合，缺失或变更时发送零回合。
- Windows 本机实际 Runner EXE 配合原生协议夹具，分别通过任务 101 必读输入与原结果输出两种公开模式验证。工作区中的安排原始字节一致，任务和服务停止已核验。
- 验证使用 SteerLine 隔离现场。证据位于 `C:/SteerlineDebug/native-101-frozen-input-20261104/diagnostics/`。覆盖范围为受控本机和文件派发门禁，Orca 真机留待后续验证。
