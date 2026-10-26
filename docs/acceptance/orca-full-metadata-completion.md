# Orca 完整元数据补证验收

2026-10-26；[Spec](../spec/orca-full-metadata-completion.spec.md) / [Plan](../spec/orca-full-metadata-completion.plan.md)。

Task18 进度修复现场的原 Run `3808918f4de62554116d03ce354545d4`、Attempt `3e8a3e01bdba5f013a348fe3cabd09c7` 已公开展示 6/6，但固定 f3 Runner 仍在 implement.execute/observation_unverified，诊断“Native original input preview is missing; awaiting original attribution evidence”。原生只读查询显示同 pane/provider 主 agent 已 done，两个投影主记录一致，完整 hook 元数据可用，ThinMetadata=false。不能据屏幕或进度转为发布通过。

只读证据根：`C:/SteerlineDebug/wt-507e-progress-0of6/live-01/diagnostics/native-attribution-20261026T0834/`、`native-session-20261026T0840/`。原生 transcript 公共 API 完整分页得到两条 user 记录，分别是原生初始化 READY 和唯一业务输入，不是两条 Caller 业务输入。用公开 Run 创建时间作诊断边界会包含 READY；该边界明确不是 Runner 的私有 baseline，也从未写回或供应给运行中的 Runner。

新修复只扩展严格现有 query proof 的元数据资格。基于原 f3 的独立 clone，旧/new 使用同一 IPC lifecycle 回归；新测试包括资源身份、正文/marker、authority/revision、重复/截断输入、观察器重建及不可序列化/篡改证明。原 Run、旧 f3 字节、home/pin 和私有数据库保持原样。

本文件当前是候选修复验收安排。正式 L1、构建 SHA、Controlled 与真实两轮结果尚待登记；没有把原 6/6 进度或模型自述移记为链路成功。原固定 f3 验收要求若需变更，只在明确授权后的未来新轮配置新二进制，不热替换当前候选。
