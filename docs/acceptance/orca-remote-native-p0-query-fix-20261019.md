# P0 补验：原生查询完整覆盖与参数契约收敛

日期：2026-10-19。结论：**查询漏项已修复，P0 按收敛契约通过。** 本次只修改查询及对应测试，不开始远端 helper 删除实施，不部署或调用远端 helper。首轮启动/短回合/反馈/发布/连接重开/活动停止及清理证据见[首轮报告](orca-remote-native-p0-20261019.md)，其原始失败不改写。

## 契约与最小修复

用户已明确：参数只承诺按配置传递，不新增实际生效核验。Spec/Plan 对 model、reasoning_effort、access_mode 只要求冻结值正确校验、编译并传递，不能被额外参数覆盖；不新增有效参数回读、相关字段、准入门或 OS 权限核验。原生观察恰有 model 是附加事实，不把请求 high 或 full_access 标为已权威核验实际生效。

Runner 生产运行代码只改 `internal/execution/adapters/protocol/orca/state_subscription.go` 的 `QueryWorktreeAgents`：

1. `worktree.ps` 当前只支持 limit，没有分页游标；保留首次 limit=100。
2. 根据返回 totalCount 扩展 limit，复用同一原 runtime 连接和现有 40 秒总预算，最多查询三次以容纳有界增长。
3. 只采用一份计数一致且 `truncated=false` 的快照，不合并多个时刻的列表。元数据缺失、计数矛盾、服务端持续截断或持续增长时返回未核实，不能把未覆盖范围当资源缺席。

没有新增执行模式、协议字段、状态表、后台服务或远端程序。`QueryOriginalAgent` 的原 worktree/pane/provider 绑定、NativeProof 完成判据和终端停止判据保持不变。共享查询对 Orca local/remote 均生效；对应本机 IPC 夹具补齐真实协议已有的 totalCount/truncated 字段。

## 真实远端补验

同一 environment `e68f4edf-8a83-42c2-a9da-6d404d3aa3d8`、runtime `f8616747-34af-4543-8487-ec6b5d8de4ff`。本机探针直接调用修复后的生产 `QueryWorktreeAgents`，通过原生 Orca 连接读取，未复制 RPC 或放宽解析。探针仅在本机编译和执行，没有上传远端。

| 读取 | 实际结果 |
| --- | --- |
| 对照：Orca CLI 原 limit=100 | 返回 100 条、totalCount=272、truncated=true |
| 修复后的原生查询 | 返回 273 条、totalCount=273、truncated=false |
| 相较此前未返回的工作区 | 173 条进入完整结果，其中有 3 条包含代理记录 |
| 远端变更/业务输入/helper 调用 | 全部为 0；不创建工作区、终端、模型回合、PR 或引用 |

两次读取不是同一时刻，外部并发活动使总数从 272 增至 273；不把这一条差异说成同快照丢失。完整性依据修复后该份快照自身的计数与截断字段。

[结果](evidence/orca-remote-native-p0-query-fix-20261019/coverage-result.json)保存计数、原 runtime、摘要及一个此前遗漏的工作区例子；未发布无关工作区正文。[来源证明](evidence/orca-remote-native-p0-query-fix-20261019/provenance.json)记录 dirty 工作副本、生产改动/测试源码 SHA256、本机探针 EXE 摘要及原始文件路径/摘要，不把未提交修改冒充 HEAD 已包含的代码。[探针源码](evidence/orca-remote-native-p0-query-fix-20261019/coverage-probe.go.txt)为可复核本机工具，不是部署组件。

本轮针对查询缺陷补验，不重复消耗模型或创建发布现场；首轮原生回合和发布结果沿用并单独标明来源。真实远端补验确认完整快照覆盖，目标在第 100 条之后的原代理查询及身份选择由下面的协议回归覆盖。

## 回归与首次失败

新增 `TestOriginalAgentQueryCoversLimitedSnapshot` 的六个场景：目标在 151 行、清单从 151 增至 152、服务端仍截断、计数矛盾、完整性元数据缺失、持续增长。前两项通过实际 QueryOriginalAgent 取回原目标；其他工作区即使拥有同 pane key 也不得被选中。后四项保持未核实，不返回资源缺席；持续增长不超过三次查询。

定向命令：

```powershell
go test ./internal/execution/adapters/protocol/orca ./internal/execution/adapters/orcatransport ./internal/execution/agentlaunch -count=1 -timeout 120s
```

首次运行协议与参数编译包通过，orcatransport 的旧 IPC 测试夹具只返回空对象，因缺完整性字段而失败。保留[首次日志](evidence/orca-remote-native-p0-query-fix-20261019/go-test.log)。仅给该夹具补齐真实协议已提供的字段后，[三个包均通过](evidence/orca-remote-native-p0-query-fix-20261019/go-test-after-fixture.log)，没有为夹具降低生产完整性要求。现有参数编译测试覆盖 high/xhigh、权限参数及拒绝覆盖；不引入实际生效核验。

统一回归：`./scripts/test.ps1 -Level L1 -OutDir <本轮证据目录>/l1`。首次 L1 另外发现既有 `TestAuxiliaryCloseReconcilesExactIdentityAndCompleteInventory/already-absent` 夹具将空终端集合编码为 null；生产要求真实协议的非 null 空数组。仅将该夹具的 `nil` 改为 `[]any{}`，不放宽生产停止规则；[该测试的七个场景已重新通过](evidence/orca-remote-native-p0-query-fix-20261019/inventory-fixture-recheck.log)。L1 还暴露测试入口漏登记既有显式启用探针 `TestOrcaStoppedNativeProbe`：脚本本就禁止其 live opt-in 环境变量进入 L1，正常 skip 却未在允许名单。只补该精确名称，继续禁止自动运行该远端探针，不扩大任意 skip 或放宽失败处理。最终用修正后的入口完整复跑，[L1 通过](evidence/orca-remote-native-p0-query-fix-20261019/l1-final-summary.json)：310 项顶层 Go 测试、28 项扩展测试、驱动编译与零匹配保护均通过。原生 opt-in/helper 子进程辅助测试的 8 项预期 skip 单列，不当作真实原生通过。此前[首次失败](evidence/orca-remote-native-p0-query-fix-20261019/l1-summary.json)及[仅入口 skip 名单阻断的一轮](evidence/orca-remote-native-p0-query-fix-20261019/l1-after-fixture-summary.json)均保留。

## 当前边界

查询缺口与参数门槛已结清，P0 可行性门槛满足。认证继续依靠远端已有登录，停止继续限 Orca 原终端；完整环境 Check、单仓库凭据撤销、全 OS 写者树保证仍按 Spec 收缩，不因本轮查询通过而恢复这些保证。

P1–P5 的 helper 接线删除、配置/旧记录处理、消费者改动及最终 Runner 验收仍待实施。此次不是完整产品交付，也没有改安装目录、原故障 Run 或远端共享配置。补验没有创建远端资源，无新增清理待办。
