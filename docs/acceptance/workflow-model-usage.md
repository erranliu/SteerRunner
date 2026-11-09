# Workflow 配置模型与分阶段用量验收

日期：2026-11-09。代码基线 `210d1d8c1c659d32957f35bd1de794a247c75045` 加本次工作树修改，完整 L1 测试时生产输入摘要 `fe611970e953a5639249739603283e38ead683372db87c4ce84fd3101344ae3b`（`scripts/build.ps1` 的路径集合，含 internal 测试源码）。当前契约见[模型与用量](../model-usage.md)。本次仅修改当前 checkout 的源码与文档；后续按用户授权提交、推送并创建 PR，不修改、部署或重启已有安装现场，也不启动真实模型。

## 变更

- 保留内置 Workflow 的 execution/audit 明确角色切换；补齐普通 Orca 流程逐阶段冻结模型。缺失审计模型或未知模型角色不能落回执行模型。原生选择与冻结输入不符仍拒绝；指定模型错误直接保留，不换模型发送。
- 删除本机 Orca audited 独立模型日志注册、读取、实际 model/effort 对比及阻断链路。历史 `actual_model` 字段继续可解码，新结果不伪造此字段；Token 采集仍保持已有的来源身份及授权边界。
- 用量事件新增可选 `step_id`、`configured_model`、`configured_reasoning_effort`；`model/provider/metadata_basis` 继续只表示实际观测。能力和事件 schema 同步发布；不新增外部逐阶段模型配置接口或 Runner 价表。
- 按原 attempt/command 绑定配置归属；重复响应去重，配置归属冲突隔离，观测元数据冲突仍保留配置和 Token。缓存用量只据原冻结输入补齐新 revision，不额外读模型日志，不改旧 revision。
- scope 仍为 `main_session`：独立 parser、子 agent、工具内部模型及不可验证的 Pi 用量没有被追认为完整覆盖。费用估算由 SteerLine 按配置模型分组；缺失配置或用量不按其它模型单价填补。

## 验证及证据

所有证据位于此 checkout 的 `.evidence/`，只证明本次未合并工作树。Git 用例的短临时目录为本次独占测试现场，不属于已有安装。

- 初次受影响包检查发现删除链路后残留的未用 import，以及旧用量投影测试要求丢弃模型的断言；修正后 `model-usage-core.log` 的 usage/access/api/collect/parsers、storage、resources 七包全部通过。
- 原沙箱测试先后受临时 Git config、MSYS 命名对象与只读 Git 对象清理限制；保留 `model-usage-initial.log`、`model-usage-application.log`、`model-usage-focused.log`，不记为通过。新增结构化审计测试最初没有提供原 Git owner，补齐测试 owner 后按真实 Git 检查原工作树。
- 完整 L1 初轮在测试补写期间被构建源码稳定性检查拒绝（`model-usage-l1/summary.json`）；第二轮在进一步修正夹具前主动中止（`model-usage-l1-final/`）。这两批不授予完整 L1 资格。
- 受影响定向测试在沙箱外、工作区内独立临时目录运行，`model-usage-focused-final.log` 九包全部通过。覆盖模型冻结、审计无模型日志、明确审计模型拒绝且一次发送、同会话 writer/reviewer 用量分离、配置与观测别名分离、配置冲突隔离、旧快照不变、缓存补齐无日志读取及新 schema 校验。该批早于下述阶段授权门槛保留修正，最终源码另行验证。
- 最后复查保留了原模型日志块附带的阶段授权门槛；新增 revoked/resume-required/stopping/closed 四种状态断言，确认不会推进审计。最终 `model-usage-authority.log` 的两项 application 顶层测试通过（22.53 秒）。为包含此修正，中止此前 `model-usage-l1-verified/` 批次（其框架检查已通过），不授予该批完整 L1 资格。
- `model-usage-l1-complete/` 使用工作区内临时目录，其既有 `TestSRRepositoryCacheWithLongBranchPath` 在进入深层 clone 目录启动 Git 时返回“目录名无效”。同一源码改用 `C:\srt\mu-*` 本次独占短目录后，该项单独通过（1.663 秒，`model-usage-long-path.log`）；未修改测试或产品源码来规避。短目录路径登记于 `model-usage-short-temp.txt`；整轮重新运行在 `model-usage-l1-short/`，不把环境失败批与单项复验拼成完整通过。
- 上述长目录批次完整结束：752 个 Go 顶层通过、1 个顶层失败、12 项既有允许跳过，Node 扩展 21 项通过；application 包用时 906.111 秒，失败仍只对应上述 Git 工作目录过长。该批整体失败，未取得完整 L1 资格。
- 最终同源码完整入口 `scripts/test.ps1 -Level L1 -OutDir .evidence/model-usage-l1-short`（TEMP/TMP 与 FixtureParent 使用上述独占短目录）退出 0，`summary.json.status=passed`，总用时 1083.995 秒。Go 顶层 **753 通过**，**12 项既有允许跳过**，零失败；Node 扩展 **21 项通过**，框架门禁、原生 driver 编译和零匹配保护通过。`native_preflight=1`、`l1_core=1`、`controlled=0`。最终生产输入摘要与本页开头一致；完整 L1 资格仅属于该批，不能把早先失败批改记通过。
- 跳过的是独立子进程 helper 入口及需显式 opt-in 的本机/远端原生采集、IPC、清理探针；跳过不代表真实模型验证。未运行完整 Controlled、L2/L3/L4、真实模型或合并后验证。

## SteerLine 交接

SteerLine 需要读取新增能力和事件字段，在固定 snapshot_revision 下消费完整事件页，以 event_id 去重，仅计 counted 事件；按 step_id/configured_model 分组应用双方约定价格。实际观测模型和供应商信息另列；缺失或别名不同不成为配置估价的硬门槛。总 Token 不能乘单一模型价格。未知配置、部分 Token、未覆盖独立 parser 等情况应展示已覆盖金额和缺口，标签为“按约定配置模型估算”。具体处理规则见[当前契约](../model-usage.md#steerline-配套)。本次没有修改 SteerLine 仓库。

## 目标分支

提交前再次刷新 `origin/main`，仍为 `210d1d8c1c659d32957f35bd1de794a247c75045`，与本次开发基线相同，不存在因目标分支前进产生的内容冲突。PR 创建后的实际合并状态另行查询 GitHub；优先 Squash and merge，本次不执行合并。

## 提交前复审

按用户后续授权复审模型派发、原生拒绝、用量归属、缓存补齐、冲突隔离、旧快照不可变及阶段授权边界，未发现需进一步修改的运行逻辑。复审阶段仅更新本验收文档。

复审定向回归 `model-usage-pr-review.log` 的 11 个受影响 Go 包全部通过（`-count=1 -timeout=5m`），application 包用时 29.956 秒；包括模型不可用不回退、审计无模型日志、撤权门槛、多模型分账、缓存补齐与历史不可变、冲突隔离、模型参数绑定及公开 schema。它是本次额外定向验证，不称为重新执行完整 L1，也不把默认关闭的原生 opt-in 测试记为通过。

复审源码与完整 L1 的 `input_finish.files` 的 838 项逐文件核对，零差异，结果保存在 `.evidence/model-usage-pr-review-source.json`；删除文件仍保持缺席，其余测试输入字节一致。Git 提交后不再将已删除文件作为 `missing` 清单项，因此重新枚举得到的 build source digest 可能改变；该差异不代表运行源码改变，也不重写原测试摘要。提交后的字节一致性另行复核，不把合并前测试称作合并后结果。
