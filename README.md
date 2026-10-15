# SteerRunner

SteerRunner 的目标是接收固定 Spec / Plan，在本地或 Orca environment 执行开发 workflow，处理反馈并产出可核验 PR。SteerLine 保留任务、DAG、候选关系和 TaskDelivery，通过 CLI 使用它。

当前已接通独立 Run 执行：一个 Windows/amd64 EXE 同时提供常驻服务和 CLI；固定 Spec/Plan、实现→报告→发布→核验 PR、原会话反馈、显式恢复、停止授权及事件查询均由 Runner 持久化。真实 EXE 配受控原生协议已跑通本地闭环；SteerLine 实际产品 CLI 受控闭环已通过，旧执行 owner 已移除；恢复和真机验收尚未全部通过，完整跨仓交付仍在进行。

- [首版产品与 CLI 契约](docs/spec/steerrunner.spec.md)
- [跨仓拆分实施计划与最小测试安排](docs/spec/steerrunner.plan.md)
- [专属验收要求与结果登记](docs/acceptance/steerrunner.md)
- [来源清单及 P2 交接门槛](docs/extraction/source-map.md)

首版衔接 SteerLine 的执行三层重构，提取 Workflow、ReliableExecution、ExecutionAdapter 及独立宿主。SteerLine 保留任务规划、DAG、候选关系和最终 TaskDelivery，通过 CLI 消费 SteerRunner 的运行事实。

独立项目默认资源内嵌。任务材料、仓库、认证及具体执行后端需要的 Git / 代理工具是执行输入与环境依赖。OpenHands、SWE-ReX 或持久化 workflow 引擎的集成可以后续改变内部实现，不改变本版对外职责。

## 构建和启动

开发构建需要 Go 1.24.2 或更新版本、Windows/amd64 和 Git。在仓库根目录用 PowerShell 执行：

```powershell
.\scripts\build.ps1
.\bin\steerrunner.exe version --json
.\bin\steerrunner.exe service start --state-dir C:\Temp\SteerRunnerDemo --json
.\bin\steerrunner.exe service status --state-dir C:\Temp\SteerRunnerDemo --json
.\bin\steerrunner.exe capabilities --state-dir C:\Temp\SteerRunnerDemo --json
```

构建输出 `bin/steerrunner.exe` 及构建来源/摘要记录。运行只需 EXE；默认流程、四份提示词及 schema 已内嵌，不需要 SteerLine、模板目录、Python 或 Node。CLI 启动成功后退出，常驻服务继续运行。CLI 与服务使用同一个 EXE 文件，但各自是独立进程。

Windows 自动启动先验证挂起子进程的 Job 隔离；原生 breakaway 被明确禁止或创建后仍有 Job 关联时，使用系统自带 Windows PowerShell/CIM 挂起创建独立进程，核验用户、权限与 EXE 后才恢复执行，不请求提升权限。该系统能力不可用时返回明确错误。`startup_unverified` 表示原启动结果未核清，须核实 `service/bootstrap.json` 对应的原进程，不会自动重建另一份启动。`serve --state-dir <绝对路径>` 可从已有独立宿主前台运行，生命周期由该宿主负责。

## 提交与查看执行

把[契约示例](docs/spec/steerrunner.spec.md)中的仓库、完整 commit、材料、adapter 和 publication 改为自己的环境，保存为 UTF-8 `request.json`：

```powershell
Get-Content -Raw -Encoding utf8 .\request.json | .\bin\steerrunner.exe submit --state-dir C:\Temp\SteerRunnerDemo --input - --json
.\bin\steerrunner.exe inspect --state-dir C:\Temp\SteerRunnerDemo --request-id my-submit-001 --json
.\bin\steerrunner.exe inspect --state-dir C:\Temp\SteerRunnerDemo --run-id <run_id> --json
.\bin\steerrunner.exe events --state-dir C:\Temp\SteerRunnerDemo --run-id <run_id> --json
```

accepted 表示材料及原请求已提交，执行结果以 Run 和命令 action_status 为准。丢失响应时查询原 request_id；同 ID 的重试必须保留原正文。服务重启只读取原执行事实；继续执行需要当前 revision 的新 resume，旧答案和反馈不会随重启自动发送。

本地 Codex 读取本 Run 的冻结文件。Orca 在实施输入内读取附带的冻结文档；本机和远端都不读取调用方的移动文件。当前 Orca CLI 通过参数传文本，材料 JSON 与四份正文总计上限为 12000 UTF-16 单元，超过在接受前拒绝；其远端原生闭环尚未验收。Orca 的 local_path 指向已注册项目，environment 必须预先配置；执行主机的同名 Git 仓库须已获取固定 base commit，创建工作区时核验实际 base/head。Runner 不复制调用方的认证或安装配置。

## 停服及回执

PowerShell 7 示例（stdin 为 UTF-8 JSON）：

```powershell
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
$runnerExe = '.\bin\steerrunner.exe'
$runnerState = 'C:\Temp\SteerRunnerDemo'
$runnerStatus = (& $runnerExe service status --state-dir $runnerState --json | ConvertFrom-Json).data
$runnerRequest = @{
    protocol = 'steerrunner-cli/v1'
    request_id = 'demo-stop-001'
    input = @{ expected_instance_id = $runnerStatus.instance_id }
} | ConvertTo-Json -Compress
$runnerRequest | & $runnerExe service stop --state-dir $runnerState --input - --json
```

exit 0 表示命令已持久接受。停服后再次启动服务，可用 `inspect --request-id demo-stop-001` 查询原回执。同 ID 同正文重试返回原命令，换正文返回冲突；旧实例的停服重试不会停止新实例。只读查询不会自动启动服务。

## 状态目录

调用方使用绝对 `--state-dir`；省略时为 `%LOCALAPPDATA%/SteerRunner`。目录下有 `runner.db`、`service/`、`inputs/`、`repos/`、`workspaces/`、`logs/launch.log` 和 `logs/service.log`。由 SteerLine 使用时，应指定其项目运行目录下的 `runner/`，使数据和日志归属于该项目。CLI 不读取 SQLite，同目录只有服务持锁写入。

不兼容旧 SteerLine 数据或其他数据库，不提供旧格式迁移。替换构建前先停服；构建脚本不会结束或替换正在运行的服务。

## 定向验证

```powershell
go test ./internal/contracts ./internal/cli ./internal/storage -run '^(TestSRCLIContract|TestSRShutdownReceipt|TestSRUnsupportedStorage)$' -count=1 -timeout=2m -v
.\scripts\test-foundation.ps1
```

进程测试脚本构建真实 EXE，创建独立测试宿主后返回证据路径。等待该路径的 `result.json`，其中 `exit_code: 0` 且 `scenario_count: 2` 才表示两组测试通过；脚本启动成功不等于测试成功。详细断言在 `integration.log`，调用输出在 `caller-commands.jsonl`。测试使用本次临时目录，不启动真实代理或 PR，也不使用 SteerLine 安装目录。

程序清单资源已生成并纳入源码；只在修改 `app.manifest` 后运行 `go generate ./cmd/steerrunner` 重新生成。项目采用 [Apache-2.0](LICENSE)，来源说明见 [NOTICE](NOTICE)。第三方运行依赖和 Go runtime 的许可声明见 [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt)。正式发布须完成对应验收。

当前支持/验收范围为本机 Codex 和远端 Orca environment；用户已放弃本机 Orca 路径，不列为待验目标。
