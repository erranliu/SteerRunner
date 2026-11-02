# Plan：新意图写入边界与展示残留清尾

日期：2026-11-02。状态：P0–P4完成，见[验收](../acceptance/new-intent-boundary-cleanup.md)。依据[Spec](new-intent-boundary-cleanup.spec.md)和[长期契约](steerrunner.spec.md#长期产品承诺)。

## P0：设计与基线

- [x] 在最新main独立分支核对合法新写入、历史/owner/对账/停止入口，盘点标签与共享夹具消费者。
- [x] 审计Spec的A/B删除/保留范围和最小验收，禁止把新动作限制加到Validate/decoder/Invoke。
- [x] 捕获原源码身份、指定持久根/代表编码/公开能力和Spec/Plan摘要；PreparationFacts是瞬时声明，单列授权变化。
- [x] 保存设计通过结论，之后才编辑生产或测试。

## P1：A——新写入构造及空准备分支

1. InputValue仅去掉四类旧动作的新构造case；保留submit_input和全部原保存路径。
2. 只删除PreparationFacts.Cleanup及Next对应分支，不动LifecycleInput/原Request/Receipt/decoder。
3. 增加真实saveIntent/SQLite拒绝且零提交回归；以原保存字节验证解码、校验、只读对账和保留动作构造，不用New造被拒绝旧操作。

## P2：B——展示残留

1. 删除Config.Display，原Config/启动逻辑不改。
2. 仅移除ObservationEvent.Summary/Title和赋值，Kind/Status及其异步测试保持。
3. 原Observe通过精确删除变换比较，journal/拒绝/转发/事件选择/单确认等正文不变；所有原测试及共享夹具不改。

## P3：冻结和验证

- [x] 指定五个修改声明（InputValue、Next、PreparationFacts、ObservationEvent、Observe）和Display删除以外的保留声明完全相同；原测试/共享helper保持原字节。
- [x] 持久根类型/旧动作代表编码/公开能力前后相同，保存原消费者和授权差异复核。
- [x] NIB-01/02新边界回归及NIB-04组件/application定向通过，记录数量和skip/失败。
- [x] live编译、正式Runner及helper构建，三个公开受控场景同产物3/3、无skip；按最新Controlled约定登记选集计时，不扩大资格。
- [x] 复核保存、unknown/原结果/预算/恢复、owner和停止边界无新增耦合或限制。

## P4：收尾

- [x] 更新Spec/Plan、长期实现说明、验收和索引，A/B净删与能力取舍分别说明。
- [x] 保存最终源/产物、命令/退出码/数量/失败/skip及最小证据；为新证据目录配置Git原字节保护，不修改历史证据。
- [x] 检查最终diff、链接、源身份和证据摘要；不自动提交、推送、提PR或部署。

停止条件为Spec NIB-00–05全部满足。没有显著加速收益或全局新资格声明；不因收尾顺带退役持久旧模型或合并双操作表示。后续用户已授权实现再审计后提PR，按该授权提交/推送、创建PR，不合并或部署。

实际结果：A/B净减11/15行，持久类型/编码、90个保留声明及全部原测试/共享夹具保持；四类新写入拒绝/原保存请求对账与8类保留构造回归通过。最终498个组件/application顶层及三个公开受控场景通过；2轮新增夹具失败、8个skip/3个无测试包及计时导入资格边界按验收单列。原请求/owner/恢复/预算/停止规则未改，未提交或部署。

后续[实现再审计](../acceptance/new-intent-boundary-reaudit.md)完成：无确认产品缺陷，快进main66cc0ac的测试/文档更新，任务生产及新增测试不再编辑。27个定向及当前主线最终EXE的三个公开场景通过，既有resultFixture初始化首轮失败与原选集复验单列，编码仍一致；按新增授权提交并提PR。
