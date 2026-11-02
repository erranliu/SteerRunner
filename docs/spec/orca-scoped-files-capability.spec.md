# Spec：Orca 受限文件能力 B4c

日期：2026-11-02。状态：待实施。对应 [Plan](orca-scoped-files-capability.plan.md) 和[长期设计 §4.3](orca-capability-layer.spec.md#43-最小职责迁移表)。调查基线 `7023035`。

## 目标和边界

把 protocol/orca/progress.go 的 Run ID、progress/result 路径和固定业务限额迁回 orcatransport/progress.go。协议层提供明确环境/runtime/host/worktree/workspace/instance 的 WorkspaceFileBinding、FileScope（根目录与相对路径）、FileLimits 和 ScopedFiles，保留原主机/工作区核验、有限 RPC 读取/初始化及完整性检查。能力可以操作约束内普通相对路径，不理解 Run 或 result 布尔值；执行消费者明确传入旧路径与 65536 字节限额。

artifact.go 改为 UploadScopedFile：明确目标、根目录/相对路径、chunk 字节上限和 source。能力保留有界分块、expectedExecutionHostId=local、原连接取消/关闭及零重试。`.sr-runtime-stage` 的用途/旧严格路径拒绝与 384 KiB 分块策略移至执行侧 UploadWorkspaceArtifact；停止材料和 native 受控场景改用所属消费者或显式原生范围。不能保留旧协议 wrapper 解释 staging 用途。

通用路径验证必须拒绝绝对路径、穿越、反斜线、控制字符、scope 外前缀及 Windows drive/ADS；调用方只可显式缩小范围。原 progress/result 路径和 staging 允许范围保持，不能因迁移扩大业务访问。工作区实例、路径、runtime/host 核验不弱化，禁止调用端本地文件 fallback。上传仍使用原明确 selector，不因本批增加发现或资源绑定账本。

## 接线和删除

ProgressSource/ResultSource 继续统一一份 Run 绑定策略，生成各自原路径并调用 OpenWorkspaceFiles；application 的两次稳定读取和用途时机不变。删除协议 OpenProgressFiles/OpenResultFiles、按 run 拼路径、ProgressLimit 业务常量和 result 用途分支。原业务测试迁至执行侧，协议测试用普通路径验证原生能力，原负例不能因迁移删除。

本批不修改任何持久字段、profile/target 摘要、初始化时机或 stop scope，不扩大上传总预算；流式上传继续由原调用者/上下文限制总量，能力只管理原单块上限。原 RPC 方法、offset/length、base64、append、错误标签及读完整性保持。

## 验收

| ID | 条件 | 证据 |
| --- | --- | --- |
| OSF-01 | 普通受限路径可用，无业务语义 | 原生普通路径读写/分块、参数/原身份、越界零 RPC、限额和完整性负例 |
| OSF-02 | Run/staging 策略在消费侧且只有一份 | 原 Run 路径、非法 Run/host、两 Run 隔离及 staging 旧范围拒绝；旧协议策略删除 |
| OSF-03 | 原连接/读取/停止消费者保持 | IPC/配对传输、原 result 反例、application runtime 文件和 stop/upload 定向回归 |
| OSF-04 | 格式/证据/审计闭环 | 持久白名单、依赖守卫、实际非零命中和归档、独立审计 |

受控原生 IPC/WS 与消费方等价验证不计真实模型、Runner 或新停止资格。
