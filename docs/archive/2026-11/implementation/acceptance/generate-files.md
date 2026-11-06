# generate 文件材料验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-02，基线 `66cc0acf50b675c75458d9d6281139e16158a3a3` 加本轮工作树。复用原 Document/Material/输入文件 owner，原 C:/Project/SteerRunner 未修改；当前独立源码在 C:/SteerlineDebug/wt-762b-roadmap-materials/runner-source。证据根为 C:/SteerlineDebug/wt-762b-roadmap-materials。

## 实际结果

- `diagnostics/runner-owner-current-20261102T155337968Z/go-test.jsonl`：contracts/resources/application 定向通过。含 file 路径/保留名/大小写/前缀、原字节/不内联、缺失/篡改/恢复、Windows junction 首次准备及同内容重定向拒绝、原 execute 冷停止。
- `test-results/runner-L1-20261102T072435495Z/summary.json`：完整 L1 保持失败，framework/native compile/零匹配守卫通过。workflow boundary 引用不存在 conversation 目录、resume 缺冻结 budget 均在未修改基线复现（diagnostics/runner-baseline-boundary-20261102T155221618Z、runner-baseline-20261102T074257673Z）；另有 Orca sample-feed 时间断言，基线定向复跑通过，仅记录为不稳定失败。未修改这些测试或放宽检查，不宣称全 L1/全 L2/发布资格。
- `diagnostics/live-platform-digests-20261102T155125711Z/summary.json`：SteerLine 正式生产调用链 + 真实 Runner/Codex，229.971s，通过。工作树草稿随机 SumInput51fbe397 未进入 Git，解析及文档实际包含该标记。原 generate.materials 的 file 请求及 Run/content_result.file_materials 保存原始证据；源码 manifest 独立。两次 generate、六份文档、隔离 Git/provider 发布合并、SQLite 两任务一依赖、正常停止资源结算通过。
- 实际解析 Run=7cfda41580f5cc643a07b91a6674b24e，文档 Run=1b83695dc0d5edf65ae68403a1ccbbbc，均 completed/stopped。evidence/runner-settlement.json 的 resources_settled=true。
- EXE `bin/steerrunner-verified.exe` SHA256=df27f15bc61767edcfdd6713b6572d5747d4889cfccaace8846d1dcef3dba9ec，resource digest=a968df6ee06ae6a16650ed9f0eee3ab878e9406a4541ab44fa6548674161c707。

## 正文与审核

既有 content frame 固定指令正文不变，不增加模型行为规则。InputContract 的公开材料描述从 `JSON array: content (escaped caller text), path, sha256 (Runner framing and facts)` 改为 `JSON array: inline content or frozen file location, path and sha256; file bodies are not expanded`（UTF-8 82→97 字节，描述发现能力）。业务接线对 file 仅发送 path/file/sha256 原事实；正文固定在原 Run 私有文件，无 inline 兜底、无二次存储或原请求重构。独立只读审核 review_roadmap_newlines 已检查正式渠道、原请求/恢复/manifest、Windows 重定向和真实调用方接线，修复复审后无剩余发现；未用样本更新代替审核。

曾有 execute/1 回归因内容 final 检查范围过宽失败，现限定为内容 workflow，原失败日志保留。真实链中调用方曾不接受 input_file、提前采用 settling、模型抄错引用摘要；调用方分别修复来源类型、等待正常停止、由平台核验原 Git 后填写摘要，未放宽 Runner 原始字节检查。每次新现场独立验证，没有重发未知请求。

当前仅交付配套源码与隔离验证。未部署安装版，未以本机隔离 provider 代替真实外部 GitHub/Gitea PR 或浏览器资格。

## 2026-11-02 后续授权：修复完整 L1

用户要求修复 Runner L1 后把两仓 PR 转正式。原三处失败已修复：边界扫描改为枚举全部实际 execution 子目录，保留 forbidden 规则和负例；报告恢复夹具通过正式 FreezeInput 补齐原预算，并额外核对恢复后的 Prompt/Budget 不变；样本流测试在锁内保存接收时间、等待墙钟晚于原接收时间后 Drain，保留严格 Before 并逐项核对原时间戳。生产提示、预算门槛和原生接收代码未修改；原夹具正文 old frozen prompt 为 UTF-8 17→17 字节，仅补原预算事实。独立只读审核 review_roadmap_newlines 对三处实际差异复审无发现。

- 定向 `diagnostics/runner-l1-repair-owner-20261102T161529692Z/go-test.jsonl`：22 项含子例通过，零 fail/skip；原预算缺失拒绝等反例保留。
- `diagnostics/orca-feed-repeat-20261102T161530849Z/go-test.jsonl`：样本流叶例重复 100 次全部通过（父/子共 200 个 pass 事件），零 fail/skip。
- 同步 Runner main=761f916af352d22f51eab4b5967f9a390eeebd17 后，干净提交 06a9214ad54dcaa9283368103de80be1c73e602f 的完整 L1 `test-results/runner-L1-repaired-20261102T161644437Z/summary.json` 仍为失败：三处原失败已通过、734 个顶层通过，但 application 整包达到固定 12 分钟后 panic，TestRAAuditLifecycle 仍在正常执行原 Git 调查。framework、29 项终端扩展、native compile/零匹配守卫均通过。该失败不改写为成功。
- 聚合预算调整 12m→20m，包/用例清单、允许的原生 opt-in skip、断言及各操作/用例期限不变；不排除应用包、不拆分重选以伪装完整通过。最终完整入口复验结果完成后追加。
- 配套真实链使用上述干净 Runner 构建 EXE SHA256=41de9144c4c999298628e24511f24767f5f3cb5d2959b23edfd80fc55b35be08，source digest=30dc64bec16156fdb0e62af9b5911d4c21392aa5bccf1789e44d7a8f2cba1463。`diagnostics/live-runner-l1-repaired-20261102T161722633Z` 被模型抄错 confirmed_digest 的 schema 校验拒绝，原 Run stopped、resources_settled=true，原失败保留；独立新现场 `diagnostics/live-runner-l1-repaired-confirmation-20261102T162140859Z/summary.json` 通过（243.930s），两次 generate、六份文档、实际隔离 Git/provider 发布合并与 SQLite 两任务一依赖，公开正常停止及资源结算通过。无原请求重发或校验放宽。

第二轮完整入口 `test-results/runner-L1-complete-20261102T163508518Z` 又暴露原 `TestOrcaPreSendReconnect/provider-query-exhausted` 的20ms调度竞争（首次连接前预算耗尽，opens=0）；已保留为失败。核实本轮 go.exe/子 application.test.exe 的PID、精确EXE及唯一输出路径后，中止该已知失败轮次剩余应用包，operator-abort.json保存身份与原因，不计通过。修复仅在测试：保持正常3s readiness预算，在原连接/查询回调等待该真实 deadline 后返回失败，核验 DeadlineExceeded、原查询一次、零发送/零重连；取消用例在实际首次连接中注入cancel，明确核验Canceled，避免把短设定时间当作首次调用发生的保证。没有改生产重连/操作预算或略过测试。

## 最终完整 L1 通过与正式审阅资格

最终干净提交 `b11219e8fc9bfa5dc357202e58aded5aa4591df7`，正式命令 `./scripts/test.ps1 -Level L1 -OutDir <独立目录>`，报告 `test-results/runner-L1-final-20261102T164640208Z/summary.json`：status=passed、validation_scope=complete、exit_code=0、738 个顶层测试通过（2806 个父/子 pass 事件、36 个包通过）、零失败事件；29 项终端扩展、framework、native driver compile 和零匹配守卫全部通过。16 项原有显式原生 opt-in/helper skip 保持原白名单，不计为原生通过，没有意外跳过或新增例外。总耗时 1028.197s；application 实际 771.223s（超过旧720s聚合上限），审计用例本身9.26s，证明修正包级预算而非放宽某个卡死用例。

重连全表与取消定向重复10轮在 `diagnostics/reconnect-deadline-repair-20261102T164422437Z/go-test.jsonl` 中为150个父/子pass、零fail/skip；最终完整L1中对应全表亦通过。四处夹具/守卫修复及12m→20m聚合预算均经过独立只读审核，未修改产品操作期限、预算准入、重连或原生接收代码，也未删减测试。此最终结果更新前文暂缓L1资格的结论，原始失败及操作者中止报告仍保留。

配套真实链仍按上节固定生产构建记录，之后仅测试/脚本与验收变更；不冒称重跑生产行为。按照用户授权，在补齐本次记录后将 Runner #89 和 SteerLine #430 转入正式审阅；以公开PR回执核对最终状态，不执行合并或安装部署。
