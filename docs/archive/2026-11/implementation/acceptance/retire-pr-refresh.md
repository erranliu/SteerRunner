# Runner PR 刷新退役验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-04，Runner 基线 `79451502`。已确认 SteerLine #445 合入 `aa08dfee`，其交付和废弃路径已退出 Runner 刷新调用。对应[Spec](../spec/retire-pr-refresh.spec.md)和[Plan](../spec/retire-pr-refresh.plan.md)。

| 验证 | 结果 |
| --- | --- |
| CLI、宿主、应用、contracts 和 livetest 定向 Go 回归 | 通过。旧 CLI 参数与旧协议字段拒绝，普通和离线 inspect 保持原快照。平台证据测试覆盖原绑定、错误 head/仓库/base、缺失时间及未知合入状态。 |
| `scripts/test-foundation.ps1 -Pattern '^TestSRPublication(Identity\|BeforeWorkflowReady)$'` | 两个受控场景通过。平台夹具漂移后普通 inspect 保留原 revision、PR 观察时间和绑定，零新增 provider 查询和原生输入。早期绑定和原停止路径通过。原报告在本 worktree 的 `.evidence/retire-pr-refresh-controlled/result.json`。 |
| `go test -tags live -c -o .evidence/retire-pr-refresh-controlled/native.test.exe ./tests/native` | 原生验收驱动编译通过。最初尝试以零匹配执行驱动，驱动要求显式 evidence 目录，改用编译入口确认。未运行真实原生和真实 PR 合入。 |

## 退役审计

- CLI 解析、宿主请求和 Engine 均已删除刷新开关。原刷新写回、专用 PR query_error、事件和旧反馈验收辅助入口已删除。活跃源码中的退役名字仅用于拒绝回归；历史记录按原构建保留。
- `verifyPublication`、`probePRClues`、provider 查询和 Worker 自主推进路径未修改。
- 原生验收直接向 GitHub 查询，并分别记录原绑定和平台证据。报告校验器核对原 Run、PR、分支和精确 head，并要求同一平台 merge commit 和时间。Git 祖先、内容和 checker 验证沿原驱动执行。
