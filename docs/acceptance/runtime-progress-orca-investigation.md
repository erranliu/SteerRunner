# 运行时汇报：Orca 可行性调查

2026-10-23；历史调查结果，不是功能/L3 通过记录。用户随后明确收敛为固定原主机/worktree/文件的只读采集，不以防恶意链接替换为前置；下文严格抗替换接口讨论不再阻断本次实现，当前契约见 ../spec/runtime-progress.spec.md。用户补充：Orca 路径是必需范围，先解释并核实所谓缺口。

实施与各轮实际结果更新到[专项验收](runtime-progress.md)：远端公开 EXE/native 文件通道 L3 真机通过，已有文件 RPC 足够。历史 r2 本机失败是工作区初始终端创建回执缺失，发生在文件绑定之前，与下文抗链接接口讨论不同；后续回执修复及当前构建资格以专项验收为准。

## 已核实

- 使用 `C:/Users/lmas1/AppData/Local/Programs/orca/resources/bin/orca.exe` 的版本匹配 `skills get orca-cli --json` 和 CLI help，检查已安装包 `resources/app.asar` 及 unpacked RPC 参数源码；只读检查，未改 Orca 安装或保留业务资源。
- `host list --json` 显示既有配对环境“台式机64G”。对 environment ID `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8` 的 `status --json` 实际成功：runtime `f8616747-34af-4543-8487-ec6b5d8de4ff`、远端版本 `1.4.223`、ready/connected。不能据本机包源码推断远端每个 RPC 具有相同实现；尚未创建新远端探针现场。
- 仓库 protocol/orca/artifact.go 已用原 runtime + worktree selector 经加密/IPC 通道调用 files.readChunk，传输 contentBase64/bytesRead/eof。它目前仅限停止回执路径，需要新增通用固定汇报文件读入口，不能直接拿已有函数的路径白名单冒充可用。
- 已安装 Orca 的 files.writeBase64 和首次 writeBase64Chunk 使用 `writeFile(...,{flag:'wx'})`，已有独占创建能力；files.createDirNoClobber 也存在。因此“缺少独占初始化接口”不是成立的已知结论。恢复须先读取原文件并对账；已初始化后文件丢失不能再次调用初始化。
- agentlaunch/config.go 已把有限 PowerShell 启动前置逻辑编译到同一个原终端；linked_git.go 已在执行主机做原工作区准备，github_auth.go 已配置原生工具环境白名单。设置 STEERLINE_PROGRESS_FILE 可以沿此链路实现，不要求新常驻 helper。Orca local 既有 helper/TerminalRuntime 也需接入同一冻结配置，不能只设置本机 Runner 环境。

## 方案与实际缺口

主链方案：原 worktree 创建/绑定 → 通过原 Orca 文件 RPC 独占初始化固定相对文件 → 原启动命令在执行主机从实际 cwd 构造绝对路径并导出变量 → 独立采集器只向原 environment/runtime/worktree 请求该文件的有界原始字节 → 本机持久化独立快照版本。观察不依赖终端画面、最终回复或业务结果核验；重启只接原绑定。无需远端常驻 Runner。

初始化和注入属于 Runner 接线工作，不能作为不可实现理由。真正尚未满足的是原始要求中的严格文件边界：

- 已安装实现 files.readChunk 的入口解析 worktree 相对路径后，调用 rJ 做 Orca **全体获准文件根**校验及 realpath；Soa 再用 ZW 打开普通文件并返回原始字节。其普通文件打开没有该汇报专用的目录身份参数或 no-follow 约束。
- 因而仅用普通 readChunk，指向其它获准项目/文件的链接可能仍符合 Orca 通用文件权限，但违反“只读本 Run 固定文件”。普通 stat 仅返回 size/isDirectory/mtime/ctime，不返回可持久核验的目录/file ID 或 isSymlink。
- browseServerDir/readDir 可用于静态链接预检，但先检查再读取之间仍有替换窗口；读取后才发现问题也不能声称此前没有读取其它文件。仅轮询/重复摘要不能补足这项安全保证。
- terminal artifact 的授权读接口偏文本并绑定 stat identity，不能直接代替需要任意字节且允许原子替换叶文件的本通道。document preview/visual 接口也不是本契约。

完全满足严格边界的候选补充接口：执行主机在一次有界操作内，核验 expected runtime/host、原 worktree 与持久目录身份，拒绝各路径组件链接/父目录替换，打开固定普通叶文件并复核身份，最多返回 65537 字节和必要元数据。允许叶文件替换，拒绝目录绑定替换。可以是既有 files.readChunk 的可协商 safe-bound 选项，不涉及业务结果或常驻 Runner。远端 1.4.223 是否已有等价能力尚须独立真实探针确认，不能把本机源码缺参数推导成远端已实测失败。

如果等价原生能力确实不存在，仅改 Runner 可以完成普通安全预检与真实远端读写链路，但不能宣称同时满足严格的抗链接竞态/目录替换要求。当前先验证这条具体边界，不提前把整个 Orca 组合定为最终 unsupported，也不以受控测试冒充远端资格。
