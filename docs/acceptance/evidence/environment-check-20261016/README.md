# 环境检查实施证据

2026-10-16，本机 Windows / Codex 0.159.2 / host-gh broker 的 sandbox 与 full_access 两个完整原生 case。
[验收正文](../../environment-check.md) 说明范围、失败历史及未合入/发布状态。

- `qualification.json`：两个完整 case 的固定构建身份、结果与只读复验命令。
- `*-report.json`、`*-identity.json`、`*-native-audit.json`：本机两模式原报告、原构建/driver 身份与有限原生审计。
- `boundary-*.json`：当前策略下有效认证的定向正/负 native canary。
- `initial-failure-reconciliation.json`：两个早期失败现场、精确残留引用/SHA 与停止/本地资源事实。
- `development-rounds.json`、`original-final-batch.json`：保留的原始批次摘要；不把包装层错误或未核实回合改写为通过。
- `schema-validation.json`：13 份内嵌公开样例的 JSON Schema 校验。
- `build.json`、`source-files.json`：本地待评审构建与实现来源；无秘密。

原始日志、SQLite、driver 自有 source/base 夹具留在各记录指向的 `C:/srt/<fixture>` 中。
Worker owned 的检查引用与本地资源按各完整报告结算；首轮两个残留引用单列，不借中心宽权限补删。
