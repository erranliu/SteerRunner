# Orca helper 退役验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

2026-11-04。本机 Orca 1.4.246。独立验收目录和 Runner 原记录在工作树 `.evidence/helper-retirement-final/`。正式场共同使用源码摘要 `e460bf1689404dd6202ff601f9eef593bf21dce3dda5362744f225a3dadc64b2`，Runner SHA256 `05ea74e8688a099516bca6920af67f7c1f4cc9fcf4e0319570c5fe74ff605f91`。第二冷读 EXE SHA256 `7c176b51a6dbca68bbe4be78bd98c79a35e8b8b514aeab61f872df8a2babd23d`，来自同一源码。

## 本机 audited 正式发布

Run `849915b692615da75708ddb5bdd0fc68` 用同一会话完成实施、审计修复和发布。实际模型逐轮证明为 `gpt-6.1-sol` 的 `medium → high → medium`，原有限定来源登记均已撤销。实施 HEAD `b0c702c5457b9a6b751792030b18f23536a65c81`，tree `a66d637939a75bdd80462dd467af474495d09e07`。审计修复后的 HEAD `940f78af450e092a90b71bbca99d15b7e5e75bca`，tree `532d61aa2fca6e37e7b213f2e30c5d06a343c52d`。发布核验得到相同 HEAD/tree，GitHub 的原 base/head 及 PR 开启状态均通过核对。 [专用测试 PR #764](https://github.com/erranliu/SteerlineTest/pull/764)

`pr_ready` 触发 15 字节的 `steerrunner-result-file-snapshot/v1`。后续显式停止保持同一 `observed_at`、版本 1 和 SHA256 `e56f9f3212affdfa81cb9008078d09c1056006f47e60571b3bd4a03f162f3432`。停止撤销恢复资格，正常停服后原目录双 EXE 排空通过。首轮活动停止 Run `169f946645a81595700a0742bef81e41` 在原活动标记后派发 stop，关闭原会话并取得同范围双 EXE 排空。

首次正式尝试的审计回合被宿主账户拒绝，公开原回执报告 `gpt-6.1-luna` 不支持。该 Run 没有进入发布且已停止。随后用账户已支持的模型配置不同 effort，实际三轮核验保持。续轮活动停止的第一次尝试在第一轮读取来源时报告 `IDENTITY_CHANGED`，未进入续轮且已停止。分开执行的复验 Run `c7171823ff48019c46831f186badf838` 在原审计续轮 `01a1a66a-73e0-73b2-9aba-82d2a5dcb067` 活动时停止，取得同范围双 EXE 原目录排空。

## 已合入路径与 Pi

| 范围 | 验收 |
| --- | --- |
| 本机 Codex `development/2` | `c46a75ca5028593771d09854d971917e`。原会话接受多行反馈并完成第二轮，停止、原 `result-file/v1` 及双 EXE 原目录排空通过。 |
| 配对远端 Codex `development/2` | `db2dc88dd1a4f62e5b6521c2bc644210`。同样取得同会话两轮、原 `result-file/v1` 与双 EXE 排空。 |
| 本机 Pi `execute/1` 和 `development/2` | `8099d096f7e38fccd55d054935502567` 完成多行输入和只读 shell 验证，`0f166b24c012552114e4d8a3cf357d18` 完成同会话两轮。两场均采集一次文件快照，原会话停止及双 EXE 排空通过。 |
| 双端 Codex `execute/1` | 本机 `519a67ef6c3ca84e3f30ce8ee471bdec`、`f30bd944e2500f2e5248943d3c16c3b3`，配对端 `ef68b6ca240430ef840f193eab423ec8`、`36a6c42f9a52fa04ee414c24eb0ae44f`。两端各做多行正常回合和活动停止，`result-file/v1`、同构建重启和离线停止均通过。此组构建 SHA256 `5097b19bdc07cb289c9c4baf142c1c15b008ea32b73418b4bab5198c08dc7411`，在本机 audited 同模型不同 effort 准入调整前。 |

排空使用 SteerLine `6dfb5286` 的 `VerifyUpgradeDrained`。临时测试 overlay 只调用原入口，SteerLine 生产代码和门禁保持。候选构建之后仅补齐 Pi 的公开用量能力与采集不可用诊断，按资源和 usage 定向回归验证。

受控验证覆盖配置与公开 schema、工作流层依赖边界、原输入与结果归属、旧终端原停机夹具、结构化停止及原检查点负例。正式验收是专项范围。旧部署仍须用其原构建排空实际原目录，Pi 原生 usage 和远端 Pi 没有取得资格。[退役范围](../spec/orca-helper-retirement.spec.md)
