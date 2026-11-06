# 2026-11 过期文档归档

归档日期：2026-11-06。核对基线：`dda9a82727b6e7ac88e687a250750e8bbae236dc`。

本批归档 33 份正文，首批归档时旧路径保留短导引；后续[目录收敛](implementation/README.md)改为集中[旧路径映射](../path-map.json)，只保留证据/诊断固定引用的必要导引。原日期、构建、计划状态、失败及未验结论保持，正文中的“当前”只指原记录时点。

原始 `docs/acceptance/evidence/` 文件保持原路径和字节，正文里的取证路径和源码摘要也不改写。

[现行文档入口](../../README.md) · [全部历史归档](../README.md)

| 历史正文 | 原路径 | 归档原因 | 当前入口 |
| --- | --- | --- | --- |
| [sandbox-authentication.spec.md](spec/sandbox-authentication.spec.md) | `docs/spec/sandbox-authentication.spec.md` | broker、CA、socket 与认证租期已退役；新开发执行使用宿主登录 | [当前说明](implementation/spec/host-native-auth.spec.md) |
| [sandbox-authentication.plan.md](spec/sandbox-authentication.plan.md) | `docs/spec/sandbox-authentication.plan.md` | broker、CA、socket 与认证租期已退役；新开发执行使用宿主登录 | [当前说明](implementation/spec/host-native-auth.spec.md) |
| [sandbox-authentication.md](acceptance/sandbox-authentication.md) | `docs/acceptance/sandbox-authentication.md` | broker、CA、socket 与认证租期已退役；新开发执行使用宿主登录 | [当前说明](implementation/spec/host-native-auth.spec.md) |
| [personal-runner-gh-only-20261019.md](acceptance/personal-runner-gh-only-20261019.md) | `docs/acceptance/personal-runner-gh-only-20261019.md` | broker、CA、socket 与认证租期已退役；新开发执行使用宿主登录 | [当前说明](implementation/spec/host-native-auth.spec.md) |
| [service-generation-recovery.spec.md](spec/service-generation-recovery.spec.md) | `docs/spec/service-generation-recovery.spec.md` | 跨构建活动接管与 verify-takeover 已删除；采用升级前排空和人工只读对账 | [当前说明](../../cli-guide.md#人工对账与资源丢失) |
| [service-generation-recovery.plan.md](spec/service-generation-recovery.plan.md) | `docs/spec/service-generation-recovery.plan.md` | 跨构建活动接管与 verify-takeover 已删除；采用升级前排空和人工只读对账 | [当前说明](../../cli-guide.md#人工对账与资源丢失) |
| [service-generation-recovery.md](acceptance/service-generation-recovery.md) | `docs/acceptance/service-generation-recovery.md` | 跨构建活动接管与 verify-takeover 已删除；采用升级前排空和人工只读对账 | [当前说明](../../cli-guide.md#人工对账与资源丢失) |
| [content-takeover.md](acceptance/content-takeover.md) | `docs/acceptance/content-takeover.md` | 跨构建活动接管与 verify-takeover 已删除；采用升级前排空和人工只读对账 | [当前说明](../../cli-guide.md#人工对账与资源丢失) |
| [pr38-review-20261020.md](acceptance/pr38-review-20261020.md) | `docs/acceptance/pr38-review-20261020.md` | 2026-10-20 的一次性 PR 复审与补测；保留原构建的失败、通过和跨仓限制 | [当前说明](../../testing.md) |
| [pr-test-gaps-20261020.md](acceptance/pr-test-gaps-20261020.md) | `docs/acceptance/pr-test-gaps-20261020.md` | 2026-10-20 的一次性 PR 复审与补测；保留原构建的失败、通过和跨仓限制 | [当前说明](../../testing.md) |
| [native-composer-reflow.md](native-composer-reflow.md) | `docs/native-composer-reflow.md` | 视口重绘、草稿匹配与旧终端启动/完成补证已被结构化会话及旧写链退役取代 | [当前说明](../../orca-execution.md) |
| [native-start-confirmation.spec.md](spec/native-start-confirmation.spec.md) | `docs/spec/native-start-confirmation.spec.md` | 视口重绘、草稿匹配与旧终端启动/完成补证已被结构化会话及旧写链退役取代 | [当前说明](../../orca-execution.md) |
| [native-start-confirmation.plan.md](spec/native-start-confirmation.plan.md) | `docs/spec/native-start-confirmation.plan.md` | 视口重绘、草稿匹配与旧终端启动/完成补证已被结构化会话及旧写链退役取代 | [当前说明](../../orca-execution.md) |
| [native-start-confirmation-20261027.md](acceptance/native-start-confirmation-20261027.md) | `docs/acceptance/native-start-confirmation-20261027.md` | 视口重绘、草稿匹配与旧终端启动/完成补证已被结构化会话及旧写链退役取代 | [当前说明](../../orca-execution.md) |
| [native-completion-confirmation.md](acceptance/native-completion-confirmation.md) | `docs/acceptance/native-completion-confirmation.md` | 视口重绘、草稿匹配与旧终端启动/完成补证已被结构化会话及旧写链退役取代 | [当前说明](../../orca-execution.md) |
| [pr67-integration.md](acceptance/pr67-integration.md) | `docs/acceptance/pr67-integration.md` | 视口重绘、草稿匹配与旧终端启动/完成补证已被结构化会话及旧写链退役取代 | [当前说明](../../orca-execution.md) |
| [orca-native-submit.spec.md](spec/orca-native-submit.spec.md) | `docs/spec/orca-native-submit.spec.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit.plan.md](spec/orca-native-submit.plan.md) | `docs/spec/orca-native-submit.plan.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit-control-stream.md](acceptance/orca-native-submit-control-stream.md) | `docs/acceptance/orca-native-submit-control-stream.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit-implementation.md](acceptance/orca-native-submit-implementation.md) | `docs/acceptance/orca-native-submit-implementation.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit-integrity.md](acceptance/orca-native-submit-integrity.md) | `docs/acceptance/orca-native-submit-integrity.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit-local-final.md](acceptance/orca-native-submit-local-final.md) | `docs/acceptance/orca-native-submit-local-final.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit-local-stream.md](acceptance/orca-native-submit-local-stream.md) | `docs/acceptance/orca-native-submit-local-stream.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit-local.md](acceptance/orca-native-submit-local.md) | `docs/acceptance/orca-native-submit-local.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit-reassessment.md](acceptance/orca-native-submit-reassessment.md) | `docs/acceptance/orca-native-submit-reassessment.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit-reaudit.md](acceptance/orca-native-submit-reaudit.md) | `docs/acceptance/orca-native-submit-reaudit.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit-remote-final.md](acceptance/orca-native-submit-remote-final.md) | `docs/acceptance/orca-native-submit-remote-final.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit-remote-rerun.md](acceptance/orca-native-submit-remote-rerun.md) | `docs/acceptance/orca-native-submit-remote-rerun.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-native-submit.md](acceptance/orca-native-submit.md) | `docs/acceptance/orca-native-submit.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-auth-socket-rootcause.md](acceptance/orca-auth-socket-rootcause.md) | `docs/acceptance/orca-auth-socket-rootcause.md` | terminal send/Enter、原始流及 socket 诊断属于旧执行批次；当前新 Run 只走结构化会话 | [当前说明](../../orca-execution.md) |
| [orca-remote-native.spec.md](spec/orca-remote-native.spec.md) | `docs/spec/orca-remote-native.spec.md` | 旧终端 NativeProof 执行与其认证/停止方案已被结构化合同取代；旧资格仍只属于原构建 | [当前说明](../../orca-execution.md) |
| [orca-remote-native.plan.md](spec/orca-remote-native.plan.md) | `docs/spec/orca-remote-native.plan.md` | 旧终端 NativeProof 执行与其认证/停止方案已被结构化合同取代；旧资格仍只属于原构建 | [当前说明](../../orca-execution.md) |
| [orca-remote-native.md](acceptance/orca-remote-native.md) | `docs/acceptance/orca-remote-native.md` | 旧终端 NativeProof 执行与其认证/停止方案已被结构化合同取代；旧资格仍只属于原构建 | [当前说明](../../orca-execution.md) |

## 保留范围

- 使用与运维指南、长期产品承诺、Workflow、测试指南和测试清单仍为现行入口。
- 能力层设计、结构化会话、冻结历史读取、当前停止合同及仍有适用机制的专项继续保留。
- `l3-execution-stability`、`workflow-input-budget` 等仍含待验范围；`run-usage-and-cost` 仍含用量契约。本批不因日期早或部分机制退役就整体归档。
- 本批整理不运行模型或取得新构建资格；历史测试结果不合并成当前通过。
