# 本机认证 socket Windows 1920 根因与最小修复方案

2026-10-30。源码基线 `e854378`。承接[原始流补验](orca-native-submit-local-stream.md)。本轮不修改生产代码、认证、全局配置或安装环境；目标仍为三个全新本机原生业务会话。

最新状态：环境恢复后，三个正式本机原生会话已全部通过；详见[最终补验](orca-native-submit-local-final.md)。下文审批前方案及前批失败保持其原批次边界。

## 结论与凭据

本次已定位为进程启动上下文相关的 AppData/UDS 差异：当前工具子进程和实际 Orca 子进程均观察到 Codex 包 AppData 重定向，独立正常 Windows 用户进程在同一机器、同一目录及同一诊断 EXE 上操作正常。证据与继承的打包桌面应用上下文一致；原 SteerRunner/a 的普通标记仍指向原目录，尚未单独证明 Windows 内部哪一层机制使原 socket 路径失效，不把这层推断当作内核归因。

证据分别来自真实文件句柄的最终路径、实际 UDS 回环和元数据/清理操作，不仅是错误字符串或目录猜测。`GetPackageFullName` 在两种上下文均返回 15700（无名义包身份），因此不能用这一结果排除继承的重定向环境。

| 上下文 / 目录 | socket 创建 | Root.Lstat / os.Lstat | 实际连接回环 | 清理 |
| --- | --- | --- | --- | --- |
| 当前工具进程 / TEMP，3 个独立进程 | 成功 | 全部成功 | 全部成功 | 全部完成 |
| 当前工具进程 / 原 SteerRunner/a，3 个独立进程 | 成功 | 全部 Windows 1920 | 全部 Winsock 10022 | 原地失败，自有目录移至 TEMP 后完成 |
| 独立正常用户进程 / 相同原 SteerRunner/a，3 个独立进程 | 成功 | 全部成功 | 全部成功 | 全部原地完成 |
| 实际 Orca 子进程 / 原 SteerRunner/a | 成功 | Windows 1920 | Winsock 10022 | 自有目录移至 TEMP 后完成 |

固定版本诊断 EXE 的 3 对对照均新建随机目录和真实 socket，不重放快照；没有模型或认证凭据。独立正常进程使用系统 WMI 创建、隐藏窗口，没有请求提权或改变 AppData/TEMP 环境变量；令牌对照另行记录，不仅凭用户名假设权限等同。WMI 仅用于诊断基线，不进入 Runner 实现或模型执行流程。

令牌只读对照确认两侧用户 SID 相同、完整性级别均为 Medium（S-1-16-8192）、均未提权，elevation type、restricted SID 数、restrictions 和 UAC virtualization 指标相同。该记录排除这些权限级别差异，不宣称两个令牌的全部内部状态相同。

另一个判别证据：当前工具与实际 Orca 子进程在 `C:/Users/lmas1/AppData/Local` 下新建的标记文件，经 `GetFinalPathNameByHandleW` 显示实际落到 `.../Packages/OpenAI.Codex_2p2nqsd0c76g0/LocalCache/Local/...`；独立正常用户进程同类标记的最终路径仍在原 `AppData/Local`。TEMP 在两侧均直接落到 TEMP。

Microsoft 文档说明打包桌面应用的 AppData 文件操作可被重定向到每用户、每包的位置；文件句柄最终路径 API 用于取得实际打开的路径。[AppData 重定向](https://learn.microsoft.com/en-us/windows/msix/desktop/desktop-to-uwp-behind-the-scenes)、[最终路径 API](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfinalpathnamebyhandlew)。这些文档解释机制，实际本机结论以上述原始对照为依据。

## 对 Runner 的判断

原认证实现 `net.Listen("unix", ...)` 后用 `Root.Lstat` 固定 socket 身份，失败时阻断，是必要保证。相同上下文中 socket 的真实连接也失败，因此删除/放宽 Lstat 校验不能修复认证。

独立正常上下文已证明原目录、Go 1.26.4 及同一 socket API 可用；没有证据要求改变认证协议、权限、凭据、完成算法或 Orca 提交 API。当前问题应先按本机应用启动环境恢复处理。此前“目录条件相关”的结论在本轮细化为上述可重复的进程上下文差异，不把历史失败改写为通过。

直接 `Shell.Application.ShellExecute` 在本次试验中仍出现同样差异，不能将它当成已验证的正常启动器。不存在“GetPackageFullName 无包身份就一定正常”的假设。

## 最小修复方案

1. 保持当前 Runner agent 二进制、认证实现和默认运行目录。先正常退出当前本机 Orca runtime，再从独立的普通 Windows 用户启动上下文打开**同一已安装 Orca**；不部署、不安装、不迁移或复制 MSIX 私有配置，不新增 Runner launcher、runtime 或本机分支。这是待实际 Orca 验证的环境恢复步骤，不承诺仅重启就一定成功。
2. 在重新启动的实际 Orca 子进程中先运行本次无凭据原目录探针。必须达到原目录 socket 创建、身份核验、连接回环及原地清理全部成功；AppData 标记最终路径不再落到 Codex 包私有目录。仅进程启动成功或新 runtime ID 不够。
3. 固定新 runtime/目标和现有受测构建，使用原正式入口创建三个全新独立本机会话，逐一保存原请求/受理/开始/完成、冻结输入/正式原文/final 及正式停止认证证明；超时或未知不重投、不补 Enter。
4. 复审并更新原 PR；保留所有前批失败。若正常启动看到的工作区配置与原视图不同，先记录差异，不复制全局配置或套用历史身份。若新阶段失败，保留真实原因，不绕过。

审批前只读全机清单在自有诊断终端清理后仍有 **1 个非本次所有的已连接本机终端**，清单完整、原 local host 已覆盖。重启 Orca 可能中断该终端；当时未经用户确认，没有关闭它或重启现有应用，该批本机资格为 **0/3**。随后的明确授权和恢复结果如下。

## 授权后的恢复实证

用户随后明确允许重启及中断本机终端。用户自行重启后 runtime 已换代，但实际子进程仍失败；从正常用户上下文仅重启 GUI 后亦相同。父链实证显示新终端仍由旧 PTY host PID 8832 派生，该 host 的父 PID 仍为已退出的原 GUI 26940。不能将“GUI/runtime ID 变化”视为执行上下文已经恢复。

在同一授权范围内正常退出 GUI，核实该遗留 host 的精确 PID、父 PID 及安装路径后停止它，再从独立普通用户上下文打开同一 Orca 1.4.242。安装 EXE 摘要不变，没有安装、修改配置或运行目录。新 GUI PID 5968、PTY host PID 30820，父链已绑定新 GUI；runtime 为 `92c410b8-3132-4c48-a07f-2888faa81c53`。

此时实际 Orca 子进程的原 `SteerRunner/a` 目录 socket 创建、Root.Lstat/os.Lstat、真实回环及原地清理全部通过；AppData 新标记最终路径保持原目录，不再进入 Codex 包 LocalCache。因此本机环境恢复已获得直接前后对照，Runner 认证实现不需要修改。该恢复结果不等于三个模型会话已通过；正式结果另列。

探针的 `orca_package` 查询固定原 GUI PID 26940，仅对重启前有意义；重启后不将该字段作为凭据。恢复判断使用公共 CLI 的原 runtime/terminal 创建记录、实际子进程父链和文件句柄最终路径。

## 清理与交付

本次自有诊断主机/进程探针均无凭据；新建的 Orca 诊断/default shell 经身份和空闲提示符核验后精确关闭，独立工作区以完整空清单及 Git 干净为前提精确删除。探针仅处理自身随机目录和已知文件，不递归删除或改变历史状态。

最终从当前与独立正常进程两个视图核验 39 条本次原路径、实际包私有路径及临时清理目标，全部不存在。该跨视图核对防止“当前重定向视图为空”掩盖另一视图中的残留。

证据归档目录为 [evidence/orca-auth-socket-rootcause-20261030](evidence/orca-auth-socket-rootcause-20261030/manifest.json)。这些是根因对照，不是三个模型会话验证；此前 L1、4860 次回归结果沿其原源码批次有效，本轮没有生产变更，不重新包装为新实施通过。
