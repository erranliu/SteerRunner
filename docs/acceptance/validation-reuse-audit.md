# 验证复用实现审计与 PR 验证

日期：2026-10-31。状态：实现审计修复完成，最终完整前置及真实 L2 待验证。前序实现证据见[原验收](validation-reuse.md)，不改写原始结果；契约见 [Spec](../spec/validation-reuse.spec.md)。

## 确认问题与修正

| 问题 | 影响与修正 | 新验证 |
| --- | --- | --- |
| P1：只拒绝部分扩展名的 ignored 输入 | Git 忽略的 txt/bin 等数据可能参与测试或 embed，却不进入摘要；现在拒绝输入范围内任意 ignored 文件，native-only 排除仍按阶段区分。目录/gitlink/reparse 输入也拒绝，不猜测外部依赖 | 实际 ignored txt/bin、隔离 gitlink 反例；默认执行仍不受不可复用状态阻断 |
| P1：子进程创建时间二次解析丢失时区 | PowerShell 可将 JSON ISO 时间直接转换为 DateTime，再 Parse 会通过无时区字符串转换，在非 UTC 主机误判活跃 PID 已复用；改为保留 DateTime 类型直接转换 UTC | 真实仍存活子进程先暴露半个 JSON，退出后结果才允许读取；原判断在此测试实际失败，修复后通过 |
| P2：先解析 result，再等 harness 退出 | result.json 的可见性不代表写入或进程结束；现在沿原聚合期限先确认子进程退出，再解析结果，不改变断言、场景或超时预算 | 实际半写结果发布及超期拒绝合同；完整 Controlled 重新验证 |
| P2：构建/运行设置遗漏 | GOAMD64/GODEBUG 改变未改变输入身份；现记录这两项；不支持非空 NODE_OPTIONS 的预加载/附加设置，原默认入口继续允许原环境执行 | 真实 Go 环境切换摘要差异、Node 设置拒绝 |
| P2：合同入口误把不可复用当作默认失败 | 仓库含不支持复用的目录输入时，直接调用严格 snapshot 的合同冒烟会阻断原默认测试；改用默认入口相同的 unavailable 回退，并在普通隔离仓库单独验证严格 snapshot | 不支持输入的默认回退合同；正常严格 snapshot 冒烟仍保留 |
| P1：原始证据 Git 换行转换 | 新证据目录继承 text/eol=lf，原始 manifest 经索引过滤后 Git 对象 SHA 与原始字节不同，破坏归档 SHA；增加目录 -text/-whitespace 保护 | 暂存和提交后的 blob SHA 与原始内容一致；资格证据 SHA 全量重算 |

不修改生产 Go、catalog/registry、integration/native 测试主体、权限/发布/停止逻辑、循环、skip allowlist 或场景期限。L3/L4 不开放复用，原可选字段缺省路径保持。

## 合同检查

75 项通过，源记录 `.evidence/validation-reuse-audit-contract-03/prerequisites/verified.json`。包括前序 66 项、任意 ignored 数据、目录输入及默认回退、构建设置、Node 附加设置、真实子进程结果发布与原 deadline。原失败合同尝试保留在 audit-contract-01，不追认通过。

## 最终验证

待从已提交、冻结的源码执行默认 `test.ps1 -Level L1,Controlled`，生成新的直接阶段证明，再经正式 L2 复用入口取得新现场结果。旧 source manifest 因实现及摘要范围变化失效，不回填或重签。原来一次创建身份场景失败及后续全量通过仍按原验收保留。
