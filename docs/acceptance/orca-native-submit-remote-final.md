# Orca 远端新目标原生输入补验

2026-10-30。用户指定现有配对环境 `192.168.1.151`。远端三个全新独立原生会话 **3/3 通过**，加上[本机批次](orca-native-submit-local-final.md)的 3/3，双端要求均完成。本页保留三份准备失败，不改写此前失败。

## 原目标与实施范围

现有 environment ID 为 `9cf377a1-42de-4f2b-8874-70f63a7bbab2`，endpoint `ws://192.168.1.151:6768`，原 runtime `64108a37-610d-478c-a565-194c376217c5`，Orca 1.4.245，具有 `terminal.prompt-delivery.v1`。它不是旧 `台式机64G` 的配对身份；没有将旧未知请求改投新 runtime，也未修改配对、全局配置或远端安装。

本轮生产增量为空。只补正式 `scripts/test-native-input.ps1 -Mode long-input` 的远端测试适配：独立随机 Run/Attempt/资源代次，使用既有冻结规则和 operation.New 保存原输入/intent/receipt，create/show/应急清理绑定最初 runtime/terminal/incarnation/worktree，正式清理调用现有 StopLifecycle 和 verifyStopped。正式 stop 一经尝试，未知结果不重复 close；业务始终只调用一次原通用提交，不新增恢复、发送重试或补 Enter。

原普通 `codex` 解析为已安装 0.159.0；另有已安装 `C:/Users/liuming/AppData/Local/OpenAI/Codex/bin/0ab0e7a8f3779036/codex.exe`，版本 0.165.0-alpha.1.2，SHA256 `0c09e45bc76433756eb2d492f6db0ce6f269967e6618f4a6ae73e2fa239694b8`。最终目标显式冻结后者，不安装、不改 PATH。模型仍为 `gpt-6-luna/high`；使用原 sandbox、链接 Git 和既有执行主机 gh 认证启动前缀，没有清空 RepositoryURL、变更模型或放宽完成判定。Codex 的已安装版本不表示新增实验性 Orca 能力。

## 准备失败与测试时限

`session-01`、`session-qualified-01`、`session-qualified-02` 均在原夹具独立 90 秒准备上限内未获得原 READY，业务提交 0 次、completed=false；仅原终端应急清理成功，不算正式业务资格或正式 stop 通过。前两份使用 0.159.0，第三份使用显式 0.165.0-alpha.1.2，不能据此断言某版本缺陷。

无模型短启动、等长启动和原认证/链接 Git 前缀诊断通过，两个现有 Codex 的登录状态均为已登录。另一个独立 READY-only 诊断显示 `Reconnecting… 5/5 / Request timed out`，约 1m58s 后恢复；随后用原 QueryState/DecodeOriginalState/QueryOriginalSample 核实原 pane/provider 的 done/READY。屏幕只解释超时，不参与正式准备或提交判定，该诊断不计资格。未认证网络诊断 TCP 443 在五秒期限内未完成、.NET HEAD 返回 403；两种客户端/路由不等价，不能据此证明 API 登录、模型可用性或具体网络根因。

独立审计同意删除测试专属 90 秒上限，让远端准备消耗与现有本机相同的六分钟 fixture 总期限，保留原只读查询循环。它不重置期限、不重发初始化或正文，不修改生产 profile、ready、PrepareSession、wait-submit、CLI、observer 或 stop 时限。记录准备与业务耗时；不据此宣称生产冷启动 SLA 达标。原三份失败和诊断仍为非资格证据。

诊断终端早期原始流还出现非本次发送的 Get-Process 前缀，归因未确定；已询问并记录，关闭该终端后，正式现场要求完整空清单及全新原终端。没有通过屏幕逐字比较来保证任意人工写者排他，Spec 原有并发边界保持。

## 正式现场与证明边界

| 现场 | 原 Run | 原请求 | 准备耗时 | 业务耗时 | submit/完成/正式 stop |
| --- | --- | --- | --- | --- | --- |
| final-01 | `d74720b1b68373e9fa6389d1e8efde5f` | `4e41c4e4-286a-5239-9111-023fb4d8c7c7` | 19.801s | 11.755s | 1/通过/confirmed |
| final-02 | `2efd0df6378901a4a9c9aeea930c2372` | `1ee11e46-7712-402e-266b-57f3292b6c54` | 123.262s | 11.200s | 1/通过/confirmed |
| final-03 | `83c493f6508f435ed031cfc5eb554365` | `3b4ba79e-3456-10b2-8642-9bfe47eff42d` | 126.404s | 12.465s | 1/通过/confirmed |

三份 Run/Attempt、terminal/incarnation、provider session/user item、operation/request、nonce 及输入摘要均独立；同一最终源码摘要 `c1a6550561df3b602f58658e2102364913f5e1f427926ecfe2d91256f2f36a96`，同一 TestEXE SHA256 `d559da6edc10e88ab8f40f7ad74a8aeb948290bd5dacbe9ea0f5b5db8cfa5eae`。后两份准备实际超过原 90 秒测试上限，但仍消耗原父总期限，没有重发初始化。

每份正文与本机使用同一组无工具最小任务：中文、多行、CRLF、内部空行及长输入，只返回不同 nonce。正文冻结 4898 UTF-8 字节，普通远端按原规则编译为 4694 字节 wire；不加入本机模型包装，不改变既有双端差异。

实际 CLI 原正文/请求、provider=codex/observation=supported 的 input_accepted/turn_started、正式唯一原输入查询、新 main-agent done、nonce final 及正式停止均逐项核验。离线核验器只读复核保存的字节、身份和原始回执，不从序列化时省略的内部 query fence 字段重建运行期证明。

远端正式停止保持 `orca_terminal` 范围，原诊断明确“不证明脱离终端的后台进程消失或撤销执行主机凭据”。不以本机签名退出/认证撤销结果替代远端，也不把一次 RPC 解释成 Orca 内部一次按键或原子全文消费。提交契约及历史故障未完整复现的边界沿 [Spec](../spec/orca-native-submit.spec.md) 保留。

## 稳定回归、清理与归档

- 最终正式 controlled 入口：15 必需 root、168 叶例各 30 次，**5040 执行、5040 通过、0 失败、0 跳过**；源码冻结及零漏选门禁通过。三份成功现场与该矩阵使用同一最终源摘要，不混入前两批矩阵充数。
- 最终项目 L1：714 顶层通过，含子例 2657 通过，0 失败、17 个明确 opt-in/helper 测试跳过，另 1 个无测试包跳过；34 包通过。框架、29 项 terminal extension、driver 编译及零匹配门禁通过。
- 七个本次 worktree 均在 Git 干净与原 runtime 完整空 census 核验后由公开 `worktree rm` 精确删除，没有 force 或 hooks。独立只读终端再确认七条路径和 Git 注册均不存在，清理前后 info/exclude 摘要相同；该核验终端随后关闭并确认原 handle 不在清单中。其它仓库终端未操作。此清理不扩大远端正式 stop 的证明范围。
- 归档保存失败、诊断和三份成功原记录、逐叶次数、原始日志 gzip、源码/EXE/CLI 摘要及清理证据。涉及无关工作区注册行的四份原记录留在私有现场，公开最小派生记录注明原 SHA 和省略字段；不是将失败改成通过。

[三会话摘要](evidence/orca-remote-final-20261030/three-sessions-summary.json)、[公开字节/SHA 清单](evidence/orca-remote-final-20261030/manifest.json)、[只读离线核验器](evidence/orca-remote-final-20261030/audit-remote-evidence.mjs)。目录名中的 `20261031` 是现场命名，实际时间以保存的 UTC 时间为准，本批发生于 2026-10-30。

[最终独立只读复审](evidence/orca-remote-final-20261030/independent-review.md)已通过，无 P0–P2；复核了最终源码、三份原始调用/结果、原事件、279份归档摘要与清理。本复审记录随后单独加入清单。

未部署安装、修改全局配置/认证、修改 Orca 或业务材料，未运行 Unity/环境检测业务、创建业务 PR、重发历史未知 Run 或人工补 Enter。历史 dirty 源码未恢复，本次不宣称完整复现历史原生故障。
