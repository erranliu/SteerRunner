# Orca 原生合同正式启用 B6 验收

日期：2026-11-03。状态：ONR-01–05 已通过，B6 最终独立审计 clean；全部批次已签收，统一 PR 交付准备中。依据 [Spec](../spec/orca-native-rollout.spec.md) / [Plan](../spec/orca-native-rollout.plan.md)，B5 前置已签收，不代替以下最终资格。

## 变更与保留边界

Spec/Plan `766fb5a`；实施 `e7acedc`；集成 main `7f6c538` 的最终源码 `e30fc82315148efa0bde2bcd01485d9e108a1df5`。公开 capabilities 同步双端原生选择、orca_terminal 描述、模型/审计/流边界及保留例外。schema/catalog/admission、外层 state/v8/user_version、private codecs、路由摘要和 receipt/stop progress 均未改。

删除 PreparationFacts 被紧邻 agentExecutionSupport 更强条件完全覆盖的4行 Pi/DSH guard，原 agent 拒绝/错误/零副作用保持。原生组合已在 B5 不消费 managed helper/control/history；显式 helper/audited、Pi/DSH 和其他 workflow/旧状态的实际消费者仍保留，清单和未验证边界见[当前合同](../orca-execution.md)。没有把保留链路冒称已统一或无消费者。

主线集成仅两处文档冲突，双方章节/索引均保留；共享 application 自动合并，保留 main 的 readCompletionCheckpoint / ValidatePolicyResourceForProfile 解释边界，以及本批 profile9 scope、native workspace unreported、input.Continue 反馈 gate。没有另扩原直接 Codex 任务。

## 最终源码与构建

生产 source_digest `cdd045705fb9e6b63bd5581215826247422f31a91a8de2ef6b04db46fb6c19f7`，clean；Runner SHA256 `8af60c5507efa0a4774f21acb36169cda41f046a7d4ab5922185f132c654e162`，Go1.24.2、Windows/amd64。四场 Runner/driver/CLI/转发器相同。原 CLI 1.4.242 SHA256 `e7a6fa40a59d237cdb7cf32444df6660da850d1b31e4162634287d91fdfc3cba`，原本机 runtime `8a0d209e-094d-4ec3-a923-5cf6c064df67`、明确 **台式机64G** runtime `64bdec68-d3ed-4216-a894-0cc143ed4653` 均先只读确认后冻结。

| 最终场次 / fixture | Run | 结果 |
| --- | --- | --- |
| 本机活动 `baf0fb631927` | `6ccdbba360cb724dcd7bb0d4588eac86` | implement.execute 已 Started、原 root/provider working、尚无 final 时正式 stop；在线及真实离线通过，无 PR |
| 远端活动 `d066f33fe011` | `9e2bb0a967d52a4aacd7ec88b3277424` | 同本机活动边界；在线及真实离线通过，无 PR |
| 本机开发 `0b74a34ce9b4` | `a1c15690489006b27e689a754a1445ed` | 原会话反馈、真实原主机修改/检查/commit/push/[未合并 PR #756](https://github.com/erranliu/SteerlineTest/pull/756)，head `e2ae70fd148b9425189af9f51be84dc48000a446`；停止离线通过 |
| 远端开发 `6386ff2b3347` | `59c167c724b52268fa3dd91c9243c169` | 同合同，[未合并 PR #757](https://github.com/erranliu/SteerlineTest/pull/757)，head `916107db9da64457f033fc6823121e2094426346`；停止离线通过 |

全部经正式 CLI 受理/冻结/Engine/持久结果；driver 不构造 Run/Snapshot，不代模型发布、不合并。反馈在 pr_ready 前保持原 terminal/incarnation/provider；独立 GH 与精确 Git 对象/base/branch/head/diff/材料、私有 QA checkout 的真实 check.ps1 均通过。

四场各原 close=1，零旧 helper/control/history/扩展调用；活动撤销前后 send 保持2，未派发后继。完整原 runtime/worktree/local host 清单、online stopped、service writer 退出、真实 service verify-stopped 一致，scope 均 orca_terminal。准备终端各自确认关闭，不混进业务清单。远端活动准备曾出现短暂 operation_outcome_unknown 后只对账原操作完成，未重建/重发；最终调用计数核实。

B5 的旧构建成功及历史失败仅前置事实；本表四场均 fresh。测试 base/worktree 与未合并 PR 保留；没有部署、安装或更改用户原服务/目录/配置。

## 受控回归与升级

| 集合 | 实际结果 |
| --- | --- |
| resources/contracts/两个 Orca 包/workflow 五包 | 293 根/1354 子节点/1470 叶例，1640 pass、7 opt-in skip、0 fail |
| 正式候选 Engine | 4 根/26 子节点/26 叶全 pass：双端9种停止模式、FIFO取消、blocked首次 stop、feedback idle/active |
| 正式 CLI ownership/offline | 1 根/32 子节点/28 叶全 pass：different_release、旧及候选 offline、queries_do_not_start |
| 公开合同最终补验 | 1 根/2 子节点/2 叶全 pass；与生产 StopScope/正式配置 admission/catalog 交叉验证 |
| 集成后共享 application | 10 根/30 子节点/34 叶全 pass：包含上述候选及 main 新完成/权限知识边界、native 来源回归 |
| 其他 | 生产/配置 helper/live-tag 构建、依赖守卫通过；独立审计另复跑关键定向通过 |

各集合重叠不相加。五包及初轮 Engine/CLI 在 merge 前执行；merge 后共享 application 定向及最终四场验证集成源码。7 skip 为各 opt-in native 探针，不计真实资格；精确名称与所有原始测试统计在归档。没有本批受控失败，构建和零匹配不充作资格。

原 EXE 先排空 Run/未决操作并停止、原目录真实核验，新构建场次使用 fresh 目录。different_release 的实际验证是协议兼容客户端选择原实例、没有替换 writer，不是不同客户端 EXE 一律拒绝。旧格式/记录/摘要不迁移，不增跨构建活动任务接管器。

## 独立审计与证据

实现及集成后代码独立审计 clean：重复 guard 不可达，公开 metadata 与选择/scope/保留消费者一致，main 证明边界与本批 native/反馈规则保留；独立关键测试通过。同一独立审计者最终收口 clean，无未关闭 P1/P2：四场反馈/开发/活动/原 scope/真实离线成立；远端 unknown 后只有原对账；经 GitHub 只读复查 #756/#757 当前 open、merged=false、精确 base/head 一致。164 文件、本地原件、归档及5组统计全部重算匹配；8个公开派生副本仅裁除10条无关仓库行、加入6处归档说明，相关语义/计数完全一致。ONR-01–05 可签收，限 Codex/development/2 原生合同与 orca_terminal；保留例外不继承资格。

[证据](evidence/orca-native-rollout-20261103/README.md)：164 个 UTF-8 文件，逐文件字节/SHA，可机械复核；commands、integration、source/build、test-statistics、scene-ledger 及完整正式回执/原资源/PR/停止证据分开。公开副本只裁除无关注册仓库行，derivations 记录原始源 SHA；相关操作语义和计数保持，原完整日志本地保留。无凭据、认证帧、EXE 或数据库。
