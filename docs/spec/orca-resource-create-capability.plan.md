# Plan：Orca 资源创建 B4b

日期：2026-11-02。状态：待实施。依据 [Spec](orca-resource-create-capability.spec.md) 和[长期设计](orca-capability-layer.spec.md)。B4a 审计确认问题先独立修复。

## P0 范围

固定 workspace.go、workspace_initial_terminal.go、control_terminal.go 的创建参数/原生解码，保留 creation_owner.go 全部可靠算法及持久声明。先记录三类旧 argv 和 workspaceCreation JSON 作为独立 oracle。

## P1 实施

1. 新增协议 resource_create.go：自有 WorktreeCreate/TerminalCreate 参数值、纯 argv 编译、WorktreeCreation/TerminalCreation 解码。没有调用者业务命名、标题生成、Run 路径或自动调用/重试。
2. workspaceCreationArgs 委托纯编译，固定原顺序（含 json 后 agent）；共享 decodeWorkspaceCreation 组合原生字段与执行 checkpoint 扩展字段，取代两个重复完整原生解析。保持原 checkpoint 类型和 marshal 编码。
3. initial/control 创建共用 terminal 参数/解码；原 creationCall、结果不确定/错误、runtime/worktree/incarnation 校验、持久绑定与确认查询顺序保持。
4. 删除对应 argv 和原生结构重复；不调整 owner 计划校验、调用次数、排队回调或原期限。

## P2 验证

新增 TestOrcaResourceCreateCapability 验证完全硬编码旧参数、原生解码及 Result 无增权；新增 TestWorkspaceCreationCodecPreservesCheckpoint 比较旧 checkpoint JSON/摘要及兼容扩展的原字段类型拒绝。

运行原 TestWorkspaceNamePrefixCreationAndRecovery、TestWorkspaceCreation*、TestWorkspaceTerminalSurfaceReadiness、TestRemoteRepository*、TestCreationOwner*、TestFixedControl*、TestNativeControl* 及 B2a 的 TestOrcaCLI*；先用 rg 核对实际名称，再统计非零命中。受控夹具需要子进程/互斥锁时使用正常权限，不操作真实 Orca。

workflow TestSRExecutionDependencyBoundary、git diff --check、旧构造删除及原持久声明白名单必须通过。根、子节点和叶例分开，不重叠相加；skip 不计资格。

## P3 收口

按 ORC-01–04 保存验收、命令/源码身份/原日志，更新路线和索引，提交实现；独立只读审计确认修复另提 commit。所有批次完成后才提统一 PR。
