# Orca 带能力声明 RPC 包装复用验收

2026-11-05，合并前验证。基线 `a66038c89208392c47531e640c84e97728c62424`，隔离分支 `codex/orca-capability-rpc`；依据 [Spec](../spec/orca-capability-rpc.spec.md) 和 [Plan](../spec/orca-capability-rpc.plan.md)。Windows amd64，Go `go1.26.4`。

生产仅修改 `state_transport.go`、`usage_files.go`、`catalog.go`：现有包装移名为私有 `capabilityState`，structured usage 替换类型引用，目录两端连接后共用 `callState`。生产 19 行增加、25 行删除；没有新接口、连接策略层或持久状态。Orca、连接认证、身份检查、能力清单、错误分类和原 usage 授权机制均未改动。

## 等价验收

```powershell
go test ./internal/execution/adapters/protocol/orca ./internal/usage/access -count=1 -json
go test ./internal/usage/collect -run '^TestStructuredUsage' -count=1 -json
.\scripts\build.ps1 -OutFile .evidence/capability-rpc/steerrunner.exe
.\scripts\test-foundation.ps1 -Pattern '^TestSRNativeCatalogPublicCLI$' -RunnerExe .evidence/capability-rpc/steerrunner.exe -OutDir .evidence/capability-rpc/public-cli
```

| 执行 | 顶层通过 | 通过事件，含子测试 | 跳过 | 耗时 |
| --- | --- | --- | --- | --- |
| 原主线 protocol/orca | 61 | 355 | 4 | 27.312 秒 |
| 原主线 usage/access | 9 | 18 | 0 | 1.024 秒 |
| 候选 protocol/orca | 62 | 359 | 4 | 26.133 秒 |
| 候选 usage/access | 9 | 18 | 0 | 0.996 秒 |
| 候选 collector 定向 | 2 | 12 | 0 | 0.596 秒 |
| 同 EXE 公开 CLI 专项 | 1 | 1 | 0 | 1.42 秒（案例本身） |

以上均 exit 0、失败为 0。网络/IPC 测试在沙箱外执行。协议包跳过的四项仍为 `TestOrcaLocalPipeNativeEvidence`、`TestOrcaLocalIPCNativeProbe`、`TestOrcaTerminalQueryLive`、`TestUsageLocalAcceptanceRepoCleanupLive`，未提供其显式前置，不能计为真机通过。[测试摘要](evidence/orca-capability-rpc-20261105/tests.json) 保留实际计数；本地完整日志为 `.evidence/capability-rpc/{baseline,candidate,collector}.jsonl` 及 `public-cli/integration.log`，未将全文纳入仓库。

| 边界 | 核对结果 |
| --- | --- |
| Structured usage 能力及原授权范围 | 强化原加密 fixture 的精确握手能力和引导 status 的缺省能力/null 参数断言。原 Sessions 用例通过；新增 `TestStructuredUsageScopeOperationsKeepCapabilities` 经生产公开入口执行 group/folder 创建、删除、`selector_not_found` 撤销证明，参数和后续能力与原调用方一致，恰好 5 次 scope RPC，业务派发 checkpoint 为 0。实际授权准入、撤销及未知删除不重放仍由原 usage/access 9 个顶层测试验证，不将 fixture 的可调用性解释为任意写授权。 |
| 目录能力与回执 | 原本机矩阵由能力数量断言收紧为精确内容和顺序。远端矩阵精确核验加密握手、token、引导请求不带 clientCapabilities/params=null、之后每次携带原目录能力；覆盖正常、原 listing 等待、能力不足、错 runtime、Pi 目录，加上错请求 ID、错 pairedDevice、丢回执。每场连接恰好 1 次，方法序列与基线规则一致，错误不补发。 |
| 目录关闭 | 原本机夹具等待连接 reader 退出；远端新增在 QueryCatalog 返回后等待服务端连接结束的断言。正常及错误路径均通过，多次原目录 RPC 仍复用同一连接；未提前在单次 RPC 后关闭。 |
| #113 文件绑定 | 原六种配对场景、Windows 文件 scope/runtime/host 负例通过。每次解析计数 1，同一配置拨号，替代端点连接与文件请求均 0；生产文件入口无 diff。进度、停止后封存和排空合同未动。 |
| 原身份与预算 | 包内 usage 原身份拒绝、目录元数据边界/身份校验、错请求、断线、取消和超限矩阵通过。`state_local.go`、`state_stream.go`、nativecatalog 预算/投影、公开错误及 CLI 均无生产 diff。普通 usage、本机 usage 和 agentSession 业务派发沿原实现。 |

## 同一候选双端正式目录联验

固定构建身份见 [build.json](evidence/orca-capability-rpc-20261105/build.json)：版本 `0.2.0-dev.4`，源码基线为上述 main，dirty=true，source_digest=`c0119bbbc6725f6f6378d2a3c5473b383f23361bdb9974e1a791c8b1f6cb4400`，EXE SHA256=`aa251cc81f474ccff5997cc2d65f2798641dbaeabcc7e65eed0860e61854dc12`。该源码摘要包含最终测试修改；联验后重算一致。公开 CLI 专项、四次真实查询均使用此 EXE，公开响应的 executable_sha256 与构建记录一致。

每次正式调用 `steerrunner.exe native-catalog --input - --json`，仅通过 stdin 提交原查询对象，未创建业务 Run、调用模型、安装远端 Pi 或修改 Orca。完整公开请求、响应与进程退出记录见 [live.jsonl](evidence/orca-capability-rpc-20261105/live.jsonl)。凭据字段检查通过，四次 stderr 均为空；未保存 Orca 配对文件或认证正文。

| 目标 | 请求 | 结果 | 数量 | 耗时 |
| --- | --- | --- | --- | --- |
| 本机 Orca | agents | ready、complete=true、exit 0 | 8 | 0.126 秒 |
| 本机 Orca | models，agent=codex | ready、complete=true、exit 0 | 8 | 4.510 秒 |
| 配对远端 | agents | ready、complete=true、exit 0 | 7 | 0.190 秒 |
| 配对远端 | models，agent=codex | ready、complete=true、exit 0 | 8 | 3.378 秒 |

远端为已登记 `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`；双次响应原 runtime 一致，本机为 `acbe88cf-6a61-4eeb-8178-6d4886af40c8`，远端为 `21d98aa2-d284-41f3-a6cc-42fb22d728a2`。四个短命 CLI 进程均已退出，因此不再持有本次管道/套接字；协议夹具另核对显式关闭。未将目录 ready 推导为正式环境检测、模型可调用性、usage 真机或全部 Agent 执行资格。

## 修改审计与资格边界

逐行对照确认：包装的固定 false 仍只绕开业务输入 gate，不限制原 usage 授权范围操作；next/close 透传同一 socket。目录共用调用闭包建立在两端原连接验证之后，原 runtime 闭包、pairedDevice、引导状态请求、能力、listing 次数、错误透传、取消、预算和所有权未变。没有复用按次关闭的 agentSession 业务调用器。未发现需修复的运行逻辑缺陷。

本次补齐了原 fixture 对精确能力、授权范围操作、原回执及连接关闭的验收缺口，全部按影响验证通过。gofmt、`git diff --check` 与文档链接核对通过。以上为受影响包、定向 collector、单个公开 Controlled 案例及双端真实目录资格；未运行完整 L1/Controlled 或 L2/L3，也未新建 development Run 或重做原目录排空。不追认历史 #113/#117 证据为本候选资格，已有发布与停止/封存/排空门禁保持，遵循 [测试指南](../testing.md)。
