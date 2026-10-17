# PR 合并提交元数据验收

2026-10-17。[Plan](../spec/pr-merge-metadata.plan.md)。

| ID | 结果 |
| --- | --- |
| PMM-A01 | 通过：broker scope/secret boundary、missing/wrong target、实际 gh wire 边界定向测试，含新增 mergeCommit 正反例；命令见 Plan |
| PMM-A02 | 通过：最终独立 SteerLine 包三阶段/真实 PR #429、原查询、实际 Git 产物与资源停止核验；见最终结果 |

原失败发生在 SteerLine 调试包 `dev-20261017-080103-48c56719-dirty`，Run `8529c84576597c8872b986ca46a3e670`。worker 在 sandbox 内成功 push 并创建 SteerlineTest PR #428，随后带 `mergeCommit` 的查询被当前 helper 60035e89 的 broker 以 403 拒绝。诊断读取只查看原 session 中的 PR 命令，不读取/导出认证；外部独立查询确认 PR OPEN 且 base/head 正确。不把创建成功记为该轮 publish 通过。

## 构建证据

基线 `d4969d07e41406c8a85179b6a483d42ee367b1e9`，分支 `codex/pr-merge-metadata`。仅两份 Go 文件 diff，LF 编码补丁 SHA256 `c94001dc1469e57a982faea9eeb925e3edd3236eb292e45bec623c18307e0f12`。本机及执行主机均以 Go 1.26.4、`go build -trimpath ./cmd/steerrunner-agent` 构建，helper SHA256 完全相同：`5d072d5df6dc5fd85f395a550156cb52430168335dfb2fb3dbcde3e8c1f0e2b6`。

执行主机的新独立目录为 `C:/sr-runtime/steerline-orca-wiring-20261017/`；从固定基线克隆后应用同补丁并运行 PMM-A01，实际通过。未替换旧 helper 或安装环境。首次 patch check 因传输换行失败，在 apply 前拒绝；核实源码未修改后，以 LF 补丁通过。该准备失败不涉及业务输入或任务重试。

本机新 Runner EXE SHA256 `868af3b5798c44d3cb8169fc8547df59d50643dc0b8d544506cb63591b953d97`，来源如实标记 d4969d0-dirty。证据在 `.evidence/pr-merge-metadata/`：`targeted.log`、`fix.patch`、`remote-build-facts.json` 与两份 EXE；产品最终接线结果继续单独登记。

## 同仓 owner qualifier 补充

首版补丁包 `C:/SteerlineDebug/dev-20261017-081319-48c56719-dirty` 的 Run `fa7d1d9d10d65154ef6ec9d6e19ae9b3` 仍失败：gh 创建 PR 传入同仓 owner-qualified head，旧 guard 一概拒绝冒号。Git push 已成功；此轮无 PR，之后明确停止原 Run 和产品宿主，原失败保留。

第二版在 repositoryId 已匹配后只允许目标 owner 的 qualifier，保留其他 owner/空分支/多重限定拒绝。定向测试在两台机器均通过；Go 1.26.4 构建的 helper SHA 再次一致。新目录 `C:/sr-runtime/steerline-orca-wiring-20261017-v2/`，不替换原目录。

最终候选构建证据在 `.evidence/pr-merge-metadata-v2/`：Runner SHA256 `a5e05a64fb682cd260288ac9f0691d1f87bfd72877136bf0ac114dccbe295676`；helper `fdae6d687e49a57e5036c02d2a74274f64b5865356608e2e4058a98548ee3d0b`；两份 Go 源的补丁 SHA256 `aef5503541fcf4f12435b0e64868b85ca64d59a58dbd5dd31dea0a941ce393f9`。原始 PMM-A02 失败仍保留，最终新包重新验证。

## 最终结果

现场 `C:/SteerlineDebug/dev-20261017-082031-48c56719-dirty` 固定上述第二版 Runner/helper，默认 sandbox、gpt-6-luna/high。Run `40fa14d710bd4883d765d368593cb363`、Attempt `2f0ca9e41bbe9d91f5298444dcadc51a`，原 terminal `term_16bf7655-ca3d-4bb2-9e12-c4140b8ac5cd` / incarnation `b55bac64-7fc6-45f5-9cc6-a29822352713`，完成 implement.execute/report/publish 到 pr_ready；三份公开 final 均 verified/succeeded、原 app-server 来源、同 session/Attempt，无 publish.report。

worker 在原沙箱实际执行包含 mergeCommit 的完整 gh pr view 查询并成功，摘录在该包 `evidence/publish-commands-diagnostic.txt`。实际 [PR #429](https://github.com/erranliu/SteerlineTest/pull/429) 为 SteerlineTest 专属 base `codex/debug/dev-20261017-082031-48c56719-dirty/base`、head `erranliu/sl-40fa14d710bd4883d765d368593cb363`；提交 `ad7305043690b54feab6678a93d27bd8128d9675`。独立 fetch Git objects 校验唯一变更 result.txt、UTF-8/LF 与精确 branch，77 字节，SHA256 `2e90f2613d7371afc4df10d6922fd805ecc074acc2d619607d6685f0ba422416`。见 artifact-verification.json / runner-events.json。此次模型使用未限定 head；qualified 同 owner 仅在两台机器的定向正/负测试验证，不声称该次实际 create 使用 qualifier。

产品显式 stop_candidate 后，Runner stop_status=stopped、new_inputs_allowed=false；宿主停止返回 0；Runner service not_running 且绑定本 EXE 的 last_shutdown.resources_settled=true，原 worktree 终端 0。证据为该包 stopped-runner-run.json、final-service-status.json、final-orca-terminals.json。PR 保留 OPEN 未合并；本次不是完整 L2/L3 或 TaskDelivery 验收。旧失败、旧构建与原安装环境保持，不把新结果回写为旧现场通过。
