# Orca 双端原生 Runner 候选 B5 验收

日期：2026-11-02。状态：OSR-01–05 全部通过；B5 已签收并通过实施后独立审计，允许进入 B6。依据 [Spec](../spec/orca-shared-native-runner.spec.md) / [Plan](../spec/orca-shared-native-runner.plan.md)。

## 实现及冻结身份

实施 `a5ac9dd`，独立修复 `77c7ad8`（CLI 路径）、`1334745`（补 count 反例）、`9905fa4`（既有业务反馈入口）、`ac361dd`（活动 Started 及清理边界）。最终生产源码 `ac361ddda57cb3ba143fc90dd1893f5a7af90bf1`，build source_digest `7ef1a5cc631f2fb120adcaea01edb3df6e4b2d42fc0fa5f6e05bb2f8bff4d985`，clean；Runner SHA256 `351da2322015d477d99a08b14cddb60bb3c64ace12ca74e33e40adef4874b7bf`，Windows/amd64，Go 1.24.2。

四场均为同源码、Runner、live driver；各 Manifest 冻结 CLI/转发器/材料/目标和 public 配置，经正式 service start→submit→freeze→Engine→持久化→Run stop→service stop→真实 service verify-stopped。CLI 为 Orca 1.4.242，SHA256 `e7a6fa40a59d237cdb7cf32444df6660da850d1b31e4162634287d91fdfc3cba`。本机 runtime `8a0d209e-094d-4ec3-a923-5cf6c064df67`；明确配对环境 **台式机64G** runtime `64bdec68-d3ed-4216-a894-0cc143ed4653`。注册测试源仓库为各原主机 `C:/Projects/SteerlineTest`，没有改变已安装服务或旧目录。

## 最终真实资格

| 场景 / fixture | 正式 Run | 业务及停止结论 |
| --- | --- | --- |
| 本机开发 `fb72b0846670` | `74b7b6835f4cc31b6f87380871fdbc62` | 原会话反馈、真实检查器、commit/push/[PR #754](https://github.com/erranliu/SteerlineTest/pull/754)，head `117fc74ee3723d568dcdf39548b203f26991a350`；未合并，停止及离线通过 |
| 远端开发 `d7ca2e402fc3` | `6bfa277dbab17ee7bd5c75741d96f8d5` | 原会话反馈、真实检查器、commit/push/[PR #755](https://github.com/erranliu/SteerlineTest/pull/755)，head `ef0917f29e434290f5c029531b499402259eccb7`；未合并，停止及离线通过 |
| 本机活动 `09e7cd2cbd8d` | `fc4b91491e56a120823c189efa348147` | `implement.execute` 已 Started，原 provider/root 仍 working、hook metadata 有效、尚无 final 时正式撤销；停止及离线通过，无 PR |
| 远端活动 `4e43581c5513` | `87eb26472e340ba37edb786ee86aff89` | 同本机活动边界；停止及离线通过，无 PR |

两成功场次的 needs_input 和正式 reply(kind=feedback) 均在 pr_ready 前，保持原 terminal/incarnation/provider；模型在原主机完成工作，driver 独立 GH 查询 repo/base/branch/head/open/unmerged，再取精确 Git 对象、核材料/diff、在私有 QA checkout 运行真实 check.ps1。没有测试脚本代模型发布或合并。

四场各原 close=1，完整原 runtime/worktree local host 清单未截断且原终端缺席，online stopped、writer 退出、离线 resources_settled 一致。活动场次撤销前后 send 数保持 2（bootstrap 和原业务各一次），没有后继输入。所有场次零 managed helper/control/history/扩展调用；原生 session reader 不伪装为 helper history。准备终端独立登记、确认关闭，不混入业务 scope；测试 base/worktree/未合并 PR 保留。

## 受控回归及边界

| 集合 | 实际结果 |
| --- | --- |
| 六包初始全集 | 456 根/1565 子节点/1789 叶；2013 pass、7 opt-in skip、1 fail；唯一失败为旧远端缺冻结 scope 时误回退 helper，已修复并定向复验，不改写初始失败 |
| 最终两 Orca 包全集 | 236 根/1161 子节点/1237 叶；1390 pass、7 opt-in skip、0 fail |
| 随后最终活动 Started 消费者反例 | 2 根/23 子节点/23 叶，全 pass；包括同前缀第二 user、foreign/wire proof、旧 profile、provider/双源 authority/incarnation、partial/截断 final、通知误归属 |
| working/done 受控 IPC 原资源围栏 | 2 根/62 子节点/62 叶，全 pass；唯一完整 user、原 terminal 前后围栏、provider/path/root/stamp/metadata 不变，working 证明不能授 done |
| 正式停止/来源及排队补验 | 3 根/8 子节点/8 叶全 pass；另补双端 count mismatch 两叶、blocked Run 合法首次 stop 两叶全 pass |
| 正式 offline CLI | 1 根/30 子节点/26 叶全 pass：旧 offline_verification/remote_offline_scope 与新双端 native_candidate_offline_scope |
| 测试配置/Manifest 包 | 41 根/191 子节点/215 叶全 pass；CLI slash 正例另补，旧 helper/其他 driver/workflow 和哈希反例保持 |
| 其他 | 依赖守卫、生产构建、live-tag 编译、目录脚本、控制/反馈来源回归通过；零匹配/编译不算真实资格 |

子节点包含分组节点；集合重叠，不能相加。两包全集在最终双源 metadata 小补充前执行，后续 23 叶覆盖最终消费者；blocked 清理补充只改 driver/测试，另验证正式首次 stop。七 skip 为 TestOrcaLocalPipeNativeEvidence、TestOrcaLocalIPCNativeProbe、TestOrcaTerminalQueryLive、TestUsageLocalAcceptanceRepoCleanupLive、TestOrcaNativeStateLiveLifecycle、TestShortInputProbeInspect、TestOrcaWorkspaceStartupLive，均不计真实资格。

候选两端 profile/state codec=9、orca_terminal scope。活动 Started 只由独立完整唯一输入及原 working 事实证明，不使用 send hint；它不赋首个 WorkingAt/root/authority 或流覆盖，done（包括非空 preview）仍须 fresh done-only unique-input fence。旧 4/6/7/8 合同不重释。scope 不证明 Codex 全部后代退出或撤销凭据；实际模型未独立核验，工作区审计 unreported，audited/Pi/DSH/其他 workflow/显式 managed 组合不继承本批资格。

## 失败、修复与历史场次

- `5a64408f87c1` 在 service start 前因测试 Manifest slash 比较失败；无 Run/PR，准备终端确认关闭。`77c7ad8` 独立修复。
- `3e4b30af354f` / Run `03dabd99220327b14179c99ad70c0058` 已有完整 needs_input final，但正式反馈被 Engine 旧 gate 拒绝；`9905fa4` 修复入口与既有 input.Continue 不一致，活动 execute/milestone 仍拒绝、原 idle 资格不变。Run/service/offline 全停止，无 PR，场次仍 failed。
- `9905fa4` 历史成功场次 `0ac84bd0e288` / PR #752、`a5e6b5ae39f0` / PR #753 完整通过且未合并；因后续 Started 变化未平移资格，以上最终四场 fresh 复验。
- 同旧构建活动场次 `02c06ff8e08a` / Run `118c865d9b79f5dea6670e9ba7554000`、`1ca1c9d3e829` / Run `6edf242af2c497003cba50f33e6c3ad9` 错过活动窗口并推进发布；原分支无提交差异，未取得 PR。本机显式 b5-stop 受理；远端显式 stop 明确 stale_revision 后，driver 自身 b5-stop 受理清理。两者 close=1、Run/service/offline 停止，仍 failed。`ac361dd` 修复独立 working Started、done/通知间接授权和 driver 窗口、blocked 清理。
- `674b961e26f4` 仅准备、未执行，因构建更新作废，不计资格。初始控制测试 request_id/command_id 混淆、隔离构造器绕 FIFO、目录计数和沙箱 Git 权限失败均保留；最终测试使用真实构造器/FIFO，不把失败改判通过。

## 独立审计与证据

原独立审计者已关闭 `77c7ad8`、`9905fa4`、`ac361dd` 的代码修复，无剩余确认 P1/P2，并独立复核最终双端真实活动证据通过。同一独立审计者最终确认双端开发与归档全部通过：424 文件逐字节/SHA、归档 SHA、31 组测试统计一致；20 个公开派生副本仅裁除 27 条无关仓库行并添加归档说明，相关操作语义与计数保持；经 GitHub 只读复查 #754/#755 当前仍 open、merged=false、精确 base/head 一致。最终结论 clean，无未关闭 P1/P2，OSR-01–05 可签收，限已声明的 Codex/development/2 与 orca_terminal scope。

[证据归档](evidence/orca-shared-native-runner-20261102/README.md) 含 424 个 UTF-8 文件与逐文件字节/SHA256，可机械复核；commands/source/test-statistics/scene-ledger 分开记录命令、身份、统计和场次。公开副本仅裁除无关注册仓库行，derivations 记录原始文件 SHA256；原完整日志本地保留，所有相关原操作/资源/请求/结果均保留。无 EXE、认证帧或数据库归档。
