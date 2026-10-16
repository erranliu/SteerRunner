# L2 生产发布路径与 Git 根级锁回归

## 问题与修复范围

2026-10-16，任务 101 的 Run `e1c45e6a07af77e66e10d3b6a00d6c32` 在 publish
阶段被阻断。Git 2.46.0.windows.1 提交退出码为 0、产生 `d3d37b4`，但 stderr
同时报告共享 `.git/packed-refs.lock` 的 Permission denied；worker 遵循权限拒绝
后停止的规则，尚未 push 或创建 PR。

这不是残留锁。Git 的提交后清理会删除 `AUTO_MERGE`，files ref backend 为删除
操作锁定 packed refs，即使引用不存在。见 Git 2.46 的
[提交后清理](https://github.com/git/git/blob/v2.46.0/sequencer.c#L2938)
与 [refs 事务](https://github.com/git/git/blob/v2.46.0/refs/files-backend.c#L2773)。
原 native 同权限探针重现了四个 Git 子目录授权下的错误；仅额外授权两个文件路径
也不能允许创建该锁，授权 common 目录则提交无错误。

新本机 Run 从经过原 URL、base 分支刷新和固定 commit 验证的缓存创建独立 Git 副本，
禁用 hardlinks 并 dissociate alternates，保留刷新后的精确 tracking ref。
只有与原 Run ID/仓库来源绑定的私有 common 目录获得根级写权限；共享缓存及其它
Run 不在授权范围。原四个子目录和认证 transport 的绑定保持。
已有 Run 不迁移或扩大冻结权限，需要新 Run 使用修复。每个 Run 会多占一份仓库空间。

## L2 判据

- 使用生产默认实施、报告、发布提示词，不覆盖 `prompt_overrides`；夹具只装载任务、
  checker，不装载固定 publisher。
- 夹具显式 pack refs；原生无模型前置在相同 MXC/workspace-write/never 下执行
  loose/packed head 提交，要求零退出且 stderr 为空，并验证 peer 仓库写入仍被拒绝。
- 业务 smoke 要求真实 worker 提交/push/创建或复用 PR，核验 Git 内容和 `pr_ready`，
  无资格时保留一次真实反馈，driver 精确匹配 head 合入专属 base，最终停止原 Run/服务。
- 审计 `item/completed` 的实际命令输出，权限错误不能被退出码 0 或最终 PR 成功掩盖。
  提示词和模型正文不当作命令结果；缺实际命令证据不能通过。
- 确认 worker common 目录的 packed refs 和唯一 worker worktree；记录原生权限、
  命令结果、准备资格与认证撤销事实。L3 故障门及固定 publisher 保持。

## 本轮验证

验证记录将在本轮真实 L2 收尾后写入；原失败现场和历史资格不覆盖。
