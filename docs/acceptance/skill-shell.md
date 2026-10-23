# 双 Skill 薄壳与独立 Viewer 验收

日期：2026-10-23。对应 [Spec](../spec/skill-shell.spec.md) / [Plan](../spec/skill-shell.plan.md)。实现完成；本机 Codex 和一个既有 Orca environment 的 Spec+Plan → 问答 → 经 Runner 核验 PR → 原 Run 停止 / 停服核验均取得真实证据。本报告只覆盖以下构建与场景，不宣称全部后端组合或干净 Windows 虚拟机均已验证。

## 同日 Rebase 集成复验

已 rebase 到 `origin/main` 的 `5401914`，保留主线 Orca 最后消息及 Codex `>= 0.159.2` 支持。README 和 CLI 指南已更新；共享 helper 不再把 `0.159.2_or_newer` 当作固定版本字面值，按所选 Runner 的能力声明筛选版本，正式协议与权限核验仍属于 Runner。

本次重新构建 Runner SHA-256 为 `185802B37291EEB0162415A2A92BD53C72FCE1055E72D9CE774F1F14EEFDFAC8`，来源为 `17b7d96-dirty`，见[构建记录](evidence/skill-shell-20261023/rebase-5401914/runner-build.json)。[复验汇总](evidence/skill-shell-20261023/rebase-5401914/result.json)及[新包清单](evidence/skill-shell-20261023/rebase-5401914/manifest.json)对应本节；后文原始真机 PR / 发行包证据仍对应当时旧构建，不改写为本次真机通过。

- [客户端 32 项](evidence/skill-shell-20261023/rebase-5401914/client-controlled.json)通过，新增 9 个版本反例/正例，包括 `0.159.10` 数值比较、较新 `0.163.0-alpha.10.2`、基线预发布版、坏版本等。首次测试受到本机备用原生 EXE 影响；最终矩阵同时隔离 PATH 与 APPDATA，零真实模型或 PR。
- 六个[生产包回归](evidence/skill-shell-20261023/rebase-5401914/production-tests.log)及 application/codex 的[投影、最后消息、版本/权限探测定向测试](evidence/skill-shell-20261023/rebase-5401914/compatibility-tests.log)通过；未重复整包 application 长回归或 L2–L4 全集。
- 同版新 EXE 的[两个服务展示集成用例](evidence/skill-shell-20261023/rebase-5401914/display-result.json)、[Viewer 原生窗口 9 项](evidence/skill-shell-20261023/rebase-5401914/viewer-native.json)、[安装包 6 项](evidence/skill-shell-20261023/rebase-5401914/package-install.json)通过。Viewer 自身模块测试和 vet 通过，EXE SHA 与原 Viewer 相同，文件契约保持 schema v1。

历史快照/PR/停止回执保持原字节；本次只增加本节与新证据目录，不重新发起真实执行。

## 构建与实现边界

Runner 基线 `1cfe2f9a065830ab62d540d3de4c0011ed603b56-dirty`，版本 `0.2.0-dev.3-r2`，Windows/amd64，使用 Go 1.26.4。受测 Runner SHA-256：

```text
1824CFBFC55EC9737745E0A3D46A884331120DA40770848460332A18A17890CE
```

最终 Viewer SHA-256：

```text
10F785F272773FB2EDD714E5EDDBA59D61DB9CC4A267C937ADCBF6D394943692
```

最终文件摘要见 [manifest-final.json](evidence/skill-shell-20261023/manifest-final.json)，构建来源见 [runner-build.json](evidence/skill-shell-20261023/runner-build.json)。两条真实链路使用同一 Runner EXE；最终 helper 补充了原请求字节完整性检查及 Orca 查询失败分类，另以最终客户端矩阵复验。证据目录中无 `-final` 的包/客户端报告是中间验证，保留原字节，不替代最终清单。

| 组件 | 实现与边界 |
| --- | --- |
| 两个 skill | `steerrunner-configure`、`steerrunner-run`，短说明及 UI 元数据；没有第三个查看 skill |
| 一份 helper | `client/steerrunner-client.ps1`，Windows PowerShell 5.1；参数转义/严格 JSON、配置、保存请求、固定 CLI 调用。无新业务状态机或持续采集 |
| Runner 展示 | `Engine.DisplayRuns`、`internal/runview`、原服务的一个串行导出 goroutine；复制白名单后释放锁再写文件，不新增业务 CLI、数据库字段、workflow 或 provider 查询 |
| 独立 Viewer | `viewer/` 自身 Go module、标准库、原生 Win32 四列窗口；只读 JSON，不 import Runner，不查 CLI/数据库/Orca，不写缓存或控制执行 |
| 安装/打包 | `install-client.ps1`、`package-client.ps1`；两个 skill 定位同 helper，已有目录/同名 skill 拒绝覆盖，清单记录组件身份及文件摘要 |

客户端配置/记录、Runner state-dir、展示快照目录分开。发行物不带测试夹具、私有数据库、执行主机凭据或开发 checkout 路径；生成的定位资产只含该次本机安装位置。

## 本次验证

| 验证 | 实际结果 |
| --- | --- |
| 受影响生产包 | `runview/application/host/cli/storage/resources/contracts` 全部通过，application 578.812 秒。新增投影测试另补真实 SQLite 事务回滚：未提交 title/status 不可见。见[命令汇总](evidence/skill-shell-20261023/verification.json) |
| 最终客户端矩阵 | 23 项通过：实际生产 validate、无参数配置、零业务写、保留选择、原 ID 对账/并发/丢响应、旧问题、省略 question、UTF-8/引号/中文/绝对路径/CRLF、材料变更阻止、坏配置、Orca 缺失/空列表/离线/认证失败区分。见[最终报告](evidence/skill-shell-20261023/client-controlled-final.json)。执行接收部分是受控公开 CLI 夹具，不算真实模型闭环 |
| Runner 文件联验 | 实际 EXE/服务、受控原生协议、直接 CLI 提交的非薄壳 Run、PR 行及客户端退出后心跳通过。见[结果](evidence/skill-shell-20261023/display-service.json)、[快照原样](evidence/skill-shell-20261023/display-snapshot.json) |
| 来源 / 换代 | 实际服务、空列表、大小写路径同 writer、两个 state-dir 不同来源、重启后同 source 新 instance；通过。见[结果](evidence/skill-shell-20261023/display-lifecycle.json) |
| 导出故障 | 排序、变化/心跳、未知替换不复用序列、失败退让、读取失败不造空数据、Windows 共享删除原子替换、取消后等待原 publisher 收尾；通过 |
| Viewer 模型及 I/O | 四项测试通过：14 秒旧文件剩 1 秒、同序列恢复不续期、前跳后回拨不复活、睡眠经过时间、新实例、重复键/未知版本/危险链接。私有 Windows 管道制造真实 pending ReadFile，另一来源仍可读，取消后原读收尾 |
| Viewer 原生窗口 | 单独复制 EXE、真实窗口/四列/JSON 行、旧文件过期、坏文件保留行、同 Run ID 多来源、有效整份快照删行、文件删除保留旧行、关闭后干净退出；9 项通过。见[结果](evidence/skill-shell-20261023/viewer-native.json) |
| 最终包安装 | 源码外安装、两 skill 同 helper、默认 Configure 定位包内 Runner、已有安装/skill 不覆盖、摘要损坏拒绝、搬迁保留配置且原定位失效；6 项通过。见[最终安装结果](evidence/skill-shell-20261023/package-install-final.json) |
| 静态检查 | 两份 skill quick_validate、Viewer 独立模块测试/vet、受影响 Runner 包 vet、git diff --check 通过 |

服务每秒采样、变化发布、五秒心跳；Viewer 每两秒后台读取，每 250ms 更新期限，恢复绘制前重算。期限由采样 UTC 和包含睡眠的 `GetTickCount64` 保守判断；同份快照一旦过期保持过期。单文件读取预算两秒，同来源一份在途读，取消后不接收迟到结果。错误、删除、未知 schema 不造空列表或 stopped。

写入关闭非 JSON 临时文件后，覆盖使用 `ReplaceFileW`，首次发布使用 `MoveFileEx`。测试中 `MoveFileEx` 覆盖共享打开目标失败，因此采用前者。超时尝试取消同步 I/O，始终等待原操作结束；Windows 不承诺所有文件操作可取消。停服等导出收尾才释放 writer lease；25 秒未收尾记“未核实”并继续持 lease，不能当作成功停服。没有新增写者或后台采集进程。

## 真实闭环与失败保留

使用授权测试仓库 `erranliu/SteerlineTest`，默认 `development/2`、Codex `0.159.2`、sandbox。材料要求先询问口令，收到 `shell-client-ok` 后仅添加 `shell-proof.txt`。成功 PR 均经 Runner provider 核验，独立 gh 查询确认差异只有预期文件与内容。

| 路径 | 原 Run | 结果 |
| --- | --- | --- |
| 初次本机深目录 | `251ddbac4f91b099c9055430296bbcb5` | authentication_api_socket_path_too_long 阻断；显式停止并核验，没有自动换目录重提 |
| 本机显式短目录 | `47d2f408076ea79d5e283c240dd87669` | 问答、实施、提交成功；模型选择 broker 禁止的 REST PR 列表查询，发布阻断。后续业务反馈不满足原阶段续执行资格；保留失败并停止 |
| 既有 Orca environment | `cef6e9d2f895de2da3d7122c9e3d7163` | 问答 → pr_ready → [PR #655](https://github.com/erranliu/SteerlineTest/pull/655)，head `f03b004073ea3366fb730550f843a95df590b8c6` |
| 本机新材料、原权限 | `250d9b17e5d0c631ae6c1a6ee12529d7` | 问答 → pr_ready → [PR #656](https://github.com/erranliu/SteerlineTest/pull/656)，head `fda725481765e08f0c7da2918012f177082e000f` |

本机成功用例的新基线材料说明既有允许的发布查询方式；没有改 workflow、旧请求、认证策略或放宽为 full_access。“本机通过”证明默认 adapter/workflow/权限形状及显式短 state-dir 的闭环，不证明任意默认深目录都可运行。现有 Runner 的 106 字节认证 socket 预检仍会阻断过深目录；用户可为未来 Run 明确选短目录。

远端为既有 Windows Orca environment、已注册 `C:\Projects\SteerlineTest`，公开 repo list/show 核对 Git remote 身份；没有复制调用方项目路径。测试准备经一个 owned Orca 终端获取固定材料基线，已显式关闭；配置发现没有创建该终端。工具/登录及基线可达性是现场前提，静态发现不冒充证明。远端没有部署调用方 helper，也未用远端完整 environment-check unsupported 阻断普通 Run。

四个原 Run 均显式 stop，原服务再 service stop。最终公开核验均为 resources_settled:true：[深目录](evidence/skill-shell-20261023/longpath-verify-stopped.json)、[本机发布阻断](evidence/skill-shell-20261023/local-verify-stopped.json)、[本机成功](evidence/skill-shell-20261023/local2-verify-stopped.json)、[远端成功](evidence/skill-shell-20261023/remote-verify-stopped.json)。远端 stop_evidence_scope=orca_terminal，仅核验原终端，脱离终端的后台进程和执行主机凭据未核验/撤销。

[PR #655](evidence/skill-shell-20261023/pr-655-closed.json)、[PR #656](evidence/skill-shell-20261023/pr-656-closed.json)已关闭、未合并。两条发布 head 与两条材料 base 以原 SHA lease 删除，见[清理结果](evidence/skill-shell-20261023/owned-branch-cleanup.json)。保留隔离 state-dir、固定请求及本地 Git 材料作证据，不删除其它运行/仓库对象。公开字段见 [live-results.json](evidence/skill-shell-20261023/live-results.json)，完整现场在 `.evidence/skill-shell/live/`。

## 验收资格与未验范围

| Spec 条目 | 本次资格及限制 |
| --- | --- |
| SS-01 / SS-10 | 两技能/一 helper、系统 PS5.1、源码外安装通过；无测试设施运行依赖。未另装干净 Windows VM，未运行通用 L2–L4 全集 |
| SS-02 / SS-03 / SS-04 | 实际发现、生产校验及上述矩阵通过；未穷举 SSH-only、多远端项目歧义、未知工具版本和部分超时排列 |
| SS-05 / SS-06 / SS-07 | 真实 Git/生产 validate、原 ID/正文、丢响应/并发/服务替换、旧问题和控制接线通过；未逐指令点注入保存前后崩溃 |
| SS-08 | 显式短 state-dir 的本机默认执行形状通过；深目录和模型发布查询失败保留 |
| SS-09 | 一个 Windows Orca environment 的 Codex sandbox 普通开发闭环通过；其它远端 agent/权限/OS 组合未验 |
| SS-11 / SS-12 | 白名单/回滚/直接 CLI Run/多目录/重启、独立 EXE 与原生窗口通过；未在第二台无开发工具机器重测；junction 别名复用 host.Normalize，未另做专项 |
| SS-13 / SS-14 | 期限/多来源/实际慢读、心跳/换代/原子替换/原写者收尾通过；实际磁盘永久挂起、ACL 拒绝、不可取消 replace 跨完整停服期限未注入，休眠为受控经过时间而非真机挂起 |

这些未验项不折算通过，后续按具体组合补证，不增加新服务、配置层或测试专用生产接口。

## 复验入口

包输出和安装目标须为新目录；使用同次 Runner EXE 复验：

```powershell
.\scripts\build.ps1
.\viewer\build.ps1
.\scripts\test-client.ps1 -RunnerExe .\bin\steerrunner.exe -ViewerExe .\bin\steerrunner-viewer.exe -OutDir C:\Temp\sr-client-new
.\scripts\package-client.ps1 -OutDir C:\Temp\sr-package-new
.\scripts\test-client-package.ps1 -PackageDir C:\Temp\sr-package-new -OutDir C:\Temp\sr-install-new
go test ./tests/integration -run '^TestSRDisplaySnapshot' -count=1 -timeout=4m -args -runner-exe C:\absolute\steerrunner.exe -evidence-dir C:\Temp\sr-display-new
Push-Location viewer
go test ./... -count=1
go vet ./...
Pop-Location
```

这些入口不自动执行真实模型或创建真实 PR。Viewer 是独立 module，不能从 Runner 根模块用 `go test ./viewer/...` 代替其测试。`display-snapshot.json` 是原样证据；作为 Viewer 夹具时，文件名须用其中 source_id 加 `.json`，旧生成时间会立即显示过期。
