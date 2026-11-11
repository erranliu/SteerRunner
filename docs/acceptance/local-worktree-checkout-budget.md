# 本机工作树部分检出与准备预算

2026-11-11 验证。Runner 修复构建来自 `26193f20f133fb17a77e117084c29e74002b48e6`，SHA-256 为 `00b22b35c7ac42342cd32d445bef4658b051e656e15d6da5f4ceb34a25289ea6`。未更新正式安装。

原 308 Run `8c370c988d7ff079c7a9d815515709b7` 在本机 Git LFS 检出阶段超时。工作树留下零字节 `index.lock`，未写入正式索引。准备恢复读取到不干净的工作树仍返回 `confirmed`，随后首条输入校验拒绝。

新建本机工作树使用原准备阶段的剩余期限执行 `git worktree add`。其他 Git 子命令各自保留原时限。超时返回准备失败并保留原 Git 诊断。原准备回执丢失时，只有固定提交、有效索引且无索引锁、干净工作树均核实后，才确认准备成功。

## 回归

- 本地 Git filter 夹具覆盖 LFS 检出超时和完整检出。超时留下索引锁，正式索引缺失，执行与恢复路径均无准备成功回执。完整检出用时超过夹具的通用 Git 时限，仍在原准备期限内成功，回执丢失后可读回。
- 应用层验证原准备期限传入本机检出。准备失败后无工作区资格、无后续输入。原工作区恢复、已绑定路径及 Git checkout hook 边界回归通过。
- 正式 L1 通过。使用 `scripts/test.ps1 -Level L1`，报告位于证据根的 `test-results/L1/summary.json`，记录干净源码 `26193f20`、786 项 Go 测试及预期跳过名单。

## 308 真实复测

使用独立 SteerLine home、真实 `merge-h-308-code` 项目 clone、`AIRefactor` 提交 `470b61998332c0dbbbfd3678b1c24d43e9068318`，以及原本机 Codex 方案 `project-default`。通过 SteerLine 正式环境检测入口发起新检测 `4aca6be3df194a8688cec9768dd2a3b3`、Runner Run `bb0fd79d74280575a9b1002652812aa5`、Attempt `f8a908fe9b19d8855f4cedf19231adb7`。

Runner 确认准备工作树和会话，接受并执行首条输入。核实正式 Git 索引存在、索引锁不存在，原先缺失的六个 LFS 资源全部落盘，其 SHA-256 均与固定提交中的 LFS 指针匹配。

Runner 原生执行完成并停止，结果封存版本为 1。项目报告 `completed=true`、`ready=false`，缺项为 `unity.editor`、`unity.android`、`unity.license`。隔离应用退出码为 0，正式停机回执确认 Runner 停止且资源结算，已删除此次使用的隔离认证副本。

原始证据位于 `C:/SteerlineDebug/wt-runner-local-checkout-308/974026b94eb84af089af5d530686ab7c/`。真实复测见 `diagnostics/builds.json`、`diagnostics/checkout-proof.json`、`diagnostics/lfs-sha256.json`、`diagnostics/runner-inspect.json`、`diagnostics/detection-final.json` 和 `home/application-api/shutdown.json`。
