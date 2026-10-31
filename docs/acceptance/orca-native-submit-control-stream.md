# 持续控制原始流读取修正

2026-10-31，复审基线 `2beeb90c512f9434292fc7f93279d604ed61e342`。
本页单列复审发现的 P2 与文档状态修正，不覆盖此前双端各 3/3 或远端单次重跑的存档构建结果。
当前增量已实施，稳定矩阵和 L1 通过；真实批次和独立证据复审在本次活动结束后登记。
本次新本机失败保留，PR 暂为 Draft。

## 问题与最小修复

`controlLines` 不只读取初始 READY，也用于 original-turn、workspace-audit 和 authentication-stop。
之前固定 cursor 0 / limit 1000 并拒绝任何 limited 或非零 oldestCursor，导致正常长期累积输出
达到上限、或旧前缀淘汰后，所有后续控制查询失败。该回归属于 Runner 读取条件，不是 Orca
提交能力缺陷的证明。

只修改共享读取函数和两个调用点：准备仍严格保留起点；持续查询允许首读旧前缀淘汰，按公开
nextCursor 分页，读取截至首次 latestCursor。每页前后验证原 runtime/terminal/incarnation，
核验 stream 来源、cursor 与行数，拒绝中途缺页、无进展、身份变更或取消；不解析半行及快照
以后的新增后缀。原请求 ID/binding/request digest、正式 history/模型/原输入与停止证明不改。
分页变量仅存在于本次只读调用，无新持久状态、框架、发送、重试预算或屏幕回退。

Spec + Plan 先修订并独立只读审计通过，再实施。公开能力通过本机版本匹配的 `orca-cli`
技能说明及 `orca terminal read --help` 核实；不是实验接口或安装包内部生产依赖。

## 确定性回归

扩展原 fixed-control fixture，提供累计输出、真实 cursor/1000 行分页、旧前缀淘汰与实际调用计数。
最初新增的 12 个正式流程叶例在未修生产实现上全部失败；原退出码 1 和逐例原日志保留。

| 原正式入口内的测试 | 叶例 | 核验内容 |
| --- | ---: | --- |
| TestFixedControlRetainedStream | 12 | 1001 行、2505 行、oldest=5000；正式 history/workspace/auth-stop 和清理、原 checkpoint 跨 Lifecycle 恢复；错误请求摘要/错误原输入拒收 |
| TestFixedControlStreamFaults | 19 | 逐页来源/元数据/身份、缺页、倒退、行缺失、回执丢失、取消、无进展和快照后增长；原响应与零额外输入计数 |
| TestFixedControlStreamBoundaries | 6 | 空快照、淘汰后的空窗口、半行、单页与整 1000/2000 行边界 |

三个 root 接入原 `scripts/test-native-input.ps1 -Mode controlled -Repeat 30` 的必需门禁，
不新增正式 L3 case。最终 **18 个 root、205 叶例 × 30 = 6150 执行/6150 通过/0 失败/0 跳过**，
新增 37 叶例为 **1110/1110/0/0**；每叶实际运行 30 次，无漏选，源码冻结门禁通过。
准备阶段原 32 叶例的 screen/截断/受限/起点丢失拒收不变，包含在同一矩阵中。

本次冻结源摘要 `232b050240b7a17ecd1766de28dc56f298598ef11a79a64141978d29823a6a8d`。
矩阵是受控 Runner 行为证明，不冒充实际 Orca 在长期输出后的完整消费证明。

项目正式 `scripts/test.ps1 -Level L1` 通过：**717 顶层/2700 含子例通过、0 失败**，
17 个明确 opt-in/helper 测试跳过，另 1 个无测试包跳过；34 包通过。框架检查、29 项终端扩展、
原生 driver 编译及零匹配门禁通过。跳过不计真实资格。

## 真实增量与边界

新隔离构建的现有 Runner agent 未部署安装。本机、远端使用同一正式原生入口、新工作区、
独立 Run/request/terminal/incarnation/session 与 nonce，原正文、受理、开始、完成及正式停止
各自记录。失败不重投、不补 Enter，不改模型、认证、全局配置或生产等待算法。

当前已结束的本机 local-01 保持失败：原业务提交一次，Orca 回执 accepted/turn_started，
但 Codex 自身界面显示无法确认发送及 queued follow-up，正式 history 始终未确认原输入开始，
原六分钟父期限耗尽后 completed=false。正式 stop、认证撤销、control 关闭与完整空清单通过，
cleaned_up=true、emergency_cleanup=false。只读屏幕用于解释现象，不参与提交/完成判定。
此现象未归因于此次读取修正，也不能将该会话计为真实通过。

新远端 remote-01 已通过：一次原业务提交、原输入/final 及正式停止/空清单通过。其余新现场
仍在本批验证中；旧真实资格不自动变成本增量资格，不能以 6150 次受控通过替代。

## 文档与范围

修正 docs 索引和 Spec/Plan 的“未实施/未验收”过时状态；实施页明确历史批次归属。
测试指南区分严格准备读取与持续保留流读取，并准确说明真实长输入仅覆盖中文 BMP、CRLF、
空行；非 BMP/代码块受控覆盖不计本批真实资格。未重写历史失败或 Get-Process 来源未定记录。

未部署安装、改 Orca/全局配置/业务材料、运行 Unity、追查历史命令来源或操作历史 Run。
