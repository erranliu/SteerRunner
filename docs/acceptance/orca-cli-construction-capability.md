# Orca 公共命令构造 B2a 验收

日期：2026-11-02。状态：实现、定向回归及实施后独立审计通过。依据 [Spec](../spec/orca-cli-construction-capability.spec.md) / [Plan](../spec/orca-cli-construction-capability.plan.md)。本批只迁移纯命令构造及基础解码，不增加产品或真实后端资格。

## 1. 职责迁移

protocol/orca/cli_command.go 的 CompileCLICommand 消费 executable、前缀参数、明确 environment 和操作 argv，返回自有 CLICommand。它保留原参数顺序、三项路由环境屏蔽和独立 slice，不读隐式路由环境、不执行进程、不理解 Run。cli_response.go 承接原 Result/Envelope/ParseEnvelope，并提供 DecodeCLIResult，保留 payload/runtime/错误及调用方提供的不确定事实；两文件均无执行包依赖。

执行侧 cliLaunchCommand 只映射工作目录、timeout 和原 backend 命令值。Lifecycle 普通调用、creationCall 新计划、readCreationPlan 都消费同一构造；三处专属参数/屏蔽拼接已删除。Client.MetaCallWithTimeout 和 creationCall 原结果消费同一解码，owner 专属 Result 拼装已删除。

原命令长度检查、PreparationOwnsCLI、SerializeCLI、CheckpointDispatch、取得队列后的回调及 creation owner marker/deadline/首次派发/持久对账不变。原 ParseEnvelope 算法及所有持久类型声明保持；不改 profile/state/target、业务正文、创建专项操作或停止规则。包中原执行依赖仍按后续 B2–B4 迁移。

## 2. 直接验证

源码为文档基线 `93096ac` 加本批差异；source_digest 为 `9e2f640f8bac2a0d14029e8de67a34df7f0816375d4aca22f023edd61b881651`，使用原 Get-SRSourceIdentity 的相同源码范围。原始 test2json、命令及源码身份保存在[证据目录](evidence/orca-cli-construction-capability-20261102/README.md)。

| 验收 | 证据和结果 |
| --- | --- |
| OCC-01 自有构造 | TestOrcaCLICommandConstruction：本机/远端、空参数/前缀、空格/引号/中文/换行，固定路由环境屏蔽及双向 slice 隔离 |
| OCC-02 原字节/消费者 | TestOrcaCLICommandEncodingAndConsumers：生产构造的日常调用与真实 creationCall/owner 夹具分别捕获命令，local/remote 均与旧 LaunchCommand 字面量 JSON/摘要一致；旧字面量命令持久计划仍可读取 |
| OCC-03 解码 | TestOrcaCLIResultDecoding：成功、BOM、拒绝、无错误体、坏包、空包；每项保持 false/true 不确定事实；nil/真实进程退出/传输未知/未调用 queue 的客户端分类及零重试 |
| OCC-04 原派发边界 | 全部 8 个 TestCreationOwner 根、FIFO、跨进程锁/杀 owner 后恢复、准备队列及回调、撤销前零派发、发送/stop 共享队列；未知/外来回执无重建，期限不重置 |
| OCC-05 收口 | 两项 StateCodec、workspace 创建/恢复/替换身份和远端创建回执；workflow 依赖守卫；生产白名单及 git diff --check |

新增 3 根、13 子例通过；原回归 21 根、35 子例通过；依赖守卫 1 根通过，均为 0 fail/skip。测试使用隔离 ProcessHost 和原文件/进程/互斥锁机制，没有创建真实 Orca 资源、启动模型或重复 B0/B1 现场。

首轮新增测试中，fixture 更换 executable/前缀后未同步其冻结 Target，两个消费者子例按原生产校验拒绝，未派发命令。修正 fixture 的 Target 后重新执行全部新增根通过；原校验保持，首轮失败日志保留。

## 3. 独立审计

对实现提交 `7240c16` 相对 `93096ac` 的独立只读审计通过，未发现确认的 P0/P1/P2。审计者复核三处构造和两处解码、全部持久类型/派发边界，以及 6 个归档文件 bytes/hash 和源码摘要。

新增三根、原 21 根回归、workflow 依赖守卫及固定提交 diff 检查独立复跑通过；进程/锁夹具使用正常权限。审计未启动模型或操作真实 Orca 资源，不扩大资格。无确认问题，因此无代码修复 commit。
