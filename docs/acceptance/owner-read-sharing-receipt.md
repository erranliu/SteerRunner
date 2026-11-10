# owner 就绪文件共享冲突与回执未知

日期：2026-11-10。在 `codex/reduce-verification-overhead` 独立 worktree 内修复；基线为 `5ccc3f0aa07764657b3346aa6023b73b878d76e0`。先前核验成本改动的结果见 [原验收](verification-overhead-reduction.md)，原失败记录不改为通过。

## 原现场与因果链

原现场 `C:/Users/lmas1/AppData/Local/Temp/sr-54729125`、Run `005699ee47ad997a2f059fe9afa6103b` 保留。只读查询原数据库事件后发现，最终诊断覆盖了更早的首错：

1. revision 6，15:49:46.5349082（北京时间）：`prepare_session` 原意图提交。
2. revision 7，15:49:48.0408559：读取 `app-server/owner/owner-ready.json` 返回 `The process cannot access the file because it is being used by another process`（Windows `ERROR_SHARING_VIOLATION`）。`callOwner` 在读取与身份校验后才 Dial/Encode，这次读取失败尚未发送 owner IPC；返回的空回执又产生原操作绑定全 false 的次级诊断。
3. 调用已保存为 outcome_unknown，既有恢复仅对账原操作。revision 8，15:49:48.1140031：owner 返回 `original native operation receipt unavailable; no action repeated`。owner 没收到原 native 请求，不能产生其回执；`native/calls.jsonl` 缺失与此一致。
4. 原 Run 后续 stop 保持 unknown，`resources_settled=false`。不从缺调用日志推断资源已停止，也不重发原业务输入、替换 owner 或修改原意图。

原 launch.log 的直接 Job 隔离检查后存在 broker 路径，且 owner-ready 已发布，因此单条 Job 日志不是此次根因证据。未取证当时持锁进程和冲突持续时间，不归因于某个扫描器或声称所有文件读取失败均已修复。

证据根为 `C:/SteerlineDebug/wt-5d04-receipt/`。`original-readonly-evidence/runner.db` 为只读连接导出的 SQLite 一致快照，`event-timeline.json` 保存完整事件，原始日志另存；`original-manifest.json` 记录关键原文件读取前后 SHA-256 一致。未启动原服务或向原 owner 发送请求。

## 与 Git 优化的关系

在独立、detached 的 `SteerRunner-receipt-baseline` worktree 检出基线，仅加入同一 Windows 复现测试；优化后、修复前工作树也运行该测试。真实 `CreateFile` 独占句柄分别锁住 config/ready，先确认 `os.ReadFile` 返回同类共享冲突，再验证首次 owner 调用。

两者均于首次发送前立即失败，错误与原首错一致：

- `baseline-sharing/go-test.jsonl`：基线 `5ccc3f0a`，两个文件子例均失败。
- `optimized-before-fix-sharing/go-test.jsonl`：包含 Git 优化而尚未修复的版本，同样失败。

因此这是基线已有的“发送前读取瞬态冲突 → outcome_unknown → 仅查询不存在的回执”缺陷，修复前的共同 owner 读取实现相同。没有证据对 Git 优化前后偶发触发概率作结论。

## 修复与保留边界

`callOwner` 对 config/ready 使用同一个发送前读取 helper。Windows 仅遇到 `ERROR_SHARING_VIOLATION` 才以 5ms 间隔重读，每文件最多 500ms 的重试预算，并服从原 context 的取消/期限。其他错误直接返回，非 Windows 保持普通读取。

重试范围没有跨过 Dial/Encode：不重试 IPC、prepare_session、turn/start、发布或 stop；不改原 ID、正文、owner 或预算，不修改恢复状态机。读取成功后原协议、目录、构建、loopback endpoint、已绑定 owner、真实存活进程及回执身份检查仍执行。持续锁定、取消或错误身份均零发送；耗尽预算继续保留未知，不伪造成功或停止证明。

## 独立审核与验证

只读审核者 `/root/review_runner` 核对原事件、基线 worktree、两份失败报告及生产增量，批准本次修复，未发现 P0–P2；确认仅重读文件，不重放外部操作。原持锁者未确定、原 Run 未恢复或清理的限制继续保留。

| 验证 | 结果及证据 |
| --- | --- |
| Windows 共享冲突定向回归 | `fixed-sharing/go-test.jsonl` 通过：冲突解除后同一 ID 仅发送一次，取消/持续锁定/错误身份零发送。 |
| `go test ./internal/execution/adapters/codex ./internal/execution/reliable/operation -count=1 -json` | `owner-regression-baa452876e094f40afd8b86ed5a82c12/go-test.jsonl`：92 个顶层用例、280 个含子用例通过，0 失败；含新增 cancel-during-read。`TestServiceStopWorkerHelper` 为既有 helper 默认跳过，不计独立原生验证。 |
| `./scripts/test-foundation.ps1 -Pattern '^(TestSRNativeUnknownAndStop\|TestSRRestartPreservesAuthorization\|TestSRRunWorkflow)$' -OutDir <独立目录>` | `controlled-recovery-8133d22fc4bd4d499838d73973c9f536/`：新构建 3 个顶层场景全部通过，四项重启恢复 graceful/kill/creation_receipt/pending_stop 全部通过；`result.json` 为 exit_code=0、scenario_count=3，原 harness 已退出。命令中的竖线为正则分支，实际参数不含 Markdown 表格转义。 |
| `git diff --check` | 通过。 |

本次为组件与定向受控验证，不声明完整 L1、Controlled、真实模型/PR 或 L2/L3 资格。原失败、只读快照、基线预期失败和新现场成功分别留存，不拼装或改写旧报告。

最终验证构建 SHA-256 为 `975c14fb3d539ea4fa436b6fe7ea7577f0724de212dca4ec763dbcc08d7af7c5`（按新现场 source-identity/公开 service 记录核对）。组件完整报告还包含等待共享冲突时取消的通过记录；正式原未知与补验记录不变。
