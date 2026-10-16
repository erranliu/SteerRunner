# 原会话查看验收

归属：[实现 Plan](../spec/native-session-viewer.plan.md)。

| ID | 自动验收项 | 状态 |
| --- | --- | --- |
| NSV-R01 | 原身份验证、过期/停服/unsupported 零开窗，Run 状态不变 | 通过，Engine 9 个正反例；`.evidence/native-session-viewer/runner-tests.log` |
| NSV-R02 | 原进程/binding/thread/workspace 核对，只读 handshake/thread-read，无新输入 | 通过，loopback WebSocket 5 场景及既有无 prompt/权限覆盖参数回归；同一日志 |
| NSV-R03 | CLI 参数、零自动启动；独立窗口句柄及字面参数 | 通过，CLI 契约及缺服务零创建目录、Windows helper 实际控制台句柄/字面参数；同一日志 |

人工 NSV-RM01：真实 Codex CLI 的持续会话显示与关闭体验待验；SteerLine 的实际产品接线验证单独登记在其专属验收。

源码为 `95d24e5a44df8813b736f052e2ba831363a97b20` 加本次工作树，`go build ./...` 通过（`.evidence/native-session-viewer/compile.log`）。本次不是已发布 release；SteerLine 按实际构建 SHA 固定独立 debug 包。自动测试未消耗模型，不宣称真实业务会话人工体验已验收。

供调用方验证的 EXE SHA-256 为 `f68eda99d9627978efc0bcc12dc30682c23574fdb7cf0ca788ab6a6ce9df3cd2`。SteerLine 的 `C:/SteerlineDebug/dev-20261016-133808-6f40f2dd-dirty/evidence/smoke-cleanup.json` 登记模拟浏览器场景通过、精确自有进程/监听均为 0；浏览器查看与同请求重放只触发一次公开查看调用且零新增业务输入。该 caller fixture 不替代 NSV-R02/R03 的原生读取与控制台测试，也不替代真实 Codex 人工体验。
