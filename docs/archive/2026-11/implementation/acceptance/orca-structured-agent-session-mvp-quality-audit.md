# Orca 结构化 execute/1：提 PR 前质量审计

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-03。按用户要求自审、修复及复验，未使用子代理。范围绑定原 [Spec](../spec/orca-structured-agent-session.spec.md) / [Plan](../spec/orca-structured-agent-session.plan.md)，保留[实施验收](orca-structured-agent-session-mvp.md)的原始真机记录。

## 确认问题与修复

| 级别 | 问题 | 修复与验证 |
| --- | --- | --- |
| P1 | 结构化 observer 的审批诊断事件没有 EventStatus 和原 session/turn/source/event ID，Engine 会按既有门禁拒收，用户无法得到预期阻断原因；后续 history 不完整时也会丢失这些已核验标识 | 复用原 EventStatus 与随机事件 ID，完整绑定原会话/回合；不完整读取保留已核验原标识，不重新认领 input/liveness。两种诊断反例先失败、修后通过，Engine 既有诊断持久化测试继续验证事务与去重 |
| P2 | item 补全文缺少 sequenceIndex 核对；跨页重复项未核对排序位置及 turnScope，同正文/同 revision 的变化可能被覆盖；不同项同位置时聚合顺序不确定 | 补齐这些字段的一致性检查，拒绝有歧义的顺序；4 个新增反例在旧实现均复现。真实正常、长输出、取消三份 public history 用修后 reader 回放，正常/长结果的字节数和固定 SHA-256 一致 |
| P2 | 当前 CLI 指南仍声明 DSH 工具及旧场景数量，更新到 main 后停止说明也残留 Pi/DSH 表述 | 删除当前 DSH 支持说明，数量指向唯一 JSON 目录，明确新 session scope 的停止证据限制；带日期的历史验收保持 |
| P2 | 旧停止契约夹具的 200ms 总预算在真实 SQLite/CLI 排队前耗尽，私有 cache 仍不能消除负载下误失败 | 本轮复现 `/missing-host` 在 close 前返回 queue ended；该组预算改为 5s，副作用计数、未知结算、丢回执、重启/反馈拒绝全部断言保留。专门的排队取消测试不改预算/行为；没有放宽生产超时或停止资格 |

只改变既有 lifecycle/protocol 的实际消费者，不添加插件注册层、通用会话平台、第二存储、重试框架、provider 直连或 terminal fallback。权限/model 原子接线、未知 mutation 零重放、原 worktree/shell 绑定与结果/停止分离已复核。DSH 专用代码已删除，唯一旧 metadata 字段仍用于既有 Pi helper 的严格解码兼容。

## 更新基线与证据

分支更新到 main `69a4d4b`，包含原 terminal 已确认停止证据的离线保留修复；新 `orca_agent_session` 分支仍不给出完整结算证明。没有修改 Orca 或替换已有服务，没有再次创建业务会话或发送正文。

修后构建及检查见[本轮证据](../../../../acceptance/evidence/orca-structured-agent-session-mvp-audit-20261103/manifest.json)。新构建只对原三个 Run 执行离线 verify-stopped，三个资源和 aggregate 仍为 false，并有“未核实”诊断；不构造 Engine、启动观察或补发 close。原三场端到端派发来自实施记录中的审计前 EXE；本轮通过修后组件回归和原公开 journal 回放验证改动，不能将旧 EXE 真机记录重标为新构建全流程资格。

首次默认 sandbox 的相关全包/Node 扩展测试遇到 Access denied、命名管道不可达和临时文件 rename EPERM，改用主机权限仅运行测试夹具后复验；原失败日志保留。200ms 夹具在主机权限下的真实失败另行保留，未解释为 sandbox 失败。

最终测试结果在证据摘要中逐项记录：Orca 适配器/协议全包、contracts/resources/usage/livetest 全包、Engine 5 组停止/冻结/诊断测试，以及 Pi 扩展 21 项通过。停止组保留 18 个生命周期子例和原排队取消子例；真实 journal 回放同时检查 7,027 / 13,927 UTF-8 字节与固定 SHA-256。Go 构建、gofmt、diff 空白检查及证据哈希/敏感字段检查完成。本轮没有重跑官方完整 L1/Controlled 或新的 Pi/远端模型场景。原广泛回归的失败及局部复验边界仍按实施记录保留。

## 收口

上述确认问题已修复；本轮未发现需要新增框架的缺口。完整资源停止仍未核实，provider final-only 和 audited 实际模型证据、其它 Codex/Pi 组合迁移仍不在本 PR 范围。原临时 EXE、Runner DB 与未核实资源线索留在本机，不进入提交。
