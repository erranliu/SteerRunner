# 远端原生输入单次重跑验收

2026-10-31，用户要求不再追查历史 Get-Process 文本来源，以一次全新隔离现场的正式远端原生验证通过作为本项接受条件。本次 **1 次执行、1 次通过、0 失败、0 跳过**，按此口径接受本项并结束来源追查；历史现象和来源未定记录保持，不改判为已查明根因。

沿用 `scripts/test-native-input.ps1 -Mode long-input`，目标为现有 `192.168.1.151` 配对环境及 runtime `64108a37-610d-478c-a565-194c376217c5`，Orca 1.4.245。使用全新工作区 `codex-sr-native-remote-rerun-20261031-afe3aedc`、新终端和以下独立身份，没有操作旧 Run：

| 凭据 | 本次原值 |
| --- | --- |
| Run | `81707f4b9d5ea40229805e0658af25a8` |
| 原请求 | `d3cdfaf1-7dfa-b5f4-2594-9257928af168` |
| terminal | `term_479423e6-2451-4da1-ac34-c7b555e8ee0a` |
| incarnation | `2aa4d115-132a-42ee-9486-b6443ed84d37` |

冻结正文4898字节、既有规范化后wire4694字节；中文、多行、CRLF、空行及长输入完整核对。原请求accepted/turn_started、正式唯一原输入、nonce final及completed均通过。业务submit=1，text-only、bare Enter、composer read、foreign request均为0。初始化120.989秒，业务10.066秒，测试总135.74秒；共享既有六分钟fixture总期限，生产等待参数不变。

原StopLifecycle、verifyStopped及原工作区完整空census通过，停止证明保持既有 `orca_terminal` 范围，不扩大为远端账户凭据撤销或脱离终端的后台写者消失。随后核实工作区Git干净，关闭本次清理检查终端后精确删除工作区；独立只读核验确认原路径与Git注册均不存在、清理前后info/exclude摘要相同。核验终端随后关闭并确认原handle缺席，其它仓库终端未操作。

使用提交 `01aa580a65b8f4a6bbf63125c481905a22567c86` 的干净checkout；源码摘要仍为 `c1a6550561df3b602f58658e2102364913f5e1f427926ecfe2d91256f2f36a96`，TestEXE仍为 `d559da6edc10e88ab8f40f7ad74a8aeb948290bd5dacbe9ea0f5b5db8cfa5eae`。本轮没有代码、模型、认证或全局配置修改；没有部署安装，没有重新执行旧请求或人工补Enter。

本次沿用[既有只读证据核验器](evidence/orca-remote-final-20261030/audit-remote-evidence.mjs)，再次核对归档的实际CLI正文、原请求/会话、原输入及结果、正式停止和清单，全部通过。另与此前三场逐字段核实身份未复用。此前5040/0/0稳定回归、L1及独立复审属于原批次，本次未重复计算或冒称新跑。

[本次原始证据与摘要清单](evidence/orca-remote-rerun-20261031/manifest.json) · [本次离线核验结果](evidence/orca-remote-rerun-20261031/evidence-audit.json) · [此前双端验收](orca-native-submit-remote-final.md)
