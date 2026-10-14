# SteerRunner 来源与交接清单

日期：2026-10-14。本文件登记实际来源和迁移边界；行为见 [Spec](../spec/steerrunner.spec.md)，步骤见 [Plan](../spec/steerrunner.plan.md)，实时验证见[验收](../acceptance/steerrunner.md)。

## 固定基线

| 对象 | 实际身份 | 本次处理 |
| --- | --- | --- |
| Runner 仓库 | `https://github.com/erranliu/SteerRunner.git`；module `github.com/erranliu/SteerRunner` | 新建 Go 项目，无 SteerLine Go 依赖 |
| 最初交付输入 | commit `60c1b0900b25eced5cb9e4ffb9c636995bc8a69e`：README、`docs/spec/steerrunner.spec.md`、`docs/spec/steerrunner.plan.md`、`docs/acceptance/steerrunner.md` | 实施前提交固定；当前修改在 `codex/steerrunner-foundation`，后续提交补充启动实现和配置细节 |
| SteerLine 主工作区 | `C:/Project/SteerLine`，实施时 HEAD `1fdb7265a6043646b383e815cfe5dbf54ca302e3`，分支 `codex/web-frontend-full-regression` | 他人在途修改，只读，不在此抽取旧 Service |
| SteerLine 主干 | 启动时 `origin/main` = `ab814da9d5fcec17f3ba46b59827734d171ba6b7`；2026-10-14 最终 fetch = `2f7312b485cfeebf9a5cfdea127ce0041eef6094` | 两次版本均无 `internal/execution/workflow` / `reliable`；不满足 P2 交接 |
| 三层工作区 | `C:/Users/lmas1/.codex/worktrees/0998/SteerLine`，HEAD `44f54055abe54fa08944afb6dfb31393e39a4a56`；核心提取 `4a6034f0ff8d8d9fb2ce1a56ca9dd58af4228255` | 后有修复且工作树有在途验收/夹具变更，仅作调查；最终抽取基线待主干交接，不修改或代为合并此工作区 |

## P0/P1 已建设来源

| Runner 范围 | 来源 / 责任 |
| --- | --- |
| `cmd/steerrunner`、`internal/cli`、`contracts` | 依据 Runner 契约新建；公开字段与错误分类不沿用 SteerLine 内部模型 |
| `internal/host` | 参考 SteerLine `internal/app/host` 与 `internal/execution/process` 的纯平台所有权思路，新建规范化目录、两个文件锁、服务发现、用户 ACL 命名管道和 Windows 独立启动；未复制产品 actor/facade |
| `internal/storage` | 新建 Runner 专属 SQLite 格式，仅服务打开；当前只存服务命令回执，不含 Task 或伪造 Run |
| `internal/resources` | 新建并内嵌 development/1 定义、四份提示词、CLI/settings schema；流程及后端配置声明不等于已接通执行 |
| `tests/integration`、`scripts/test-foundation.ps1` | 新建真实 EXE/进程夹具；Job 允许/拒绝、父进程退出、并发别名、独立目录、查询不启动、陈旧进程、停服回执重查；不导入 SteerLine testkit |

Windows Job 继承无法仅凭后台进程组保证。[Microsoft Job 文档](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)描述 breakaway 及 `Win32_Process.Create` 不继承调用者 Job；[Win32_ProcessStartup](https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-processstartup)提供挂起启动参数。实现优先原生挂起创建；明确被禁止则失败；仍有关联时经系统 CIM 创建，核验原用户、完整性级别、授权组/权限及可执行文件后恢复。启动意图先落盘，未知不盲重建。手工 `serve` 如实返回自身 `detached` 事实。

`go-winio v0.6.2` 实现命名管道；该固定版本创建管道时拒绝远端客户端，Runner 另外限制当前用户 ACL。`modernc.org/sqlite v1.38.2` 提供纯 Go SQLite，`golang.org/x/sys v0.38.0` 提供 Windows API。未集成 workflow 引擎或额外代理运行时。依赖固定在 go.mod/go.sum；完整运行依赖声明见根目录 THIRD_PARTY_NOTICES.txt。

## P2–P7 待交接清单

| 来源 | 目标 / 保留边界 | 删除门槛 |
| --- | --- | --- |
| 最终三层 `workflow` / `reliable` / `backend` | Runner 的执行规则与窄原生契约；移除 Task/Service 依赖 | 主干源码及必要修复固定后开始搬迁，纯规则和依赖守卫通过 |
| Codex / Orca adapter、workspace / publication / process owner | Runner 的原生协议、工作区和 PR 查询；保留原回执/身份/取证 | 对应 controlled 及承诺的最小 live 验证通过 |
| Task、DAG、claim、候选关系、TaskDelivery | 留在 SteerLine；候选仅绑定 Runner Run 和冻结目标 | 不因所在包名 execution 而整目录删除 |
| app 接线、原 execution actor/worker、执行表/写端口 | Runner 替代执行写权；SteerLine 新 CLI client | 独立 Runner 可用且 SteerLine CLI 桥接/交付事务、回归通过，在切换 PR 一起接通删码 |
| query / API / TUI / Web | 留在 SteerLine，读取已提交的 Runner 投影 | 在途前端基于切换时主干适配，不重建另一套产品模型 |
| debug-template / 构建/安装/测试 driver | 固定 Runner EXE 版本及摘要，项目运行目录下 runner/ | 使用独立 debug 生成器复验，不读取或修改安装现场 |

当前没有迁出这些执行包，也没有删除 SteerLine 代码。没有进行双写、旧数据迁移或旧执行回退；没有修改远端仓库业务状态、创建 PR 或发布版本。P2 前重新 fetch、确认最终主干包含三层及修复、记录实际抽取 commit/path/测试，然后进入提取。

P0/P1 实现固定为 `7cfad777b0e41e19000cc6fda6cc21973109e0ad`，来自干净工作树的实际 EXE/单测/进程证据见[基础层验收记录](../acceptance/evidence/steerrunner-foundation-20261014.md)。后续证据登记提交不改变该已验实现。
