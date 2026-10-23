# SteerRunner

SteerRunner 是面向个人本机与多机开发的 AI 执行器。提供仓库中的 Spec 和 Plan，选择执行路径后，Runner 在独立工作区中完成开发、处理执行中的问答，并发布和核验 PR。执行进度与回执会持久保存，便于后续查询、恢复和停止。

项目可独立使用，也可通过 CLI 接入其他工具。日常使用由两个 skill 完成配置和执行，独立 Windows Viewer 提供最小进度视图。

## 可以做什么

- 在本机执行，或通过已配置的 Orca environment 在远端执行；同一服务可管理多个独立 Run。
- 按 Spec + Plan 完成开发，可选择增加一次代码审计与修复，再发布 PR。
- 查看进度、回复执行中的问题，以及显式恢复或停止原 Run。
- 通过只读 Viewer 查看 Run、当前 Workflow/步骤、Status 和 PR。

Runner 还提供持续会话与一次性内容生成流程，详见 [Workflow 指南](docs/workflows/README.md)。

## 使用前准备

当前交付目标为 **Windows/amd64**。运行 Runner 和 Viewer 无需安装 Go；skill 的共享 helper 使用系统自带 Windows PowerShell 5.1。

开发执行需要 Git、执行主机上的代理工具，以及访问目标仓库和发布 PR 的权限。本机受管 Codex 路径支持原生 **Codex >= 0.159.2**；更新版本的预发布版也可通过最低版本检查，例如 `0.163.0-alpha.10.2`，各版本仍须通过协议与权限核验。GitHub 发布使用执行主机已有的 `gh` 登录。远程执行另需安装 Orca，并预先配置 environment、远端项目、工具与登录。

Spec 和 Plan 应已提交到执行基线，并能由执行主机取得。Runner 使用固定 commit 创建工作区；提交执行前应确认材料与目标分支正确。

## 快速开始

### 1. 安装交付包

在解压后的完整交付包根目录，用 PowerShell 执行：

```powershell
.\scripts\install-client.ps1
```

默认安装到 `%LOCALAPPDATA%\Programs\SteerRunner`，并将以下两个 skill 安装到 `$CODEX_HOME\skills`（未设置时为 `%USERPROFILE%\.codex\skills`）。安装器会校验包内文件摘要；已有安装目录或同名 skill 时会停止并提示，不覆盖现有内容。

| Skill | 用途 |
| --- | --- |
| `steerrunner-configure` | 自动发现执行路径，保存默认选择及当前项目的仓库、发布配置 |
| `steerrunner-run` | 执行 Spec + Plan、查询进度、处理问答与恢复/停止，以及打开 Viewer |

### 2. 配置当前项目

在项目目录的代理会话中直接调用，无需参数或额外说明：

```text
$steerrunner-configure
```

配置 skill 会发现本机和已有 Orca 链路，只对无法确定的目标或配置缺项提问。未安装 Orca 时，会提示安装 Orca 以启用远程执行；发现结果不代表已经通过真实执行或发布验证。

### 3. 执行 Spec + Plan

在同一项目会话中指定材料，例如：

```text
$steerrunner-run 按 docs/spec/my-task.spec.md 和 docs/spec/my-task.plan.md 执行，任务名称为“实现我的功能”。
```

skill 使用已有配置和 Runner CLI 提交执行，并返回可追踪的 Run 标识。之后可以继续要求“查看这个 Run 的进度”“回复这个 Run 的问题”或“停止这个 Run”。执行中的问答沿原 Run 处理。

`accepted` 表示请求已受理，`pr_ready` 表示 PR 已核验；PR 就绪与执行资源停止是不同状态。遇到提交超时或回执丢失，应让 skill 查询原请求，保留原提交标识。

### 4. 查看进度

让 `steerrunner-run` 打开 Viewer，或直接运行：

```powershell
& "$env:LOCALAPPDATA\Programs\SteerRunner\steerrunner-viewer.exe"
```

Runner 每秒采样，变化时导出，无变化时每 5 秒发送心跳。Viewer 只读取 `%LOCALAPPDATA%\SteerRunnerView\snapshots` 中的 JSON 快照，每 2 秒刷新；从快照采样时间起超过 15 秒会标记过期并保留上次内容，反复读取旧文件不会续期。过期不表示任务已停止。Viewer EXE 可单独复制运行，关闭 Viewer 或聊天窗口不会停止 Run。

## 直接使用 CLI

不使用 skill 时，可以直接调用交付包中的 `steerrunner.exe`。以下命令在交付包根目录执行：

```powershell
.\steerrunner.exe version --json
.\steerrunner.exe capabilities --offline --json
.\steerrunner.exe service start --state-dir C:\SteerRunnerData --json
.\steerrunner.exe service status --state-dir C:\SteerRunnerData --json
```

提交 JSON、查询执行、环境检查、停服及升级说明见 [CLI 与运维指南](docs/cli-guide.md)，请求字段见 [CLI 契约](docs/spec/steerrunner.spec.md)。

## 从源码构建

需要 Windows/amd64、Git 和 Go 1.24.2 或更新版本。在仓库根目录执行：

```powershell
.\scripts\build.ps1
.\viewer\build.ps1
.\scripts\package-client.ps1 -OutDir .\dist\SteerRunner
```

Runner 和 Viewer 输出到 `bin`，完整交付包输出到指定的新目录。开发验证入口为 `.\scripts\test.ps1 -Level L1`；真实代理和远端测试需按[测试指南](docs/testing.md)配置。

## 更多文档

- [文档索引](docs/README.md)：全部使用说明、设计契约与验收入口。
- [CLI 与运维指南](docs/cli-guide.md)：详细命令、权限、恢复、状态目录和服务维护。
- [Workflow 指南](docs/workflows/README.md)：开发、带审计开发、持续会话和一次生成。
- [双 skill 与独立 Viewer 验收](docs/acceptance/skill-shell.md)：配置、文件契约、刷新/过期、本机与 Orca 闭环及已知限制。
- [测试与支持范围](docs/testing.md) · [验收记录](docs/acceptance/README.md)：按具体构建与执行路径查阅验证结果。

项目面向本人管理和信任的执行主机；工具安装、仓库权限和登录由使用者维护。具体后端与 Workflow 组合以当前构建的 `capabilities` 和对应验收记录为准。

## 许可

[Apache-2.0](LICENSE)。来源说明见 [NOTICE](NOTICE)，第三方许可见 [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt)。
