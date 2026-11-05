# PR #107 合入前复验

审计发现旧 Orca 终端 Run 原先要等到创建执行端口才会被拒绝。此时新服务可能已经清除原停机证据并打开可写状态。修复后，`service start` 在写入启动意图前做只读检查，服务取得独占 writer 锁后、清除旧停机证据前再次检查。含旧终端 Run 的目录返回 `legacy_orca_state`。活动和已停两种夹具均验证原数据库及停机证明逐字节保留，并可离线查询。空目录、已有结构化 Orca Run 和直连 Codex Run 仍可通过启动检查。

新 Orca Run 在受理时冻结为 `orca_agent_session_v1`。旧 helper 命令入口已删除，旧终端记录的只读停止验证仍可读取。旧部署继续使用原 Runner 排空，随后用新目录启动。

正式运行证据沿用[退役验收](orca-helper-retirement.md)。抽查原回执确认双端 `execute/1` 在正常完成和活动停止场景都封存原 `result-file/v1`，二次启动后文件版本及摘要一致。双端 `development/2` 原 Run 均已停止、撤销恢复资格并保留原 `result-file/v1`。本机 audited 发布前后的单次快照版本、观测时间和摘要一致。

合入主线 #106 后，用干净源码 `9d95786e6d026cf7462d73e76b2062f76eeff4f5` 构建复验 EXE，SHA256 为 `8761361519f9c46ae032a23442b632a264049dcf7ea53d2ff488fe4061f008d4`。通过 SteerLine `aa08dfee` 的生产 `VerifyUpgradeDrained` 入口，分别用原写入 EXE 和修复 EXE 冷读以下六个原目录。该入口与原验收所用 `6dfb5286` 的 `internal/runnerclient` 内容一致。

| 原目录 | 复验 |
| --- | --- |
| 本机及配对远端 Codex `execute/1` | 每端各两条原 Run，含正常完成及活动停止 |
| 本机及配对远端 Codex `development/2` | 每端各一条原双轮 Run |
| 本机 Codex audited | 一条三轮发布 Run 和一条原续轮活动停止 Run，分属两个目录 |

八条原 Run 的停止证明范围均为 `orca_agent_session`，六个目录的升级排空门禁均通过。复验期间只读原目录。该记录覆盖启动门禁的受控测试和基于真实 Run 的专项排空复验，双端业务回合及 Pi 执行沿用原正式运行证据。

合并 #106 后，host、CLI、公开配置和 usage 采集包，以及应用层 Orca 冻结、结构化会话、audited 与文件结果定向测试通过。CLI 集成定向通过 `TestSRStandalonePackage`、`TestSRServiceOwnership`、`TestSRPublicationIdentity`、`TestSRRestartPreservesAuthorization` 和 `TestSRResultFile`。

首轮扩展应用测试触发 10 分钟命令总超时，停在配对 worker 的 `receipt-commit-failed` 子例。主线 `fddffa7a` 的同名夹具会在受理阶段要求解析真实配对主机，无法进入受控回执断言。候选以冻结的历史终端记录构建夹具，独立复跑 `TestSharedNative` 组通过，耗时 437.306 秒。18 个停机回执用例全部通过，含原超时所在子例。
