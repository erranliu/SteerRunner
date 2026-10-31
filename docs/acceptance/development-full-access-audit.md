# 开发 full_access：实施后审计与修复

日期：2026-10-31。范围为当前分支相对 `a086a9984dc0bd32b3f02b9698cd26c9ba313d90` 的完整实现、配置迁移、恢复及测试消费者。主会话逐项审计并修复，没有调用独立子代理。原实施阶段的构建和结果保留在[原记录](development-full-access.md)，不得将其摘要当成审计修复后的构建身份。

## 确认的问题与修复

| 问题 | 影响及修复 | 回归证据 |
| --- | --- | --- |
| Pi/DSH profile v5 未进入用量版本白名单 | 新 full_access Run 的用量绑定返回 FORMAT_UNSUPPORTED；允许 v5 并保留未知版本拒绝、原会话及 command 归属检查 | `TestUsageFullAccessTerminalProfileVersions`，Pi/DSH 与旧版本、未知版本分别覆盖 |
| 部分 Codex 消费者仍只接受 policy v11 | 旧显式 full_access 的原签名退出证明和 initialization 用量被拒；停止/用量读取按原 v10 或 v11 契约验证，身份、签名、冻结配置和原控制终端限制不变 | `TestLegacyFullAccessCodexStopsWithOriginalSignedExit`、`TestUsagePreparedLocalCodexInitialization` |
| 独立用量采集绕过退役分类 | 旧 sandbox/缺省 Run 仍可能读取原生日志或申请远端日志访问；host 将主引擎同一冻结分类传给 collector，在新采集前拒绝，保留已存查询。既有 lease 清理独立保留，不创建新的日志访问 | `TestUsageRetirementGatePreservesPersistedUsageAndSkipsNewAccess`、`TestRetiredDevelopmentNeverStartsBackgroundWorkOrRewritesAuthorization` |
| owner/启动失败证明仍有当前 bundle 回落 | 冻结 bundle 摘要缺失时可能用当前扩展补全身份；现在缺失或不完整摘要直接不能核验，原冻结摘要与当前 bundle 不同时仍按原值核验 | `TestOriginalTerminalOwnerNeverInfersMissingBundleFromCurrentBuild`，以及原启动失败、停止和签名反例 |

新 profile 绑定、v10 初始化及当前 bundle 回落的失败已在修复前复现；原签名退出证明的严格常量比较经源码审计确认。四项问题修复后的正负回归全部通过。原始日志在 `.evidence/full-access-diagnostics/audit-reproduction.log`、`audit-bundle-reproduction.log` 和 `audit-fixes-1.log`；不把失败记录改成通过。

## 审计后固定构建与验证

代码提交 `ba18cb7cf83708740abef306e8b74d086a22b480`，Runner 构建时工作树干净。后续仅新增本验收记录和便携证据，产品源码摘要保持一致。

- Runner SHA256：`86ce042ce92327eb663660da82123bf0e9ff72c52bab370d2700ad9e0ee2fef2`。
- 源码摘要：`a7b635f52df4f3e39d04b5662285da417556e874308bb07ed635360e6e6f40c0`。
- resource digest：`21725cc1e4fffbf6808a74a5eaf736bf2ac2003626eb103d11323b1fb3ea692d`。
- Runner、test-config 与终端 helper 位于 `.evidence/full-access-pr-qualified`。完整版本、helper/bundle SHA、分组及原始目录见[便携摘要](evidence/development-full-access-audit-20261031/summary.json)。原实施阶段的摘要不替代这些记录。

| 验证 | 审计后结果 |
| --- | --- |
| 完整 L1 | 724 项 Go 顶层测试、29 项 Node 通过；framework、native driver 编译及零匹配 guard 通过；17 项仅为既有 allowlist 的 helper/opt-in skip。[原始结果摘要](evidence/development-full-access-audit-20261031/l1-summary.json) |
| 完整 Controlled | 同一固定 Runner，results 1 项 + rest 24 项，精确匹配注册集合，无缺项、重复或 skip；25/25 通过。[分组及构建身份](evidence/development-full-access-audit-20261031/controlled-summary.json) |
| 原生无模型 | `TestAuthenticationNativeToolBoundary` + host broker 19.69 秒通过，`TestLinkedGitNativeCommit` 9.60 秒通过；与真实 smoke 分开计数 |
| 离线配置准入 | full_access 接受；sandbox、空、缺省、null、非字符串拒绝，6 项通过 |
| 本机真实 L2 | `local-codex.smoke/default` 通过；[专用测试 PR #716](https://github.com/erranliu/SteerlineTest/pull/716) 合入，merge commit `d21c173cc0090b8cb67d41eb25c7e729c5c34c9a`。原 Run、服务停止；认证撤销确认，native history 为 full_access/never，零审批、升级、绕过。[证据](evidence/development-full-access-audit-20261031/local-l2.json) |
| 远端真实 smoke，原配置 | 在现场准备时原 Orca runtime 不可达，未进入 Runner 业务执行；原失败保留 |
| 远端真实 smoke，可达路由新现场 | 失败：`Orca queried original provider metadata currently unavailable`。没有发送业务输入；原 Run、服务均停止，offline verification 为 `orca_terminal` scope，不证明远端全部 OS 后代或登录撤销。未弱化 provider/session 身份门槛。[失败及收尾](evidence/development-full-access-audit-20261031/remote-smoke.json) |

两个真实场景使用现有 `Invoke-SRCaseBatch` / `run-test-case.ps1` worker；派发前单独核验上述同源码 L1 和精确 25 项 Controlled、产品及配置 SHA，保留各自原目录，没有把分组结果伪装成同一次完整分级脚本报告。聚合 live 结果为 failed：本机通过，远端未通过；不能称完整 L3 或本专项全部真机资格通过。

## 找到的配置与授权

用户要求查找配置后，在 `C:/Users/lmas1/.codex/worktrees/6556/SteerRunner/.evidence/ocu-20261030/config.json` 找到 10 月 30 日使用的测试配置，并核验固定 `erranliu/SteerlineTest` 仓库与 commit。只在本工作树生成测试副本，将三项 sandbox 配置显式改为 full_access；原文件 hash 保持不变。

L2/L3 的静态清单共 32 case，16 项 available、16 项 unavailable；缺少 Pi/DSH、audited 配置及管理员故障设施。原远端不可达后，在另一份已保存的路由配置中只读核验 runtime 可达，再用独立副本和新现场复测同一个远端 smoke。两个原配置均未被修改。[配置来源、迁移及静态清单](evidence/development-full-access-audit-20261031/config-discovery.json)记录来源及 SHA；实际可达路由来源另附在远端失败证据中。

真实执行前，用户额外明确授权这两项 smoke 及其专用测试 PR。此前自动审批拒绝的命令没有执行；新增授权后才启动。测试协调脚本首次在 catalog 展开阶段退出，未创建现场或 PR；已改用仓库现有 `Get-SRTestCatalog` 展开器并更换证据目录，原失败日志保留。

## 后续远端排查

后续修复 provider 元数据启动竞态及两个原生审计消费者遗漏；用户授权设置远端代理与本机绕过并重启对应主应用/终端 daemon。最终 `b2d622c` 固定构建的 L1（728 Go / 29 Node）、完整 Controlled（25/25）及真实远端 smoke 均通过，专用测试 PR #718 已合入，原 Run 与服务收尾。当前状态、构建身份和独立证据见[远端排查记录](development-full-access-provider.md)，本节之前的失败记录保持原样。

## 原审计阶段交付边界

以下保留首次审计阶段的状态，后续最终验收见上一节链接。

代码审计确认的四项问题已修复，基础回归、Controlled 和本机 L2 通过。远端 smoke 的原生 readiness 资格失败；其他 L3、Pi/DSH、audited、管理员窗口及 read_only/parser 真机专项仍未验证。PR 标为草稿，保留真实失败和未验范围，不声明完整 Spec 真机验收通过。

开发要求显式 full_access，read_only 和既有认证链路保持；本次没有自动迁移用户安装配置或旧 Run。专用测试 PR 的创建/合入来自后续单独授权；本项目代码 PR 只发布变更供审阅，不合并 main。
