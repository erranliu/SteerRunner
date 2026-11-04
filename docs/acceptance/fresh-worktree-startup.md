# 新建 worktree 启动边界验收

2026-11-04，验证来自主干 `1df8e26d` 加本次 Runner 源码差异的 `0.2.0-dev.fresh-start`。EXE SHA-256 为 `02a0b68ad6f77a767011db02a1722aec3d28811c9c684d909c737ac585a902cb`。原始证据位于 `C:\SteerlineDebug\fresh-worktree-startup-20261104`。

- `runner-targeted.log` 验证目标分支改写后仍从指定对象创建，固定对象无法取得时拒绝。显式槽位和空数组按调用方选择工作，旧 Run 对账保持原检查。
- 本机真实 Git 夹具检查禁用 checkout hook、创建返回 revision 事实、绑定写权限路径时无额外 HEAD、diff、status 或祖先查询。
- 公开 CLI 的任务 101 联验中，受控 Agent 从新工作区读取安排。注入目录创建失败后仍派发，由受控 Agent 记录读取失败。真实本机 Codex Run `2e3c83b4e2bb2224e6baa30a431a94c2` 从固定项目提交读得 111 字节的相同摘要，原 Run 和服务正式结算。首轮真实读取通过，停止回执未知，隔离进程已确认退出。见跨仓验收及 `test-results/native-101-retry`。
- 定向回归和本机只读联验的通过范围限于上述入口。本机及远端 Orca 的真实 smoke 和原生审计交付待验。

## 提示词

原结果文件说明为 `STEERLINE_RESULT_FILE: independent raw<=65536B; coordinate writers, same-dir atomic replace. Sealed once after execution/producers stop, never turn end/pr_ready; not a success signal.`，183 UTF-8 字节。

现为 `STEERLINE_RESULT_FILE: raw<=65536B. When asked to write, coordinate writers and use same-dir atomic replace. Sealed once after execution/producers stop; not a success signal.`，174 UTF-8 字节。

明确写入条件，使同一通道也可供调用任务只读使用。大小上限、原子替换和执行停止后的单次封存保持。独立审查已核对正文、字节数和接线。
