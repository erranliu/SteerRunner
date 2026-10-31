# 宿主认证实施与验证

> 后续审计与本机完整 L2 已通过，最新范围见[审计与 L2](host-native-auth-audit-l2.md)。下文保留首轮实施验证的构建与缺口。

日期：2026-10-31。调查及提交基线 `cf8df21374555bde90baf081c21f4d9581580f42`，工作分支 `codex/host-native-auth`，包含未提交改动。依据 [Spec](../spec/host-native-auth.spec.md) / [Plan](../spec/host-native-auth.plan.md)。代码已实施；最终 L1 与完整 Controlled 重跑已通过。本记录首轮未执行实际模型/PR/Orca 真机交付；后续本机 L2 已在独立记录验证。

## 实施范围

开发执行直接使用执行主机已有 Git/gh/SSH 登录。删除凭据 resolver、broker、CA、API socket、capability、期限、认证控制 RPC 和认证关闭证明。Codex shell 环境和 Orca hooks 独立接线，Pi/DSH 工具使用隔离 Agent HOME 之前保存的真实宿主环境。原生模型桥、READY/bootstrap、输入回执、Job、签名 writer-tree 退出与 publication provider 查询认证保留。

使用全新 state/v6、SQLite user_version 6；拒绝旧目录。默认新目录 `%LOCALAPPDATA%/SteerRunner-v6`，不改写已保存客户端 state_dir/实例绑定。开发 Codex profile 4/resources 5/v12；read_only profile 3/resources 4/v10。Orca 本机 profile 6/state 8，原生 environment profile 4/state 7。helper metadata 5、inline transport v4；Pi/DSH tool contract v3、receipt 4、config/startup-exit/exit 2。

控制 checkpoint v3 不依赖可空 readiness 计算路径；无查询和未就绪时仍可关闭原控制终端。关闭前结算已有 reader，先持久化 Closing，再按原身份关闭/核验缺席，最后保存 Closed。未知关闭不重发。该证明不替代业务 helper 的签名树退出，也不把无 readiness 的未知 writer 当作已停止。

## 最终结果

- `go build ./...` 与 `go build ./cmd/...`：通过；live 主测试驱动编译通过。历史证据目录新增独立 `go.mod` 边界，旧 Go 快照不参与当前产品构建，原文件字节未改。
- 正式 L1：717 个顶层测试通过，29 个扩展测试通过；16 个既有显式原生 opt-in 按规则跳过，不计真机通过。原生 driver 编译及零匹配失败门通过。
- Controlled：完整 catalog 25/25 通过，零跳过；修正旧规则断言后使用原有 foundation harness 全量重跑。运行的是与 L1 后固定构建完全相同的 Runner SHA。
- 宿主环境、helper metadata、控制关闭定向回归：通过。
- Orca adapter 全包与 usage collector：通过；只读内容/parser 接管定向回归通过。
- 正式 Pi/DSH 扩展：29/29 通过。
- 实际 Codex 0.159.2，无模型输入：loose/packed refs 各一次 commit 通过；read_only command/exec 可读且拒绝写入，原进程退出已核验。未运行 push/PR，未修改账号登录。

## 原生未验范围

首轮实施验证未提供专用 fixture/publication 及执行配置。正式 ListAvailability 已记录 local-codex.smoke、orca-local.codex、orca-local.pi、orca-local.dsh、orca-remote.smoke 全部 unavailable（missing_fixture/missing_configuration），零实际派发。因此实际 Git/gh 发布、多模型切换、Orca 本机/远端真实运行、DSH 真机 HOME 及真实双 Run/重启不声明通过。其受控回归不能替代实际资格。

## 失败记录

首轮在执行沙箱内运行，Windows PowerShell package verification 被 AuthorizationManager 拒绝；另一个沙箱内全包探测有 loopback/Git/temp file 权限失败。保留原日志，Windows 原生检查改用实际用户环境运行。

第二轮确认只读内容 profile 仍使用开发版本，已修复为只读 codec 3；旧 Orca codec 升级义务按用户允许丢弃旧记录删除，改为新 codec 回放与 usage 绑定测试。该轮还发生测试编译期间修改测试函数名导致的 _testmain 失败，不能计作最终稳定源码的回归结果。

原生只读探针先暴露夹具目录未创建与收尾未等待进程退出，修复测试夹具后通过。Controlled 首轮的 ServiceOwnership/remote_offline_scope/legacy 仍要求退役 profile 触发 inventory 查询，与新格式的零副作用拒绝冲突；改为固定旧 codec 3，并断言零外部调用。生产代码未因此修改；同一 Runner 下重跑完整 catalog 的 Controlled 集合，保留首轮失败报告。失败没有追认为成功。

历史 evidence/archive 原文件未修改。仓库级 build 曾扫描历史 Go 快照，因引用删除的认证 API 失败；增加证据目录的独立模块边界后通过，不为归档恢复生产 API。正式报告与原始失败均已保留，见下列证据。

## 命令与证据

- 正式入口：`.\scripts\test.ps1 -Level L1,Controlled -OutDir .\.evidence\host-native-auth-final`。其中 L1 通过；首轮 Controlled 为 24/25，旧 profile 查询断言失败。原报告状态保持 failed。
- 修正仅在 `tests/integration/verify_stopped_test.go`；复用 `tests/catalog.json` 的完整 `controlled_scenarios` 构造精确 Pattern，通过原 `.\scripts\test-foundation.ps1 -Pattern <完整集合> -TimeoutMinutes 45 -RunnerExe <固定 Runner>` 重跑全部 25 场景。未重复运行不受该测试文件影响的 L1，也未修改测试前置制度或伪造原报告。
- 原生无模型命令：`go test -tags=live ./internal/execution/adapters/codex -run '^(TestLinkedGitNativeCommit|TestHostNativeReadOnlyCommand)$' -count=1 -timeout=3m -v -args -host-native-executable <Codex 0.159.2绝对路径>`。
- 完整现场：工作区 `.evidence/host-native-auth-*`；可随源码审查的摘要与日志见 [qualification.json](evidence/host-native-auth-20261031/qualification.json)、[L1](evidence/host-native-auth-20261031/l1-result.json)、[Controlled 重跑](evidence/host-native-auth-20261031/controlled-final-result.json)、[首轮失败](evidence/host-native-auth-20261031/formal-first-summary.json)、[原生探针](evidence/host-native-auth-20261031/native-final.log)、[真机可用性](evidence/host-native-auth-20261031/native-availability.json)。
- 固定 Runner SHA256：`72432bfe7f3d6bce98d5f3b27d533d317cbc9c3cdc31cdf9f5e472d336fc6c2e`。构建与来源：[runner-build.json](evidence/host-native-auth-20261031/runner-build.json)、[生产代码文件哈希](evidence/host-native-auth-20261031/source-files.json)、[测试断言修正](evidence/host-native-auth-20261031/controlled-assertion-fix.json)、[helper/扩展 metadata](evidence/host-native-auth-20261031/terminal-metadata.json)。

首轮实施未部署、未改写真实账号登录或已安装版本，未创建真实测试 PR；后续本机 L2 的测试 PR 单列于审计记录。历史报告中的成功、失败和未验结论保持原样。
