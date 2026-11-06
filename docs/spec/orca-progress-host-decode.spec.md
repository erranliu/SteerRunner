# Orca ProgressHost 结构化 profile 单次解码 Spec

2026-11-06，状态：已实施并完成 PH01–PH06 合并前专项验收及交付前审计，进入已授权的 PR 交付，未合入或部署，见[实际验收](../acceptance/orca-progress-host-decode.md)。调查基线为 `main=a12b24dc836218d65e1ae295fc958375c09773c1`（含 #121/#123/#124），实施刷新至 `6dd4360a42314e65b427627508ffd2ee3e4287da`（#125），ProgressHost 与两份解码器没有变化。配套 [Plan](orca-progress-host-decode.plan.md)。承接 [#121 冻结读取边界](https://github.com/erranliu/SteerRunner/blob/a12b24dc836218d65e1ae295fc958375c09773c1/docs/spec/orca-frozen-reader.spec.md)、[#113 文件连接绑定](https://github.com/erranliu/SteerRunner/blob/a12b24dc836218d65e1ae295fc958375c09773c1/docs/spec/orca-workspace-file-connection.spec.md)及 [#123 单次构造解码](https://github.com/erranliu/SteerRunner/blob/a12b24dc836218d65e1ae295fc958375c09773c1/docs/spec/orca-structured-profile-decode.spec.md)。历史验收不作为本候选通过证明。

## 目标与范围

`application.runtimeFile` 在进度或结果文件尚未绑定 HostID 时调用 `ProgressHost(progressSetup(r))`。调查基线中该函数先调用 `decodeLifecycleProfile`，结构化协议内部已完整调用 `decodeAgentSessionProfile`；配对远端再直接调用一次相同解码器，只为取得 PairingHostID。本机结构化成功路径只有一次，配对远端成功路径有两次；失败不一定执行到第二次。后续 fileSource 的独立校验不属于这个重复点。

只调整 `internal/execution/adapters/orcatransport/progress.go` 中 `ProgressHost` 的分支：结构化协议一次调用原完整 `decodeAgentSessionProfile`，校验成功后按其 Transport.Environment 返回本机 `local` 或原 PairingHostID；非结构化协议继续沿原 `decodeLifecycleProfile`、本机判断和远端 `native.UsageHostID`。函数签名、调用方和解码器本身不变。单次指一次完整解码器调用，不删除解码器内部子校验，不宣称未经测量的性能收益。

## 等价合同

| 边界 | 要求 |
| --- | --- |
| 严格校验与错误顺序 | 返回 host 之前完成原完整校验，保留缺失/重复封套、codec、归属/摘要、严格 JSON、Transport、host/target、agent/access、Audited/model/effort 与退役字段的原校验和首错。错误类型、分类及诊断原样返回，失败 host 仍为空；不能先因 environment 为空返回 local。只按原协议选择解码器，不新增 agent、ProcessHost 或运行状态前置检查，不将此函数的错误顺序改成 NewLifecycle/frozen 构造顺序。 |
| 返回身份 | 本机结构化仍返回字面量 `local`，不能返回配置中本机空 PairingHostID；配对远端返回严格校验后的原 PairingHostID，不读取当前配对替换它，不推导或规范化另一份身份。Codex/Pi、development/audited 与 codec 的原合法/非法组合不变。 |
| 旧 terminal | 支持的旧 terminal profile 继续按原解码器校验，本机返回 local；远端继续查询 `native.UsageHostID`，保留当前配对读取、公钥解码与错误传播。未知协议及旧坏配置继续按原首错失败，不为旧记录补结构化字段或启用旧执行。 |
| #121 读写边界 | `progressSetup` 只有冻结路由事实，不带 ProcessHost/执行 owner。不得复用 NewLifecycle、decodeFrozenRecord/newFrozenReader 来强加执行或 proof 构造依赖，不补 ProcessHost，不创建/停止会话或上传文件。每次调用校验本次输入，结构化 host 解析保持无 I/O、输入不变。 |
| #113 连接边界 | ProgressHost 仅给出原 host；OpenWorkspaceFiles 继续在每次打开远端文件连接时解析一次配对、比较原公钥指纹，并用同一个已校验配置拨号。该保证不是整个旧 terminal 调用链只读取一次配对。实际配对替换必须在原连接入口拒绝，替代端点零连接/零文件请求；本机原 runtime 校验保持。 |

不新增抽象、类型、接口、缓存、解码计数器、生产测试开关或持久字段；不扩展 #123 的 frozenRecord，也不跨调用复用上次结果。不改 ProgressSource/ResultSource/fileSource、连接与授权、usage、文件路径/字节上限、初始化/封存、停止与发布门禁。不修改 Orca、非 Orca Codex、公共 wire 或现有 workflow 准入。

## 验收要求

| 编号 | 通过条件 |
| --- | --- |
| PH01 | 以主线相同夹具对照本机 Codex execute/development/audited、本机 Pi、配对 Codex execute/development 的原合法代表值及相关 codec：host、错误和输入不变等价；ProcessHost=nil 仍可解析。结构化两端不依赖当前配对文件或原生调用。 |
| PH02 | 缺失、重复、非法 codec、归属/摘要、未知字段/坏 JSON/尾随、错 host/target、权限、非法 audited 与退役字段继续失败；选择可区分首错的多故障对照，比较原错误类型、完整诊断和有分类时的分类，不能只断言有错。原旧 terminal 本机/远端与未知协议也对照，旧远端配对读取及错误不得消失。 |
| PH03 | diff/调用图证明结构化 ProgressHost 成功路径只调用一次原完整解码器，原内部校验不动；fileSource 的独立核验仍在。#121 旧执行拒绝/冻结读取和 #113 配对替换、错误分类、一次解析及同配置拨号回归通过。 |
| PH04 | 同一候选源码/写入 EXE 在本机及已配对远端默认 Codex development/2 各完成一次真实进度和结果文件流程。活动中取得实际新进度字节，读取前后核对原 Run/turn 仍活动；result-file 此时 pending。等待输入后显式停止，原会话关闭后才取得 result-file/v1 closed/version=1 的实际结果字节，重复读取稳定，恢复授权撤销。不能用 execute/1（不接受 progress_file）或仅初始化种子替代此项。 |
| PH05 | 同一候选两个不同 EXE 身份对上述两端各自原目录，通过现行 SteerLine VerifyUpgradeDrained；原 Run/input 集、停止范围、无未决控制及 DB/shutdown 冷读前后摘要一致。不启动旧 terminal/helper 获取证明。停止额外资源清理仍未核实，不扩大 scope。 |
| PH06 | 独立验收记录列明基线/候选源码和 EXE 摘要、测试计数/失败/skip、双端原 Run 与文件/排空证据；先前 PR、定向组件及 Pi/audited 组件通过不追认为本轮真机或完整测试资格。环境阻断如实记录，不修改 Orca、不安装远端 Pi或放宽门禁。 |

实际测试入口与资格按[测试指南](../testing.md)。本轮仅取得上述专项资格，完整 L1/Controlled、完整 L2/L3 及 Pi/audited 真机未执行。
