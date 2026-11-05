# Orca 带能力声明的 RPC 包装复用 Plan

已按以下步骤实施，完成等价验收、同候选双端目录联验及修改审计。[Spec](orca-capability-rpc.spec.md) · [验收记录](../acceptance/orca-capability-rpc.md)

1. 实施前刷新主线，在隔离分支记录原 structured usage、普通 usage 和两端目录的连接、引导请求、能力、错误及关闭顺序。特别区分目录的首次 runtime 发现与 usage 的固定原 runtime，确认 usage 的临时授权范围操作也会经过现有包装。保存受影响协议包基线，避免把环境失败归因于提取。
2. 将现有 `structuredUsageState` 移至 `state_transport.go` 并改为私有中性名称，保持原字段和固定 `mutation=false` 的委托。structured usage 只替换类型引用。`QueryCatalog` 保留各分支原连接取得与身份核验，将已取得的连接放入现有 `stateConnection`；远端使用该包装，分支后以一个闭包调用 `callState`。保留原 runtime 取值与连接关闭责任，不增加新接口或统一连接工厂。
3. 审查生产 diff，确认未改变引导握手及 `status.get`、能力清单、错误映射、请求/等待次数、runtime 绑定、预算和关闭时机。包装不按方法推断能力、不合并列表、不新增业务 gate 或只读方法白名单。确认普通 usage 与 #113 文件通道保持原路。原回执未知仍不补发；现有 listing 等待是原查询规则，不能扩成通用重试。
4. 在 Windows 执行 `go test ./internal/execution/adapters/protocol/orca ./internal/usage/access -count=1`，并定向执行 `go test ./internal/usage/collect -run '^TestStructuredUsage' -count=1`。协议包重点复用 `TestStructuredUsageChannelDeclaresAuthAndRequestCapabilities`、`TestUsageFilesRejectOriginalIdentityBeforeDial`、`TestLocalOrcaCatalog`、`TestOrcaCapabilityWire`、`TestNativeCatalogMetadataBoundaries`、`TestOrcaCatalogRejectsInvalidIdentities` 和 `TestWorkspaceFilesUseTheVerifiedPairingSnapshot`。核对正常、错身份/回执、能力不足、取消、断线、超限及关闭；精确比较握手和后续请求的原能力内容/顺序与缺省字段，不改引导请求。仅缺覆盖时在现有夹具中补 usage 临时授权范围操作与清理仍可通过、业务派发 checkpoint 零消耗，以及失败不补发/提前关闭的断言。#113 继续要求单次解析、同配置拨号、替代端点零连接和零文件请求。
5. 从同一候选源码构建固定 Runner EXE，在本机与一个已有配对远端分别调用正式 `native-catalog` 的 agents、models 入口，保存公开请求、脱敏响应、runtime、来源、完整性和 EXE/源码身份，确认查询后关闭连接且不创建业务会话。沿现有 CLI 入口和用例，不新增验收框架；有缺口如实记录。此联验不推导模型可调用性或环境检测 ready 资格，不启动业务 Run，也不将 #113 历史真机/排空当成本候选资格。已有发布、停止、文件封存和排空门禁继续适用。
6. 在独立 `docs/acceptance/orca-capability-rpc.md` 记录基线、实际 diff、命令、计数、跳过/阻断及同候选双端结果；区分包测试、定向 collector、真实目录联验与完整 L1/Controlled、L2/L3 资格。审计确认重复调用实际减少且没有新增策略层，发现问题按影响修复复验。提 PR 前刷新主线并核对 GitHub 实际内容合并状态，不把历史证据追认为新候选通过。

2026-11-05：基线仍为 `a66038c8`。生产修改仅三个文件，19 行增加、25 行删除；包装从 usage 移至 state_transport，目录两端共用一个调用闭包。候选协议包 62 个顶层测试、usage/access 9 个、collector 定向 2 个通过；协议包原 4 项 opt-in 跳过。精确能力、usage scope 创建/删除/撤销证明、目录错 ID/device、丢回执不重连/补发及关闭断言通过。固定 EXE 的公开 CLI 专项和本机/配对远端 agents/models 四次联验通过。审计未发现运行逻辑缺陷，未修改 Orca；未执行完整 L1/Controlled 或 L2/L3，不扩大资格。
