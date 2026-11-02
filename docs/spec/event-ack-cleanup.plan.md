# Plan：单事件确认与旧环境分支收口

日期：2026-11-02。状态：P0–P4已完成，见[验收](../acceptance/event-ack-cleanup.md)。依据 [Spec](event-ack-cleanup.spec.md) 和[长期契约](steerrunner.spec.md#长期产品承诺)。

## P0：设计与基线

- [x] 核对 main/工作树，逐个盘点 Processed、通用字段、trace、底层问答及 B 的实际消费者/合法入口，包括 live 与共享夹具。
- [x] 审计持久事实、重复/可选消息、独立 owner 的三类确认语义；唯一回复仍在原处理返回点，不新增持久化义务。
- [x] 捕获持久编码/类型、公开能力/资源摘要、有效实现的声明基线，生产编辑前保存设计审计与文档摘要。

## P1：A——单确认与通用事件字段

1. LifecycleEvent 只保留 PublicMessage/Observation/Fact/Implementation/Persisted 五字段，删除 Candidate clone；不删底层 ObservationEvent 字段。
2. worker 和 owner 仅删除 Processed 回复；Orca send 仅保留原 Persisted 分配、等待及错误/取消处理，去掉标签生成；Codex 去掉通用字段透传。
3. 全部实际 Processed 夹具删除第二回复，原业务/恢复/发送次数断言不动；短输入 trace 按事实重建相同分类，不删除真实诊断消费者。
4. 新增真实 worker/SQLite 确认边界回归与 Orca 单确认/错误/取消回归；A 生产增删独立统计。

## P2：B——旧环境和零消费者声明

1. 删除 BindingSnapshot 两个瞬时字段及旧 EnvironmentBinding/PolicyBinding、仅为它们服务的合并分支；保留有效 MergeReceipt 核验和错误返回规则。
2. 删 Spec 五项零消费者声明；保留真实同名类及版本/查询限制、所有持久字段/动作/decoder。
3. B 单独记账，不合并操作账本、不改变 prepare/qualify 或真实资源权限/停止。

## P3：冻结和验证

- [x] 复核差异、候选引用和保留声明；持久编码/能力逐字相等，trace 分类保持，确认无误删测试/共享 helper。
- [x] EAC-01/02 新行为回归、整组组件、application 指定定向；保存精确名称、数量、skip/失败。
- [x] 正式 native/integration live 编译；脚本构建最终 Runner 并记录稳定源/产物 SHA。
- [x] 同一最终 EXE 的三项公开受控场景，确认3/3、无 skip 和夹具清理。
- [x] 对照长期契约再次审计保存失败、unknown、原资源/结果/预算、owner journal、取消和停止边界。

## P4：文档与交付

- [x] 更新长期实现说明、Spec/Plan 状态、专项验收/索引，归档最小证据和失败，记录 A/B 真实收益。
- [x] 最后检查源码/产物身份、diff/链接/摘要，报告资格范围；初轮不自动提交/推送、创建 PR 或部署。后续用户已授权再审计后提 PR，按该授权交付。

停止条件：EAC-00–06 全部满足；单确认在原语义边界生效，合法消费者与保留可靠性不变，持久编码未变。任何不等价或新消费者先修订设计再继续，不扩大为通用框架。

实际结果：A/B生产净减38/67行；两项新回归及完整组件/application共492顶层通过，最终三个公开场景3/3。原持久格式和确认语义保持，9处夹具有效断言不删，trace/底层问答消费者保留。新 observer 回归首轮使用零快照失败，改用合法原准备夹具后通过；失败归档，未修改生产身份校验。未提交/推送、创建PR或部署。

后续[实现再审计](../acceptance/event-ack-reaudit.md)完成：无新增确认缺陷，精确原回复边界、全部实际夹具、格式及保留承诺复核通过，本次33个定向顶层通过，生产/测试及原证据不变；按新增授权提交并提 PR，不合并或部署。
