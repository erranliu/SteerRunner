# 原生目录查询与调用方边界 Plan

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

对应 [Spec](native-catalog-boundary.spec.md)，基线 `e7876c0d`，R1–R3 已实施。审计和结果唯一登记于[专属验收](../acceptance/native-catalog-boundary.md)。本 Plan 使用普通开发流程，各阶段可以独立交付。

## 1. R1 · Orca 目录查询

1. 在 internal/contracts 定义有限的 NativeCatalog 查询/响应及严格解码，在 internal/resources 从单一定义提供 capabilities.native_catalog 和自包含 schema。以 Spec 中公开字段及错误语义为接口基线，使用现有 Response 和版本协议。
2. 在 protocol/orca 内补登记项枚举和 status/agents/modelCatalog 操作，复用本机与远端连接及认证。原生类型、方法和限额与 Run 状态隔离。通过既有能力层测试证明两种连接消费同一目录实现。
3. 在小型 internal/nativecatalog owner 中编排读取，在 internal/cli 的静态/无服务路径分派 native-catalog。公开类型归 contracts，静态 schema 归 resources，owner 只依赖窄协议/进程能力，避免 resources 与实现互相导入。参数验证必须早于 host.Normalize、Ensure、Call 和任何日志/状态目录初始化。首批声明仅包含 Orca 三类查询。
4. 增加 RNC-A01–A04 和 A07 的回归，注册到既有组件/受控执行器，确认明确选择实际非零执行。保存公开 EXE 的版本、SHA256、resource_digest、完整响应与清理摘要，交付给 SteerLine S1。

## 2. R2 · Codex 模型目录

1. 核对当前 app-server 连接与进程 owner，将确有复用价值的元数据读取放在窄协议/进程边界。保持原执行 adapter 的入口、初始化和业务方法行为。
2. 实现 model/list 分页和输出映射，沿已选择的原生 executable/CODEX_HOME 执行。将本机 Codex models 增加到同一 native_catalog 声明。
3. 在 Windows 正常完成、取消、父 CLI 强制结束与超时路径核实精确元数据进程树清理。清理失败优先于目录成功，原生业务进程须有不受影响的对照。
4. 完成 RNC-A05/A06/A07 后，以同一 EXE 做 Codex 无模型联验，交给 SteerLine S2。删除为迁移临时保留的无消费者查询代码。

## 3. R3 · 既有策略目录说明

检查 model_input_contract.workflow.project_policies.slots 与 PolicySlots、请求字段校验和 Workflow 路由的同源关系。复用现有 input_contract/project_policies 测试补充必要断言，CLI 指南补充调用方预检/显式空数组示例。无须新增目录字段或更改提示词正文，SteerLine S3 可以直接消费本基线公开字段。

若实施确实涉及模型可见正文、策略路由、资源资产或对应测试/CI，记录实际完整正文差异、UTF-8 字节数前后值、业务原因及验证，并取得独立审核。更新黄金样本不能替代批准。该要求来自跨仓调用方治理约束，需在相应 PR 明示。

## 4. 验证计划

命令均从本 Runner worktree 根执行。`<out>` 是本批新建的仓库外独占目录，每次调用使用新叶目录。新增测试名为实施时需建立的用例，不表示当前已有入口。

| 项目 | 级别、环境与命令 | 时点、通过条件及证据 |
| --- | --- | --- |
| RNC-A01–A05、A08 | 组件定向，隔离进程/RPC/文件夹具。`go test ./internal/cli ./internal/contracts ./internal/resources ./internal/nativecatalog ./internal/execution/adapters/protocol/orca -run 'TestNativeCatalog|TestSRNativeCatalog|Test.*Catalog|Test.*ProjectPolic|Test.*InputContract' -count=1`；进程 owner 另跑 `go test ./internal/metadataprocess ./internal/textanalysis -count=1` | 各阶段开发时，各被测包匹配数非零，首个失败保留。测试 JSON 与 stderr 写 `<out>`，不算完整级别通过 |
| RNC-A01/A04/A06 | 公开 EXE 受控专项。先 `./scripts/build.ps1 -OutFile <out>/build/steerrunner.exe`，用当前公开 CLI 测试的独立进程机制登记并执行新测试 `TestSRNativeCatalogPublicCLI` | R1/R2 接入前。空 home、无服务、固定 pin，覆盖非法在线参数、stdin/输出限额、退出码、取消/父进程退出、源身份及零 Run/目录副作用 |
| RNC-A07 | 正式完整基线 `./scripts/test.ps1 -Level L1,Controlled -OutDir <out>` | 受影响实现完成后及合入候选。完整选择、零未授权 skip、零失败、原 Run/停止/Workflow 回归及清理成立。既有 L1 显式 opt-in/helper skip 依原框架白名单记录，不计作真实目标通过；Controlled 不得跳过。报告为 summary.json |
| RNC-A09 | 同最终 EXE 的无模型 native-catalog，分别 environments、local Orca agents/models、明确 environment agents/models、local Codex models | 各目标实现后一次联验。显式原生登录和测试用户目录，保留脱敏响应、构建身份、耗时和清理。每个目标单独结果，不推导执行资格 |
| 文档与结构 | 检查所有新本地 Markdown 链接、`git diff --check`，核对公共字段表与 SteerLine 配套 Spec 一致 | 文档阶段及每次合入前，无坏链接、无空白错误、两仓 NC 契约一致 |

纯文档阶段仅执行最后一项。实现阶段若原生协议、执行进程公共代码被调整，应补其既有 owner 测试。目录迁移本身无需真实模型/PR；若出现执行语义变化，先收窄并重新明确范围和测试安排。

实际落地新增 `TestSRNativeCatalogPublicCLI` 至完整 Controlled 及 foundation 归属，保留原 27 个根测试，当前完整集合为 28。原框架总数断言、catalog 与指南同步；新增元数据安全读取、相对 PATH 排除和父 CLI 强杀时精确子 PID/Job 退出反例。L1 发现的历史 helper 正例夹具改为原生 driver，并增加退役 helper 必须拒绝的反例，生产 gate 未调整。

## 5. 跨仓集成与发布

每次交接记录 Runner 实际 commit、EXE SHA256、resource_digest、native_catalog 中已实现组合、公开 EXE 测试结果和各目标已验/未验范围。SteerLine 使用同一固定构建核对 capability 与响应，不导入 Runner Go 包。

旧常驻服务及原 Run 继续按其原构建结算。安装切换沿现行先排空、换构建使用新状态目录的规则。本期目录接口属于无服务入口，可以在隔离 home 中独立接入验证，能力缺失仅限制相关目录功能。产品发布前须完成对应目标的 RNC-A09 与调用方页面验收。
