# Plan：Orca 受限文件能力 B4c

日期：2026-11-02。状态：已完成，受控验证及实施后独立审计通过。依据 [Spec](orca-scoped-files-capability.spec.md) 和[长期设计](orca-capability-layer.spec.md)。B4b 确认审计问题先独立修复。

## P0 固定职责

记录 progress.go 原 worktree.show/路径/host/instance、65536/65537、base64 上界 87388 及错误标签；记录 artifact.go 的原 append/384 KiB 和 scope 拒绝。application/runtime_files.go 保留原时机、两次稳定读取及预算，不为抽象修改该业务算法。

## P1 能力与策略迁移

1. 用 scoped_files.go 代替协议 progress.go：自有 FileScope/WorkspaceFileBinding/FileLimits，OpenWorkspaceFiles 保留原生身份核验，ScopedFiles 保留原 read/initialize；路径和限额改由明确值提供，验证有限正值、防整数溢出和相对根范围。
2. orcatransport/progress.go 生成/验证 Run ID 与原 progress/result 路径，提供原业务限额；同一 fileSource 负责现有绑定并消费公共能力，ProgressHost 保持。
3. artifact.go 改为通用 UploadScopedFile，显式 chunkBytes 与 FileScope；新增执行侧 staging wrapper 保留旧范围/诊断/384 KiB，并接入 lifecycle 的 uploadStopArtifact。native 场景、协议测试和 live opt-in 探针显式消费原范围或执行 wrapper，不保留兼容生产副本。
4. 迁移原 Run 相关 IPC 测试至执行侧，保留完整错误/跨 Run/本地替代拒绝断言；原字节/完整性测试保留在协议层并改普通路径。补普通根路径、scope 越界和非法 Run 零连接验证。

## P2 验证

先 rg 固定 TestProgressWireBytesAndBounds、TestResultFile*、TestOrcaLocalIPC*、staging/RuntimeFile/Progress/RuntimeResult/LocalCodexStop/FixedControl 相关根名，按实际受影响集合执行；live opt-in 不执行、不计通过。新 TestOrcaScopedFileCapability/Run 路径测试直接验证职责分层；不将只测私有路径生成器视为生产接线证据。

实际 IPC/WS、application runtime 文件/停止回归按需要使用正常权限。最终 workflow 依赖守卫、生产编译、旧策略删除、持久声明 diff 及 git diff --check 通过；根/子节点/叶例、fail/skip 和迁移前后测试集合记录到验收。

## P3 收口

写验收/原始证据、同步路线和索引并提交实现；实施后独立只读审计，确认修复另 commit。不启用新合同、不提阶段 PR。

职责迁移、实际生产接线、旧实现删除及验证见[验收](../acceptance/orca-scoped-files-capability.md)。
