# orcaCLIOutput 恒真策略参数退役验收

2026-11-11，合并前定向验收。实施前与交付前均刷新 main 至 `c404d6e5c50cc32650f9412f9b8b3449c5789e15`（#141），本批工作分支基于该提交。设计、实施和验收范围见 [Spec](../spec/orca-cli-output-policy-retirement.spec.md) 与 [Plan](../spec/orca-cli-output-policy-retirement.plan.md)。

## 独立审计与变更范围

独立子代理核对了删除清单、两个构造入口的合法 local/paired profile、历史 reader 白名单、配对核验以及最小验收。审计后补录 #141 的 `ReuseManagedWorkspace → NewLifecycle → originalAgentWorkspace → client.ShowWorktree` 消费者，并纳入现有 worktree reuse 定向测试。该测试走共享 fixture，两个原构造的输出接线另有专门用例。`ManagedWorkspaceIdentity → newFrozenReader → agentState` 读取冻结状态，无 CLI 请求。准备测试补入 checkpoint 内的同键取锁探针，分别验证 15 秒命令预算与更短父 context 的实际截止时间。独立审计为只读，未执行测试。

`orcaCLIOutput` 只有 `NewLifecycle` 与 `newFrozenReader` 两个调用点，均传恒真闭包。生产只改 `frozen_record.go` 和 `lifecycle.go`，净删 6 行。删除回调、两处实参、条件包装和不可达的 `CheckpointDispatch → invoke` 分支。普通调用直接走原 `WithTimeout → SerializeCLI`，准备查询已持锁时仍直接调用原 `invoke`。命令编译与校验、结构化配对绑定、冻结读白名单、`Lifecycle.nativeAccess` 字段和消费者、协议队列及实际派发 checkpoint 均按原路径保留。既有测试和共享工具原字节相同，仅新增一个接线测试文件。[原引用](evidence/orca-cli-output-policy-retirement-20261111/before-references.txt) · [精确变换与文件范围](evidence/orca-cli-output-policy-retirement-20261111/scope-audit.json)

## 两个原构造与取锁验证

| 用例 | 实际断言 |
| --- | --- |
| 构造及路由 | 当前结构化和历史 reader 各验证 local/paired，构造零 host 调用，合法读取各一次。核对独立组装的 executable、参数、环境路由、目录、环境屏蔽、2 秒调用期限与返回值。setup/profile/snapshot 事实保持。 |
| 配对及只读 | current paired 用合法临时 store 取得冻结公钥指纹，成功后分别改公钥、删除 store，两次均拒绝且 host 计数不增。local 和历史 reader 在无有效 store 时仍成功。历史 reader 合法读取后拒绝关闭，host 计数不增。 |
| 普通取锁 | 两个原 client 均在同键持锁期间返回 `CLIQueueError` 和 `DeadlineExceeded`，无 host 调用，返回零值结果。释放并确认持锁者退出后，新请求成功且 host 恰一次。 |
| 准备取锁 | 真实 `PreparationCall(true)` 的 checkpoint 内，同键竞争超时且竞争回调零次。原请求成功，checkpoint 和 host 各一次，host 收到真实 marker、15 秒命令预算及 5 秒父期限。拒绝子例保留原 checkpoint 错误和队列错误分类，host 零次。 |

测试只在构造前替换 setup 的 `ProcessHost`，使用两个构造器产生的原 client。配对文件和 Windows cache 锁键位于各例临时目录，命令回执由隔离 host 提供。测试未连接真实 Orca 或 paired 服务。

## 实际执行

Windows/amd64，PowerShell，Go `-count=1`。执行节点包含顶层及子例。

| 阶段 | 顶层 | 执行节点 | 结果 |
| --- | ---: | ---: | --- |
| 原 adapter（10 项）及 #141 复用补测（1 项） | 11 | 62 | 通过 |
| 原 protocol | 7 | 9 | 通过 |
| 旧生产实现上的新增接线测试首轮 | 3 | 11 | 测试夹具断言失败 |
| 旧生产实现上的新增接线测试修正后 | 3 | 11 | 通过 |
| 删除后 adapter | 14 | 73 | 通过 |
| 删除后 protocol | 7 | 9 | 通过 |
| 正式 Windows 构建 | | | 退出 0 |

首轮新增测试将输入的 `nil` 操作列表与 `Clone` 规范化后的空列表直接比较，在发起 CLI 调用前失败。夹具在冻结前改用已规范化的输入，复跑通过后才编辑生产代码。所有运行均无 skip。基线捕获首轮启动早于捕获工具写入完成，工具就绪后在生产文件改动前重跑成功。[命令](evidence/orca-cli-output-policy-retirement-20261111/commands.txt) · [测试名及结果](evidence/orca-cli-output-policy-retirement-20261111/summary.json) · [证据 SHA256](evidence/orca-cli-output-policy-retirement-20261111/SHA256SUMS.json)

## 文件、编码与构建身份

原清单 849 项，847 项字节相同，只有批准的两个生产文件改动，另新增一个接线测试。两处生产文件均逐字节匹配批准变换。301 个保留类型声明、代表 Profile/State/Record/Operation/envelope 编码、schema、capabilities 和资源摘要的捕获前后相同，共 340,855 字节，SHA256 为 `3cad9f3838682a35febe71c9bc2bafa479733c5857588b28fa206ff730fe6741`。私有 creation 样本比较固定 JSON，相关类型与真实编解码正文由文件范围核对。resource_digest 为 `732a1c7e211563648ea5b4d5c155345419693f4c78e29c724bb3168244714502`。[删除前捕获](evidence/orca-cli-output-policy-retirement-20261111/before-capture.json.gz) · [删除后捕获](evidence/orca-cli-output-policy-retirement-20261111/after-capture.json.gz)

正式构建记录 source_commit 为 `c404d6e5c50cc32650f9412f9b8b3449c5789e15`，source_dirty 为 true，source_digest 为 `b9027c840d246004c81f0028a065f444608bc5cd5132e1213fea63b1aae26e0c`。工作副本 EXE 的 SHA256 为 `589cbf128b29914c27310a6f620a2799f9641ce6924afbc36e7ef3a54bd97ce4`，与构建记录及通过点输入复核一致。[构建记录](evidence/orca-cli-output-policy-retirement-20261111/build.json) · [构建输入](evidence/orca-cli-output-policy-retirement-20261111/build-inputs.json.gz)

OCO-01–06 完成。验收限于上述定向集合和正式构建。公开受控、完整套件、L1/Controlled、L2–L4、全恢复、Node 及真实 Orca/模型/PR 场景未运行。本批尚未提交、推送、创建 PR、合并或部署。
