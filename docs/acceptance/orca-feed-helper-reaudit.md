# StateFeed 与 helper 生产端：实现复审

2026-11-06，合并前复审。范围为 [A Spec](../spec/orca-state-feed-retirement.spec.md)/[Plan](../spec/orca-state-feed-retirement.plan.md)、[B Spec](../spec/orca-helper-producer-retirement.spec.md)/[Plan](../spec/orca-helper-producer-retirement.plan.md) 的实际实现。重新刷新 main 后仍为 `dda9a82727b6e7ac88e687a250750e8bbae236dc`。原工作副本 HEAD `da788478` 与该 main 文件相同；保留全部已验证工作，从最新 main 新建 `codex/orca-feed-helper-retirement`，不改写已合并 #127 的分支历史。

## 结论与删除闭包

未确认生产缺陷或需要修正的测试迁移问题，本轮未改生产、Go/Node 测试、脚本或资产。当前848个源码/测试/脚本等输入文件与 B 独立通过点逐字节相同，清单 SHA256=`b1f21951b3117386d254a49e0eae7fe6c4426a21f0619d26be1ca0fd18807474`。本次不是仅复述原验收：重新核对原源码全部候选引用、合法入口、全生产声明、现存测试文件及资产路径，并实际重跑完整协议、adapter 和 Node。

| 重点 | 复审结果 |
| --- | --- |
| A 删除闭包 | 原 OpenStateFeed 只有聚合层声明和两个测试调用根；删除11个函数/方法、5个瞬态类型及3个常量，无当前 observer、driver 或持久消费者被删除，无兼容壳 |
| B 删除闭包 | TerminalRuntimeMetadata/TerminalMetadata 无当前受理、CLI 或历史 decoder 调用；TerminalConfig.Command 仅原声明。生产移除原 asset embed/摘要函数后，实际测试消费者改接私有测试 helper |
| 原查询/订阅 | SubscribeState、Next/Close、QueryState/QueryWorktreeAgents 的原实现及原 socket/IPC/身份/取消错误保持；QueryOriginalAgent 和原 decoder 保留。probe-subscribe 与 networkPreflight 仍真实调用原订阅，初始化 context 与独立 Next 生命周期未缩短 |
| 迁移后的测试 | 原八个 stream 场景全部保留；新两场景消费真实 snapshot/updated，取消初始化 context 后仍读取原更新，查询与订阅并存，读取取消注册屏障之后再取消/断开，保留原错误分类。本机测试保留 pane/provider、完整正文、替换拒绝、零后台订阅及阻塞调用取消 |
| 共享夹具 | 原382个 Go 测试文件均保留，仅修改批准的四个既有测试文件并新增资产 helper。frozen/current/capability、owner/receipt/Job、终端并发、CLI/creation/IPC、usage 与稳定性注册等其余夹具原字节保持 |
| Node/诊断 | 五个 JS 和 probe-orca-pi.ps1/test-prerequisite-contract.ps1 保持原字节、原 import/复制路径和 TAP 规则；pi-managed 仍属于原四资产摘要。未运行模型 probe、安装依赖或将旧诊断升级成当前结构化资格 |
| 格式/可靠性 | 全生产 AST 仅有批准的删除，2,253个保留声明正文相同，无生产新增或正文修改；TerminalConfig/owner/receipt、原冻结 hash、Observation 缺失/显式零、state/v8/user_version=8、原 codec/wire 和当前结构化状态均保持 |

[新引用、消费者与测试清单](evidence/orca-feed-helper-reaudit-20261106/audit.json)包含基线候选的原引用位置；[全生产声明差异](evidence/orca-feed-helper-reaudit-20261106/declarations.json)精确列出本批删除，不能把“TerminalMetadata 有 JSON tag”当作实际持久消费者，也不能据此扩删 TerminalConfig。原类型/函数相同是语义审计的补充，不将源文件或测试计数相同单独视为充分证明。

PC-R01–R10 的保留实现均不在生产修改范围：原受理/提交、unknown 原操作只读对账及零重放、预算缺失只阻新派发、原资源同构建恢复、stop 撤销及原冻结 scope、结果归属/收取/封存均保持。旧 Feed 取消不是停止证明；原证明不足仍等待/未知，缺 owner/hash 不从当前构建补足，原 close 失败/丢回执不变为 stopped，WritersAbsent 不置真，scope 不扩大为主机清场。

## 本次实际验证与明确复用

| 范围 | 本次结果 |
| --- | --- |
| 完整 protocol/orca | 62个顶层通过、4个原有现场 opt-in 跳过，360个执行节点通过，0失败 |
| 完整 orcatransport | 81个顶层、351个执行节点通过，0失败/skip；包含缺原 bundle 不得用当前 bundle 补足等原反例 |
| 原 L1 Node TAP 命令 | exit=0，21项通过，fail/cancelled/skipped 均为0，摘要与实际 ok 数一致 |
| 原状态/wire/能力/资源/资产重新捕获 | 158,875字节，SHA256=`f5d69cd15df16ef2ba029b137e4fc6caea774aadf11565963dd0813254d78b62`，与实际原基线逐字节相同 |
| 持久/结构化格式代表编码 | 明确复用 A/B 已实际独立捕获的15,528字节及原字节比较；本次生产声明、encoder/decoder 和输入均相同，不虚记重新运行该捕获测试 |
| 应用103项、共享组件135项、driver/helpers 编译 | 明确复用 B 原通过结果；实际源和测试全量匹配，未重新执行或将编译记为真机通过；原3个 usage现场 opt-in 跳过保持 |
| 正式 EXE 三公开受控/四重启窗口 | 明确复用 B 的原同 EXE 结果，核实产物 SHA 与实际文件相同、exit_code=0、scenario_count=3，四个重启子例全部通过；本次未重启该宿主或替换 EXE |

新命令为 `go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport -count=1 -timeout=5m -json` 及 `node --test --test-reporter=tap internal/execution/adapters/orcatransport/extensions/pi-evidence.test.mjs`。[原组件日志](evidence/orca-feed-helper-reaudit-20261106/components.jsonl.gz)、[原 TAP](evidence/orca-feed-helper-reaudit-20261106/node.tap.gz)、[本次原状态捕获](evidence/orca-feed-helper-reaudit-20261106/wire.json.gz)。

复用构建 source_digest=`bc865f625d72a5c61aa9d898c9bc5b09ec1e63472e4b22805042221839efb378`，EXE SHA256=`ba4b32d2048b65140f8a3004c3b45bc3c9de9b4358dd5c8cbe898d52008d85f3`。其原 source_commit=`da788478`、source_dirty=true 保持，不因新建 PR 分支、后续提交或文档更新重写为新 commit。旧产物对应的源码归档及清单可还原；本轮生产和测试未变化，分支切换不是新构建或新公开资格。

原 A 的沙箱连接失败、错误 livetest 路径，以及原 B 的未用 import/沙箱 Node EPERM 均保留在[初轮 A](orca-state-feed-retirement.md)/[初轮 B](orca-helper-producer-retirement.md)及各自证据中，不追认通过或宣称本轮修复其环境根因。没有实际代码问题需要修复，本轮只新增复审证据与文档导航。

## 交付边界

按用户追加授权提交并提 PR；目标分支创建 PR 前再次刷新，GitHub 实际内容合并状态以该 PR 为准。优先 Squash and merge，不因 Rebase 按钮提示改写已验历史，不自动合并或部署。新验证与复用验证均为合并前定向结果；未运行完整 L1/Controlled、L2/L3、真实模型或合并后回归，不授予完整交付或真机资格。
