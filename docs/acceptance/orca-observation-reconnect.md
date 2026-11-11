# Orca 原轮次观察重连

已确认启动的 Orca 轮次丢失观察连接时，Runner 保留原 Run、Attempt、Session 和 Turn，按 1、2、4、8 秒退避，后续间隔 15 秒。自首次观察失败起最多重试 2 分钟，单次读取最多 30 秒，且受剩余预算约束。重连时间及截止时间随 Run 保存。重连期间公开状态为 `running / observation_reconnecting`。

重连复用原生命周期只读观察，继续前核验原会话、原输入和原轮次。核验成功后接回原观察，已确认结束则交给既有结果处理。明确的身份冲突或重试耗尽进入人工恢复。停止可取消在途重连查询，撤销后的旧结果不能接回观察。

## 回归范围

`TestSROrcaObservationReconnectsOnlyTheConfirmedTurn` 覆盖观察超时、持久退避、同一原轮次恢复、身份冲突、证据不足、预算耗尽及关闭保护。夹具从原 Run 的读接口取证，不调度原生写入。此验收范围为受控自动化，未使用在运任务进行故障注入。

## 历史向前翻页

首个 tail 页仍拒绝 `hasNewer=true`。使用原 cursor 读取更旧页时，`hasNewer=true` 表示此前已读取的较新页。会话、epoch、删除项、cursor 前进及重复项一致性检查继续生效。

`TestStructuredObservationRecoversPagedCompletedTurn` 用两个历史页核实原轮次和完整最终回复，只读取两页，无原生创建或关闭操作。`TestStructuredHistoryFailsClosed` 保留 tail 缺页和历史冲突的反例。
