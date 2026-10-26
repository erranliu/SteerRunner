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

## 新候选 CRLF paste 发现

原代 `8e1ce9f` / `30b3bc...` 的 candidate-01 真实执行八项测试，公开进度已为 8/8，但实施结束后仍未进入发布，不能计通过。独立只读查询证明完整元数据资格及 terminal-fenced 唯一输入查询本身均成功，私有 baseline 未被读取或供给。这次问题不是忽略 working 的资格分支本身。

公开冻结 prompt、原材料路径及源码定义的完成标记/进度后缀可以重建 9498 字节逻辑提示，其中包含内联 Windows writer 的 57 个 CRLF。原 native transcript 为 9497 字节，逐字节等于把每个 CR 换成 LF 后移除结尾换行的 paste 结果。现有归属匹配要求 CRLF→单 LF，对应 9440 字节，两者正好多出 57 个换行；因此拒绝全文匹配符合原守卫。证据 `candidate-01/diagnostics/native-attribution-20261026T1125/exact-wire-reconstruction.json`，真实 native 输入 SHA256 为 `8b565aeffdfde180f09c72649a53f1ec164b411f530ffe41066ae88acd4092cc`。

修正在原发送前编译 CRLF→LF，并与原匹配路径共享函数，不折叠事后 transcript 空白或改写历史。新回归从实际捕获的唯一发送构造 ConPTY paste 结果，再走原 IPC 和 lifecycle；旧实现确实多出空行而失败。首版 observed fixture 使用了不同 incarnation，被原身份守卫正确拒绝；修正夹具同源 incarnation 后正例通过，产品守卫未放宽，原失败保留。

该追加修复尚需新一轮完整 owner/L1/干净构建登记；此前用户对 `30b3bc...` 的批准不自动覆盖新二进制。candidate-01 原报告/进度及 SDK 环境变化证据保留，完整复验仍为 0，不声称发布或 clean-head / 完整业务通过。


candidate-01 已完成原 Run stop、宿主 stop 和 public offline 核验：resources_settled=true，完整精确 owned native census0 / truncated=false / omittedHosts 为空，本地 home/bin 进程0。原八项报告、XML、输入与清理只读核验通过，进度 version33 恰好八个 true、302字节，SHA256 `0a1b323fd2072babff36eb71e998194fa8162cd564be764ed9c034c00287a5fe`，API/UI8/8；没有真实 PR，故完整连续资格仍为0。证据为独立 scope 的 `candidate-01/verification-20261026T1150-reports/`、`verification-20261026T1155-inputs/` 和 `cleanup-blocked-20261026T1200/`。

首版换行修正放在通用 wire 分支，完整 owner 检查发现 Pi 的原历史 wire 断言受影响。后续将编译精确限制于 Codex native 唯一发送，Pi 原断言保持不变；两代完整 owner 已通过，正式 L1/构建02仍在核验。前次失败日志保留，不用定向通过替代完整资格。原代与主线新增四文件的 aggregate stable patch-id 均为 `f1a4a919f4cd29a7e1366d44f26e8b0ab38ce409`。

原 worker 在本轮另行临时安装 SDK8.0.427，而机器原有 SDK8.0.426 确已存在。该环境变化单独保留、不追认不变环境资格。拟议独立启动器只向子进程 PATH 加入现有 SDK8.0.426，沿原 Unity 与 native 字节；不安装 SDK、不改全局/原 home/profile。新 Runner 与此环境例外尚未获得未来新轮批准；不存在热替换或新业务输入。


正式验证02已完成：主线干净受测源 `20d3bd98e40797b0dc70b8edaf38fa2ef560e11d` L1 703 顶层+29扩展通过，原代干净受测源 `7c0df6cafb050f294e7066dfdeb896424fe78230` L1 676顶层+29扩展通过；两个完整 owner、framework、native compile、零匹配守卫通过。原代干净构建 EXE SHA256 `00051e332f19805db1f814069c98da48be7411b8b2799955283bc3ee81edca0d`，保存在 `runner-wire-build-02/`。main-build02 时本验收文档补录已进入工作区，因此构建元数据 source_dirty=true；如实保留，不冒称干净 main 构建。主线后续纯文档提交后的干净 build03 单独保存来源，L1 受测产品代码仍为20d。

拟议 SDK 启动器 qualification03 及无模型原生 initialize/config/read/command/exec 探针通过，真实解析现有 `C:/Users/Administrator/.dotnet/dotnet.exe` 8.0.426 与要求的 Unity 路径，父环境未变、无 thread/turn、原生进程和诊断终端已停止。启动器 SHA256 `061b713d2c65d35bb9005ac10bcf21c8f73b83ec867f22a3d9dee21c869182c3`，默认 Defender 开启、定向扫描无检测。前两次资格在编译前因 PS7 不支持目标 ConsoleApplication、以及本地 CRLF 文件与原 LF payload 的 SHA 区别失败，均保留；未覆盖已有二进制或绕过防护。原代新 Runner 与该子环境提案仍须未来新轮明确批准，连续真实完整复验仍为0/2。
