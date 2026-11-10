# 工作区核验成本精简

日期：2026-11-10。基线 `5ccc3f0aa07764657b3346aa6023b73b878d76e0`，受测对象为分支 `codex/reduce-verification-overhead` 的未提交改动。全部修改位于独立 worktree `C:/Users/lmas1/.codex/visualizations/2026/11/10/01a1c455-f92f-7c60-9a1f-45ab317a09bf/SteerRunner`，未改原仓库工作文件或安装版。

## 行为与成本

`ResolveRepository` 一次 `rev-parse` 读取绝对 checkout/common-dir，要求无歧义的两行结果；`Verify` 复用同一批量读取核对 worktree 身份。`InspectAudit` 将索引标志与 stage 的读取合为 `ls-files -v --stage -z`，保留 NUL 记录边界及隐藏索引标志/gitlink 拒绝。

本机正常审计检查、不带 preHead 时，Git 进程数 **19 → 14**（减少 5 次，约 26%），由真实 Git 测试断言计数。带 preHead 仍额外执行祖先关系核验。发布查询前后各一次完整来源检查继续保留，对应两次正常检查 **38 → 28**；不将此计数宣称为总耗时收益。

源仓库、worktree、common-dir、路径重定向、分支、固定 base 祖先、sparse/隐藏索引/gitlink、干净提交、最终 HEAD 一致性均继续核对。审计前后完整身份围栏、provider 查询前后新鲜来源核验、revision/授权/stop/CAS，以及未知操作与资源结算不变；无跨请求或可变工作区缓存。

## 独立审核

只读审核者 `/root/review_runner` 独立检查实际 diff、主要调用者、InspectFixedAudit 读者结算及发布来源/CAS 路径，明确批准，未发现需修复的 P0–P2。确认 19→14 计数；两行路径协议与既有 file-source 协议一致，在当前 Windows/amd64 目标无新增合法路径限制。

## 验证

证据根：`C:\SteerlineDebug\wt-5d04-verification\`。首次沙箱内 owner 测试因临时 Git config、junction 和文件替换权限失败，未计通过；正常权限下使用独立临时目录重验。首轮正式 L1 的 `TestSRRepositoryCacheWithLongBranchPath` 在较深临时根下 clone 后无法识别 Git 仓库；不涉及本次工作区检查调用。保持代码与用例断言不变，将临时根改为 `C:\slt\verify-5d04` 后该用例定向通过；失败日志保留，完整 L1 已在短临时根重验通过。

| 命令 | 结果与证据 |
| --- | --- |
| `go test ./internal/execution/workspace -count=1 -json` | 29 个测试及子测试通过，无失败/skip；`diagnostics/runner-workspace-cdcc0b9f7c25422b891a0dbd0d9a0d11/go-test.jsonl`。含 14 调用计数、dirty/sparse/隐藏标志/gitlink、扫描期间分支变化、HEAD 变化、未结算读者及外仓库拒绝。 |
| `./scripts/test-prerequisites.ps1 -Stage L1 -OutDir <独立目录>` | 首轮失败：仅 `TestSRRepositoryCacheWithLongBranchPath` 失败，757 个顶层用例通过；`runner-l1-9b61ac6e9b14481cb7a35e9f72158b87/stages.json`。短根完整重验通过：758 个顶层 Go 用例，框架/Node 扩展及原生 driver 编译与零匹配保护通过；`runner-l1-short-aca5d5d25e2549c580623bb48f28ca10/stages.json`。原生 opt-in 默认跳过沿正式 L1 白名单，不计真实链验证。 |
| `./scripts/test-controlled.ps1 -Group workflow,recovery -OutDir <独立目录>` | 原组 12 个顶层场景，11 通过、1 失败，组状态保持 failed；`runner-controlled-de2cd599cfce4a7e929e1e7f7f48797e/summary.json`、`controlled/integration.log`。唯一失败及定向补验见下节。 |
| `go test ./internal/application -run '^TestSRRepositoryCacheWithLongBranchPath$' -count=1 -json` | 短临时根定向通过；`diagnostics/runner-longpath-ed09fda3f69d426eb4e2f81b1ddedd64/go-test.jsonl`。 |
| `git diff --check` | 通过。 |

本次不运行真实模型/真实 PR，不部署或修改原服务，不声明完整 L2/L3 资格。

## 受控原生回执失败与定向补验

原组 `TestSRNativeUnknownAndStop` 无 `native/calls.jsonl`，原操作回执未核实，清理报停止未知及 `resources_settled=false`。保留现场 `C:/Users/lmas1/AppData/Local/Temp/sr-54729125`，原 Run `005699ee47ad997a2f059fe9afa6103b`，未删除或伪造清理。其他 11 个场景通过，含 8 个结果获取子场景、7 个资源创建恢复窗口，以及 graceful/kill/creation_receipt/pending_stop 四个重启窗口。

只读审核者 `/root/review_runner` 复核发现：launch.log 的直接 Job 隔离拒绝后仍可走 broker，现场存在 owner-ready，不能将单条日志认定为最终启动失败。原 Run 已受理并准备工作区，owner ready 后原生准备/回执对账未核实；尚未到 Git 审计完成 checkpoint。受理前已调用本次优化的 ResolveRepository，因此不宣称完全未经过修改函数。根因未定，原停止未知及现场继续保留。

使用同一冻结 Runner EXE、原测试源码和停止断言，独立新现场运行：

```powershell
./scripts/test-foundation.ps1 -Pattern '^TestSRNativeUnknownAndStop$' -RunnerExe C:/SteerlineDebug/wt-5d04-verification/runner-controlled-de2cd599cfce4a7e929e1e7f7f48797e/controlled/steerrunner.exe -OutDir C:/SteerlineDebug/wt-5d04-verification/runner-unknown-stop-ae8fd02f574543fcaa45aeb381974f37
```

定向补验通过：`result.json` 为 exit_code=0、scenario_count=1，`integration.log` 完整 RUN/PASS；保留 `TestSRNativeUnknownAndStop-native-calls.jsonl`。补验覆盖丢原生回执不重发、撤销输入授权、原 Run 停止及结算，不替换原组失败，也不计完整 Controlled/L2/L3 资格。真实交付/恢复核验未为获得绿灯而放宽。

最终复核：只读审核者核对原组与补验的 Runner、integration/native fixture SHA 完全一致；补验原生 journal 只有一次 turn/start，公开记录显示 stop_status=stopped、new_inputs_allowed=false，最终 service 的 last_shutdown.resources_settled=true。原组 failed 和原停止未知保留。正式短根 L1 已完成，无需以补验拼装其资格。

## 后续根因与修复

后续读取原 revision 7 历史事件并进行基线对照，已定位为 owner 就绪文件发送前的 Windows 共享冲突读取缺陷，基线亦可复现；详见[独立根因与修复验收](owner-read-sharing-receipt.md)。原组 failed、原 Run 的停止未知及原现场仍保留；后续修复不追改上述历史结果。
