# 零消费者生产入口与孤立夹具清尾：实现再审计

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。结论：重新审计 [Spec](../spec/orphan-entry-cleanup.spec.md) / [Plan](../spec/orphan-entry-cleanup.plan.md) 的最终实现，未发现误删真实消费者、共享夹具、持久格式变化或保留可靠性被破坏的问题，无需修改生产或测试源码。修复一处证据交付配置遗漏：为两个新证据目录补充Git原字节保护，避免检出后SHA失效。按后续用户授权提交并创建PR，不合并、部署或发布。初轮结果与失败仍见[专项验收](orphan-entry-cleanup.md)。

## 1. 审计范围与结论

重新fetch确认main仍为 `111bcd35bdfd7969d4c4575b5fe62204b41486fb`，没有基线漂移或新增消费者。逐项阅读全部201行生产删除及333行孤立夹具删除：仅删17个无入口函数、独占变量/导入与无调用夹具，没有接入新路径或更改保留函数。

从基线源码重新扫描全部工具/测试/生产根的33个删除符号，包括平台后缀、构建标签、配置和脚本文本；没有删除闭包外的真实消费者。publication.commitHash是另一个包中的活跃变量，保持原字节。当前CLI到Lifecycle的入口仍经原saveIntent提交及Invoke的请求校验、派发/只读对账分流，原Receipt.ValidateFor保留；删除的旧start/stop/reply校验器没有接入这些入口。[消费者复核](../../../../acceptance/evidence/orphan-entry-reaudit-20261102/consumer-audit.json)

共享MockAppServerStdio/Main/Connection与AppServerState、StagedBusiness、historyFixtureBinding及其测试，正式native helper的--orca-create-fixture均保留，不能把整个testkit/protocol包判为无用。对18个关键消费者文件逐字比较HEAD原文，并复核全部源文件身份：777项其他源/配置、所有实际测试/断言保持原字节；生产/测试源码与初轮最终验收一致。[源与证据复核](../../../../acceptance/evidence/orphan-entry-reaudit-20261102/evidence-audit.json)

## 2. 格式、耦合与可靠性

重新运行AST工具，13个变更Go文件中的100个保留声明（12个类型）相同，无新增或改写声明；33个删除包符号与初轮清单完全相同。当前重新编码结果与实施前后均为相同145116字节，SHA256 `47d983e8958718ee57ba1a13f00e3504c51ff6268e964062f2ad6ef35c471367`。[声明](../../../../acceptance/evidence/orphan-entry-reaudit-20261102/declarations.json) / [编码](../../../../acceptance/evidence/orphan-entry-reaudit-20261102/wire-comparison.json)

state/v8、SQLite user_version=8、所有持久字段/动作/decoder、profile/owner/native协议、Intents/Snapshot.Operations不变。实际acceptEvent/commitLocked后单确认、原生journal、保存后派发、unknown只读对账、预算不刷新、原结果归属、停止撤销及精确原资源范围全文保持。建议C的Cleanup分支仍未动。变更只有删除，没有新增耦合；移除无调用包装不减少底层目录、profile或资源身份核验。

## 3. 本轮验证与资格边界

| 本轮定向 | 结果 |
| --- | --- |
| application原操作/单确认/预算缺失原结果/停止/服务代次授权 | exit0，15顶层、13子用例通过，0skip |
| 共享appserver/staged夹具、原生异步问答、历史恢复、Lifecycle请求/回执、实际Orca单确认 | exit0，10顶层、18子用例通过，0skip |
| 源/持久/证据完整性 | 原证据SHA清单全部有效；从原JSONL重数447组件/49application通过及8skip，与初轮一致；三个原公开场景result为exit0、3/3且无skip |

本轮共25顶层、31子用例通过，精确名称与命令见[统计](../../../../acceptance/evidence/orphan-entry-reaudit-20261102/test-summary.json) / [命令](../../../../acceptance/evidence/orphan-entry-reaudit-20261102/commands.json)。正常测试权限沿初轮已确认的Git/loopback需求使用，没有修改代码或断言规避限制。审计辅助脚本的错误路径和准备顺序纠正单列在commands，不记作产品测试失败或产品修复。

最终Runner实际文件仍为SHA256 `2606bfdafa88228761ee363d942f2850344b93945593fdaafbe32e6eba1555f1`，source_digest `ea6c2908ede069baee09f0a14b4738e12e03a728dfc0ed1eb4da0b4b80508257`。与初轮源码/EXE身份一致，复核原live编译、正式helper和三个公开场景证据；本轮未重复完整组件或真实模型矩阵，不扩大原资格。

## 4. 确认问题与修复

首次暂存发现根.gitattributes的默认text=auto/eol=lf会归一化新证据目录中的CRLF文本。例如commands.json工作区3813字节，暂存3754字节，原SHA清单会在他人检出后失效。依照仓库已有专项证据约定，只为本批两个目录补充 `-text -whitespace`，重新暂存并逐文件比较暂存blob与原始字节、校验暂存SHA清单。原测试/编码/诊断内容和初轮清单不重写，生产/测试源码不变；修复不需要重跑行为测试。[暂存字节验证](../../../../acceptance/evidence/orphan-entry-reaudit-20261102/git-evidence-bytes.json)
