# Orca 受限文件能力 B4c 验收

日期：2026-11-02。状态：已完成，受控验证及实施后独立审计通过。依据 [Spec](../spec/orca-scoped-files-capability.spec.md) / [Plan](../spec/orca-scoped-files-capability.plan.md)。

## 能力与策略

协议 scoped_files.go 以 WorkspaceFileBinding、FileScope、FileLimits 提供 OpenWorkspaceFiles/ScopedFiles。保留原 runtime/host/worktree/workspace/instance 核验、初始化/单次完整读取及错误标签，接收显式根范围和字节预算，不解释 Run。UploadScopedFile 接收显式范围、原 selector 和单块限额，保留原 append/base64/expectedExecutionHostId=local、上下文和连接关闭，不重试。单块最多沿原 384 KiB 上限，不新增上传总量规则。

orcatransport/progress.go 保留唯一 Run/执行绑定策略，验证原 Run 格式，生成原 progress/result 路径并传 65536 限额；application 初始化时机、双次稳定读和总期限未改。执行侧 artifact.go 保留原 staging 范围/严格 `..` 拒绝、诊断和分块策略，lifecycle 的实际停止上传已接入。native 场景改用该消费方，协议现场探针显式提供原范围。

旧协议 progress.go/OpenProgressFiles/OpenResultFiles/ProgressLimit、result 布尔用途和 staging wrapper 已删除。原 result IPC 10 个场景迁至执行侧，保留原路径、两 Run 隔离、原实例/主机错误、断线/阻塞/超限和调用端本地替代拒绝。原字节/完整性测试改普通路径；原 RPC selector/独立用途测试现在通过真实 ProgressSource/ResultSource 接线，额外核对两种路径、写入 host、65537 读取长度及非法 Run/host 零连接。

范围校验拒绝绝对路径、前缀越界、穿越、反斜线/drive/ADS、控制字符和 Windows 尾随空格/点规范化变体。非规范 staging 路径可能比旧实现更早拒绝（例如双斜线或尾随点）；不扩大原业务访问范围。持久类型、Profile/Target 摘要、停止 scope 和支持合同不变。

## 构建与证据

源码基线 `e340bff` 加本批差异；删除/移动路径先暂存，source_digest `6efb1df728af18f7ff66a6f2da15ba0cebbafed85a16f337aaedef1eef804938`。实际命令、源码身份和日志见[证据](evidence/orca-scoped-files-capability-20261102/README.md)。

| 验收 | 直接结果 |
| --- | --- |
| OSF-01–02 | 最终原生/真实消费方/原 result 5 根/24 子节点、26 叶例，0 fail/skip；普通路径读写/分块、范围零连接、写超限零 RPC、读完整性、Run/staging 策略通过 |
| OSF-03 | 原 IPC、停止/control、application progress/result/runtime 52 根/131 子节点、159 叶例，0 fail/skip；配对 WS/原状态传输另 2 根/9 子节点、10 叶例通过 |
| OSF-04 | workflow 依赖守卫 1 根、两个生产入口构建、live-tag 测试仅编译、格式/旧策略删除/持久白名单及 git diff --check 通过；独立审计通过 |

子节点含分组节点，叶例另计；回归与新能力集合有重叠，不相加。52 根回归在最后增加路径规范化拒绝前执行；最终 5 根测试覆盖新增拒绝和两种真实 Run 路径。WS 覆盖共用原传输，不记为真实远端文件或模型资格。

初次新增测试因 fixture 把 map 当结构访问而编译失败，修正后通过；失败包和日志保留。最初无 live tag 的 tests/native 入口无可用文件，随后带 tag 发现 B3 三处旧类型调用，已独立修复 `e340bff`；`-run '^$'` 仍触发其 TestMain 的 evidence-dir 门槛，因此最终使用 `go test -c -tags live` 仅编译通过。没有启动 live 场景，没有真实 Orca/模型资源或 B5 资格。

## 独立审计

固定 `5cd7c79` 相对 `e340bff` 独立只读审计通过，未发现确认 P0/P1/P2。审计者核对边界、范围、原 10 个 result 反例、14 文件/归档/701 文件源码摘要与统计，并独立复跑能力/IPC/WS、application/stop、守卫、生产构建及 live-tag 仅编译；B3 单独修复的三处映射亦已复核。未操作真实 Orca/模型，无代码修复需求。
