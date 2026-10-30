# Orca 三个本机原生会话最终补验

2026-10-30。本机 **3/3 通过**；本页批次结束时远端仍为 0/3、PR 为 Draft，随后用户指定新目标的[远端补验](orca-native-submit-remote-final.md)已达 3/3。配套 [Spec](../spec/orca-native-submit.spec.md)、[Plan](../spec/orca-native-submit.plan.md)、[socket 根因与环境恢复](orca-auth-socket-rootcause.md)。本页不改写此前失败记录。

## 本轮最小修正

Windows 1920 的恢复没有修改 Runner 认证、凭据、权限、默认运行目录或 Orca 安装。仅在用户明确允许本机重启后，正常退出 GUI、精确停止被新 GUI 继续复用的旧本机 PTY host，再从正常 Windows 用户上下文启动同一 Orca 1.4.242。新实际 PTY 子进程的原目录 socket、连接和清理门禁全部通过，见根因记录。

补验随后发现测试目标缺少正式工作流本来会一同冻结的 AuditModel：probe 已设置 Turn.Model/StepImplement，却允许 Launch.AuditModel=nil。生产冻结不会产生该组合，旧 helper 因未启用既有模型凭据而返回缺少 Model/Effort 的历史记录，原校验正确拒收。

只修测试适配：在创建资源前拒收缺失/空/同 writer 名称/非法 effort 或名称的 audit 配置，并保留原 writer/StepImplement。本次目标 writer 为 `gpt-6-luna/high`，既有 audit 配置为不同名称的 `gpt-6.1-sol/high`；没有投递 audit 任务。原模型、完成和认证生产代码均未改。另修正签名退出证据采集的工作区绝对路径。

## 三份真实凭据

三份均通过现有 `scripts/test-native-input.ps1 -Mode long-input` 及原本机 v3 control/ready/model/history/stop/authentication 流程。使用同一个隔离构建的现有 agent，SHA256 `1320bf201492994da04991c0118549f0648b99d6ca1df99d966d6b72903cf7e9`；runtime `92c410b8-3132-4c48-a07f-2888faa81c53`。未安装该构建。

| 现场 | 原 Run | 原请求 | 实际业务提交 | 完成/正式停止 |
| --- | --- | --- | --- | --- |
| qualified-01 | `7a6a76e7afd9a61b287549b06afa10ce` | `ee640d23-0f4d-bff5-58a8-9e65af2df9c6` | 1 | 通过/confirmed |
| qualified-02 | `4471fc5bf4581b7eb8092071e24437fc` | `653ce0eb-804a-3bf4-28e6-faa45b111737` | 1 | 通过/confirmed |
| qualified-03 | `42720ff7930aab91a6509514e3d02adc` | `2f0ecf75-ae53-e70a-f127-9b43be83821b` | 1 | 通过/confirmed |

每份独立 Run/Attempt、terminal/incarnation、provider session、native turn/user item、请求及输入摘要均不同。实际测试耗时依次为 78.43、75.03、90.30 秒，不以旧快照重复代替真实会话。

每份冻结正文 4898 UTF-8 字节，含多行、中文、CRLF 和空行；沿原规则编译为 5093 字节模型 wire。实际 CLI `--text` 与冻结 wire 逐字节一致，正式唯一原输入摘要匹配。原请求回执具有 provider=codex、observation=supported、原 runtime/incarnation、input_accepted 和 turn_started；独立历史再确认 started/completed、原模型/effort及唯一 nonce final。

业务阶段全部为一次通用 text+enter、0 text-only、0 bare Enter、0 composer read、0 错误 request。设置/control 查询单独计数，不冒充业务输入；一次 RPC 不推断 Orca 内部按键/held-Enter 次数。

停止由原 StopLifecycle/verifyStopped 验证；认证撤销、broker/socket 清理、原 control 关闭和原 local-host 完整空清单均通过。原签名退出文件使用保存的原 Ready 公钥独立验证 Ed25519 签名、Ready 全字段、writers_absent/authentication_closed及时间。第一份原报告缺少可选 signed_exit 字段，补存原工作区公开签名文件单独验证；没有改写该原 JSON。后两份已由修正后的采集路径直接记录。原始请求、源码/测试EXE/CLI摘要及全部凭据见[三会话摘要](evidence/orca-local-final-20261030/three-sessions-summary.json)与各会话目录。

## 稳定回归与项目检查

- 最终正式矩阵：15 个必需 root、168 叶例各 30 次，**5040 执行、5040 通过、0 失败、0 跳过**，源码冻结和零漏选门禁通过。新增 6 个目标配置反例随原 `TestLocalNativeProbeTargetAndBusinessScope` 运行。
- 项目 L1：**714 顶层通过**，含子例 2657 通过，0 失败、17 个明确 opt-in/helper 跳过，34 包通过；框架、29 项终端扩展、原生 driver 编译及零匹配门禁通过。跳过不算真实资格。
- L1 在配置门禁补充后启动，期间唯一后续源码增量是签名证据读取的工作区绝对路径；该采集增量由最终 5040 次矩阵及后两份真实会话验证。第一份与后两份测试 EXE/源码摘要分别保存，不冒称同一测试编译快照；agent 及生产流程构建一致。

## 失败边界与清理

`session-normal-01` 保持失败：一次原业务请求已被受理/开始并返回 nonce，但缺少正式模型凭据，原校验拒绝归属并超时；正式 stop 因 reader settlement 未核实而失败，随后仅做自有资源紧急清理。补存的原签名退出可验证 writers/authentication 已关闭，仍不改判正式资格或正式 stop 通过。后续三份是全新 Run/输入/工作区，不重投该 Run，不补 Enter。

全部五个本次工作区在原 local-host 空清单和 Git 干净核验后精确删除，路径及 Git 注册消失。四个本次 agent/server 均已消失；本次运行目录仅对已知普通文件记录元数据/SHA后精确清理，不复制配置正文。三份正式认证 socket 清理通过，失败现场 socket 亦消失。对本轮及根因诊断合计 54 条原路径、包私有实际路径和临时目标，当前与正常用户两个视图均核实不存在。原 `.git/info/exclude` 前后摘要相同。

未修改全局配置、安装包、业务材料或 SteerLine；未部署、运行 Unity/环境检测业务、创建业务 PR或操作远端。历史输入故障的原始 dirty 源码仍未恢复，不能将本次通过宣称为历史故障完整复现后的修复证明；通用接口的原子消费/任意人工写者排他边界保持 Spec 原表述。

原始记录、逐叶次数、清理及独立复审见 [证据清单](evidence/orca-local-final-20261030/manifest.json)。
