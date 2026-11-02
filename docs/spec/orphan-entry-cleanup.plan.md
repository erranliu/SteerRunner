# Plan：零消费者生产入口与孤立夹具清尾

日期：2026-11-02。状态：P0–P4完成，见[专项验收](../acceptance/orphan-entry-cleanup.md)。依据 [Spec](orphan-entry-cleanup.spec.md) 和[长期契约](steerrunner.spec.md#长期产品承诺)。

## P0：设计审计与基线

- [x] 核对已合入 #84 的 main 和干净工作树，建立独立分支。
- [x] 对 A 的17个函数、独占变量/导入做全仓引用及合法入口复核；对 B 的类型、所有函数和平台锁定义检查文件外消费者。
- [x] 明确真实 MockAppServer/StagedBusiness/historyFixtureBinding/正式创建探针消费者，保存不变文件清单；建议 C 和持久模型明确排除。
- [x] 审计 Spec/Plan 的范围、格式证明、最小验收和停止条件；保存文档摘要、设计结果、引用与编码基线，确认后实施。

## P1：A——生产删除

1. 按 Spec §2 删除17个零消费者函数、两个整文件及 workflow/execution_facts.go；删除独占 commitHash、ErrRepoUnavailable、未用导入。
2. 保留真实身份/请求/回执验证、目录核验、profile编解码与目标、workspace原资源核验、单确认和原生协议全文。
3. 只格式化受影响文件；逐文件审查，禁止顺带改规则、类型、字段、动作或解码。

## P2：B——孤立夹具删除

1. 仅删除 terminal.go、native_lock_windows.go、native_lock_other.go 三文件。
2. 整个 appserver/staged 共享夹具、Codex组件测试、tests/helpers/native 和正式探针保持原字节；不删有效断言，不增加空替代实现。
3. A的生产收益与B的测试支撑收益分别统计，不把B归入生产净删。

## P3：冻结与验证

- [x] 全仓剩余引用、AST保留声明与共享夹具/原测试逐字比较；前后持久编码和公开能力完全一致。
- [x] OEC-02组件和application定向；记录精确命令、退出码、顶层/子用例与skip/失败。
- [x] native/integration live编译，正式helper与scripts/build.ps1最终构建；源码冻结且产物身份可复核。
- [x] 最终EXE三个公开受控场景，3/3、无skip，核对隔离夹具及资源清理。
- [x] 复核PC-R01–R06/R08/R09的实际代码无修改，确认未引入新入口或耦合。

## P4：记录与交付

- [x] 更新长期实现说明、Spec/Plan状态、专项验收及索引；保留历史记录、失败和未验限制。
- [x] 记录A/B增删、最终源/产物SHA、链接和最小证据；最终git diff检查，无意外源码、测试或临时产物。

停止条件为Spec OEC-00–04全部满足。初轮授权到实施验证；后续用户已明确授权审计后提PR，按该授权提交、推送并创建PR，不合并或部署；设计审计与实施顺序不能倒置。

实际结果：A/B分别净减201/333行，全部为删除；100个保留声明和777项其余源文件不变，持久编码相同。最终496个组件/application顶层通过及3个公开受控场景通过，默认skip/无测试包和首轮沙箱失败/超时按验收单列；无产品或测试源码因首轮环境失败而修改。

后续[实现再审计](../acceptance/orphan-entry-reaudit.md)完成：没有确认的产品缺陷需修复，补充两个证据目录的Git原字节保护并核对暂存blob；实际消费者/共享夹具、保留链路和格式仍保持，本轮25个定向顶层通过。生产/测试源码与初轮最终资格一致，按新增授权提交并提PR。
