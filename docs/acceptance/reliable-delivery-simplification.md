# 可靠性交付简化实施与验收

日期：2026-10-21。归属[修订方案](../reliable-delivery.md)第 6–8 节。Runner 基线 `8261310d5f2d1edb6fa0c81ba05fa4119b561386`；SteerLine 基线 `36d4f5967e9ecc2a9e02b0f124e8bbf3aff77d50`。Runner 开始时只有本聊天的 README、conversation 和方案文档变更；SteerLine 开始时干净。本轮不部署安装环境；实现与验收完成后按用户要求提交两仓 PR。

## 基线与核查结论

| 项目 | 已确认事实与决定 |
| --- | --- |
| 原命令对账 | Bridge.Step/deliver 已统一按原 request_id 查询、持久受理与结果，保留原 tick 推进。无须新增等待抽象；新增长期 pending 后仍能 stop 的反例 |
| 模型调用 | 普通/带审计正常业务输入 2/3 次为现有路径；parser 默认 false；审计 final 的合法 JSON 已可 DecodeReport，无需新解析协议 |
| D1 | README、conversation、development、development-audited，以及 SteerLine 当前接入说明和 lifecycle 当前适用范围已收敛；历史验收结论不改 |
| D2 | SteerLine 候选详情增加只读交付摘要；同 Run revision 展示，原命令回执独立列出。身份/输入摘要/revision 缺失不采纳；陈旧 final/content 不显示为当前结果。没有新增网络查询或写命令 |
| V4 静态校验 | 普通 advance 与 validateWorkflow 中存在版本检查的局部重叠；后者还有游标、未知 parser 副作用等动态检查，且由其他入口独立使用。本轮未取得值得拆分的耗时/维护收益，不缓存或删除校验 |
| V5 退避 | repo list 的 5 次/15 秒和发送前 ready 的 transport.wait_timeout 属不同动作及错误策略，不能因起始退避相同合成通用重试。本轮无最小必要提取 |
| V2/V3 | V2 等 V1 的实际格式缺口与原生 schema 资格；V3 无需改变已关闭的默认路径。保留显式 parser，未因本次试验调整策略 |

原始基线、源码摘要与日志位于本工作树 `.evidence/reliable-delivery-20261021/`。原命令参数和业务输入冻结后不因试验重算；试验另建自己的 Git/会话。

## 直接实施部分

| 验收 | 结果 | 命令与范围 |
| --- | --- | --- |
| RD-D1 | 通过 | 当前操作说明明确活动、等待、stop、资源丢失与不兼容分支；两仓最终变更文档的本地链接检查通过，diff 检查通过；文档检查不启动模型 |
| RD-D2 | 定向通过 | SteerLine `go test ./internal/app -run 'TestRunnerDelivery|TestRunnerWorkflowReadProjection|TestCM71ProductDependencyBoundary' -count=1 -timeout=2m` 通过；覆盖 7 种状态、身份/摘要/revision 缺失、陈旧 final/content、停止参与和消费者副本不反向修改 |
| RD-D2 前端 | 通过 | `npm test -- src/features/runs/delivery-summary.test.tsx src/features/runs/runs.test.tsx`：2 文件 15 项通过；`npm run build` 通过；Playwright/Edge 对真实组件渲染取图并检查，不连接产品服务。初次预览依赖 URL 错误已修正，预览临时文件和 dev 服务已移除/停止 |
| RD-D3 | 通过，无新增等待层 | `go test ./internal/runnerbridge ./internal/taskrunner -count=1 -timeout=2m` 两包通过；原 OutboxBoundaries 新增 pending 反例，原 request_id 保持，零重复业务输入，后续 stop 仍执行 |
| 测试登记 | 通过 | 新增投影用例进入既有 quick-owner-unified-startup；Outbox 子例沿 quick-owner-runner-bridge。`go test ./cmd/test-feedback -run 'TestQuick' -count=1 -timeout=2m` 通过；不宣称固定 L1 整体通过 |
| Runner 可靠规则 | 通过 | workflow、reliable、reliable/operation 三包通过；仅现有规则回归，无生产执行代码变更 |
| RD-BOUNDARY | API 边界通过，HTTP 基线失败保留 | 初版 API 直接引用 runnercontract 被拒绝，已改独立 DTO 并通过 API 闭包守卫。HTTPConsumerBoundary 仍因 projectconnection→processwindow 失败；原 HEAD 有相同 import，未放宽守卫或改无关实现。含该项的聚合命令保持 failed |

SteerLine 的独立[实施计划](https://github.com/erranliu/SteerLine/blob/ffc6665315b312b7c70e32d3f03815e44de8553c/docs/plans/reliable-delivery-simplification.plan.md)与[验收记录](https://github.com/erranliu/SteerLine/blob/ffc6665315b312b7c70e32d3f03815e44de8553c/docs/acceptance/reliable-delivery-simplification.md)登记调用方范围。内容生成/对话状态已由摘要投影和组件反例验证；本轮接入的是现有候选详情，不新增尚未接入的 conversation 产品入口。

## 审计输出试验

使用固定原生 EXE SHA256 `52f75c649bebb8001102a1dd129c1ea6d02b0940321e6d7e82ee0526753bd58a`，Codex 0.159.2、冻结请求 gpt-6.1-sol/low、workspace-write/MXC/never、parser 关闭。入口为[限定试验](../../tests/experiments/README.md)，只比较历史审计自然输出与新 Run 的 audit.code JSON 提示覆盖，不接原生 schema、不改流程图。这里记录请求模型参数及原生会话/final，不从参数或模型自述补造实际模型/effort 回执；正式审计仍沿既有原生回执门槛。

| 批次 | 结论与证据边界 |
| --- | --- |
| native-trial-01 | 版本门拒绝；独立 EXE 解析到 0.163.0-alpha.10.2，与命令 shim 的 0.159.2 不同。零业务输入；改用 shim 关联的真实 0.159.2 EXE，未安装或替换工具 |
| native-trial-02/03 | 夹具未把准备写根传入 turn/start，原生参数拒绝；只有准备资源，无业务审计。保留失败，最终夹具传入同一冻结写根；02 的退出证明时机错误也保留，后续在关句柄前核实原 Job 为空 |
| native-trial-04 | 已发真实模型，但样本整数范围不精确且 shell 路径不可运行；模型保留未完成，未放行。中断本批，不计对比通过 |
| native-trial-05/06 | detached 宿主 PATH 选中 WindowsApps/MSIX pwsh；提示词指定 shell 不能替代实际环境资格。模型未完成验证/提交，不能据结构输出宣称完成。两批中断，原事实和失败保留 |
| native-trial-07 | 零修复的原生执行和对比可完成；有修复样本因独立仓库 `.git/index.lock` 写入受限，未提交且未放行，本批中断，不计整体通过 |
| native-trial-08 | 改用受管 linked worktree 和四个固定 Git 写根；原生执行后，检查脚本的字节断言把 Git checkout 的 CRLF 与写入 seed 前的 LF 比较，误报 changed。保留原 failed/中断批次，最终改在 checkout 后、模型前冻结原字节 |
| native-trial-09 | 4 个子样本全部通过，exit 0、190.65s。零修复和实际修复的历史提示均各补报 1 次，JSON 覆盖均零补报；原检查文件字节不变，实际修复计数/提交、干净工作区与原 Job 写者退出均通过。使用既有受管 worker runtime，普通 shell 目录仅前置于本次进程 PATH |

各批结束或中断后逐原 process.json 核查，共检查 29 份原生进程回执，没有保留的原生 PID；这不将失败改成通过。新旧提示词只作用于独立试验会话，未替换正在运行的原 Run。

## 默认提示调整与保留边界

限定原生试验通过后，仅调整新 Run 的默认 audit.code 提示：复用 `ResultInstruction("audit")`，要求最终返回单一 JSON，并同步内嵌 prompts.json。没有新增原生 schema、业务边、预算、状态格式、模型/权限能力或 parser 默认；显式覆盖与旧 PromptBodies 保留。原生试验反映当前固定样本，不能推断长期质量或增加新的后端资格。

| 样本 | 历史提示额外补报 | JSON 覆盖额外补报 | 历史提示计时 | JSON 覆盖计时 |
| --- | --- | --- | --- | --- |
| 零修复 | 1 | 0 | 40.914s | 29.284s |
| 实际修复 1 项 | 1 | 0 | 70.446s | 46.886s |
| 合计 | 2 | 0 | 111.360s | 76.170s |

计时是该隔离审计/获取样本的测量，不是完整开发 Run 延迟，也不作为所有模型/后端的性能承诺。

另用变更前真实 Runner EXE 做正式 CLI/IPC/SQLite 的补充受控场景：

```powershell
./scripts/test-foundation.ps1 -Pattern '^TestSRAuditPromptDefaultUpgrade$' -BaselineExe .evidence/reliable-delivery-20261021/baseline/steerrunner.exe -OutDir .evidence/reliable-delivery-20261021/controlled-prompt-upgrade -TimeoutMinutes 8
```

`TestSRAuditPromptDefaultUpgrade` 两个子分支通过，exit 0、163.46s，零跳过。旧 Run 在原审计回合活动时跨构建接管，原 input_digest、PromptBodies 和冻结审计 State.Prompt 均保持；新 Run 冻结新 JSON 默认。两者各 3 次真实业务协议输入、零获取模型调用、到 pr_ready 并显式 stop。业务 agent/provider 为受控夹具，不冒充真实 PR 或全后端原生资格。

该构建 Runner SHA256=`34a1c9d24bf04c6585ff2e3de9212d582f277fed78e975e5eef1ee018d318583`，变更前 EXE SHA256=`f4c02c682100d057a32b2d7372b50ea1f0fe06edd6b0a7d5f14a69c4e74ecafe`。公开提示资源与实际冻结默认的一致性测试通过；重放试验保留历史基线原文，避免默认提升后比较自身。

V2 原生 schema、V3 parser 策略、V4 静态校验缓存与 V5 通用退避均未改变。应用包宽筛选的 6 分钟聚合测试曾超时，原失败保留；实际相关的冻结、原生结果/预算和 parser 取消反例小组共 16 个顶层测试通过，耗时 104.834s，不宣称宽筛选整体通过。无 `live` 标签的实验包 vet 命令也曾因构建约束被拒绝，随后带标签的 compile/vet 通过；不计为功能缺陷或原生命令成功。

## 可复核证据

归档保留[结果索引](evidence/reliable-delivery-20261021/report.json)、[源码摘要](evidence/reliable-delivery-20261021/source-manifest.json)、[原生样本对比](evidence/reliable-delivery-20261021/native-comparison.json)、[跨构建验证](evidence/reliable-delivery-20261021/prompt-upgrade-result.json)、[默认提升判定](evidence/reliable-delivery-20261021/promotion.json)、[定向回归日志](evidence/reliable-delivery-20261021/runner-focused.log)和[清理检查](evidence/reliable-delivery-20261021/cleanup.json)。[组件截图](evidence/reliable-delivery-20261021/delivery-summary.png)只证明隔离组件展示，不替代产品服务验收。完整运行日志保留在上述工作树证据目录。

## 提交 PR 前的主干整合复验

两仓另行合入最新主干：Runner `757d12456010d8c36c498410a7186c65c28b9e7a`，SteerLine `9cf844639c21f841eb743e7b4f9680b23c7db0af`。保留其完整受理绑定、同事务 Run/事件观察及部署改动；本摘要继续消费已有投影。上述原始源码摘要和模型对比固定原实施轮，不冒充本次整合构建重新做过原生模型试验。

整合后 workflow、reliable、reliable/operation、resources 四包通过；应用层同一组 16 个顶层测试通过（109.588s）；SteerLine 投影、API 边界、原子观察、服务受理绑定、Outbox 和 quick 登记的定向测试五包通过。带 live 标签的实验夹具及提示资源 vet 通过。前端源码未因整合改变，复用前述 15 项及构建证据。HTTP 依赖边界的既有 import 在最新主干仍存在，历史聚合失败保持。

使用同一旧 Runner EXE 对整合构建重跑 `TestSRAuditPromptDefaultUpgrade`：两个子分支通过、零跳过，exit 0、160.11s；整合构建 SHA256=`108691919aff75d4f51263722d87c0339bf8eaf4553f1e9526485ddb03497406`。本轮仍是正式 CLI/IPC/SQLite 加受控业务/provider，不声称新增真实模型或 PR 交付资格。

整合复验的[源码与结果](evidence/reliable-delivery-20261021/pr-integration.json)、[受控场景来源](evidence/reliable-delivery-20261021/pr-prompt-upgrade-source.json)、[受控场景结果](evidence/reliable-delivery-20261021/pr-prompt-upgrade-result.json)、[场景日志](evidence/reliable-delivery-20261021/pr-prompt-upgrade.log)、[Runner 定向回归](evidence/reliable-delivery-20261021/pr-runner-focused.log)及[调用方定向回归](evidence/reliable-delivery-20261021/pr-steerline-rules.log)单独保留。调用方 PR 为 [SteerLine #374](https://github.com/erranliu/SteerLine/pull/374)。
