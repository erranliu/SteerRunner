# 固定基线分支刷新验收

日期：2026-10-15。仅归属本次「修复 Runner 接受前缓存分支过期」；[Plan](../spec/repository-base-refresh.plan.md) 给出测试命令和完整映射。

## 问题与修复范围

安装中的 SteerlineTest 101：固定基线为 `cebdd5ab552150c49743e40593c836689ecf789b`，调用方已获取的 origin/main 和远端 main 均指向它；调用方本地 main、Runner 缓存 origin/main 仍在 `849ed9b33de86920e4b582bd7513628421bbfa99`。缓存已有固定 commit 对象，但据旧引用做祖先检查导致 invalid_input。此事实来自只读诊断；原 Runner 请求未接受，不把它记作模型权限拒绝。

修复在私有缓存锁内、远端身份检查后显式刷新配置的目标分支，再检查原固定基线。缓存和目标引用可更新，冻结基线与文档不能跟随新头漂移；刷新失败不使用旧引用。没有更改协议、数据库、任务状态、已接受 Run 或恢复授权。

## 自动验收

| ID | 场景 | 结果 |
| --- | --- | --- |
| BR-A01 | 本地主分支落后于已获取远端目标 | 同因红测已复现；修复后通过，调用方分支与冻结基线/文档保持 |
| BR-A02 | 已有缓存、自定义远端/分支、远端前进 | 通过：同一缓存刷新自定义目标，原基线及输入摘要不变 |
| BR-A03 | 可读取但不属于目标分支的提交 | 通过：invalid_input，保持原祖先检查及原请求 |
| BR-A04 | 远端强制回退排除固定基线 | 通过：invalid_input，不接受已被目标排除的固定提交 |
| BR-A05 | 删除远端分支，缓存仍保留旧引用 | 通过：environment_unavailable，返回 nil 记录，无旧引用回退 |
| BR-A06 | 远端不可用，缓存仍保留旧引用 | 通过：environment_unavailable，返回 nil 记录，无旧引用回退 |
| BR-A07 | 既有长分支路径和深缓存发布回归 | 两项均通过；真实隔离 Git linked-worktree push |

修复前定向命令：`go test ./internal/application -run '^TestSRFreezeBaseBranchRefresh$/^local-branch-behind$' -count=1 -timeout=2m -v`。实际错误为 `Pinned base is not in the configured base branch.`，子场景 3.92s、包 6.276s，exit 1；证据 `.evidence/base-refresh/repro-before.log`。

修复后 BR-C1 命令见 Plan，原始输出 `.evidence/base-refresh/after.log`：3 个顶层测试、6 个子场景全部 PASS、零 SKIP，包耗时 43.779s、exit 0。原始输出只留本机；仓库保存命令、计数、范围和最小结果摘要。

最终补强冻结材料数量断言后，执行 `go test ./internal/application -run '^TestSRFreezeBaseBranchRefresh$' -count=1 -timeout=3m -v`：1 个顶层测试、6 个子场景全部 PASS、零 SKIP，包耗时 42.920s、exit 0；证据 `.evidence/base-refresh/final.log`。`git diff --check` 与本次文档的 13 个相对链接检查通过。

本次不部署安装环境、不修改或重跑 101，不使用安装目录作为测试现场，不声明新的真实 Codex/PR、恢复或全量测试通过。
