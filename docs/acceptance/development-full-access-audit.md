# 开发 full_access：实施后审计与修复

日期：2026-10-31。范围为当前分支相对 `a086a9984dc0bd32b3f02b9698cd26c9ba313d90` 的完整实现、配置迁移、恢复及测试消费者。主会话逐项审计并修复，没有调用独立子代理。原实施阶段的构建和结果保留在[原记录](development-full-access.md)，不得将其摘要当成审计修复后的构建身份。

## 确认的问题与修复

| 问题 | 影响及修复 | 回归证据 |
| --- | --- | --- |
| Pi/DSH profile v5 未进入用量版本白名单 | 新 full_access Run 的用量绑定返回 FORMAT_UNSUPPORTED；允许 v5 并保留未知版本拒绝、原会话及 command 归属检查 | `TestUsageFullAccessTerminalProfileVersions`，Pi/DSH 与旧版本、未知版本分别覆盖 |
| 部分 Codex 消费者仍只接受 policy v11 | 旧显式 full_access 的原签名退出证明和 initialization 用量被拒；停止/用量读取按原 v10 或 v11 契约验证，身份、签名、冻结配置和原控制终端限制不变 | `TestLegacyFullAccessCodexStopsWithOriginalSignedExit`、`TestUsagePreparedLocalCodexInitialization` |
| 独立用量采集绕过退役分类 | 旧 sandbox/缺省 Run 仍可能读取原生日志或申请远端日志访问；host 将主引擎同一冻结分类传给 collector，在新采集前拒绝，保留已存查询。既有 lease 清理独立保留，不创建新的日志访问 | `TestUsageRetirementGatePreservesPersistedUsageAndSkipsNewAccess`、`TestRetiredDevelopmentNeverStartsBackgroundWorkOrRewritesAuthorization` |
| owner/启动失败证明仍有当前 bundle 回落 | 冻结 bundle 摘要缺失时可能用当前扩展补全身份；现在缺失或不完整摘要直接不能核验，原冻结摘要与当前 bundle 不同时仍按原值核验 | `TestOriginalTerminalOwnerNeverInfersMissingBundleFromCurrentBuild`，以及原启动失败、停止和签名反例 |

前三项中的 profile/v10 故障和第四项均已用新增测试在修复前复现失败；修复后定向测试通过。原始日志在 `.evidence/full-access-diagnostics/audit-reproduction.log`、`audit-bundle-reproduction.log` 和 `audit-fixes-1.log`；不把失败记录改成通过。

## 验证与交付边界

审计修复后的完整 L1、固定 Runner 的 25 项 Controlled 与构建身份正在单独记录。最终结果及便携摘要在本记录收尾更新；原实施阶段 720 项 Go、29 项 Node、25 项 Controlled 的通过仅证明其原源码。

真实 L2/L3、read_only/parser 真机资格仍缺实际测试配置，未执行；无模型原生工具检查和 Controlled 不代替真实 PR 交付。开发要求显式 full_access，read_only 和既有认证链路不变；不自动迁移用户安装配置或旧 Run。PR 交付仅发布本次代码变更，不发布业务 PR，不合并目标分支。
