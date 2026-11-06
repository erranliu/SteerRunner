# 原生执行配置边界收口：Runner 验收规划

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-05，状态更新：2026-11-06。状态：实现、完整正式回归及本轮同版原生分项已验收，调用方远端正向资格已补验通过；进入用户授权的先Runner后SteerLine提交/推送/PR交付，范围见下方。不合并或部署。关联 [Spec](../spec/native-execution-configuration-boundary.spec.md)与[Plan](../spec/native-execution-configuration-boundary.plan.md)。Runner继续使用原独立worktree；下方历史文档阶段保留原时点。

Runner基线 `a66038c89208392c47531e640c84e97728c62424`，分支 `codex/native-execution-config-docs`。对应SteerLine调查基线 `9b2e5d886b8f56eaf07bc697ca78c578c91eec00`；其Spec/Plan为本轮未提交草案，NEC-A01–A10与本仓RCFG结果分别登记。实际测试前另填受测commit、固定EXE/version/SHA256/resource_digest、profile/目标及独占证据目录。

## 验收矩阵

| ID | 自动方法与最小通过条件 | 时点 / 阻塞范围 | 状态 |
| --- | --- | --- | --- |
| RCFG-A01 | 固定EXE的execution-config裸stdin/严格JSON/schema/能力/错误/限额/source/input_digest，非法在线参数拒绝，零state-dir/Run/Agent/PR副作用 | R1/R2交接，阻塞对应接口接入 | 固定 EXE/组件或静态审核通过；详见下方 |
| RCFG-A02 | 合法native源的execute保持、generate显式read_only，目标/Agent/偏好不换；完整当前gate拒绝退役launch/helper及非法权限，不清洗 | R1，阻塞derive | 固定 EXE/组件或静态审核通过；详见下方 |
| RCFG-A03 | 身份旧共同接受样本规范字节一致，模型/effort不改变context，目标/Agent/权限/程序变化区分；每份源摘要独立绑定 | R1，阻塞identity/调用方资格迁移 | 固定 EXE/组件或静态审核通过；详见下方 |
| RCFG-A04 | compose的base/default、完整target、selection/unset、权限/audit缺项、程序注入/跨组合来源及当前完整配置gate验证；源对象不变 | R2，阻塞Web构造/保存 | 固定 EXE/组件或静态审核通过；详见下方 |
| RCFG-A05 | native-catalog可信execution取非默认Codex程序，业务args不执行；agents不筛Agent/不接受agent，models一致性，原预算/清理/来源保持 | R2，阻塞目录去原生映射 | 固定 EXE/组件或静态审核通过；详见下方 |
| RCFG-A06 | 正式L1/完整Controlled及受影响owner回归；新根登记/实际非零，现有请求/Workflow/停止/重启不变，无新宽松skip | 各阶段，阻塞构建交接 | 正式 L1/完整 Controlled 通过 |
| RCFG-A07 | 最终同版EXE的本机Codex生成/单次、本机Orca和明确远端Orca单次分别保留Run/内容结果、权限及scope停止/封存证据 | 最终，阻塞整体完成声明；各链路单独结果 | 通过；各规定执行链路具同版证据，调用方远端ready/重启/准入/清理在live-remote-orca-04补验通过；不代表通用完整原生级别 |
| RCFG-A08 | 文档/共享字段/owner检查成立；实现影响提示词时实际正文与字节及独立审核齐全 | 文档阶段只做静态检查；不替代代码验收 | 固定 EXE/组件或静态审核通过；详见下方 |

执行命令与驱动补齐要求见Plan §4。新操作已实现，固定构建与正式/原生结果在下方分项登记；未执行或失败项不得推定完成。无需新增人工必验项，环境或自动驱动缺失不改称人工后置。

## 独立文档审核来源与本次检查

SteerLine原稿经独立只读子代理审核并确认修订六项：保存缺逻辑edit输入、自定义Codex目录程序上下文遗漏、自动材料预检遗漏、历史纯展示例外、源摘要/跨对象context比较歧义、审计模型草稿建议权属。定向复核确认修复；目录真实路径和agents/models选择器文字同步更正。

本机Runner源码进一步核对发现helper已退役，独立代理确认删除program_binding_digest/integrity_checks，不应为退役能力新增公共协议。共享CLI正文从修订后的SteerLine Spec逐字同步；独立子代理`audit_native_config_design`已完成本仓新增文档与两仓契约/阶段/职责的只读复核，通过且未发现剩余阻塞性设计问题。以上是设计审核，不代表生产契约签收或运行资格。

本次文档检查结果在实际运行后登记。原始测试将保存在仓库外独占目录，记录命令/实际选择、源码/产物身份、结果、失败与未验、Run/request/stop及清理，不向Git添加完整原始日志。

2026-11-05文档检查：本仓四个改动Markdown的20个本地链接全部存在，UTF-8/行尾空白/末尾换行检查及`git diff --check`通过；与SteerLine的共享CLI正文及目录execution段逐字比对通过。SteerLine全库文档链接检查亦通过（694文件、4284链接、0错误）。本仓仅有三个新Markdown及原生目录Spec导航变更，未提交/推送，未构建或运行L1/Controlled/模型/浏览器，RCFG-A01–A08仍待实现验收。

## 2026-11-05 实施、固定构建及结果

独立 worktree 与分支保持不变，源为基线加本轮 dirty 修改。最终固定 EXE 在工作区同级 `runner-evidence/config-handoff-build/steerrunner.exe`，version `0.2.0-dev.4`，SHA256 `fee54d50352caa5df832ddbbd250250810a4cb7baa3c83150e697041f1f988aa`，resource_digest `b0a833884534996049477c43969acf9ba40bbe3ca72edfceab9b42d96740f344`，build source_digest `8ea283162b74db21d20ddbd6c7e1fb0cb929ceea492f56c7882432ebf9f2d2ab`。构建回执保留实际 dirty 文件清单；基线 commit 不冒充完整源码身份。

三个操作、严格 JSON、source/input_digest、schema、1 MiB/30 秒总期限、原生目录 execution 上下文及同源 reasoning_efforts 已实现。identity 省略去偏好后为空的 agent_launch 容器是已审核的明确规范化例外；原 execution 不变，compose 的 base 保留隐式 Agent 和 launch 容器存在性。无需通用转换框架、程序摘要/helper协议、身份表或服务。

原始报告在上述 worktree 同级 runner-evidence，公开测试固定 `-RunnerExe <最终EXE>`；SteerLine 以相同 EXE 独立验证。组件 contracts/resources/cli 和受影响目录 owner 测试通过；public-handoff 的两个公开根通过，包括真实无 EOF stdin 30 秒超时、Orca偏好 compose add/unset 身份不变、退役字段拒绝、自定义 Codex 程序及零业务副作用。正式完整命令 `./scripts/test.ps1 -Level L1,Controlled -OutDir <同级runner-evidence/formal-handoff>` 的 L1 已通过：873 个测试，application 1556.882 秒，覆盖率80.8%；16个既有 opt-in skip 按原白名单保留，Controlled 必须零 skip，其最终结果另记。

早期 formal-final 失败保留：两个既有测试夹具把 plain implement 提示与 audited资产混用、或修改 SuccessOutput 但没有同步冻结 ResultInstruction，已由独立审核确认仅修正夹具，生产正文零变化；并发 wrong-head 查询偶发未启动的隔离原根补测通过。formal-02 是中间源码，不代替最终正式报告；早期其他构建的摘要不拼接成最终同版资格。

提示词实际 diff=0：internal/execution/workflow/prompt.go 3273→3273 UTF-8字节，definition.go 5472→5472，assets/prompts.json 1123→1123；plain implement 正文36→36、audited正文62→62。业务原因仅纯配置/CLI接线迁移，未改生产提示词/工作流；上述两个既有夹具按真实生产组装校验。独立只读子代理 audit_native_config_design 审核生产门禁、对应测试、实际正文/字节及跨仓职责通过，未靠更新 golden 批准行为变化。

调用方真机项目环境、本机/Orca组合及配置页只记录其独立结果，不冒充本仓完整 L2/L3 资格。RCFG-A07 的生成采用同级独立公开 CLI 驱动 native-configuration-generate.py：`python <driver> --runner <最终EXE> --source-profile <sl-evidence/live-inputs/local-codex.json> --out <新的runner-evidence/native-generate-NN>`；冻结原源和derive，实际只读submit与原结果、同EXE重启、原Run停止、service verify-stopped。无库内模型映射、无 Git/PR 写入，完整命令/信封逐条存证，未知输入不重发。结果完成后另记；它是 focused 公开链路，不能声明完整原生级别通过。

最终完整 formal-handoff 已完成：summary.status=passed、validation_scope=complete；L1 exit0、873测试、21个terminal扩展、framework及zero-match守卫通过；Controlled实际29个顶层根、exit0、0 skip，2026-11-05 12:39:47 UTC完成。该正式构建公开日志核对的 SHA/resource_digest 与最终交接 EXE 完全相同。

focused native-generate-02 通过：原 Run `7d24913b665aa47f07dad6d20caad5c2`，derive的read_only输入经完整validate→唯一submit→verified结构化结果；同版服务重启后execution/input_digest/content_result一致，原Run显式stop，service verify-stopped.resources_settled=true。native-generate-01的驱动最初把inspect data当Run而遗漏data.run，模型实际已生成并stopped/closed，驱动仍按失败保留；没有重发未知输入，其driver-recovery-cleanup.json另证原服务及资源停止。此修正与生成结果均经独立只读审核，不把focused结果当整个原生级别通过。

跨仓实际结果补充：SteerLine live-local-codex-02 正式通过（execute、同home双端重启、原Task Run停止及清理），本机Orca03已得到实际ready/停止但调用方测试清理遇自有报告文件残留，该失败不计整体通过，严格归档修复另轮验收。远端 live-remote-orca-02 在明确目标e68f4edf-8a83-42c2-a9da-6d404d3aa3d8的原Run c553d5f7361ac741cf70f9382c994d38 完成脚本、最终原结果/停止/封存和清理；项目报告ready=false（node.exe/npm.cmd不在目标PATH），调用方正确阻断后续资格。它证明本次实际执行与负向门禁，不证明远端项目正向ready或完整原生级别通过；未修改目标安装。

调用方本机Orca新轮 live-local-orca-04 正式通过（254.24秒），同最终SHA，实际检测/原Run、同home双端重启及严格自有报告归档/工作区/注册/原Runner收尾均通过。RCFG-A07的本机Codex生成/execute、Orca本机execute和明确远端execute已各有同版公开运行证据；远端项目ready=false和调用方正向L4失败保持，不宣称完整通用L2/L3或整体产品原生资格。

最终调用方受控配置 L3-configuration-04、固定 L1-complete/L2-delivery-final 也通过。收尾仅 gofmt 了已审核 route_test.go 测试夹具的格式，生产代码/提示词及最终固定EXE无变动。独立源码/协议/提示词及接线复审无剩余代码阻塞；未提交/推送、发布或部署。远端目标的Node/npm PATH前置仍不满足，不宣布调用方整体原生资格完成。

## 2026-11-06 调用方远端正向补验

用户授权调查、修复目标 Node/npm 并重跑，暂不提交或部署。继续使用原独立 worktree；本轮未改 Runner/SteerLine 生产代码、提示词、检测入口或最终固定EXE，SHA256仍为 `fee54d50352caa5df832ddbbd250250810a4cb7baa3c83150e697041f1f988aa`。两仓受测源码仍是前述基线加未提交改动，不把基线当作完整源码身份。

同级 `sl-evidence/remote-node-investigation` 保存远端 Process/User/Machine PATH、工具解析、标准安装/管理器目录、注册表和winget清单：目标用户缺少可用 Node/npm 安装，没有发现已有安装的PATH未继承证据。官方Node24.22.2 MSI因未提权错误1925回滚，未提升或重启Orca。改为官方Windows ZIP并核对官方SHA256 `ca1bff5d8ead13c9f2ea404a176db46bd430704d5b5113734049fa365e7799e2`，安装到已有用户工具目录 `C:/Users/Administrator/.local/bin`，37个原工具文件哈希不变。初次持久PATH追加在末尾导致Python Store别名优先，live-remote-orca-03实际Node/npm已通过但Python查询失败，原Run/结果/清理通过，正式失败保留；仅将本次新增PATH项移至用户PATH首位后，新终端五项工具版本查询通过。Machine PATH、业务配置/材料/脚本不变，无临时进程PATH注入。

新正式报告为同级 `sl-evidence/live-remote-orca-04`，入口 `go run ./cmd/test-feedback -level=L4 -suite=testing -env=live -scenario testing.project-environment-orca -out <该完整绝对目录>`，固定EXE及显式 remote-orca/profile/600秒输入与调用方前轮一致；原fixture源SHA仍 `9540eb6cd1ac3bd1276a6e77b403bfc8dcb50f6a`，本轮准入探针fixtureSHA `ff9bb231ce1dc38aae130a2507e74fe79dac8467`。完整输入与命令见 remote-input.json/managed 报告及调用方专属验收第8节。

目标保持 environment `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`、runtime `21d98aa2-d284-41f3-a6cc-42fb22d728a2`、repo `903fb315-7d5e-4246-8913-816e1d3bdf03`。检测Run `ca30e5b64ceeb184d2c94b619181fb78` 一次实际项目入口返回completed/ready=true（Node24.22.2/npm11.19.1及其他工具通过），Run stopped、原结果closed/version1。同home重启SteerLine/Runner后完整Prepared/identity/源及派生execution、原请求字节、RunID/input_digest及原封存Result一致，仍单轮ready；实际Task准入Run `1851918e61d19eccafa6d74ae165d067` 后显式停止。正式summary passed=true、cleanup_complete=true、exit0，唯一根通过、0skip，场景173.49秒；三个自有测试ref/原Runner资源收尾confirmed=true，无业务发布PR。

由该新报告补齐本轮同版远端正向证据，RCFG-A01–A08及调用方NEC-A01–A10规定项均已签收；不推定其他Agent组合或本仓通用完整L2/L3通过。诊断及旧失败原输出保留，诊断自有终端均有ptyKilled回执。只更新验收/Spec/Plan状态，不提交、推送、发布或部署。

## 2026-11-06 两仓最终实现独立审核

新独立只读子代理 `final_cross_repo_audit` 按用户要求重新审核两仓完整未提交实现、职责/调用链及原始验收证据。Runner三操作、生产配置gate、通用identity及原生目录程序上下文未发现新的确认缺陷；调用方资格/冻结/事务与saved-base授权主边界成立。实际最终EXE SHA、formal-handoff完整L1/29根Controlled、公开两根及生成专项、本机/远端原ready/停止/封存/重启及准入证据均独立核对成立。

确认并由主代理修复两项SteerLine前端P2：目录按首个同组合profile查询，可能错用第二方案的程序上下文；自动默认模型清空unset，可能使无effort模型重新继承保存base旧effort。修复后按可信base ID及完整目标选目录，base/保存摘要变化废弃旧查询；只清除已赋值字段对应的unset。新增三例覆盖多程序选择/复制、412显式重读后晚到响应、实际compose与保存保留effort清除，原20例断言保持。新夹具中退役字段经独立复核移除，仅影响新增案例；同一固定Runner公开describe实测三份base均通过，证据在同级 `sl-evidence/final-audit-20261106/fixture-public-gate`。

本轮Runner生产/最终EXE及SteerLine后端/提示词无变动。新增前端夹具复用原完整profile，`preserve`正文文字diff=0、8B→8B，无新规则或golden更新，经独立审核。最终web-unit-02正式288/288通过（配置页23例）、生产Web构建exit0、L1-01及L2-01通过且清理完整；新配置浏览器L3另行记录。不继承旧UI通过覆盖新场景，不把调用方focused验证冒充本仓通用完整原生级别。独立生产/回归复核无剩余确认问题；未提交/推送或部署，原独立Runner worktree继续保留。

调用方新 `final-audit-20261106/L3-configuration-01` 最终正式passed=true、cleanup_complete=true、exit0、0skip，execution216.53秒。两阶段真实浏览器保存/显式重启加载、两方案候选输入的实际消费/冻结、旧候选原输入保持与原Runner清理通过；独立复核核对summary/managed摘要、浏览器原日志及候选原输入。本轮快照中Codex已有Run，Pi取证止于候选输入冻结，不宣称两种Agent都已启动Run。该受控配置流程不冒充真实模型或本仓完整原生级别，本仓生产及固定EXE继续不变。

最终两仓diff及共享CLI正文检查通过；SteerLine694个Markdown/4290个本地链接、0错误。原两仓HEAD及Runner固定SHA均不变，最终审核/修复/测试与源码摘要存同级 `sl-evidence/final-audit-20261106/verification.json`。两项P2均经独立复核，无剩余确认问题；未提交或部署。

## 2026-11-06 PR交付与两仓依赖

用户随后授权按Runner→SteerLine分别提交、推送和创建PR，暂不合并或部署。本仓继续使用原独立worktree及分支 `codex/native-execution-config-docs`；SteerLine下游分支为 `codex/native-execution-configuration-boundary`，实际PR互链及交付commit在PR正文登记。

本仓先提供公开 execution_configuration 三操作、identity、schema/能力及native-catalog可信execution上下文；SteerLine依赖这些能力消费配置、冻结业务资格并授权Web保存。本仓不依赖SteerLine代码或配置即可构建/运行；调用方需先选定包含本PR能力的Runner构建，不支持时明确阻断新操作，已有冻结请求观察/停止保持。两仓PR的评审/未来合入顺序为Runner在先；本次仅建PR，不启用自动合并或部署。

前述正式和原生报告保留其原调查基线加dirty源码、固定EXE SHA及实际范围；提交不会把这些原始报告改写成已在干净提交上重跑，也不自动为后续main变更或新Runner产物授予原生资格。创建PR前刷新目标main并核对实际内容合并状态；如需解决冲突，另记集成改动和补验，不覆盖原失败/通过证据。

创建PR前已刷新到 `origin/main=d58054a78471211c1eeb86c0eb1396ae870d6ca1`，在原worktree重放本轮提交，生产提交为 `08ca252ee82960fd7995f49e4f790ef7489bfbca`。仅三处真实冲突：两份测试目录文档保留双方说明，CLI help在main命令表增加execution-config，保留main已退役estimate-run-cost的事实。resources及测试机器清单自动合并后仍为29个Controlled根；实际merge-tree预检无冲突。

新集成补验存同级 `publication-20261106`：contracts/resources/cli/nativecatalog四个owner全量286条通过，3个既有显式原生用量opt-in根跳过（LocalCodex、LocalPi、RemotePublicLive），未把跳过当作真实通过。干净生产提交08ca252e独立构建的EXE SHA256为 `c279ca725c9486bd50a2d183ca8dcd71c026f5b455a4b45ab9f5a24f1442c5b5`、resource_digest `feca8807ec988d60de79f426818e86282deefa01f7d4b3c9971eb83c63a6e508`。公开foundation两根execution-config/native-catalog实际通过、exit0、0skip，含真实stdin30秒期限；源提交/产物/原日志均冻结。此轮补验是受影响owner和公开接口，未重跑整套L1/Controlled或真实模型，不继承旧EXE的原生资格；本次只交付PR，不发布或部署。
