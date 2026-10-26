# SteerRunner

SteerRunner 是面向个人本机与多机开发的 AI 执行器。
项目可独立使用，也可通过 CLI 接入其他工具。日常使用由两个 skill 完成配置和执行，独立 Windows Viewer 提供最小进度视图。

## 可以做什么

- 如果你现在的开发方式，本机的资源占用已经影响你开更多并行，或者并行时机器卡顿，SteerRunner就是你需要的。
- 将已经定义好的Spec+Plan，放在远端机器上运行。这样你可以开无限多的并行，而不用占用本机资源。
- 你有多个订阅账号，可以每台电脑挂一个，把不同任务派过去执行。
- 只需要在本机使用SteerRunner服务，就可以调动无限多台远程机器，
- 支持不同的Workflow，可以在执行过程中定制化审计或者其他流程。
- 通过只读 Viewer 查看 Run、当前 Workflow/步骤、Status 和 PR。
- 简单说SteerRunner封装了多台机器的Agent执行能力，让你在本机上即可调用，并收到执行结果(而不是在不同机器操作)。

## 开发方式

- 每个任务 1 Worktree 1 Spec(+plan)  1 PR，我们称之为1+1+1开发方式（更适合要运行1小时至十几小时，或者更长的任务）
- 当你有能力拆出很多可以并行的长任务，形成Spec+Plan后，特别是Plan里包含了很多测试和验证，在本机开多个Session执行，很容易吃掉电脑上所有资源。
- 一般情况下，带测试和验证的任务，5个并行就能让16核32G内存的电脑明显卡顿。
- 有了SteerRunner，你可以在多台电脑上开启Orca(另一个开源项目https://www.onorca.dev/) ，就能跑任意多个并行长任务，跑完后得到一个PR，你自己选择要不要合并。
- 要想充分发挥SteerRunner的能力，你最好先习惯将更多的开发需求，转化成长任务（获得并行AI开发能力）。

## SteerLine和SteerRunner

- 怎么高效的产生长任务Spec+Plan呢？任务该怎么自带测试和验证呢？大量的测试和验证又要怎么管理呢？这个是我们Steerline项目要解决的问题。(https://github.com/erranliu/SteerLine)
- SteerRunner最初是SteerLine项目的执行单元，做了个薄壳交付，这样你即使不愿接入SteerLine完整的开发流程，只要有开更多并行的需求就能用上。
- 你当然可以把他接入你自己的系统，或者更简单，直接在习惯的Agent里，作为Skill调用。

## 当前版本的限制

- SteerRunner有自己的执行链路，但目前远端执行，只支持通过Orca(https://www.onorca.dev/) ，你想获得远端执行能力，这两个得一起装。
- 目前支持的Agent是Codex, Pi，Deepseek Harness，这个扩展很容易，有需求的话给我发私信，很快会扩展更多Agent支持(这个是Orca提供的全Agent适配能力，这边只是个薄壳)
- 目前没有资源自动清理功能，也就是每次运行会留下一个worktree，你要自己清理，推荐在Orca里开个Agent来批量清理。

## 跟使用Orca有什么区别？

- Orca本身就支持把远端机器上的项目映射到本机去操作，但还是对话式的，虽然有通知，但为任务创建worktree，找到Spec下执行指令，提PR，并不算方便。特别是很难管理远端worktree的基线。
- SteerRunner是傻瓜式的，你只需要告诉他Spec/Plan文档位置，其他就不用管了，最后收到一个PR，你选择要不要合入就行。Orca界面太复杂了，而SteerRunner没有界面。

## 跟使用Codex/Claude有什么区别？

- 获得傻瓜式的远端执行能力。本地执行也比直接在Agent里自己操作要方便。
- 当你并行需求日益强烈时，你自然会有远端执行的需求，
-

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

构建会同时生成 EXE 的 `.build.json` 来源回执；打包须使用带新格式回执的 Runner 与 Viewer，生成 version=2 包清单。旧完整包继续使用随包安装器，新校验入口只接收新格式包。交付包冻结后，在包外运行受控交付验证，再核对同一包与对应报告：

```powershell
.\scripts\test-delivery.ps1 -PackageDir .\dist\SteerRunner -OutDir .\.evidence\delivery-check
.\scripts\verify-client-package.ps1 -PackageDir .\dist\SteerRunner -Qualification .\.evidence\delivery-check\qualification.json
```

该入口使用包内 Runner、客户端脚本与 Viewer，检查隔离安装、客户端、原生 Viewer 和受控进度字节通道。它不调用真实模型或创建 PR；真实后端、调用方测试语义与页面资格仍按测试指南独立验证。测试后改变包内容须重新验证。

## 更多文档

- [运行时单文件汇报通道](docs/runtime-progress.md)：可选原始字节、独立快照版本、执行环境路径及本机/Orca 远端采集；[Spec](docs/spec/runtime-progress.spec.md) / [Plan](docs/spec/runtime-progress.plan.md) / [验收](docs/acceptance/runtime-progress.md)。
- [result-file 最终文件快照](docs/runtime-result-file.md)：仅结束时封存一次，运行中 pending，重启及 resume 不刷新；[Spec](docs/spec/runtime-result-file.spec.md) / [Plan](docs/spec/runtime-result-file.plan.md) / [本地验收](docs/acceptance/runtime-result-file.md)。
- [文档索引](docs/README.md)：全部使用说明、设计契约与验收入口。
- [CLI 与运维指南](docs/cli-guide.md)：详细命令、权限、恢复、状态目录和服务维护。
- [Workflow 指南](docs/workflows/README.md)：开发、带审计开发、持续会话和一次生成。
- [双 skill 与独立 Viewer 验收](docs/acceptance/skill-shell.md)：配置、文件契约、刷新/过期、本机与 Orca 闭环及已知限制。
- [测试与支持范围](docs/testing.md) · [验收记录](docs/acceptance/README.md)：按具体构建与执行路径查阅验证结果。

项目面向本人管理和信任的执行主机；工具安装、仓库权限和登录由使用者维护。具体后端与 Workflow 组合以当前构建的 `capabilities` 和对应验收记录为准。

## 许可

[Apache-2.0](LICENSE)。来源说明见 [NOTICE](NOTICE)，第三方许可见 [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt)。
