# 新建 worktree 启动边界验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-04，验证来自主干 `1df8e26d` 加本次 Runner 源码差异的 `0.2.0-dev.fresh-start`。EXE SHA-256 为 `02a0b68ad6f77a767011db02a1722aec3d28811c9c684d909c737ac585a902cb`。原始证据位于 `C:\SteerlineDebug\fresh-worktree-startup-20261104`。

- `runner-targeted.log` 验证目标分支改写后仍从指定对象创建，固定对象无法取得时拒绝。显式槽位和空数组按调用方选择工作，旧 Run 对账保持原检查。
- 本机真实 Git 夹具检查禁用 checkout hook、创建返回 revision 事实、绑定写权限路径时无额外 HEAD、diff、status 或祖先查询。
- 公开 CLI 的任务 101 联验中，受控 Agent 从新工作区读取安排。注入目录创建失败后仍派发，由受控 Agent 记录读取失败。真实本机 Codex Run `2e3c83b4e2bb2224e6baa30a431a94c2` 从固定项目提交读得 111 字节的相同摘要，原 Run 和服务正式结算。首轮真实读取通过，停止回执未知，隔离进程已确认退出。见跨仓验收及 `test-results/native-101-retry`。
- 定向回归和本机只读联验的通过范围限于上述入口。本机及远端 Orca 的真实 smoke 和原生审计交付待验。

## 合入审计修订

首次文件准备分别为 progress 和 result 提供 4 秒。进度超时后仍可初始化安排，两个通道均沿原尝试和诊断语义。免 Git 启动检查的授权只存在于本次服务内实际完成新建的 worker。持久回执恢复后，首次派发前核对原工作区 HEAD 与工作树。Orca Pi 的新建启动仅解析 Git 权限路径。

定向回归通过，覆盖慢 progress 与独立安排初始化、原工作区已有改动时重启阻断，以及 Pi 使用的权限路径不执行 HEAD 探针。Orca transport 与 workspace 完整包回归通过。修复构建 `0.2.0-dev.fresh-audit` 的 EXE SHA-256 为 `a41b28a5434b87760e34da18dbb4180c9211777216b1cb2d50975acb7665e89b`。任务 101 公开 CLI 正反例和真实 Codex 只读复验通过，原 Run `da64a3da77c9de7d440a924b8271a4b1` 已结算。报告根为 `C:\SteerlineDebug\fresh-worktree-audit-20261104`，真实 Orca Pi smoke 待验。

## 启动路径去重

基于合入后的 Runner 主干 `689f4190`，按以下顺序分别验证。报告根为 `C:\SteerlineDebug\startup-verification-simplification-20261104`。

1. 本机 Codex 在本次连续新建并已绑定权限资源时，首次输入复用冻结的 Git 权限路径。恢复和未绑定仍走原检查。路径复用、真实 Git 重启反例、私有与共享资源边界、原生管理目录绑定和临时目录重定向回归通过，见 `02-linked-roots-verified.log`。
2. result 初始化返回成功后省去回读。写入响应丢失时仍读取原绑定路径，读取不到时保留诊断，派发继续。回归验证零成功回读、未知时读回及缺文件、独立进度采样、停止和终态封存，见 `01-result-file.log`。

两项配对构建 `0.2.0-dev.startup-verification` 通过任务 101 复验。原固定提交 `9d81d9c2652177faf6354d4e9f9f3aec79535d11` 的就绪快照、启动安排和真实本机 Codex 读取的五项安排均为 111 字节，SHA-256 为 `b9a819296f6f3c6b0e9a3ff9ce4059f89a111a9a7633ed73baa373d0475f6cc4`。真实 Run `ea4c573b1e41f14ed090254ea88de7dd` 和隔离服务已结算。正式公开 CLI 的可读、缺文件和普通结果封存用例通过。证据见 `ready-101`、`public-101` 和 `native-101`。Runner EXE SHA-256 为 `2d98a1f6ed5d7f953aafbaf3b63ba95769038161998a420b9a51b6d6cf7eee6c`，构建记录位于 `packages`。

## 提示词

原结果文件说明为 `STEERLINE_RESULT_FILE: independent raw<=65536B; coordinate writers, same-dir atomic replace. Sealed once after execution/producers stop, never turn end/pr_ready; not a success signal.`，183 UTF-8 字节。

现为 `STEERLINE_RESULT_FILE: raw<=65536B. When asked to write, coordinate writers and use same-dir atomic replace. Sealed once after execution/producers stop; not a success signal.`，174 UTF-8 字节。

明确写入条件，使同一通道也可供调用任务只读使用。大小上限、原子替换和执行停止后的单次封存保持。独立审查已核对正文、字节数和接线。
