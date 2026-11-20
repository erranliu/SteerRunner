# Codex 启动复用原生索引

2026-11-19；基线 `8ebbc717776eb2ad85371060566776c4402773fa`，独立分支 `codex/isolate-appserver-history`。本次只修改、构建和测试源码，不部署，不恢复、停止或重新提交原安装任务。

本页保留原批次及其资格边界。2026-11-20 的后续修复已取得匹配输入的完整 L1 与 Controlled 直接通过结果，见 [#172 失败闭环](runner172-qualification.md)；下方历史失败不覆盖。

## 原因与策略

SteerLine 原 Run `d9f8ce00c99db0f6f5a8ede27dea75e3` 的 Codex 进程在北京时间 11:29:06 创建，11:37:37 才监听。每个新服务指定 `ServerDir/sqlite`，同时保留用户 `CODEX_HOME`，使新索引重复导入该 home 的全部历史。原生回填在 11:37:36 完成，索引包含 8,033 条用户历史；Runner 在 11:31:25 已对账耗尽。证据：`C:/SteerlineDebug/wt-9ed5-refactoring-blocked/diagnostics/native-startup-evidence.json`。

[Codex 0.166.0 启动入口](https://github.com/openai/codex/blob/rust-v0.166.0/codex-rs/app-server/src/lib.rs#L634)在开始监听前初始化原生索引；[状态库入口](https://github.com/openai/codex/blob/rust-v0.166.0/codex-rs/rollout/src/state_db.rs#L87)等待该索引的历史回填。因此不再替每个服务指定新 `sqlite_home`，启动和历史读取都使用原生 home/配置选定的同一个索引，继续由 Codex 处理认证、配置、索引及历史。

此前[并发 SQLite 验收](codex-concurrent-sqlite-isolation.md)发现的空索引并发初始化竞争仍须防止。本次在原生 home 下使用 `.steerrunner-startup.lock`，以 OS 文件锁跨进程串行化 Runner 发起的初始化：本机 prepare_session、远端原生 helper 和 stdio 历史 reader 共用该门。WebSocket initialize 成功或 stdio initialize 完成后释放，不串行化模型回合。等待受原调用 context 和十秒锁等待上限约束；启动异常或 caller 退出释放锁，不删除活动锁文件，不通过重发输入补救。

每个 Run 的原生进程、Job、端点、generation、Session 和停止责任仍独立。恢复已有进程不重新初始化；历史读取继续校验原 home、Session、workspace 和命令。本次没有共享 app-server、复制用户认证、读取或修改原生数据库的生产代码，也没有写回填完成标记、迁移旧资源或延长就绪超时。原生索引本身首次为空时仍由 Codex 回填一次；本锁只协调 Runner 启动，不替其它独立工具管理初始化。

## 验证

证据根为 `C:/SteerlineDebug/wt-9ed5-runner-startup/`。真实探针使用新 home、128 条合成 rollout、独立工作目录及本机拒绝服务；无用户认证、无真实模型成功或 PR。探针测试可只读统计自身合成原生库，产品代码仍只使用公开 app-server 协议。

- 定向启动回归通过：原生参数不再指定新索引，保留 home；锁支持同 home 互斥、不同 home 并行、取消、重复释放、跨进程互斥及进程异常退出释放。历史启动失败后可再次获取锁。
- 完整 Codex owner 包通过：`diagnostics/codex-owner-01.jsonl` / `.stderr.txt`。该定向 owner 不等同完整分级资格；子进程专用 helper 的原 skip 沿原规则保留。
- 最终真实探针通过：`diagnostics/native-startup-04/result.json`。两台并发冷启动约 2.09 秒和 1.45 秒，后继新服务约 0.78 秒；128 条种子和一个实际持久回合仍在同一索引，回填完成时间未变，原 Session/Turn 经公开接口读取且所有自有进程停止。时间仅代表该合成现场，不冒充原 8,033 条用户历史的重跑。
- 自查调整失败释放顺序：回执保存失败时原启动清理前仍持有锁，之后释放。最终补验 `diagnostics/cleanup-order-01.jsonl` 通过；该差异不改变已验的成功启动、回填及历史读取路径，复用上述原生探针和 owner 的其余覆盖。
- 受控 foundation/content 选测：`test-results/controlled-01/summary.json` 的 13 项中 12 项通过、`TestSRStandalonePackage` 失败，整体不通过。未修改基线的独立 `test-results/baseline-standalone-01/result.json` 复现同一断言“rejected submit persisted an accepted receipt”；该用例在缺失原生 EXE 的准入阶段已结束，不经过本次更改的 app-server 启动路径。原断言未改，原失败保留。
- 最终固定 EXE 补验 `TestSRNativeUnknownAndStop` 通过，`test-results/controlled-final-01/result.json` 为 exit_code=0、scenario_count=1，原 integration.log 无 skip；source-identity.json 的 runner_sha256 与最终构建一致。受控首轮冻结构建早于清理 defer 顺序调整，其未受影响的内容/恢复覆盖单独复用，不伪装成最终构建完整受控通过。
- L1 原批次记录了两个有界计时失败：端点持久读取重试超过两秒、CLI 测试的一秒期限在启动屏障前耗尽。未修改基线的 `diagnostics/baseline-timing-01.jsonl` 两项均通过；这两个包和对应集成断言无源码差异。保留原批次失败，另行验证最终源码，不放宽超时或改报完整 L1 通过。
- 最终源码的同两项原计时断言在 `diagnostics/final-timing-01.jsonl` 单独运行均通过；它只证明该次补验，不替换原整批失败。新增验收文档的本地链接与 `git diff --check` 通过。
- 正式 L1 已结束：`test-results/L1-01/summary.json` 为 failed，历时约 2,029 秒。Go 的 38 包均有终态，810 个顶层测试通过，两个失败根就是上述 host/textanalysis 计时项；应用层整包通过。框架检查、21 项 terminal extension、native driver 编译及零匹配保护通过，原 helper/live opt-in skip 由报告逐项保留。
- L1 的输入身份在初次启动后因本次收尾修改发生变化，`prerequisites.json` 另将 l1-core 标为 unavailable（`Prerequisite input identity is unsupported or changed`）；因此不能将整批作为最终版本的完整前置资格。Go 阶段开始前最终生产代码已固定；最终改动另由清理顺序回归和相同 SHA256 的冻结 EXE 停止专项覆盖。没有重写旧报告或宣称完整 L1 通过。
- 真实探针首轮 `native-startup-01` 的后续校验因夹具未传 workspace 失败；夹具提前 Close 后清理检查缺证据，随后按原 PID 确认两个自有进程均已退出。第二、三轮 `native-startup-02/03` 因错误假定未持久化的空线程和无预览的种子会出现在原生列表而失败，保留原日志。原库只读统计已确认 128 条种子实际存在。第三轮起将模型端点限定为 loopback 拒绝服务。

最终构建：`artifacts/steerrunner-index-reuse.exe`，版本 `0.2.0-dev.4`，source_dirty=true，生产源码摘要 `41911f53520786a67d85da4ced92a7941a17796d8334a9f44e0970f7b5bde20e`，SHA256 `71fbe6336366e87181b5a735dd2ea201ecc3542189889339995b7dcdacd1fa6e`；原 `*.build.json` 保留构建身份。未复制到安装目录。

本次选定的验证与失败对照均已完成，原始报告和构建保留。完整 L1/Controlled 未通过，不声明真实模型 L2/L3、远端实际执行或部署资格。原安装任务及安装二进制未操作。

## 后续 SteerLine 真实完整链

用户追加要求后，2026-11-19 使用上述最终 SHA256 的 EXE，运行 SteerLine 正式 L4 `refactoring.local-codex`，原始报告为 `C:/SteerlineDebug/wt-9ed5-refactoring-live/test-results/L4/local-codex-01/summary.json`：passed=true、cleanup_complete=true，1 项通过、0 跳过，总计约 581.51 秒。SteerLine 源码基线为 `3a1777a6bbf5d4b245be2ca12aa24c4b2af4e905` 加本轮 blocked 展示修复，原报告保留完整脏文件身份。

使用独立 home、既有登录的隔离认证副本、真实 Codex 和 GitHub 测试仓库：两条维护任务 Session 不同并重叠执行，选题、实施、验收、各自 draft PR 和独立评审完成；推动专属 base 形成两文件真实冲突，续作复用原 worktree 更新原 PR，登记新 head/base 和第二份评审，PR 正文保持。两版代码行为检查通过。第二份评审因收到的材料缺少测试运行证据给出 insufficient_evidence，不宣称建议合入。

本轮测试 PR #851/#852 均关闭，自有 base/head 分支和测试资源清理完成，Runner 清理 confirmed=true。原安装任务及安装二进制仍未操作。该结果补充真实模型和真实 PR 链覆盖，不改报先前完整 L1/Controlled 的失败，也不声明部署资格。
