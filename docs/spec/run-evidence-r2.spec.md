# 原 Run 证据与当前 Git：R2 最小 Spec

日期：2026-10-19；基线：9586ac4。只扩展开发 Run 的通用证据读取及当前 Git 查询，不改内容流程、测试语义、发布或生命周期。

1. 沿 steerrunner-cli/v1、原 state-dir/instance/run 围栏。`artifacts discover/list/read`、`workspace-git` 是 resident 查询，不启动服务、Agent 或恢复 Run。`artifacts collect` 使用原写信封、request_id 去重和 expected_revision CAS，只收集证据，不发送业务输入。
2. 项目测试程序将完整文件写到原 Run 工作区 `.steerrunner/artifacts/<调用方自定相对路径>`。先写临时文件再 rename，完成后再请求发现/收集；临时文件也属于可发现文件，Runner 不推断完成/通过。目录不存在明确失败。discover 有界递归发现普通文件；collect 一次收集一个相对路径。无 final 文本替代、无日志语义解析。
3. 复用原 Run Record、SQLite Commit、命令与事件：增加不可变字节记录及公开元数据，不另建写者或产物状态机。每次新 collect request 分配新 artifact_id；同 request 重放返回原身份。元数据含 run_id、artifact_id、relative_path、sha256、size_bytes、collected_at、读取命令。读取返回 base64 原字节并复核摘要。无修改/删除产物接口；原工作区清理和 Run stop 不删除已经收集的记录。删除整个 state-dir 不在保留承诺内。
4. 单文件上限 1 MiB；每 Run 最多 64 条/总计 16 MiB；发现最多 256 条目录项，超限整体失败。list 最多 64 条，无私有字段猜测或无限响应。路径仅接受目录内 POSIX 相对路径，拒绝绝对路径、..、反斜杠、ADS、链接及特殊文件。使用受限 os.Root 文件句柄防止目录逃逸。读取来源只能是该 Run 已收集记录；未知 ID/跨 Run ID 明确失败。
5. 原开发工作区必须具有一致的 Ref.RunID、Ref.WorkspacePath、WorkspacePlan、WorkspaceFact、Git common/branch/base 身份。host-local codex-app-server 和 orca-terminal local 工作区可观察；Orca environment 明确 unsupported，不退回本机路径。来源缺失/身份不符、工作区不可用、缺文件、超限、文件读取中变化均明确失败。
6. workspace-git 输出 run_id、commit、clean、observed_at、verified、diagnostic。失败为 verified=false、commit=null、clean=null。以原工作区实际 Git status porcelain v2 --branch --untracked-files=all 的 branch.oid 和状态条目同次采样；身份核验前后双样本必须一致，否则不能给出已核实 clean。仅比较观察窗口内样本，不增加脏文件指纹、快照或监控；不声称检测两次采样间发生又完全恢复的瞬态变化。
7. capabilities 内嵌支持范围、限制、schema、示例；CLI 帮助与调用文档同步。错误沿既有信封/退出码；Git 的观察失败是成功返回一份 verified=false 的观察，业务成功由调用方判断。

验收：同 Run 身份/摘要/原字节稳定；新报告追加/跨 Run 隔离；缺失/越界/超限/来源未知拒绝；Git 干净/修改/未跟踪/新 commit/工作区缺失/观察变化；零 Agent/resume/重跑；stop 和工作区删除后证据仍可读。
