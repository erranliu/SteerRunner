# 独立 parser 最小方案实施验收（2026-10-18）

**本期必要验证通过，RA-A01–A07 按最小边界完成。** [Spec](../../../spec/result-acquisition.spec.md) · [Plan](../../../spec/result-acquisition.plan.md) · [验收登记](../../result-acquisition.md)。SteerLine 未改，未安装或部署 SteerRunner。

## 实现及边界

- Runner 自身配置默认关闭 parser，仅代码审计结果整理按需启用；普通实施/发布零 parser 探测。独立新 exec、原生认证环境/已有 home 引用、独立模型、冻结材料/Schema；不占执行会话，不复制凭据或自行调用私有接口。
- Windows / Codex 0.159.2，固定 read-only/mxc/never 和只读 permission profile；禁用 shell、apps/plugins/hooks、搜索、子 agent 和继承的外部连接。公共 initialize/config/read 不创建 thread/turn，仅读取名称/来源投影，配置值不保存。MCP 空表深度合并不会清空连接，须逐项关闭非用户层继承的有效名称。
- 部分模型强制 Code Mode，保留所需运行时。提示词只约定整理行为，exec JSONL 只检测实际暴露的 item，不能证明空工具面、零隐式上下文、零工具调用或执行前拦截。本地 command/todo 违规须停止且原源有效才可单次后备；MCP/文件修改/未知工具事件保留副作用未核实，阻断补报和技术 resume。
- 原 Record/Intents/Commit/CAS 是唯一状态权威，attempt 只引用原 operation_id。主模型进程在 Job 身份提交后才恢复首条指令；重启不发输入，显式恢复先恢复原空闲执行资源，再按冻结的原 adapter 政策补报。原 final、真实审计/实施结果及 pre_head/post_head/PR 核验保持。

## 验证结果与批次关系

[验证汇总](verification-summary.json)、[实际计数](case-counts.json)、[源码/构建身份](source-identity.json)。源基线 f211e316、dirty；最终构建 **0.2.0-dev.2**，CLI v1 / workflow/2 / state/v3 / resource v2。最终构建、真实 native 所测构建、完整 Controlled 构建分别保存，不将修正前的构建当成最终源码。

| 验证 | 实际结果 | 证据 |
| --- | --- | --- |
| T1/T2/T3/T4/T5/T8 定向组 | 顶层匹配数分别 8/7/5/6/8/8，全通过；组间及与 L1 重叠，不相加为独立测试数 | [命令与退出码](directed-results.json)、t1.log–t5.log、t8.log |
| L1 | 247 顶层通过，6 个仓库显式允许的 helper/真机入口 skip；native driver 编译及零匹配防护通过 | [最后 actor 修正后的 L1](current-l1-summary.json)、[103 条 RA 通过记录（含子场景）](l1-ra-results.json) |
| 完整 Controlled | 13 顶层通过，其中结果获取 8 个内部 case；原预算 15 分钟，获取子场景等待 120 秒，其余 60 秒 | [结果](controlled/result.json)、[日志](controlled/integration.log) |
| 最后 actor 修正的真实进程回归 | 解析中停止/重启/显式恢复、工具后备两项通过；重启零自动输入，原会话最多一次补报；与上述 8 case 重叠 | [当前源码定向日志](supplemental/ra-parser-controlled-current.log)、[冻结政策回归](supplemental/ra-parser-frozen-policy.log) |
| 最终 owner 组件 | 5 顶层及其子场景通过，含 9 类 owned 进程输出、外部副作用未知阻断、OS 目录环境、身份提交失败、大转义材料回执恢复和超限拒收 | [最终组件日志](supplemental/ra-parser-final-components.log) |
| T7 真实原生 | 注册 case default 1 个，内部 3 项全部通过；零意外 skip | [原生结果](native/result.json)、[三项详情](native/parser-cases.json)、[完整日志](native/native.log) |
| 最终构建/静态检查 | 最终 EXE 和 live driver 编译成功；Go 格式、Markdown/JSON/链接、源码摘要、diff 与 SteerLine 检查 | [构建](build.json)、[格式](gofmt.json)、[最终检查](static-checks.json) |

完整入口 `.evidence/ra-parser-final-20261018` 的 L1/Controlled 均通过，原 native 权限反例因 MXC 平台目录环境缺失而失败；[原报告](test-summary.json) **仍为 failed**。之后没有重写该批：补齐 owner 的系统目录环境并通过组件回归，使用新现场 `C:/srt/780ef09fdcf1` 通过全部三项真实验证。最后只修正回执读取上限，组件回归及最终构建通过，原生参数/权限、actor/Workflow 均未改变；未重复模型或测试 PR。批次差异见 source-identity 和 case-counts。

当前副作用门槛的 L1/定向 Controlled 是对完整前置之后 actor 修正的补充；最终环境/回执改动只在 owner，沿 Plan 复用已通过前置并定向补验，不宣称某一次完整入口直接覆盖全部最终源码且全绿。全部自动条件的必要验证齐全，受控证据不冒充真实权限或模型资格。

## 真实三项及发布

1. `parser-default`：已有原生默认认证，不指定 model/home，完成审计材料整理，fixed_count=0/空列表和 done 契约通过，独立 session，原树停止。
2. `parser-boundaries`：引用同一已有 home，显式 `gpt-6.1-sol` 成功；污染材料仅整理，canary 不变。测试专用 shell 开启时实际写命令遭原生权限拒绝，canary 不变、树停止；生产 shell 仍关闭。另一解析在原生 turn.started 后取消，未接受结果且树停止。[权限反例](native/permission-probe.json)、[原始事件](native/permission-probe.jsonl)、[原生 profile](native/profile.json)。
3. `workflow-smoke`：development 真实实施/发布、程序结果/PR 身份核验、合入/merged/产物检查及停止通过；两个真实业务轮，零额外 report/parser。测试 PR [#441](https://github.com/erranliu/SteerlineTest/pull/441)，head `eccdd6ecdbd615e170f2602ee5eb8f49156963a7`，merge `8a78d93dce4634bf22d4ede4d4dc0f9cc4a05636`；Run `51fdcefe74942a09d9e04c47784a57b5` 与服务均确认停止。[场景](native/scenario.json)、[来源](native/result-provenance.json)、[停止](native/stop-facts.json)。

本次最终现场有 3 次生产 owner 的 exec 调用（默认、显式、取消）和 1 次测试权限反例；不等于 HTTP 请求数。原生未报告实际模型名，留空，不从 catalog 或模型自述推定；显式模型成功只证明本次账号可用。[usage](native/usage.json) 保存原生可观测 token；max_input_bytes 只约束 Runner 冻结包，不是原生隐式指令或 API 总 token 上限。原生认证/续期仍由 Codex 管理，未强行触发登录或续期验证。

## 失败及修正保留

- 2026-10-17 空工具/完整上下文强隔离阻断保留在[历史记录](../result-acquisition-20261017/README.md)，本轮不倒改。
- 早期 5 分钟直跑 acquisition 测试在第 7 个 parser-bad 内超时，前 6 项已通过；已按确切 EXE/实例停止唯一残留服务。[清理回执](iterations/early-timeout-cleanup.json)。后续使用仓库原有 15 分钟 Controlled 宿主预算；parser-tool 在发布中的 60 秒边界超时修为本组 120 秒，原失败见[初批报告](iterations/initial-summary.json)。
- MCP 空表深度合并问题发现后，第一完整编排在 native/PR 前主动停止，未当作完整通过。旧断言及夹具 Ref/Result/deadline 绑定错误按新契约修正，未放宽产品核验；原始日志留 `.evidence/ra-parser-*`。
- 真实默认模型要求 Code Mode 运行时，原配置在 turn 前失败；保留运行时而不提高命令权限。[原报告](iterations/runtime-failure-summary.json)、[原生事件](iterations/runtime-failure-events.jsonl)。
- 真实重启先发现原空闲执行会话未恢复，再发现补报政策依赖尚未重建的 port；现分别恢复原资源、冻结政策。[v1](iterations/restart-v1.log)、[v2](iterations/restart-v2.log)、[修复通过](supplemental/ra-parser-restart-controlled-v3.log)。
- 首次实际写入反例返回 MXC 平台目录缺失，**不是写入权限拒绝**。补齐 ProgramData/ProgramFiles/SystemDrive 等系统目录引用，业务凭据继续排除，新现场才取得有效拒绝证据。[原日志](iterations/platform-failure-native.log)、[原事件](iterations/platform-failure-permission-probe.jsonl)、[环境回归](supplemental/ra-parser-platform-env.log)。
- 最后发现 JSON/Base64 回执封装可超过旧 256 KiB 读取上限；上调至有界 2 MiB，原材料/输出上限不变。允许包内的大转义文本可恢复原回执，超限仍拒收；最终组件回归覆盖，未重发模型或变更原生权限。

原始现场保留 `.evidence/` 与上述 `C:/srt/`；本目录只归档非秘密配置引用、来源/操作/进程身份、结果、日志及摘要，不复制认证文件或 token。状态权威/故障与来源证据见 [state-authority](state-authority.json)、[fault-windows](fault-windows.json)、[定义轨迹](definition-traces.json)、[process-facts](process-facts.json)。SteerLine 的 HEAD/生命周期摘要与零写入记录在 source-identity，旧 state-dir/固定发行包限制仍按 Spec §8.1，本轮无部署。
