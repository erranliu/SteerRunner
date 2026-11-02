# Spec：Orca 公共命令构造 B2a

日期：2026-11-02。状态：待实施。依据[长期路线 B2a](orca-capability-layer.spec.md#6-分批实施路线)，对应 [Plan](orca-cli-construction-capability.plan.md)。调查基线 `c9097a7`。

## 1. 目标和边界

抽取与链路无关的 Orca CLI 参数编译、固定目标环境屏蔽和基础 envelope 解码，让日常调用、creation owner 计划生成/校验及原回执读取消费同一实现。每个消费者仍负责执行模型、预算、可靠派发和持久化；不创建新的调用状态机或 Runner 接口。

能力层的纯构造输入只含 executable、已有前缀参数、明确 environment 和操作参数，返回独立 argv 与需屏蔽的环境变量名。原 executable 解析、工作目录、timeout、Windows 命令长度校验及 ManagedProcessHost 映射留给执行侧。空 environment 仍表示明确本机路由，不读 ORCA_ENVIRONMENT 等隐式设置。

基础 envelope/result 解码使用自身 Result/Envelope 类型，保留 BOM、原 runtime、payload、原生错误码/消息及调用方显式提供的不确定事实，不接受 ProcessHost、Run、creation plan 或 backend 错误。该纯解码不判断操作可否重发。普通调用仍先辨别排队未调用与可能派发，再传入原有 CallUncertain；owner 仍使用原 result.Failed 语义，不能顺带改变不确定性分类。

本批不抽 send/read/wait/create/close 专项参数，不改 profile/state/target 摘要或 creationPlan/Result/Started 的任何字段、版本、JSON 编码和摘要。不调整原 queue、CheckpointDispatch、PreparationOwnsCLI、派发前回调位置、owner deadline/独占 marker/原结果对账。B4b 才抽创建操作。

## 2. 迁移和保留

| 当前实现 | 迁入能力 | 消费者保留及删除要求 |
| --- | --- | --- |
| Lifecycle CLI 闭包 | profile Args + 操作 Args + 显式 --environment、三项环境屏蔽 | 映射 LaunchCommand，保留长度校验和原队列/准备回调，删除专属参数拼接 |
| creationCall 新计划、readCreationPlan | 同一纯命令构造 | 原 executable 绝对化、hash、命令冻结及严格绑定；删除两处重复拼接/屏蔽列表 |
| Client.MetaCallWithTimeout | 基础 envelope→Result 解码 | 原调用错误分类、queue 零输出特例、errors.Join |
| creationCall 原结果 | 同一基础解码 | 原持久结果/started/摘要校验及 Failed 的现行解释；删除重复 Result 构造 |

新增能力文件不导入 execution/backend/process、workflow、storage、reliable 或 Codex。执行边界可增加一个小值映射函数，使日常与 owner 都接入能力；不能只移动重复字符串而留下两个构造算法，也不能把创建对账移入能力层。

## 3. 验收

| ID | 完成条件 | 直接证据 |
| --- | --- | --- |
| OCC-01 | 纯命令构造可脱离执行快照使用；参数、环境屏蔽及 slice 所有权保持 | 自有值测试，本机/远端、空/前缀参数、空格/引号/中文、输入输出不互相污染 |
| OCC-02 | 日常实际调用和 owner 计划/校验均使用同一构造；原命令和编码逐字节一致 | 实际 ProcessHost 参数捕获、固定旧格式 JSON/摘要、所有原 owner 对账 tests |
| OCC-03 | 普通和 owner 回执共用解码，保留错误和不确定阶段区别 | BOM/成功/拒绝/坏包、原 CLI queue/transport/exit 分类及持久失败回执 |
| OCC-04 | 授权、排队和原创建唯一派发保持 | 排队取消零调用、取得原队列后回调、原 deadline、caller loss、未知/外来回执零重建 |
| OCC-05 | 消费者接入和删除闭环，格式及范围不变 | 生产调用清单、白名单 diff、依赖守卫及定向组件回归 |

本批为纯构造/解码及消费接线，受控层以完整旧参数和持久格式为 oracle，无需再启动模型或创建真实 Orca 资源。B1 的真实查询、B0 的模型资格不转记为本批新现场。
