# 再审计：权限资源与原完成证明职责收口

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。审计 [Spec](../spec/native-proof-boundaries.spec.md) / [Plan](../spec/native-proof-boundaries.plan.md) 的 A/B 实现及[初轮验收](native-proof-boundaries.md)。首轮再审计基线 main `da8155e0d1fe871527bf29d63f74bfe5252c545b`；提交前整合至 `efa871cfffb950505bf2f0a325ec7751fb4be7f3`，最终验证见 §4。分支 `codex/native-proof-boundaries`。本次用户授权再审计、修复确认问题后提交提 PR；不自动合并或部署。

## 1. 结论与范围

未发现本批产品行为缺陷，5 个生产文件与初轮实现相同。补充一项结果提交边界回归，覆盖缺原证明与 stop 先到；原实现无需放宽校验。da8155e 阶段 33 个定向及三公开场景通过；最终 efa871c 整合基线 38 个定向及三公开场景也通过，见 §4。以下结论仅限 A/B 收口，不代表所有 application 私有知识已消除，也不声明完整应用组/L1/L2/L3 通过。

| 重点 | 审计结果 |
| --- | --- |
| A 私有解释 | application 不再扫描 sandbox_mode 或读 profile.AccessMode；Codex 先严格解码资源，再解码冻结 profile，最后复用 decoder 已证明的 contract/mode 关系。两种合法模式和交叉模式、错误优先级保持；构造与路径接线仍留在原宿主，不声称零具体导入 |
| B 私有解释 | workspace_source.go 不再知道 owner 类型、WorkspaceCheckpoints 或签名格式；engine.go 仅作一处静态转接。适用性和读取在 Codex，仓库/分支/base、实施来源和审计祖先关系留在业务侧 |
| 缺失原证明 | 适用的原合同读取失败返回 required=true/error，application 先返回 nil 等待，缓存及当前 Git 后备不可达；合同不适用才走原后备。保留原 ExecutionOwner 的既有解码分类，不新增历史绑定补造或可选端口缺失放行 |
| 实际结果边界 | 新回归经真实 e.finish：无证明时不写 StageResult/AcquiredResult、不增加输入/意图；原签名证明迟到后仍按原来源接收，有授权才创建后继；已提交 stop 时只补收原事实，不恢复 Allowed、不改变原 stop 或创建后继 |
| 消费者/夹具 | 零原生产声明删除；ExecutionOwner、ReadExecutionCheckpoint、签名 writer、Git/common-dir/runtime、Orca 后备、原 stop 与独立 owner 无 launcher 路径保留。相对 da8155e 的 351 个既有测试文件未改，最终 efa871c 的 353 个也无本批改动；只增加本批两个新测试文件，其中本轮仅追加一个顶层回归，全部原断言/共享 helper 保持 |
| 持久/可靠性 | 全部持久类型/标签及原编解码/签名写入未变；只有两个原协调函数改变，三个窄函数新增，无 Record/Store/提交回调进入 adapter。原提交、单确认、Execute/Reconcile、恢复预算/授权和停止正文保持 |

没有为制造净删而撤销可靠性规则，也没有把原完成证明换成当前 HEAD。B 的 required 是原合同判断的瞬时返回值，不写入持久根、不构成第二业务状态。PC-R01–R06/R08/R09 保持。

## 2. da8155e 阶段基线与源码保护

开始时 HEAD `95e53cb` 已在 main 以 `761f916` 合入。获取 main 后发现 `da8155e` 仅更新 12 个测试接线/文档文件；使用 rebase --autostash 同步并自动跳过已合入提交，恢复全部工作树改动。更新前后 5 个产品文件及两个新增测试文件 SHA 一致；之后仅在新增 native_proof_boundary_test.go 追加本轮回归。没有修改本批以外的源码或上游测试入口。

当前生产 diff 仍只有 5 文件，41 个保留声明（9 个类型）相同，两个协调函数修改、三个指定函数新增、零删除。read/write checkpoint、policy decoder/构造器、binding 摘要及未受影响的持久/执行/恢复/停止源码保持。原共享夹具没有为适配新接口而修改，因为没有增加 mandatory LifecyclePort 方法。

代表 Run/Request/Receipt/旧字段/持久根/能力编码仍为 145263 字节，SHA256 `a5ec2f5e94487c7b4cea1711a50b23a423bc41aaf0c2d5444e0b9ec5284c7c16`，与初轮实施前一致。state/v8、user_version=8、CLI v1、profile/policy/owner/checkpoint 协议不变。证据：[声明](../../../../acceptance/evidence/native-proof-reaudit-20261102/declarations.json)、[范围/夹具](../../../../acceptance/evidence/native-proof-reaudit-20261102/scope.json)、[主线更新](../../../../acceptance/evidence/native-proof-reaudit-20261102/main-update.json)、[编码](../../../../acceptance/evidence/native-proof-reaudit-20261102/wire-comparison.json)。

## 3. da8155e 阶段验证

| 项目 | 结果 |
| --- | --- |
| Codex/application 定向 | 33 顶层通过，0 fail/skip；含全部本批边界、原权限模式/签名、checkpoint/原子结果、未知/缺预算原对账、迟到创建、原 stop/手工对账及离线核验 |
| 新增结果提交回归 | original-continuation 与 stop-before-proof 两子项均通过，验证实际 e.finish，不只验证 reader 返回值 |
| live 编译 | tests/native 与 tests/integration 均 `-tags live -c` 通过，不计真实后端执行 |
| 正式构建 | build.ps1 通过，最终源码输入与构建回执一致 |
| 最终 EXE 三公开场景 | ServiceOwnership 43.73 秒、NativeUnknownAndStop 25.92 秒、RestartPreservesAuthorization 99.55 秒；3/3、0 skip、exit 0 |
| Controlled 计时导入 | 仅导入已完成场景；qualification_eligible=false，不增加完整 Controlled 资格 |

da8155e 阶段没有新的测试失败。初轮新增夹具修正、组件 8 个既有 skip、完整应用组的缺 Budget 旧夹具失败和 6 分钟总时限耗尽仍保留于初轮报告；该缺预算测试在原基线已复现，当时的 main 更新未改变其测试或生产路径。后续主线修复见 §4。本轮不重新跑整个应用组、不改旧预算规则或共享夹具，不把定向成功改称完整应用通过。

Runner SHA256：`b6c52f207b806de0310be20e79fb54cd453a59dfba1923cb8e9a87c7638480ea`；输入 digest：`b80f990e5e14ee200e1e9b6cea6dbf91596ad7bc3b8fff141e02577b932572f6`。最终 EXE 依据 `da8155e` 加本批冻结源码构建；Go 1.26.4 Windows/amd64。证据：[测试摘要](../../../../acceptance/evidence/native-proof-reaudit-20261102/test-summary.json)、[命令](../../../../acceptance/evidence/native-proof-reaudit-20261102/commands.json)、[构建](../../../../acceptance/evidence/native-proof-reaudit-20261102/runner-build.json)、[公开结果](../../../../acceptance/evidence/native-proof-reaudit-20261102/public-result.json)。原生产实现未修订，后续按用户授权提交、推送并提 PR。

## 4. 提交前整合：efa871c

提交前 main 又合入 generate 文件材料、旧 L1 夹具修复和 Restart 选集。固定整合 `efa871c`，A/B 的 5 个生产文件及两个新测试文件在 rebase 前后 SHA 保持；没有用本批修改覆盖上游。原缺预算测试已由 main 的 b566847 补齐合法冻结 Budget，本批不重复修复或放宽校验。初轮失败是历史事实，最终整合后该项定向通过。

相对该固定基线，353 个原测试文件没有本批改动，A/B 仍只有两个原协调函数修改、三个窄函数新增。新增材料不改变本批的私有知识职责、原证明等待或停止行为。

分别用 main 原源码 overlay 和本批源码捕获后，代表编码均为 145494 字节，SHA256 `2765fff305a58706195cb1b7c2d381d709af4f2ed44492181b4d050f6729b56a`，完全一致。与初轮 145263 字节的差异仅在 capabilities、resource_digest 两个顶层项，来自主线新增能力；原持久根捕获没有变化，不能把上游能力增量误计为本批格式变更。

最终基线 38 个定向顶层通过，0 fail/skip，额外覆盖主线新文件材料入口及已修复的旧预算夹具。live 编译和正式构建通过。最终 EXE SHA256 `22119449bcf163b0eae17053e01f10c5128034d824455a92e00a18a2f602d72b`，输入 digest `8d1d984f2efd706496673f19975e60b9730951181fe464772916f8e7c1ab5419`。

同一最终 EXE 的 ServiceOwnership 33.52 秒、NativeUnknownAndStop 20.91 秒、RestartPreservesAuthorization 91.96 秒，3/3、0 skip、exit 0；Restart 的 graceful/kill/creation_receipt/pending_stop 四 case 均通过，并按最新主线计时约定导入，不授予完整 Controlled/L1/L2/L3 资格。证据：[最终测试摘要](../../../../acceptance/evidence/native-proof-reaudit-20261102/integrated-main/targeted-summary.json)、[原基线/当前编码](../../../../acceptance/evidence/native-proof-reaudit-20261102/integrated-main/wire-comparison.json)、[上游捕获差异](../../../../acceptance/evidence/native-proof-reaudit-20261102/integrated-main/upstream-wire-changes.json)、[最终构建](../../../../acceptance/evidence/native-proof-reaudit-20261102/integrated-main/runner-build.json)、[最终公开结果](../../../../acceptance/evidence/native-proof-reaudit-20261102/integrated-main/public-result.json)和[最终命令](../../../../acceptance/evidence/native-proof-reaudit-20261102/integrated-main/commands.json)。本报告在提交前定稿；后续按用户授权提交推送及创建 PR，不自动合并或部署。
