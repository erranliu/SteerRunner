# 原生执行配置边界收口 Plan（Runner）

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-05，状态更新：2026-11-06。状态：实现、完整 L1/Controlled 及本轮同版原生分项已验收，调用方远端正向资格已补验通过；范围见专属验收。对应 [Spec](native-execution-configuration-boundary.spec.md)及[专属验收规划](../acceptance/native-execution-configuration-boundary.md)。基线 `a66038c89208392c47531e640c84e97728c62424`；独立分支 `codex/native-execution-config-docs`。用户已授权先Runner后SteerLine分别提交、推送和创建PR；不合并或部署、不切换安装现场。

调用方配套为 SteerLine 的 `SteerlineFlow/spec/native-execution-configuration-boundary.spec.md` 和同名 Plan，源调查基线 `9b2e5d886b8f56eaf07bc697ca78c578c91eec00`。其逻辑编辑/保存、profile库、项目资格、冻结和SQLite事务由该仓实施；Runner只提供公开纯配置协议与目录配置上下文。

## 1. 开工前契约签收

1. 逐项核对两仓 Spec 的共享 CLI 字段、缺省/null/unset、JSON规范化、权限、源/响应摘要、source、限额、错误和阶段。共享CLI正文已同步并独立审核；实际源码与公开输入/输出样本见专属验收。
2. 记录当前生产 gate 对合法 native Orca、退役 launch/helper、DSH、本机Codex read_only及权限缺省的接受/拒绝。不得把较宽的 ValidateExecution 当作完整原生配置语义，也不清洗退役源。
3. 对identity列共同接受的源/旧context规范字节、模型/effort变化、Agent/目标/程序/权限变化与空对象/缺省/显式值对照。身份只用于通用配置等价，不决定项目资格。
4. 固定每阶段实际操作和能力；R1只describe/derive，R2再compose/目录入参。不建设program_binding_digest、integrity_checks、helper探测、身份表/注册库或新缓存服务。
5. 接受当前Runner gate的输入才可进入派生；若调用方旧配置被当前构建拒绝，保留具体诊断，不增加兼容转换。旧活动/原资源由原构建处理，不把文档工作当作升级授权。

## 2. R1：最小配置投影与派生

| 位置 / Owner | 修改与复用 | 退出条件 |
| --- | --- | --- |
| internal/contracts | 新增execution_configuration的有限请求/响应/identity纯值及严格解析；复用现有Response和JSON约束 | 无Task/profile/项目配置依赖；非法/重复键/null/未知字段和摘要样本有测试 |
| 现有配置解析/生产gate | 从contracts/run.go及adapter配置中抽取确需共享的纯配置语义，避免再写支持矩阵 | 不改现有submit判定；退役字段仍拒绝，缺正文/审计参数不误触完整Run gate |
| 小型纯配置实现 | 实现describe的view/identity与derive的execute/1保持、generate/1 read_only；仅现有消费者所需 | 原输入不变、原目标/偏好不换、权限显式、无提示词/材料/发布构造 |
| internal/resources | 从同源配置描述发布execution_configuration及自包含schema/有限derive组合 | 在线/离线静态内容同源，引用既有capability_catalog，实际支持与schema/gate一致 |
| internal/cli | 新增固定execution-config无服务分派及严格选项，早于host.Normalize/Ensure/Call | 无state-dir、服务、Agent/Run/回执/PR；严格预算、source与安全错误 |

纯配置实现已放在 internal/contracts/execution_configuration.go，由 resources/cli 组装；它不得导入application、storage、host、Task或执行资源owner。不为一组纯函数引入工厂、插件注册或完整adapter初始化。若复用会形成resources/contracts循环，先把共用纯定义放入真实owner，再由CLI组装；不复制生产规则避开依赖。

R1只返回view/identity和派生execution。对合法native Orca execution通常原样返回；generate产生明确read_only并只复制合法偏好。identity把完整源摘要与不含模型偏好的context分开；活动与新候选可有不同源摘要但相同context，Runner不替调用方作ready结论。

## 3. R2：逻辑构造及目录上下文

1. 复用R1解析与配置规则实现compose；base必须匹配adapter、完整target/environment和Agent，或明确采用公开默认。selection/unset仅有限逻辑字段，无任意程序参数/patch；access_mode显式，完整workflow由调用方提供，缺audit_model拒绝而不擅自代选。
2. 构造后用同版完整RunConfiguration验证，再返回execution/view/identity。合法程序来源由调用方的可信base/default和完整重新构造比较保证，不新增程序授权摘要或票据。
3. 在既有native-catalog请求增execution字段：environments保持原kind-only；agents仅核对adapter/target，拒绝agent且列完整目录；models另核对agent。与顶层executable互斥，来源只改变Runner内部解释程序上下文，不执行传入业务args/helper，不创建thread/turn。
4. 复用现有Codex metadata程序选择及nativecatalog/metadataprocess；自定义Codex executable继续用于原model/list。Orca目录仍沿原协议与目标，配置中的Agent不得使agents列表被过滤；无profile请求保留原选择器用途，不用其掩盖一个失败的profile查询。
5. 更新公开schema、能力、CLI帮助及配置说明，只发布真实实现的操作。字段不支持/来源不匹配明确拒绝；不改原30秒查询预算、1MiB结果预算和精确元数据清理。

## 4. 测试、分级与证据

遵循 [testing.md](../../../../testing.md)。命令从本Runner worktree运行，`<out>`为本批新建的仓库外独占目录，例如`C:/SteerlineDebug/wt-20a4-runner-config/`下各次新叶目录。下面新测试根及纯配置包为需补齐的实施内容，先固定实际包名/驱动并登记非零选择；不把计划命令当现有能力。

| 验收项 | 级别 / 环境 / 命令 | 时点及通过条件 |
| --- | --- | --- |
| RCFG-A01–A04 | 组件：`go test ./internal/contracts ./internal/resources ./internal/cli -count=1` | R1/R2开发，三操作按阶段选择；全部受测包非零，JSON/权限/identity/default/unset/当前gate和原输入不变 |
| RCFG-A01/A06 | 公开受控根`TestSRExecutionConfigurationPublicCLI`；登记至Controlled/foundation。`./scripts/build.ps1 -OutFile <out>/build/steerrunner.exe`后，`./scripts/test-foundation.ps1 -Pattern '^TestSRExecutionConfigurationPublicCLI$' -RunnerExe <exe> -OutDir <独立out>` | 各阶段交接，固定真实EXE的裸stdin/错误/限额/身份、非法在线选项、取消及零业务副作用。只查公开CLI/可观察输出和自有目录，不读私有DB |
| RCFG-A05 | 组件：`go test ./internal/nativecatalog ./internal/metadataprocess ./internal/contracts ./internal/resources ./internal/cli -count=1`；扩展原`TestSRNativeCatalogPublicCLI`并用foundation明确选择 | R2，非默认Codex程序保持、业务args零执行、agents不滤/不收agent、models身份冲突、取消/父退出/清理和原限额成立 |
| RCFG-A06 | 正式完整基线：`./scripts/test.ps1 -Level L1,Controlled -OutDir <out>` | 各阶段完成/交接，按当前完整集合核对，不删既有根/断言/预算或放宽skip；已有opt-in按白名单记录，Controlled不跳过 |
| RCFG-A07 | 同一最终EXE：用本仓现有公开Run/内容测试驱动接入derive→submit→inspect/result→stop；本机Codex generate与execute、Orca本机/明确远端execute分别固定配置。缺真实单次/生成驱动时仅补公开入口最小驱动，精确命令与正式/focused登记在执行前补齐 | 最终，保留源/派生输入、read_only、实际原结果与scope停止/封存；不以development smoke或metadata成功替代execute/generate；调用方真实项目资格由SteerLine独立验收 |
| RCFG-A08 | 文档链接/空白、共享协议正文比对；实现阶段加owner依赖和提示词实际正文/字节核对 | 文档阶段仅静态检查；实现若影响固定正文/接线/资产/对应测试，须独立审核，更新golden不替代批准 |

新根同步catalog/执行器/foundation归属、framework集合校验和test-catalog/指南。真实依赖只选择受影响组合；不复制SteerLine的Task/DAG/资格/浏览器测试，不默认扩大全部L3/L4。定向结果不宣称完整级别或发布资格，缺能力/零匹配/失败/未清理如实登记。

## 5. 交付、当前文档工作与非目标

每阶段交付固定commit、version、EXE SHA256、resource_digest、operations/派生组合、完整schema/正反例与RCFG结果。SteerLine用同一个EXE独立核对NEC接口/产品链；两仓证据不互代。接口能力缺失不得阻断已冻结Run的读取或停止，也不授权重发输入。

已在原独立worktree完成纯配置接口与目录扩展、组件及公开CLI测试；固定构建和正式回归/原生范围逐项登记，不处理安装原活动、不部署。没有独立配置服务、部署注册库、业务ready接口、任意程序编辑或兼容迁移。
