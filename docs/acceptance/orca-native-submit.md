# Orca 原生提交调查与验收

日期：2026-10-30。Runner 基线 `a086a9984dc0bd32b3f02b9698cd26c9ba313d90`。
[Spec](../spec/orca-native-submit.spec.md) / [Plan](../spec/orca-native-submit.plan.md)。

## 当前结论

**尚未实施 Runner 修复；本段结论已按[重新核查](orca-native-submit-reassessment.md)修正。** 已证实 Runner 多行 raw paste/Enter 绕开 Orca 通用 prompt 提交。Orca 已有同请求去重、开始观察和只读 `orchestration request-show`；先前漏查后者。内部计时/草稿/held-Enter机制及缺消费摘要不能单独证明 Orca 违反既有契约或解释历史故障。原始正文消费/排他强目标尚未获证，但不能把它泛化为“必须先改 Orca 才能处理回执恢复”。

当前所有生产代码、配置、测试路径和历史 evidence 保持；屏幕依赖删除数为 0。不存在新旧执行路径并存的新增实现。设计审计、隔离契约行为与现有回归的结果分别记录，不能代替修复验收。

## 调查来源

- 当前 checkout 的五个指定文件，以及 `local_codex_history.go`、state codec、正式测试目录/脚本。
- `orca terminal send --help`、当前 CLI handler/schema、安装包 `resources/orcad-template/orcad-server.js`。只读文件；没有修改安装。
- 本机只读 status：runtime `c1f3d7cb-9b38-4074-b7f6-2a04e127d720`，`1.4.242`，`terminal.prompt-delivery.v1`。
- 配对远端只读 status：runtime `64bdec68-d3ed-4216-a894-0cc143ed4653`，`1.4.242`，`terminal.prompt-delivery.v1`。没有读取/重建/投递历史终端。
- 原始 `C:\SteerlineDebug\wt-5069-native-input-repro\diagnostics\reproduction-summary-20261030.json`：3 个独立原生 Run 均开始、均检测到 TTY 错误、负报告和停止清理正常；持续受理未知未复现，正式 L4 未通过。旧草稿重放缺原 dirty 源码及原始冻结输入，不是完整历史复现。

本机 status 初次在 sandbox 内返回 `runtime_access_denied/EPERM`；随后以获准的 sandbox 外只读调用成功。没有重启 Orca、执行 `open`、安装或更改用户配置。CLI 的只读目标访问可能更新既有 pairing store 的 last-used 元数据；没有主动设置配置、配对或凭据。

## 隔离契约行为核验

[只读核验脚本](evidence/orca-native-submit-20261030/audit-installed-contract.cjs) 从指定本机安装包提取五个原函数，不 import 或启动 Orca；使用虚拟计时、PTY sink、activity fixture 和明确列出的依赖替换。保存安装 bundle SHA、函数 UTF-16 offset/摘要；不发布整个安装包。

最终 [核验结果](evidence/orca-native-submit-20261030/installed-contract-final.json)：8 个不同反例，每例30次，共 **240 次，失败0，跳过0**。断言观察到契约不足即通过核验，不代表输入修复通过：

- 无任何输出或消费 ACK，原 render gate 由虚拟截止计时释放。
- 原 write 函数在消费为空、截断、改字节、已有输入或并发混入的隔离 sink 情况下仍写 CR 并给 input_accepted；这里替换了 transport/provider，不声称真实 provider 已产生这些故障。
- 原 held retry 函数根据 screen draft 再写1个 CR。
- 原 start observer 无需输入正文或摘要即可按 activity sequence 接受开始。

首轮 [夹具失败记录](evidence/orca-native-submit-20261030/installed-contract.json) 原样保留：240次中150失败、0跳过。原因是注入的外部 Error 与 VM Error 不同 realm，使原函数的 `instanceof Error` 无法识别模拟 stalled；修正注入同一 Error 构造器后另开结果文件。首轮不是 Orca 真实故障或 Runner 测试失败；没有改写为通过。最早一次核验还因输出父目录不存在未保存结果；没有原生副作用。

中间 [Error 修正批次](evidence/orca-native-submit-20261030/installed-contract-corrected.json) 240/0/0 保留。最终批次进一步让虚拟 gate 按实际调用顺序 arm → payload → wait → CR，并记录核验脚本摘要；仍为240/0/0。三个已保存批次共720次、150个夹具断言失败、0跳过；覆盖始终只有8类，不将累计次数当作新增覆盖。

范围：这是隔离安装函数的契约反例核验，**不是正式 L3 稳定回归、真实原生故障复现或远端 bundle 测试**。未执行 Spec 的 NS-T01–T10 新方案矩阵，不能用这240次替代。

## 独立方案审计

以下是首轮审计记录；首轮主会话及审计者均漏查 request-show，随后独立重新核查已纠正，不再将原审计通过视为“Orca 已确认存在历史缺陷”的依据。原始证据不改字节，当前归因以重新核查为准。

独立只读 agent `design_audit` 对照当前 Runner 与已安装 Orca 实现核实上述上游缺口。首轮发现1项P2：本机 `native-command/model` wire 与 `observeRecorded` 可能使“同一流程”仅停留在 submit 函数。已在 Spec §1/§6 与 Plan P3 明确迁入共同原输入/final链路，并保留准备、认证、模型及停止保证；不能保留本机完成旁路。

同时采纳计数澄清：NS-T06 原业务请求尝试1次，实际 commit/执行按故障位置为0或1，不要求消费前失败也提交。独立审计者复核修订后通过，无新增P0–P2；仅方案通过，Orca gate、实施和真实验收仍未完成。

## 最终独立复审

新的独立只读 agent `final_review` 核实提交 `203af58` 的 diff、安装 bundle SHA、五个函数偏移/摘要、脚本依赖替换、计数和删除范围，无P0–P2。它确认这是契约阻断调查，不能宣称 Runner 已修复、L3通过或已完成真实会话。对 `b83f6d7` 的最终 gate 顺序、脚本摘要、三个批次和首轮 L1 记录补充复核也通过。最终项目检查记录亦已复核：公开summary与raw字节一致，独立解析JSONL的计数/SHA一致，失败/中止记录保留，无P0–P2；并只读确认全局执行策略未改。

## 项目检查

正式 `scripts/test.ps1 -Level L1 -OutDir .evidence/orca-native-submit-l1` 首轮中止，exit=1，报 `Runner source changed during build; build again from stable inputs.`：构建期间主会话提交了本批文档，源码身份改变；Go测试尚未执行。原失败 summary 保留，固定提交后使用新目录重跑，不覆盖首轮。

后续均固定 `b83f6d79925171b71af39425867f4c00c6a9b087`，期间没有改文件或提交：

| 批次 | 实际结果 | 证据与边界 |
| --- | --- | --- |
| `l1-stable` | exit1，Go尚未执行 | [summary](evidence/orca-native-submit-20261030/l1-policy-failure-summary.json)、[日志](evidence/orca-native-submit-20261030/l1-policy-failure.log)；Windows PowerShell 包核验被 AuthorizationManager 拒绝 |
| delivery 定向复核 | 23项通过 | [结果](evidence/orca-native-submit-20261030/delivery-process-policy-recheck.json)；只设置子进程执行策略，不修改全局策略 |
| `l1-process-policy` | 框架24项通过；Go测试发生权限/IPC失败后中止，非完整L1 | [部分计数](evidence/orca-native-submit-20261030/l1-sandbox-interrupted-counts.json)：574个顶层通过、131个顶层失败、16跳过；含子测试2288通过、248失败、16跳过，2包未完成；raw summary仍为running，未改成通过。记录了临时Git config权限拒绝/IPC问题，不声称全部失败原因相同 |
| `l1-unsandboxed` | **正式L1 passed / exit0** | [原summary](evidence/orca-native-submit-20261030/l1-final-summary.json)、[JSONL派生计数/摘要](evidence/orca-native-submit-20261030/l1-final-counts.json)；712顶层通过、0失败、17跳过；含子测试2611通过、0失败、17跳过；34包通过，1包无测试 |

最终批次以批准的 sandbox 外进程执行相同正式入口，仍只在当前测试进程设置 `PSExecutionPolicyPreference=Bypass`；全局策略只读核验为 LocalMachine RemoteSigned，其它 scope Undefined，未执行 Set-ExecutionPolicy。最终[框架24项](evidence/orca-native-submit-20261030/framework-final.json)、Node原生扩展29项/0失败/0跳过、native driver编译与零匹配guard通过。17个skip均是正式入口允许的既有opt-in/辅助项，不是17个真实原生通过；summary的live case计数0不表示Go测试零执行。

完整 raw JSONL、coverage、TAP与所有框架夹具保留在本工作区 `.evidence/orca-native-submit-l1-*`；公开精简摘要和JSONL SHA以便对照，失败/中止现场不覆盖。另通过 `git diff --check`、核验脚本 `node --check` 和三份专项文档相对链接检查。最后仅增加文档/证据记录，生产源码与受测 `b83f6d7` 相同。

## 未验与删除状态

P2原始强目标未获证，P3–P5实施/新稳定矩阵/真实验收没有完成。原生提交新方案真实 local **0/3**、remote **0/3**，未创建模型会话，业务提交0，新增待清理Orca资源0。Controlled/L3新方案未执行，不能用上述基线或240次契约反例核验替代。旧草稿门禁、重绘代码、配置/fixture和当前运行文档没有删除。原runtime/session/request、原输入/final和停止证明代码均保留，无安装、部署或历史失败迁移。追加核查只实测两个目标的只读原请求查询，不计入六个真实提交会话。
