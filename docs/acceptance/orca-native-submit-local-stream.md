# Orca 本机准备原始流补验

2026-10-30。用户明确授权最小读取修正；关联 [Spec](../spec/orca-native-submit.spec.md)、[Plan](../spec/orca-native-submit.plan.md) 和[授权前失败记录](orca-native-submit-local.md)。两处读取修正已完成，**本机真实业务输入资格仍为 0/3**。

## 修改与审计

仅 `controlLines`、`readLocalCodexReadiness` 的生产读取显式增加公开 `--cursor 0`，保留原 `--limit 1000`。要求 `source=stream`、`oldestCursor=0`，`truncated/limited` 明确为 false；来源/完整性缺失或受限时拒收，空完整流仍沿原 pending 语义。保留原 runtime/handle/CallUncertain、回执解码、身份及绑定核验。

没有拼接屏幕行、分页器、cursor 状态、默认读取回退或输入重试。提交、模型、history/原输入、完成和停止算法均未调整。新增 `TestLocalPreparationOriginalStream` 的 32 个叶例纳入现有正式 controlled 脚本，验证两处完整/空流、screen/缺字段、截断/输出受限/起点丢失、错误身份/编码/绑定及零额外创建/发送。

design_audit、final_review 独立只读核对生产增量与测试，无新增 P0–P2。公开 CLI 的 cursor 参数可绕过 screen 投影，生产代码不调用安装包内部接口，不依赖实验能力。

## 稳定回归

正式入口 `scripts/test-native-input.ps1 -Mode controlled -Repeat 30`：15 个必需 root、162 个叶例各 30 次，**4860 执行、4860 通过、0 失败、0 跳过**，源码冻结和零漏选门禁通过。独立复审重新统计原始 JSONL，数字一致。

项目 L1 在本次最终生产/测试源码上执行：714 顶层通过（含子例 2651 通过）、0 失败、17 个明确 opt-in/helper 跳过；34 包通过。框架门禁、29 项终端扩展、原生 driver 编译及零匹配门禁通过。跳过不算真实资格，全部原事件与独立统计归档。

## 全新现场结果

隔离构建现有 `cmd/steerrunner-agent`，SHA256 `1320bf201492994da04991c0118549f0648b99d6ca1df99d966d6b72903cf7e9`，工具链 Go 1.26.4。Orca 1.4.242，原 runtime `4f073b25-6af0-4b4b-bcb8-d2174d704504`。新建 `codex-sr-native-stream-20261030-01` 测试 worktree，setup skip；未安装、部署或修改全局配置。

原 Run `c72f3ac4d98cd25ad198663702db090e`，Attempt `7ee76f2e20f8cf441afd9614b414dd9e`，主 terminal `term_5a553813-c5f7-4b68-b93f-151690802984`、incarnation 及 control 身份均在原日志保存。业务正文预先冻结 4898 UTF-8 字节，包含中文、CRLF、空行和长输入；**实际业务提交 0 次**，尚无业务 request 受理/开始/完成证据。

`actual-cli-012.json` 已返回完整单行 `source=stream` 的控制 READY，原严格 binding 校验通过，随后进入原 agent launch。新阻断是 `remote_agent_preparation_failed:authentication_api_socket_unverified`，不是旧屏幕折行拒收。

源码定位：原 `codex/authentication_broker.go` 中 `net.Listen("unix", socket)` 已成功，紧接的 `lease.root.Lstat("s")` 失败。Codex app-server 尚未启动，业务正文尚未发送。认证生产代码没有修改；不通过改空 RepositoryURL、替换 runtime 根、切换权限或注入测试回调绕过。

## 目录条件诊断与边界

无凭据独立 UDS 探针使用同工具链：

| 条件 | net.Listen | Root.Lstat / os.Lstat | 自身精确清理 |
| --- | --- | --- | --- |
| 宿主 TEMP 新目录 | 成功 | 成功 | 成功 |
| Orca 新诊断 shell 的 TEMP 新目录 | 成功 | 成功 | 成功 |
| 宿主 LOCALAPPDATA/SteerRunner/a 新目录 | 成功 | 两者均 Windows 1920：系统无法访问此文件 | 原地删除失败，移至全新 TEMP 后删除成功 |

对原失败 socket 的只读检查也返回 1920；同机 L1 的原 `TestAuthenticationSocketLayoutAndIdentity` 在 TEMP 根通过。现有证据支持“与当前运行目录条件相关”，不能断言 Windows/Go 普遍不支持 UDS、Orca 能力缺口、安全软件或具体权限根因。目录大小写属性均未启用，未修改 ACL、系统设置或安装环境。

当前默认运行目录尚不具备本机准备条件，未继续创建两个同前置条件的失败现场充数。没有业务 send、受理、开始、完成、正式原文/final 或正式停止证明，不能将 READY 恢复、shell 探针或 4860 次受控通过冒称三个真实成功会话。远端沿此前未验证状态。

## 清理与证据

失败现场使用原身份紧急关闭主/control，完整原 local-host census 为 0；自有诊断 shell 也精确关闭。核实测试 worktree Git 干净后精确删除，路径及 Git 注册均不存在。

两处本次 socket（失败 agent 及无凭据探针）原地删除/清理 reparse 标记均返回 1920。核实各目录仅有自有已关闭的 `s` 文件、校验源与全新 TEMP 目标绝对路径后，逐个移动并精确删除文件/空目录，源与目标全部不存在。原失败 runtime 仅有本次生成的 configuration.json；核对记录摘要且无其它内容后精确清理，runtime 目录已不存在。这是失败现场清理，**不是正式 stop/authentication 资格通过**。

原 `.git/info/exclude` 前后摘要一致，无本次全局配置修改。自动审批拒绝复制运行时完整配置正文；改为仅归档清单、时间和摘要，没有复制该文件正文或请求放宽审批。所有原始失败日志保留。

可审计证据见 [evidence/orca-native-submit-local-stream-20261030](evidence/orca-native-submit-local-stream-20261030/manifest.json)，含原 CLI/intent/receipt、构建摘要、矩阵逐例统计、L1、无凭据诊断源码/输出、清理与复审。没有部署、安装、重投历史 Run、补 Enter、执行 Unity/业务环境检测或修改 SteerLine。
