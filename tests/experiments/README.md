# 审计 JSON 输出能力试验

显式选择 `-tags live`，使用已有 Codex 0.159.2、当前账号可用模型和普通非 MSIX PowerShell。所有仓库、会话、日志和进程均归新的 evidence-dir，不安装工具、不操作 SteerLine 安装目录、不创建真实 PR。

```powershell
go test -tags live -c -o <新目录>/audit-experiment.test.exe ./tests/experiments
# 从独立、隐藏的测试宿主运行该 EXE，避免继承调用方 Job；参数如下
audit-experiment.test.exe -test.run=^TestAuditJSONOutputExperiment$ -test.v -test.timeout=18m -native-exe=<原生EXE绝对路径> -native-shell=<普通pwsh.exe绝对路径> -native-model=<可用模型> -evidence-dir=<新的绝对路径>
```

通过真实业务 app-server adapter 分别执行零修复/有修复样本，对比固定的历史审计提示与 audit.code JSON 覆盖；历史基线在夹具中保留原文，生产默认更新后不会悄悄变成“新默认对比自身”。独立 linked worktree 和会话使用相同输入代码、模型/effort、受管 worker runtime 与四个固定 Git 写根；每个源最多一次原会话补报。记录原 final、session/turn journal、前后 Git 提交、额外获取次数及耗时；checkout 后、模型输入前冻结检查文件原字节并独立执行；原 Job 写者退出核实后才释放句柄。

选择 shell 路径并在本试验进程的 PATH 前置其目录，是为了避免 detached 宿主选到 WindowsApps 的 MSIX 别名，不修改系统 PATH、权限或认证。原生日志会包含试验材料，只用于本次隔离证据。

本入口是限定能力试验，不进入普通 L1，不计为真实 Run、发布或跨构建接管资格。通过也不能单独证明全后端默认可改；结果与默认决策登记在[可靠性交付简化验收](../../docs/acceptance/reliable-delivery-simplification.md)。失败/中断批次保留，修夹具须用新目录，不能覆盖旧结果。
