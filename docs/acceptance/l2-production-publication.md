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
- 默认 broker 的独立可信收尾事实必须包含 `denied_requests: 0`，且认证撤销、broker
  关闭与 socket 清理均已确认；缺事实不能解释成零拒绝。
- 确认 worker common 目录的 packed refs 和唯一 worker worktree；记录原生权限、
  命令结果、准备资格与认证撤销事实。L3 故障门及固定 publisher 保持。

## 本轮验证

首轮 `C:/srt/9000197bda8a` 的 Runner 构建为干净 `7c44b8c`，完成真实提交、PR #345、
原会话反馈及专属 base 合入，Run/服务均停止，原批次汇总显示 passed。
复核发现一条 `gh api repos/erranliu/SteerlineTest/pulls/345` 被 broker 拒绝（HTTP 403），
外层 PowerShell 在后续只读命令成功后仍返回 0，旧审计只检查文件系统拒绝而漏检。
因此该轮不作为最终权限资格；原现场及报告保留，不改写历史 passed 字段。

补充修复只允许当前仓库的 `GET /repos/<repository>/pulls/<正整数>`，无 query 参数、
无列表/子资源/REST 写操作。跨仓库、畸形编号及写操作仍在 broker 内拒绝，受控测试
确认拒绝不访问 upstream。命令审计同时拒绝 operation denied 与 HTTP 401/403，
仍不依赖外层退出码。首轮的[原汇总](evidence/l2-production-publication-20261016/initial/batch-summary.json)、
[零退出 API 拒绝](evidence/l2-production-publication-20261016/initial/zero-exit-api-denial.json)、
[新审计复核失败](evidence/l2-production-publication-20261016/initial/posthoc-command-audit.json)
及 [broker 拒绝计数 1](evidence/l2-production-publication-20261016/initial/broker-cleanup.json)独立保留。

## 最终正式结果（2026-10-16）

`scripts/test.ps1 -Level L2 -Config .evidence/l2-config.json -OutDir .evidence/l2-production-final`
返回 exit 0，[完整批次](evidence/l2-production-publication-20261016/batch-summary.json)为 passed、
1/1 case、零排除。配置为 Windows/amd64、Git 2.46.0.windows.1、Codex 0.159.2、
gpt-6-luna/high、sandbox/MXC/never、执行主机已有 gh 登录的仓库限定 broker。

| 检查 | 实际结果 | 证据 |
| --- | --- | --- |
| L1 与框架 | 236 个顶层测试、16 组框架检查通过；仅既有六项显式 helper/native opt-in skip；native driver 编译和零匹配门通过 | [批次](evidence/l2-production-publication-20261016/batch-summary.json)、[框架](evidence/l2-production-publication-20261016/framework-result.json) |
| 受控前置 | 16/16，通过、无 skip | [结果](evidence/l2-production-publication-20261016/controlled-result.json) |
| 最终测试工具包 | 完整包复验通过，包含隐藏拒绝、缺计数、未关闭和未清理反例 | [日志](evidence/l2-production-publication-20261016/final-livetest-package.log) |
| 无模型 Git 前置 | loose/packed head 提交 exit 0、stderr 为空，peer 写入被拒绝，原生进程退出确认 | [日志](evidence/l2-production-publication-20261016/l2-git-boundary.log)、[事实](evidence/l2-production-publication-20261016/l2-git-boundary-result.json) |
| 生产路径 | 无 prompt overrides、无 publisher，packed refs 和唯一 worker worktree 已确认 | [输入](evidence/l2-production-publication-20261016/production-workflow-input.json)、[仓库](evidence/l2-production-publication-20261016/worker-git-repository.json) |
| 真提交及 PR 查询 | 发布阶段提交成功；原失败的同仓库单 PR 查询实际返回正确 JSON、exit 0 | [提交](evidence/l2-production-publication-20261016/production-publish-commit.json)、[查询](evidence/l2-production-publication-20261016/production-pr-query.json) |
| 命令和权限 | 23 条实际命令、权限错误 0；6 contexts/23 tools、提权/审批/bypass 全部为 0 | [命令](evidence/l2-production-publication-20261016/worker-command-audit.json)、[原生权限](evidence/l2-production-publication-20261016/worker-sandbox-audit.json) |
| 认证及停止 | broker allowed=41、denied=0、receive-pack=2、PR create=1；认证撤销、broker closed/socket cleanup、原 Run/服务停止均确认 | [broker](evidence/l2-production-publication-20261016/broker-cleanup.json)、[场景](evidence/l2-production-publication-20261016/scenario.json)、[停止](evidence/l2-production-publication-20261016/cleanup-run.json) |

最终现场 `C:/srt/0a02b593ad83`，Run `a11091eab835ce80d55f3c4f6daefa1c`。
[测试 PR #346](https://github.com/erranliu/SteerlineTest/pull/346) 首次 head 为
`4380a77390cc8fad68302bbac18d78147f084d9d`；原会话反馈更新为
`ce7d1ec11b1dee63bdeb1380a22cce6c7487ab5b`，复用原 session/attempt/工作区/分支/PR。
driver 按精确 head 合入专属 base `codex/srtest/0a02b593ad83/base`，merge
`4ff7ea118546dd5b3ba488558871cf22836069a2`；fetch 校验真实 Git 内容、祖先关系并再运行 checker。
没有合入 SteerlineTest 的 main。见[最终场景](evidence/l2-production-publication-20261016/scenario.json)
及 [Runner merged 事实](evidence/l2-production-publication-20261016/merged-run.json)。

实际 Runner/driver 冻结在干净 `ecd4b068cdd18bf2932cb0ca2de8c1e0a174348b`，
Runner SHA256 为 `f8170b43db7ebd813fdc7a9c8e71d20e24f6f18d5a106822951943671789c995`，
见[构建身份](evidence/l2-production-publication-20261016/source-identity.json)。总入口汇总保留
启动时 `ff90bc7`；新增的 broker 收尾断言在 native driver 编译前纳入，并执行了最终测试工具包
完整回归。交付前与[最终实现源码清单](evidence/l2-production-publication-20261016/source-hashes.json)
比对零差异，后续仅整理文档/证据。原失败 Run 未部署、恢复或扩大授权。
