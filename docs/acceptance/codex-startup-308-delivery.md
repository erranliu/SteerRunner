# Codex 启动与 308 LFS 修复交付汇总

2026-11-20，按用户要求将本轮 Runner 修复整理提交 PR；基线 `8ebbc717776eb2ad85371060566776c4402773fa`，分支 `codex/isolate-appserver-history`。不部署、不合并，不操作原 SteerLine 重构任务。

## 修复和原始证据

- [启动索引复用](codex-startup-index-reuse.md)：不再为每个 app-server 指定新 sqlite_home，保留原生 home/config 的索引选择；本机 prepare、远端 helper 和 stdio history reader 在同一 home 取得 OS 初始化锁。成功 initialize 后释放，模型回合和每个 Run 的资源/停止责任仍独立；失败清理先于锁释放。
- [Windows LFS hooks](windows-lfs-worktree-hooks.md)：worktree 创建的临时 core.hooksPath 改用 os.DevNull，保留 LFS 内容过滤、原 hooks 和干净门禁。新增真实本地 LFS 回归，不改项目持久配置、不清除用户工作区文件。

已有完整 Codex owner、启动互斥/取消/跨进程退出回归、128 条合成 rollout 的真实无认证探针和 workspace owner 通过，原命令/断言/清理与失败对照详见上述记录。证据分别为 `C:/SteerlineDebug/wt-9ed5-runner-startup/` 和 `C:/SteerlineDebug/wt-9ed5-308-docs/diagnostics/runner-workspace-fixed-01/`。后者 `result.json` 的 exit_code=0，`tests.jsonl` 保留完整 14 个顶层测试终态，不存在正式 summary 的文件名不另行推断。

原完整 L1 的两个有界计时失败及输入身份变化、Controlled foundation/content 中 standalone 的既有失败全部保留；基线对照及最终定向补验见启动专项。它们没有被真实消费方通过覆盖，不能声明完整 L1/Controlled、完整 Runner L2/L3 或部署资格，也不调整测试断言和超时。

## 最终产物和消费方验证

最终冻结 EXE 为 `C:/SteerlineDebug/wt-9ed5-308-docs/artifacts/steerrunner-windows-lfs.exe`，版本 `0.2.0-dev.4`、原构建 source_dirty=true，SHA256 `31153b1f865364fc1c6a0ddef84e2596f2d67222a3ac25c904af795f853dd10f`。本次提交前使用正式 `Get-SRSourceIdentity`（build.ps1 原生产输入列表）核对当前源码摘要，仍为 `75824ff79737b01e5fd974f4ea91dcc098a4095f059f6a8f6d757d0ed53de9d3`，与原 build.json 一致；不改原构建 commit/dirty，不重签历史结果。

SteerLine 基线 `3a1777a6bbf5d4b245be2ca12aa24c4b2af4e905` 加本轮修复的消费方结果：

| 验证 | 结果与证据 |
| --- | --- |
| 正式 GitHub 维护全链 | `C:/SteerlineDebug/wt-9ed5-refactoring-live/test-results/L4/local-codex-01/summary.json` 通过并清理完成；使用此前仅索引修复的固定 EXE，详见启动专项，不伪装为最终 LFS 构建资格 |
| 真实 308 文档维护 | 使用最终 EXE，`C:/SteerlineDebug/wt-9ed5-308-docs/packages/run-04/evidence/test-result.json` 通过；真实选题、实施、验收、发布 draft PR 和评审完成，工作区干净，无 dev/；当时评审建议 insufficient_evidence |
| #463 最终源码评审 | 同一最终 EXE，`C:/SteerlineDebug/wt-9ed5-review-source/packages/pr463-01/result.json` 通过，固定 head `13394163`、只读源码可用，建议 revise |
| #463 文档收窄后复审 | 同一最终 EXE，`C:/SteerlineDebug/wt-9ed5-pr463-narrow/packages/edit-01/result.json` 通过；固定 head `8c484f37`，建议 merge，1 项/零 skip/263.85 秒，内置格式重试一次，cleanup.json 核实资源/源码/认证回收 |

最后两项从 report 阶段创建新隔离现场，不接管旧数据库或 Run，不重新实施/发布；测试入口为 SteerLine 的 `go test -tags=runneracceptance ./internal/app -run '^TestRefactoringPinnedSourceReviewLive$' -count=1 -timeout=15m -json`，实际输入和 Runner 身份保存在各自根目录。评审建议只针对文档与所查源码，不表示 Unity/恢复行为经过运行验证。

本次仅补验收记录，不改生产输入/依赖/测试定义，按原测试规则复用上述证据，不因提交和提 PR 重跑同范围。提交前本仓 4 份本轮指南/验收文档的 25 个本地链接全部存在；SteerLine 文档检查为 824 份 Markdown、5,024 个本地链接、零错误；两仓 git diff --check 通过。已刷新 Runner main，仍为原基线，PR 创建后再核对 GitHub 的实际文件合并状态。

## 原现场

原任务 `f292bdcfcf4b4854ad10819f0c4834a9` 没有接收停止、恢复或重新提交命令。只读核对原 308 checkout 干净且 HEAD 仍为 `9a084fb61a3bfdac79cf30f0ff8b3a1cc280474a`；安装 Runner `C:/SteerlineInstall/bin/steerrunner.exe` SHA256 仍为 `0d733a1fcd09f06ad7e2891ad90030f16d3fb4fae1b877adc86fbbe1bf74137e`。308 的 [draft PR #463](http://192.168.0.66:3000/zymobile/merge-h-308-code/pulls/463) 保留未合并，本次不再修改。提交中不包含安装产物、登录文件、现场数据库或原始模型日志。
