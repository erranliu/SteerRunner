# 开发 full_access 收敛：实施与验证

本记录固定实施阶段的源码及结果。后续发现及修复、当前交付构建和复测见[实施后审计](development-full-access-audit.md)；以下构建摘要与通过结论不自动继承到审计后源码。

日期：2026-10-31。源码基线 `a086a9984dc0bd32b3f02b9698cd26c9ba313d90`，本次修改位于未提交工作树。契约见 [Spec](../spec/development-full-access.spec.md)，执行映射见 [Plan](../spec/development-full-access.plan.md)。

## 实施范围

- 开发准入、公开 schema/catalog、客户端候选和测试基线要求显式 full_access；sandbox、缺省、空值/null 拒绝，不自动迁移冻结请求。
- 删除开发 workspace-write/MXC 和 Git/认证目录写授权、Orca 原生 sandbox 启动包装。Pi/DSH 工具通过原 owned PowerShell host 执行，运行及能力声明均不再依赖 Codex sandbox EXE。
- read_only 内容、独立 parser、GitHub broker/socket/期限/环境过滤和原停止清理保持。开发 policy v11；read_only 沿 v10；Pi/DSH tools v2、metadata 4、receipt 3、local profile 5。
- 旧 sandbox/隐式 Run 的后台推进、PR refresh、采集、控制及原生会话 open 被拦截，读取只暴露持久事实；原授权不因服务加载而改写。精确 stop 使用原 owner/transport，旧 sandbox 回执仅在 stop-only decoder 中读取，不运行旧开发逻辑。退役本身不证明原进程或认证已经结算。
- 合并两组重复场景及两项 active restart 权限重复 case。L3 从 25 个 scenario / 35 个 case 收敛到 23 / 31；正常/强杀与 active/boundary 窗口及其独有断言保留。

## 验证记录

| 项目 | 结果与范围 |
| --- | --- |
| 正式 L1 | 通过：720 项 Go 顶层测试、29 项 Node；framework、native driver 编译及零匹配 guard 通过；17 项仅限脚本既有 allowlist 的 helper/opt-in skip。原始目录 `.evidence/full-access-l1-qualified`，便携 [L1 摘要](evidence/development-full-access-20261031/l1-summary.json) |
| 终端扩展 Node | 29/29 通过，无 skip；`.evidence/full-access-diagnostics/.full-access-node-2.log` |
| 新旧模式、schema/catalog、退役及回执版本定向 | 通过；包括新旧字段混用/null、old sandbox 仅精确 stop、Pi/DSH 无 Codex 的冻结准备 |
| 生产 CLI 权限拒绝 Controlled | 1/1 通过；`.evidence/full-access-permission-controlled/result.json`；无 Run、原生输入及执行资源准备 |
| 完整 Controlled | 最终固定 Runner 25/25 通过，无 fail/skip、重复或缺项；results 1 项 + rest 24 项，核对精确注册集合；[便携摘要](evidence/development-full-access-20261031/summary.json) |
| 原生认证，无模型 | `TestAuthenticationNativeToolBoundary` + host broker 通过，最终源码复测 17.76 秒；原现场 `C:/srt/auth-boundary-1183266220`；只读 Git/GitHub/API socket、提交及收尾，无真实 PR 发布 |
| 原生 Git，无模型 | `TestLinkedGitNativeCommit` 通过，最终源码复测 5.38 秒；loose/packed 两项提交；独立 peer 未被正常提交修改，不再声称 peer 拒写；原现场 `%TEMP%/sr-git-boundary-3656240732` |
| 真实 L2 / Orca local、remote L3 / read_only、parser 真机专项 | 缺实际测试配置，尚未执行；不以原生无模型诊断、历史证据或受控场景代替真实交付资格 |

首轮正式 L1 暴露了终端测试 helper 仍输出 metadata 3 的旧夹具，已更新到 4 并在不含 Codex 的 PATH 下定向通过。首轮原生认证诊断仍要求旧沙箱拒写，已移除这些断言，改为 full_access 的提交/状态事实并复测通过；失败原始记录保留，不改写成通过。中间 `full-access-l1-release` 在并发回归期间由 application 包达到脚本 12 分钟上限而失败，保留 695 项通过及超时堆栈，不追认为通过；最终源码另在 `full-access-l1-qualified` 登记。

## 配置迁移及构建身份

可编辑开发配置改为显式 `full_access`，保留原目标、模型、仓库和发布设置，更新 Runner/helper/扩展后先做纯 `validate`，再以新 request_id 创建新 Run。旧 Run 不改正文、权限、digest 或授权；升级前用原版本完成或精确停止在途执行，并独立核实认证资源结算。本次未自动改写用户安装配置。

最终产品及 test-config helper 位于 `.evidence/full-access-qualified`。Runner 版本 `0.2.0-dev.4`，协议 `steerrunner-cli/v1`，平台 `windows/amd64`。构建基线 commit 如上，`source_dirty=true`；因此不宣称存在对应的已提交 commit。

- Runner SHA256：`09c7290bc65adea80f136eed2dff2948580270d71adc73d68b525884bc0434f8`。
- Runner source digest：`4e9b07243ea401b0aa3bc5b5b1ec1b25faa2c429b29b1a5e478645c069b0bff2`。
- resource digest：`21725cc1e4fffbf6808a74a5eaf736bf2ac2003626eb103d11323b1fb3ea692d`。
- 当前 helper、bundle、fixture 摘要及最终回归集合见[便携摘要](evidence/development-full-access-20261031/summary.json)；旧的 `final-artifacts`、`release` 目录保留各自构建身份，不能混称为最终产品。

L3 静态清单使用上述最终 Runner/helper，31 个请求 case 均因缺少配置不可用，实际执行 0、通过 0；该查询不准备业务现场。原生无模型测试不证明真实 PR 交付，受控内容/parser 回归也不证明其真机只读资格。本记录不声明完整 L2/L3 或真机专项通过。

## 收尾结论

代码、配置/schema、客户端、恢复分类、终端协议和测试矩阵已实施。最终 L1 与完整 Controlled 均通过；原生无模型工具及认证专项通过。P5 的真实 L2/L3、read_only/parser 真机资格因缺配置尚未完整执行，不能据此声明完整 Spec 真机验收通过。未部署、未修改用户安装配置、未提交或推送本次工作树。

最终构建后源码 digest 已重新计算并保持一致；所有失败和中间构建原始目录均保留。便携记录位于 [evidence/development-full-access-20261031/](evidence/development-full-access-20261031/)，完整运行日志位于上述 `.evidence` 目录。
