# Orca 配对公钥主机指纹收敛 Spec

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

以 `main` 的 `f6a389cf` 为基线，包含 #113 和 #114。同一协议包中，会话连接、文件连接、usage 连接和 `UsageHostID` 四处重复解码公钥、计算 SHA256 并生成主机指纹。本 PR 提取一个私有方法 `func (p statePairing) hostID() (string, error)`，由四处调用，集中维护同一算法。

方法只读取接收值的 `PublicKey`。继续使用 `base64.StdEncoding.DecodeString`，成功时对解码字节计算 SHA256，返回 `sha256:` 加小写十六进制摘要。解码失败时返回空字符串和原解码错误。沿用解码器对填充和换行的处理，保留现有指纹格式。endpoint、token、公钥长度和配对清单的有效性继续由原解析器检查，新方法不新增验证规则。

四处调用保留当前顺序和各自责任。

| 调用处 | 保留的处理 |
| --- | --- |
| `NewRemoteAgentSessionClient` | 原 identity 完整性检查、配对解析、host 比较、`stateIdentityError` 分类、超时及能力声明。使用原配对值拨号，派发和未知回执处理沿原逻辑。 |
| `openWorkspaceFiles` | #113 的每次打开只解析一次配对。比较方法返回的指纹与原 host，解析、解码或比较失败仍为 `binding_invalid`，连接失败沿原分类。用同一份已核验配置的 endpoint、公钥及 token 拨号。 |
| `openUsageFiles` | 解析与解码错误按原路返回，保留空 runtime 的 `IDENTITY_UNVERIFIED` 和错 host 的 `IDENTITY_CHANGED`，检查次序保持。保留普通及 structured usage 各自的握手、请求能力和原配对拨号。 |
| `UsageHostID` | 保留现有公开函数及调用方。先按原规则解析配对，再返回其 `hostID()` 结果。 |

本机分支继续使用原 runtime/host 校验。共享方法无配置读取、解析配对、主机比较、认证、网络、缓存、日志或状态写入。生产改动限该方法、四处重复算法及确实不再使用的 imports。现有连接、错误、wire、持久格式和业务合同沿原实现，不修改 Orca。

验收比较新旧返回字符串及解码错误类型和值，使用独立固定向量覆盖合法公钥、原 Base64 换行处理及非法输入。复用现有会话、文件和 usage 的错 host、能力及配对替换负例，确认原拒绝时机、错误分类和零派发行为。#113 的不同公钥、不同 token 且同 runtime 替代端点，继续满足每次只解析一次、替代端点零连接及零文件请求。
