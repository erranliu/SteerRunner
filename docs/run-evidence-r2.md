# SteerLine 接入：原开发 Run 的 R2 证据与当前 Git

全部使用 `steerrunner-cli/v1`，沿原 `--state-dir`、`--run-id`；建议每次带原
`--expected-instance-id`。查询不会启动服务、Agent、resume 或重跑。服务未运行时先显式
`service start` 再查询；启动后的既有恢复围栏照常生效，查询本身不授权继续。

公开能力在 `capabilities.data.run_evidence`，完整输入/结果 schema 在
`capabilities.data.run_evidence_schema.$defs`（在线/离线相同，摘要进入 resource_digest）。
`collect_input` 对应收集信封 input；`discover / collect / list / read / workspace_git`
分别对应成功响应的 data。通用 v1 信封仍由 cli_schema 发布。

## 报告和日志落盘

通过 `inspect --run-id R` 取得原 `data.run.execution.workspace`。在项目测试入口中约定：

```text
<该 Run 的实际工作区>/.steerrunner/artifacts/
  execution-001/report.json
  execution-001/raw.log
  execution-002/report.json
  execution-002/raw.log
```

入口自行创建目录。建议先写临时文件，关闭后 rename 成完整文件，再由 SteerLine 收集。
Runner 不解释 `execution-001`、报告格式、测试 ID、通过条件或日志语义；零字节文件只有实际存在时才可收集。
不以 Agent final 代替程序文件。临时文件没有特殊语义，也会被发现。
上述目录中的未忽略文件计入 Git dirty；Runner 不修改项目 ignore 规则。

单文件最多 1 MiB，每 Run 最多 64 条、累计 16 MiB；发现递归遍历最多 256 条目录项，
超过限制整体失败，不返回伪装完整的截断清单。相对路径使用 `/`，相对上述目录；
不允许绝对路径、`.`/`..` 分量、反斜杠、冒号、控制符、分量结尾空格/点、链接或特殊文件。
host-local `codex-app-server`、`orca-terminal` 的原 development 工作区受支持；
Orca environment 当前明确 unsupported，不把远端路径解释成本机文件。

## 最小调用

下面命令都追加 `--state-dir C:\RunnerState --expected-instance-id I --json`；R、I、A 使用原公开返回值。

```powershell
steerrunner.exe artifacts discover --run-id R
steerrunner.exe inspect --run-id R
Get-Content -Raw -Encoding utf8 collect.json | steerrunner.exe artifacts collect --run-id R --input -
steerrunner.exe artifacts list --run-id R
steerrunner.exe artifacts read --run-id R --artifact-id A
steerrunner.exe workspace-git --run-id R
```

`collect.json`（revision 取最新 inspect，request_id 由调用方分配）：

```json
{"protocol":"steerrunner-cli/v1","request_id":"collect-report-001","input":{"expected_revision":7,"relative_path":"execution-001/report.json"}}
```

发现响应的 data 示例：

```json
{"run_id":"R","directory":".steerrunner/artifacts","candidates":[{"relative_path":"execution-001/report.json","size_bytes":2}]}
```

成功收集响应：

```json
{"protocol":"steerrunner-cli/v1","ok":true,"request_id":"collect-report-001","data":{"command_id":"C","run_id":"R","receipt_status":"accepted","action_status":"completed","artifact":{"run_id":"R","artifact_id":"A","relative_path":"execution-001/report.json","sha256":"44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size_bytes":2,"collected_at":"2026-10-19T12:00:00Z","read_command":"artifacts read --run-id R --artifact-id A"}},"error":null}
```

`list` data 为 `{"run_id":"R","artifacts":[上面的artifact]}`。`read` data 为该 artifact
各字段加 `"encoding":"base64","content_base64":"e30="`，解码得到原字节 `{}`；摘要对原字节计算。
查询成功响应的 request_id 为 null。重读保持 ID、摘要、时间、内容不变。
同一 collect request_id/正文重放返回原回执，即使原文件已不存在。新 request 每次建立新
artifact_id；同路径产生新内容也不覆盖旧副本。读取仅接受本 Run 的 artifact_id，不接受本机路径。

收集只写原 Run 证据及命令/事件，不产生 Agent 输入。丢响应按原 request_id inspect/重放；
stale_revision 表示该次未接受，刷新 revision 后方可用新请求。stop 不删除证据；清理原工作区后
`list/read` 仍可用，原 state-dir 必须保留。Runner 未增加新的产品清理命令。

## 当前 Git 与诊断

Git 的 commit、clean 来自同次 `status --porcelain=v2 --branch --untracked-files=all`，
关闭 fsmonitor/untracked cache、可选 index 写锁。原身份核验前后两份样本一致才核实，
检测到变化时返回未知；观察区间在第二次样本结束，不缓存结果或保存脏文件指纹。
任意无锁采样无法证明两次采样间的瞬态变化又恢复不存在，不提供持续监控或源码快照保证。

```json
{"protocol":"steerrunner-cli/v1","ok":true,"request_id":null,"data":{"run_id":"R","commit":"0123456789012345678901234567890123456789","clean":false,"observed_at":"2026-10-19T12:00:00Z","verified":true,"diagnostic":""},"error":null}
```

工作区缺失/已清理、来源不明、原身份不符、Git 命令失败或观察变化时，Git 响应仍是一份
观察结果（exit 0），但 `verified:false, commit:null, clean:null, diagnostic:具体原因`。
调用方必须检查 verified，不能把 exit 0 当成 clean。

产物失败用 `ok:false,error:{code,message}`：缺文件/陌生或跨 Run ID 为
`artifact_not_found`（exit 5），非法/越界路径为 `artifact_path_invalid`（2），
超限为 `artifact_limit_exceeded`（2），来源不明为 `workspace_source_unknown`（5），
工作区不可用为 `workspace_unavailable`（4），读取不可用为 `artifact_unavailable`（4），
读取中变化为 `artifact_changed`（5），已存字节损坏为 `artifact_integrity_failed`（5）。
任何失败都不会制造空报告或成功收集回执。
