# 内容流程冻结配置接管修复

日期：2026-10-20。基线 c122d641677593d4b694e2eda7502001510d4e8b，与核验时 origin/main 相同。
根因：freeze 对内容流程保留零值 AcquisitionConfig，TakeoverFacts 却对全部 Run 无条件
Validate。旧内容单元夹具手工补 DefaultConfig，未覆盖正式 freeze 的真实形态。

生产修改限于 workflow 能力谓词、freeze 配置加载资格和 takeover 配置/owner 需求判断。
开发结果解释、report/parse 流程及实际 parser 意图继续严格校验；其他接管检查保持。
无公开字段、迁移、默认补写、兼容回退或新运行时。设计见
[内容流程 Spec](../spec/content-workflows.spec.md)及[恢复 Spec](../spec/service-generation-recovery.spec.md)。

本次要求的五项最小验证全部通过。没有运行 Full；没有使用或修改 SteerlineInstall、
SteerlineRuntime，也没有启动、重置或删除既有 i/j/k 现场资源。

## 固定交付构建

生产源码与测试提交：`4ad8cd8c368b13e141e62bedfad05ddea6a4c33b`，干净工作树构建。
后续提交仅归档验收证据，不改变该受测 EXE。

| 项目 | 实测身份 |
| --- | --- |
| EXE | `C:/Users/lmas1/.codex/worktrees/21ce/SteerRunner/.evidence/content-takeover-20261020/controlled/steerrunner.exe` |
| version | `0.2.0-dev.3-r2` |
| EXE SHA256 | `bb4e189db25d730050d5bd3b5dbf1e93eba4872b923687ff8b2e00c96dec35e2` |
| resource digest | `a48bfb1aa7250009b21b4170532d877be01c23375ac386cb39e269794263ef91` |
| 平台/工具 | Windows/amd64，Go 1.26.4，真实 Codex 0.159.2，gpt-6.1-sol/low |
| 原生 EXE SHA256 | `52f75c649bebb8001102a1dd129c1ea6d02b0940321e6d7e82ee0526753bd58a` |

[构建元数据](evidence/content-takeover-20261020/build.json)、
[公开 version](evidence/content-takeover-20261020/version-response.json)、
[原生身份](evidence/content-takeover-20261020/native-identity.json)均已归档。
受控、真实生成和原现场核验使用同一 EXE，没有复制到安装目录。

## 定向验证

| 范围 | 实际结果 |
| --- | --- |
| 正式 freeze | `TestContentFrozenTakeoverContracts`：合法 generate/conversation 经 ValidateRun → freeze → Store.Commit，零值配置不补默认；当前坏配置及后续变化不影响资格；原记录/存储/请求绑定不变 |
| 严格反例 | 未知 codec、workflow/version、执行记录、执行语义、冻结材料篡改、实际 parser 契约均拒绝；development 缺失/未知配置继续拒绝；原 owner 恢复检查保留 |
| L1 内容/恢复 | 最终定向 application、codex adapter、storage 筛选通过；application 44.381s，包含真实冻结 parser package 的契约反例。parser 子包该筛选未命中，不计其整包验证 |
| Workflow/config | workflow、resultacquisition 两包通过 |
| 公开 CLI 受控 | Generate 十子分支通过（70.33s）、Conversation 通过（12.48s）；已完成 text/JSON 和空闲 conversation 的 takeover 均通过，inspect 不变、零额外 turn/start |

实际命令：

```powershell
go test ./internal/application ./internal/execution/resultacquisition/codexexec ./internal/execution/adapters/codex ./internal/storage -run 'TestContent|TestServiceGenerationTakeover|TestTakeover|TestWorkflowRecovery|TestRAConfigRejectBeforeEffects|Test.*Recovery' -count=1 -timeout=4m -v
go test ./internal/execution/workflow ./internal/execution/resultacquisition -count=1 -timeout=2m
./scripts/test-foundation.ps1 -Pattern '^TestSRContent(Generate|Conversation)$' -OutDir .evidence/content-takeover-20261020/controlled
```

[最终定向 L1](evidence/content-takeover-20261020/targeted-pass.log)、
[workflow/config](evidence/content-takeover-20261020/workflow-acquisition.log)、
[受控日志](evidence/content-takeover-20261020/controlled/integration.log)、
[三条公开接管响应](evidence/content-takeover-20261020/controlled/takeover-responses.jsonl)。
新增测试夹具初次因 envelope 摘要/身份不完整而失败，已修正；targeted.log、
targeted-final.log、regression-recheck.log 保留失败历史，不当作通过记录。

## 单回合真实 Codex

新隔离目录 `C:/srt/gt-2587794173/state`，Run `811b0985c597a8b8f8d97caee5000a26`。
`TestSRContentGenerateTakeoverNative` 通过，21.63s。使用通过受控前置的固定 EXE，
独立 CIM 宿主执行 `-tags live` 编译的 native.test.exe；实际参数在
[source-identity](evidence/content-takeover-20261020/native/source-identity.json)。
公开 submit 的 execution.settings.executable 和[原进程回执](evidence/content-takeover-20261020/native/native-process.json)
均指向真实 Codex。测试宿主 helper 参数不参与该 Run 的路由；没有受控替身或第二模型回合。

实际顺序：公开 submit → inspect 读取原 content_result（succeeded、原 final verified）→
显式 stop → inspect 核对结果保留 → service stop 并等待原 writer 退出 →
verify-stopped（resources_settled=true）→ 同 EXE 离线 verify-takeover（compatible=true）。
generate 自身已自动结算，显式 stop 仍沿原记录关闭，不重复生成。

- [原 submit 信封](evidence/content-takeover-20261020/native/submit-request.json) / [原回执](evidence/content-takeover-20261020/native/submit-response.json)
- [原 content_result 公开响应](evidence/content-takeover-20261020/native/content-result-response.json)
- [显式 stop](evidence/content-takeover-20261020/native/explicit-stop-response.json) / [停止后 inspect](evidence/content-takeover-20261020/native/stopped-inspect-response.json)
- [verify-stopped](evidence/content-takeover-20261020/native/verify-stopped-response.json) / [verify-takeover](evidence/content-takeover-20261020/native/verify-takeover-response.json)
- [原始 CLI stdout/stderr](evidence/content-takeover-20261020/native/caller-commands.jsonl) / [原数据库不变](evidence/content-takeover-20261020/native/database-unchanged.json)

接管 revision=16，input_digest=`10857d3f0a898f367f1ea9d48e9415a6dfcb556f9adff0f5cd418cd0b0ac941c`，
request_id=`native-generate-takeover`，
request_digest=`9352c9d9b18c8e3414937a695a828fdc092568f5b7ca100bfbff85c9613fc557`。
断言与原信封、原 inspect、停止后 revision 一致，原 DB 在 takeover 前后逐字节不变。
compatible 仅证明冻结契约资格；资源结算由独立 verify-stopped 响应证明。

## 原现场只读复验

按原响应定位 `C:/slr/y6mgtaht4xl5jc2j3mufxchxoe`，未启动服务，未清理或修改现场。
修复 EXE 执行只读 service status 和以下命令：

```powershell
& $fixedExe service verify-takeover --candidate $fixedExe --state-dir C:/slr/y6mgtaht4xl5jc2j3mufxchxoe --json --protocol steerrunner-cli/v1
```

原 generate `f6942435176c8f5d5430b15591e9ee8f` 从 unsupported 变为 compatible；
development `8402cc9208fbecdd4164d59e0fedc47d` 仍为 compatible，聚合 compatible=true。
两项 revision、input_digest、request_id/request_digest 与原响应完全一致，没有其他阻断。
现场 181 个文件的路径、长度、SHA256 在检查前后相同，服务保持未运行。

[原失败响应](evidence/content-takeover-20261020/original/original-rejected-response.json)、
[修复公开响应](evidence/content-takeover-20261020/original/verify-takeover-response.json)、
[绑定对比](evidence/content-takeover-20261020/original/binding-check.json)、
[只读核验](evidence/content-takeover-20261020/original/readonly-check.json)及前后文件清单已归档。

## 范围限制

最小验收无未通过项。未运行 Full、全部 L1–L4、真实 conversation、多构建活动回合恢复、
真实 parser 或 SteerLine TM-A04 上层流程，不宣称这些项目已通过。
没有部署、推送或创建 PR；SteerLine 可按上述固定 EXE 继续复验。
