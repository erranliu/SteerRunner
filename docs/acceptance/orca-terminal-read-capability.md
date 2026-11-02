# Orca 终端读取能力 B2c 验收

日期：2026-11-02。状态：实现与定向回归通过，待实施后独立审计。依据 [Spec](../spec/orca-terminal-read-capability.spec.md) / [Plan](../spec/orca-terminal-read-capability.plan.md)。本批只迁移原始单页读取，不授予新产品或真实 Runner 资格。

## 1. 职责与删除

protocol/orca/terminal_read.go 定义自有 TerminalRead/TerminalReadPage 和 Client.ReadTerminal。每次使用明确 handle/cursor/正 limit，经原 Client 读取一次；返回原 Result、原始行、source、三个 cursor 及缺失或有值的 truncated/limited，不自动追页或 trim。缺 handle/limit 在调用前拒绝。该文件无执行层依赖。

controlLines 和 readLocalCodexReadiness 已接入，删除两处原 read argv、专属 page 结构和 runtime/handle 重复校验。control 每页仍先 B1 原身份查询、再读取、再原身份查询，保留全部 numeric cursor、boundary、完整行/保留窗口、取消和 trim 算法。readiness 保留从头完整流、唯一原准备回执和所有 helper/配置/workspace/失败校验；未增加身份查询。

基础页与完整 next/latest 字段类型分别记录本地完整性。readiness 不消费可选 next/latest 类型，control 保留原字符串类型要求，包括从头准备读取；重复字段先出现类型错误的事实仍由完整类型解码保留。缺 cursor/bool 不补为可信事实；原 numeric 及用途判据继续由消费者检查。原 CLI read 没有 incarnation 回执，不伪造该字段。

profile/state/target、创建持久格式、发送、原停止合同和控制分页算法未改；未新增重试、fallback、缓存或锁。

## 2. 验证

源码基线 `7da224e` 加本批差异；source_digest 为 `5928a2f40277bc2dfae2e24f44873f54d10d0150924fb4a37b4ec5438e8dc43b`，沿原 Get-SRSourceIdentity 范围。原始 test2json、命令及源码身份见[证据目录](evidence/orca-terminal-read-capability-20261102/README.md)。

| 检查 | 结果 |
| --- | --- |
| 新能力与两端消费范围 | 4 根/64 子例通过；能力 15 子例，原准备流增加 next/latest 坏类型的两种消费反例，B1 control RPC 保持 |
| 原消费者回归 | 22 根/149 子例通过；FixedControl 的累积流/history/workspace/stop/恢复，readiness/原 bootstrap，control/original RPC 及取消 |
| 依赖守卫 | 1 根通过；生产白名单及 git diff --check 通过 |

全部 0 fail/skip；两组有重叠，不合并为独立案例总数。能力测试捕获精确单次 argv/默认 timeout，原 CR、空行、部分末行和所有来源/分页字段保持；缺输入零调用，错身份、坏包、响应丢失和完整游标字段类型不符均不产生可信完整页。wire 不能注入两个本地完整性标记或 envelope runtime。

TestLocalPreparationOriginalStream 用同一坏 next/latest 包证明 readiness 仍按原范围消费，control 拒绝；原每次仅一 read、零额外 create/send 的断言保持。B1 两页四处身份替换及 CLI 队列绕过回归通过，未削减围栏。

未运行真实 Orca 或模型新现场，未制造共享环境故障；B0/B1 现场不转记为本批真实资格。

## 3. 独立审计

实现提交后独立只读审计一次，确认修复用独立 commit 并补受影响验证；完成前 B2c 保持待审计。
