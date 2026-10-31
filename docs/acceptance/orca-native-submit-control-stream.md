# 持续控制原始流读取修正

2026-10-31，复审基线 `2beeb90c512f9434292fc7f93279d604ed61e342`。
本页单列复审发现的 P2 与文档状态修正，不覆盖此前双端各 3/3 或远端单次重跑的存档构建结果。
当前增量已实施，稳定矩阵和 L1 通过。最新构建真实远端 **3/3 通过**、本机 **0/3，通过0/失败3/跳过0**；
六场正式停止与清理通过。本机资格仍阻断，PR 保持 Draft，不能宣称通用输入链路已稳定解决。

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

| 现场 | 原 Run | 原请求 | 原业务提交 | 完成资格/正式清理 |
| --- | --- | --- | ---: | --- |
| local-01 | 7b60e65601cd4e94df805cfbb73a0514 | 5f967268-4575-1079-780e-26df37e1df16 | 1 | 失败/通过 |
| local-02 | 31cc3329ebfa510f4c27969d98861bd4 | 09c4c86e-4e1a-6011-8334-29ef38e56de5 | 1 | 失败/通过 |
| local-03 | 0bcefebad97ca88b66e337676d99a970 | 47b314f5-4a99-0fdb-96a5-f21768ce9ff0 | 1 | 失败/通过 |
| remote-01 | 53fb0cc4af50c25755cf3a1418563d6f | f6937494-9a6b-0c8d-f3d7-b992f2592ac0 | 1 | 通过/通过 |
| remote-02 | f27c11859f7772ab871b7d9dddf3967d | 81e0d21b-cc66-6e56-4702-f086ae6af0e3 | 1 | 通过/通过 |
| remote-03 | ccfb54d32f401fe0566b0a91c511ecde | 946ce4fe-9340-9095-bbe4-e8bc811149de | 1 | 通过/通过 |

三个本机会话均保持失败：原业务提交一次，Orca 回执 accepted/turn_started，但正式 history
始终未确认原输入开始，原六分钟父期限耗尽后 completed=false。测试总耗时含原有清理分别为
379.39/371.68/370.85 秒，没有延长业务期限。首场只读诊断中 Codex 自身界面显示无法确认发送及
queued follow-up；该屏幕只解释现象，不参与提交/完成判定，也不推断另外两场相同界面或具体根因。
三场正式 stop、认证撤销、签名退出、control 关闭与完整空清单均通过，cleaned_up=true、
emergency_cleanup=false。离线原字节、冻结 wire 摘要、请求/模型元数据和退出签名核验通过。
这不证明未执行，也不能归因于此次分页修正；只能说明未取得正式原输入/完成凭据。

三个新远端会话均通过：原输入、accepted/started/completed、唯一 nonce final 和正式 stop/空清单
均核验。准备耗时 121.067/121.695/122.541 秒，业务 9.091/9.957/11.456 秒。沿用既有远端
orca_terminal 停止范围，不扩大为执行主机凭据撤销或脱离终端后台写者消失。

六场使用同一冻结源码摘要与 TestEXE `65267770abd96a2d617ce164b8e0251d382aece27d2aa6de53eced58dfa1709b`；
隔离 agent 为 `c06abe33f9be90322af4b7fc0a7a5d445d82d94bcf52148650fca00b5970bf2d`，未部署安装。
原源码先按 dirty 树受测，再提交为 `cf29e14`，按同一源摘要绑定，未冒称受测时是干净提交。

六场原完整空清单后精确移除工作区；本机路径/Git 注册与远端独立只读查询均确认不存在。
额外远端只读清理核验终端也已确认 PTY 停止并从完整清单消失，其它终端未操作。
原始字节与清单见[归档](evidence/orca-control-stream-fix-20261031/manifest.json)、
[本机核验](evidence/orca-control-stream-fix-20261031/local-three-audit.json)和
[远端核验](evidence/orca-control-stream-fix-20261031/remote-three-audit.json)。

旧真实资格不自动变成本增量资格，不能以 6150 次受控通过替代；真实超过 1000 行/前缀淘汰的
长期控制会话仍未另行实测，本次对应回归只证明受控 Runner 行为。本机构建资格未通过，PR 不放行。

## 最终独立复审

[独立只读复审](evidence/orca-control-stream-fix-20261031/independent-review.md)核验原始事件、
30 个归档数据文件和 550 份原记录、三份本机退出签名、正式 history、六场结果与清理。
本轮 P2/P3 修正无新增 P0–P2；整个 PR 因本机资格 0/3 仍不放行，保持 Draft。

## 文档与范围

修正 docs 索引和 Spec/Plan 的“未实施/未验收”过时状态；实施页明确历史批次归属。
测试指南区分严格准备读取与持续保留流读取，并准确说明真实长输入仅覆盖中文 BMP、CRLF、
空行；非 BMP/代码块受控覆盖不计本批真实资格。未重写历史失败或 Get-Process 来源未定记录。

未部署安装、改 Orca/全局配置/业务材料、运行 Unity、追查历史命令来源或操作历史 Run。
