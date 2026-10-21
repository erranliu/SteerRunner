# 可靠性交付公开读取与受理边界收口

本批只实现完整受理前绑定和同快照观察，不改变原生节点、输入授权、恢复、停止或 Task 交付规则。SteerLine 保持公开 CLI 消费边界，不读本仓私有状态。

## 接口与实现

- capabilities.instance_binding.full_binding 发布 `steerrunner-service-binding/v1`、before_effects/no_implicit_start 与逐命令覆盖。CLI `--expected-binding <JSON>` 接收 contract/protocol/instance_id/state_dir/executable_sha256/version/resource_digest，缺字段、未知命令或与 expected-instance-id 冲突均拒绝。
- resident 在 handler、PR/原生查询或命令记录受理之前验证完整绑定和 ready；命令持 host 受理读门闩，service stop 持写门闩，停止受理后拒绝新动作。保持 host→engine 锁序，不在 SQLite 事务内进行新外部 I/O；不因绑定错误改写原请求。
- `observe --run-id R --after C --limit N` 是只读公开命令，禁止 refresh-pr，输出 `steerrunner-run-observation/v1`。同一只读事务读取原 Run、revision、有界事件和 cursor，cursor 只越过实际消费事件，空页保留消费位置，事件不超过该投影 revision。保留现有历史 read_only 投影。
- 这些接口不承担安装排他；未实现安装共享/独占租约、SteerLine-owned 启动上下文或安装续装，不将 JSON 当作租约。

## 测试计划

L1 临时目录与真实 SQLite/受理入口：`go test ./internal/storage ./internal/host ./internal/cli -run 'TestSRObservation|TestSRFullBinding|TestSRExpectedInstance|TestSRShutdownProofRequiresReleasedWriter|TestSRRunAtomicCommit' -count=1 -v -timeout=90s`。接口接线后执行，覆盖所有声明命令的错误身份/构建/resource/目录/ready，原 stop 与新受理先后，零隐式启动，事务内分页/并发、空页后新增事件及原子提交；必须全部终态通过。

两仓公开 CLI 联验由固定本次构建的 SteerLine runnerclient.TestRunnerFullBindingPublicCLI 消费，显式 EXE/version/SHA，创建临时空闲服务，能力/错绑定/observe 路由及正式 stop/结算均通过；不使用原生模型、Git/PR 或安装环境。结果与证据登记在[本仓验收](../acceptance/reliability-simplification.md)及 SteerLine 对应验收，编译不替代运行证明。不选择 L2/L3/L4，因为本批不改变执行流程，已有 L1 与公开空闲 CLI 可覆盖变化。
