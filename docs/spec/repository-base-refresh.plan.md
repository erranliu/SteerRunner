# 提交前刷新固定目标分支 Plan

依据：[Runner Spec](steerrunner.spec.md) 的固定 repository/base_revision 与接受前材料规则。问题是从调用方 local_path 克隆时只得到其本地分支头，可能落后于调用方刚 fetch 的远端目标；已有缓存也会保留旧引用。仅取得固定 commit 对象不足以证明目标分支的新状态。

## 实施

1. 在既有缓存锁内保持远端 URL 身份校验，验证 base_branch 格式。
2. 每次新 submit 的 freeze 都从已固定 remote 显式 fetch `+refs/heads/<base_branch>:refs/remotes/<remote>/<base_branch>`，同时适用于新缓存和已有缓存。刷新失败返回 environment_unavailable，不能用旧引用继续接受。
3. 保持原完整 base_revision、Spec/Plan 路径、请求摘要与调用方工作区不变；刷新后仍执行原基线的祖先关系检查。远端回退排除该基线时仍拒绝。
4. 不修改已接受 Run 的恢复或材料，不重放旧请求，不增加自动 retry；工作流节点/异常/恢复授权没有新状态。共同生命周期规则仍由 SteerLine 的 `docs/product-design/worker-lifecycle.md` 维护，本次只修正接受前 Git 准备事实。

## 测试计划

验收归属：[BR-A01–BR-A07](../acceptance/repository-base-refresh.md)。环境为 Windows/amd64、Go、Git；使用 Go 临时目录和隔离 Git 全局配置，将测试 URL 映射到本次本地 bare remote。真实 Git clone/fetch/push，freeze 使用惰性的测试 EXE 路径，不启动 Codex/模型或发布真实 PR，不使用安装目录。

修复前先跑 BR-A01 证明原错误，修复后、提 PR 前执行 BR-C1：

```powershell
go test ./internal/application -run '^(TestSRFreezeBaseBranchRefresh|TestSRRepositoryCacheWithLongBranchPath|TestSRCompactCacheCanPublishLinkedWorktree)$' -count=1 -timeout=4m -v
```

| 验收项 | BR-C1 场景 | 通过条件 | 证据登记 |
| --- | --- | --- | --- |
| BR-A01 | `TestSRFreezeBaseBranchRefresh/local-branch-behind` | 原代码出现同一 Pinned base 拒绝；修复后成功 freeze，缓存引用刷新，调用方本地分支仍落后，固定基线和文档路径保持；不读取旧文档 commit 或正文 | 专属验收；`.evidence/base-refresh/repro-before.log`、`after.log` |
| BR-A02 | `warm-cache-custom-target` | 缓存复用且自定义 upstream/release 分支刷新到新头；原基线与输入摘要、材料不随新头改变 | 专属验收；`after.log` |
| BR-A03 | `off-branch` | 对象可取得但不属于指定目标，仍按原祖先关系规则 invalid_input 拒绝 | 同上 |
| BR-A04 | `rewound` | 已有有效缓存遇远端强制回退，刷新目标并拒绝已被排除的原基线 | 同上 |
| BR-A05 | `deleted` | 已有缓存遇目标分支删除，environment_unavailable，不能按旧缓存继续 freeze | 同上 |
| BR-A06 | `offline` | 已有缓存遇远端不可达，environment_unavailable，不能回退到旧引用 | 同上 |
| BR-A07 | 既有长分支路径、深缓存 linked-worktree 发布两项测试 | 长路径准备和真实隔离 Git push 保持通过 | 同上 |

全部负例要求 freeze 返回 nil 记录和预期错误，原输入不变。成功例核对绑定文档路径、固定执行基线、调用方本地分支与未创建执行工作区；开发流程不再读取文档正文或生成材料快照。辅助检查 `git diff --check` 和新增文档相对链接。

不跑 full、模型/PR/live、产品 R1 或 Orca：本次仅接受前 Git 引用准备和校验，未改执行器/恢复/发布。安装版任务 101 的原候选及其 rejected 事实保留，本次 PR 不包含部署或现场重试。无需新增人工验收前置。
