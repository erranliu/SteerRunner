# Orca 完整元数据补证验收

2026-10-26；[Spec](../spec/orca-full-metadata-completion.spec.md) / [Plan](../spec/orca-full-metadata-completion.plan.md)。

Task18 进度修复现场的原 Run `3808918f4de62554116d03ce354545d4`、Attempt `3e8a3e01bdba5f013a348fe3cabd09c7` 已公开展示 6/6，但固定 f3 Runner 仍在 implement.execute/observation_unverified，诊断“Native original input preview is missing; awaiting original attribution evidence”。原生只读查询显示同 pane/provider 主 agent 已 done，两个投影主记录一致，完整 hook 元数据可用，ThinMetadata=false。不能据屏幕或进度转为发布通过。

只读证据根：`C:/SteerlineDebug/wt-507e-progress-0of6/live-01/diagnostics/native-attribution-20261026T0834/`、`native-session-20261026T0840/`。原生 transcript 公共 API 完整分页得到两条 user 记录，分别是原生初始化 READY 和唯一业务输入，不是两条 Caller 业务输入。用公开 Run 创建时间作诊断边界会包含 READY；该边界明确不是 Runner 的私有 baseline，也从未写回或供应给运行中的 Runner。

新修复只扩展严格现有 query proof 的元数据资格。基于原 f3 的独立 clone，旧/new 使用同一 IPC lifecycle 回归；新测试包括资源身份、正文/marker、authority/revision、重复/截断输入、观察器重建及不可序列化/篡改证明。原 Run、旧 f3 字节、home/pin 和私有数据库保持原样。

原现场已按原 Run 的公开 stop、宿主 stop 和 offline verify-stopped 完成清理：resources_settled=true，完整精确 owned native terminal census=0、truncated=false、omittedHosts 为空，本地 home/bin 进程=0。原工作区、失败和成功测试报告、进度及唯一业务输入回执全部保留。没有发布 PR，因此完整复验仍为 0/2。证据为独立 scope 的 `live-01/cleanup-blocked-20261026T0920/result.json` 与 `chain-result.json`。

## 已完成的自动验证

- 基于原 f3 的普通独立 clone，仅加入同一 lifecycle 测试，missed / missed-restored 两例均复现缺少原输入 preview，零输入重发；原失败日志保留。
- 原 f3 验证代源码 `8e1ce9f969c23ff011fd6d057f6c75c44ad2a335` 的两个 Orca owner 全包通过；正式 L1 通过 675 个顶层测试及 29 个 terminal 扩展，framework、native 编译与零匹配守卫通过。
- 主线代源码 `8eda1434be36671301eac91a346822e653b53f83` 的两个 owner 全包通过（24.9 / 86.5 秒）；正式 L1 通过 702 个顶层测试及 29 个扩展，以上守卫全部通过。两个正式入口仅跳过 catalog 允许的显式 opt-in / helper，未把跳过计为现场通过。
- 原代与主线代的本次修复 stable patch-id 均为 `6c2ff4585680000f87e68df9fef2686c980331b4`；原代没有回退或修改主线 result-file / 环境准入。真实旧项目复验不代表主线新增准入已获资格。

两个 EXE 均来自各自干净的已提交源码。原代候选 SHA256 为 `30b3bc832a123427b9494675b242351df975e99994e5688121ef185b4055355c`；主线代为 `f3a66f1ccbb8232e6f10c06605629fa8be2d4e49775a80e6038a4ee6da480c6a`。源码身份、构建元数据、报告及 stdout 保存在 `C:/SteerlineDebug/wt-507e-progress-0of6/runner-full-metadata-validation/` 和对应 `runner-full-metadata[-main]-build-01/`。本次文档补录在构建之后，不把后续文档提交冒充已构建来源。

WMI 隐藏独立验证 driver 只降低自身 token 中四项原普通调用方禁用的 privilege，broker preflight 实际通过；没有启用权限、更改系统或防护。完整 Controlled / 原生 L2 及修复后真实链路尚未登记通过。

## 后续真实资格

用户已明确回复“允许新轮次使用该候选 Runner（推荐）”，仅批准未来全新轮次使用原代 `8e1ce9f` / 上述 `30b3bc...` EXE。授权另存 `runner-pin-authorization-20261026T0930.json`，原 f3 Run/home/pin 不变；模型、独立 Unity 环境绑定、业务合同、原 base 与必跑测试保持不变。

后续使用新普通 clone/home、原测试入口及原始报告，逐轮核实原 Run/Attempt、唯一输入、真实发布、原进度布尔集合及 UI、一致的原报告与清理，连续两次全部合格才完成。原 6/6、模型自述或组件回归不能代替这两次。原报告的 dirty 代码身份差异、clean published-head 和完整 MagicPotion / 人工业务未验边界继续保留。
