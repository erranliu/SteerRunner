# SteerRunner P0/P1 基础层验证记录

仅对应[首版独立化专属验收](../steerrunner.md)的 P0/P1 覆盖，不代表整个首版验收通过。

| 输入 / 产物 | 实际值 |
| --- | --- |
| 原始 Spec/Plan 固定版本 | `60c1b0900b25eced5cb9e4ffb9c636995bc8a69e`，`docs/spec/steerrunner.spec.md`、`docs/spec/steerrunner.plan.md`；实施细节修订随下列实现提交 |
| 实现 / 当前契约版本 | `7cfad777b0e41e19000cc6fda6cc21973109e0ad`，`codex/steerrunner-foundation`；构建及测试时工作树干净 |
| 程序版本 / 协议 | `0.1.0-dev.1` / `steerrunner-cli/v1` |
| EXE SHA-256 | `389E4E2892C51AD5578F3F52C04009776A0978652E836F6ADFDD0B70AD1EE97B` |
| 进程测试 EXE SHA-256 | `4335E163E3D79ECF00716425EC47A2295F150A9C57843CBF8A3E0649FC2913D7` |
| 资源版本 / SHA-256 | `1` / `174db6b2d59cd21b608a71fa322e758284a9e944a010b5aefd201d251ada90e0` |
| 工具 / 系统 | Go 1.26.4 windows/amd64；Windows NT 10.0.26200.0 |
| 完成时间 | 2026-10-14 11:27:31 UTC |
| 执行后端 / provider | 未接通；只验证拒绝，不启动代理、不查询远端平台 |
| SteerLine 来源 | 最终 fetch `origin/main` = `2f7312b485cfeebf9a5cfdea127ce0041eef6094`，无 workflow/reliable，P2 交接未完成 |

在 `C:/Project/SteerRunner` 实际执行：

```powershell
.\scripts\test-foundation.ps1
go vet ./internal/contracts ./internal/cli ./internal/host ./internal/storage
```

脚本使用 `scripts/build.ps1` 构建实际 EXE，并运行：

```powershell
go test ./internal/contracts ./internal/cli ./internal/storage ./internal/host -run '^(TestSRCLIContract|TestSRShutdownReceipt|TestSRUnsupportedStorage|TestSRStartupIntent)$' -count=1 -timeout=2m -v
go test -c -o <本次证据目录>/integration.test.exe ./tests/integration
```

随后从独立系统测试宿主执行编译后的测试，筛选 `^(TestSRStandalonePackage|TestSRServiceOwnership)$`，超时 5 分钟，显式指定本次 EXE 和证据目录。没有运行全量或真实模型测试。

| 验证 | 结果 / 事实 |
| --- | --- |
| U1 / U1-S / U1-H | 4 个包的 5 个定向顶层 Test 均通过，无skip；严格CLI输入与拒绝、回执持久/冲突/旧实例、未知启动不重建、外来DB拒绝且字节不变 |
| C1 / TestSRStandalonePackage | 通过；只复制EXE，中文状态路径，version→服务→capabilities；未接通 submit 返回3且 inspect 确认不存在已接受回执 |
| C2 / TestSRServiceOwnership | 通过；只读不启动、并发别名同实例、不同目录隔离、允许/禁止父Job、独占写者拒绝、进程死亡后换代及旧目标拒绝、停服回执跨重启及旧请求不停止新实例 |
| 进程结果汇总 | `exit_code: 0`，`scenario_count: 2`，无skip |
| go vet / 文档 | 定向 vet 通过；基础实现提交前 diff --check、5份Markdown的19个本地链接及3个JSON示例通过。本文加入后的链接检查另按交付工具输出 |

原始本机证据根：`C:/Project/SteerRunner/.evidence/foundation-20261014-112709/`。其中 `source-identity.json`、`steerrunner.exe.build.json`、`unit-foundation.log`、`integration.log`、`caller-commands.jsonl`、`result.json`、`vet.log` 与实际两个 EXE 一起保留；原始大文件不入 Git。`bin/steerrunner.exe` 是该已验 EXE 的副本。

早期失败记录见专属验收 §6；修复后验证没有覆盖或删除失败目录。完整 Run 快照/执行/恢复、原生日志事件、Git/PR、SteerLine CLI接入/删码及真实PR仍未验，不由本记录替代。
