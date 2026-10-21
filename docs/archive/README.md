# 历史文档归档

归档日期：2026-10-21；核对基线：`70c19a0`。本次归档 13 份已被后续设计或汇总取代的文档，保留当时的日期、提交、状态、失败与未验结论。正文只增加归档提示并调整相对链接，不将历史状态改写为当前资格。

当前使用说明、公开契约与专项计划见[文档索引](../README.md)，实际验证见[验收索引](../acceptance/README.md)。

| 历史文档 | 原路径 | 归档原因 | 后续入口 |
| --- | --- | --- | --- |
| [source-map.md](2026-10/extraction/source-map.md) | `docs/extraction/source-map.md` | 首版提取来源与 P2–P7 交接记录已完成其历史用途 | [当前入口](../README.md) |
| [steerrunner.plan.md](2026-10/spec/steerrunner.plan.md) | `docs/spec/steerrunner.plan.md` | 首版跨仓拆分与切换计划；当前实施入口已按专项拆分 | [当前入口](../README.md) |
| [reliable-modularization.spec.md](2026-10/spec/reliable-modularization.spec.md) | `docs/spec/reliable-modularization.spec.md` | 已完成的 state/v2 重构；当前职责与恢复语义已由后续契约取代 | [当前入口](../spec/execution-boundary-refactor.spec.md) |
| [reliable-modularization.plan.md](2026-10/spec/reliable-modularization.plan.md) | `docs/spec/reliable-modularization.plan.md` | 已完成的可靠层模块化计划与历史复验 | [当前入口](../spec/execution-boundary-refactor.plan.md) |
| [workflow-selection-audit.spec.md](2026-10/spec/workflow-selection-audit.spec.md) | `docs/spec/workflow-selection-audit.spec.md` | workflow/1 双审计与自动返修设计已由 workflow/2 取代 | [当前入口](../spec/result-acquisition.spec.md) |
| [workflow-selection-audit.plan.md](2026-10/spec/workflow-selection-audit.plan.md) | `docs/spec/workflow-selection-audit.plan.md` | workflow/1 实施与验证记录；不再作为当前实施计划 | [当前入口](../workflows/README.md) |
| [l3-rebase-20261016.md](2026-10/acceptance/l3-rebase-20261016.md) | `docs/acceptance/l3-rebase-20261016.md` | 首批旧构建主线复测，保留原失败与补测范围 | [当前入口](../test-catalog.md) |
| [l3-revocation-fix-20261016.md](2026-10/acceptance/l3-revocation-fix-20261016.md) | `docs/acceptance/l3-revocation-fix-20261016.md` | 旧构建撤销修复及认证阻断复测 | [当前入口](../test-catalog.md) |
| [orca-remote-20261017.md](2026-10/acceptance/orca-remote-20261017.md) | `docs/acceptance/orca-remote-20261017.md` | 旧远端 helper 构建验收，已由原生远端方案取代 | [当前入口](../acceptance/orca-remote-native.md) |
| [orca-remote-extensions-20261018.md](2026-10/acceptance/orca-remote-extensions-20261018.md) | `docs/acceptance/orca-remote-extensions-20261018.md` | 旧 helper 扩展矩阵，包含已退役的远端 Check/App 范围 | [当前入口](../acceptance/orca-remote-native.md) |
| [orca-remote-app-case-removal-20261018.md](2026-10/acceptance/orca-remote-app-case-removal-20261018.md) | `docs/acceptance/orca-remote-app-case-removal-20261018.md` | 旧测试清单移除 App 场景的变更记录 | [当前入口](../test-catalog.md) |
| [orca-remote-native-p0-20261019.md](2026-10/acceptance/orca-remote-native-p0-20261019.md) | `docs/acceptance/orca-remote-native-p0-20261019.md` | 远端原生删除 helper 前的首轮能力探针；最终结论另有汇总 | [当前入口](../acceptance/orca-remote-native.md) |
| [orca-remote-native-p0-query-fix-20261019.md](2026-10/acceptance/orca-remote-native-p0-query-fix-20261019.md) | `docs/acceptance/orca-remote-native-p0-query-fix-20261019.md` | P0 查询修复补验；最终 Runner 闭环结论另有汇总 | [当前入口](../acceptance/orca-remote-native.md) |

## 查阅与维护

- `2026-10/` 按原 `spec/`、`acceptance/`、`extraction/` 层次存放正文；日期目录用于历史检索，不代表整月文档均已归档。
- 原路径保留短导引，供旧证据、外部引用及源文件清单追溯；当前文档中的历史引用直接指向归档正文。
- 原始 `acceptance/evidence/` 文件全部保留原位置及字节，JSON 源码清单中的旧路径与摘要保持原事实。
- workflow/1 的 Spec 审计、两次自动返修、state/v2 和旧 helper 的认证/停止保证均按其历史范围阅读。当前执行、恢复与支持边界从“后续入口”查阅。
- 仍有适用规则或待验项的文档继续维护在原专项目录；不因日期较早就归档，也不将历史测试结果拼成当前全集通过。
