# Orca 原生提交调查与验收

日期：2026-10-30。Runner 基线 `a086a9984dc0bd32b3f02b9698cd26c9ba313d90`。
[Spec](../spec/orca-native-submit.spec.md) / [Plan](../spec/orca-native-submit.plan.md)。

## 当前结论

**尚未实施 Runner 修复。** 当前 Orca 通用入口的 `terminal.prompt-delivery.v1` 不能作为“完整冻结输入在执行前可靠提交”的凭据。现有安装包实现仍有 render quiet/timeout、草稿宽松匹配和补 Enter，不能直接用它替换 Runner 草稿门禁。先由 Orca 补齐强提交、排他输入和只读原请求核验契约。

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

最终 [修正夹具后的结果](evidence/orca-native-submit-20261030/installed-contract-corrected.json)：8 个不同反例，每例30次，共 **240 次，失败0，跳过0**。断言观察到契约不足即通过核验，不代表输入修复通过：

- 无任何输出或消费 ACK，原 render gate 由虚拟截止计时释放。
- 原 write 函数在消费为空、截断、改字节、已有输入或并发混入的隔离 sink 情况下仍写 CR 并给 input_accepted；这里替换了 transport/provider，不声称真实 provider 已产生这些故障。
- 原 held retry 函数根据 screen draft 再写1个 CR。
- 原 start observer 无需输入正文或摘要即可按 activity sequence 接受开始。

首轮 [夹具失败记录](evidence/orca-native-submit-20261030/installed-contract.json) 原样保留：240次中150失败、0跳过。原因是注入的外部 Error 与 VM Error 不同 realm，使原函数的 `instanceof Error` 无法识别模拟 stalled；修正注入同一 Error 构造器后另开结果文件。首轮不是 Orca 真实故障或 Runner 测试失败；没有改写为通过。最早一次核验还因输出父目录不存在未保存结果；没有原生副作用。

范围：这是隔离安装函数的契约反例核验，**不是正式 L3 稳定回归、真实原生故障复现或远端 bundle 测试**。未执行 Spec 的 NS-T01–T10 新方案矩阵，不能用这240次替代。

## 独立方案审计

独立只读 agent `design_audit` 对照当前 Runner 与已安装 Orca 实现核实上述上游缺口。首轮发现1项P2：本机 `native-command/model` wire 与 `observeRecorded` 可能使“同一流程”仅停留在 submit 函数。已在 Spec §1/§6 与 Plan P3 明确迁入共同原输入/final链路，并保留准备、认证、模型及停止保证；不能保留本机完成旁路。

同时采纳计数澄清：NS-T06 原业务请求尝试1次，实际 commit/执行按故障位置为0或1，不要求消费前失败也提交。独立审计者复核修订后通过，无新增P0–P2；仅方案通过，Orca gate、实施和真实验收仍未完成。

## 待补记录

项目检查和最终独立复审结果在执行后补入。P2–P5 未通过；新方案真实 local 0/3、remote 0/3，未创建原生模型会话，业务提交 0，新增待清理 Orca 资源 0。
