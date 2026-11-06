# Orca 关闭与完整清单 B4d 验收

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

日期：2026-11-02。状态：已完成；实现、定向及两包全集通过，独立审计唯一 P2 已单独修复并复核关闭。依据 [Spec](../spec/orca-close-inventory-capability.spec.md) / [Plan](../spec/orca-close-inventory-capability.plan.md)。

## 职责与消费者

Client.InspectTerminal 消费自有 TerminalIdentity；CloseTerminal 返回原 Result/TerminalClosure（handle/ptyKilled）和调用/身份错误，一次调用、零自动重试。terminal_inventory.go 提供明确 worktree 列表参数、TerminalInventory 基础/完整解析事实及 CompleteWorktreeInventory。完整原 runtime/host/count/truncated/terminal 身份校验保持，活动终端仍作为原生事实返回，不解释业务 stopped。

原 InspectBoundTerminal/CloseTerminalChecked/VerifyTerminal(s)Stopped 映射迁至 orcatransport/terminal_resources.go；原 CloseReadonlyTerminal 执行策略迁至该包 readonly_close.go。双端停止、原离线核验、initial terminal、testkit/host/原探针均消费这一份策略，保留原 BackendIdentityMismatch 和 accepted/confirmed 映射。辅助只读设施原有限继续关闭规则保持，未推广到业务停止；业务未知关闭仍只读原清单对账。

soleWorkspaceTerminal 与 initialWorkspaceTerminalRecovery 接入公共列表参数/解码，删除两份专属清单结构，保留原重试/空清单等待、选择及字段范围。基础 reader 不消费 total/host 类型，initial/full reader 保留原全字段类型拒绝；新增双端真实 sole-terminal 消费方反例验证。

协议 client.go 删除未使用 Config.Target、执行签名与结算映射；旧 readonly_close.go/stopped.go 删除。能力包全部生产文件无其他 internal 所有者导入；依赖守卫实际遍历该子树，并加入 backend/Codex adapter/orcatransport/相似前缀反例。原持久类型、stop progress/receipt、Profile/State/Target/creation 编码和 scope 均未修改，本机 managed 的 helper/root 条件保持。

## 直接验证

源码基线 `649c386` 加本批差异，删除/移动先暂存；source_digest `03a741639fa69e005ca6d97189250e4feaa751339469da5a242e9de1f2339763`。命令/源码身份和原始日志见[证据](../../../../acceptance/evidence/orca-close-inventory-capability-20261102/README.md)。

| 验收 | 结果 |
| --- | --- |
| OCI-01–02 | 新能力及原身份/停止/只读策略定向 7 根/63 子节点、64 叶例通过；最终边界补验 3 根/26 子节点、27 叶例通过（与前集重叠） |
| OCI-02–03 | 两包全集 229 根/1103 子节点、1177 叶例，0 fail、7 opt-in skip；原双端未知 close、队列未派发、incarnation、host/完整清单、owner 与 startup 回归保持 |
| OCI-03 | application/integration/testkit/host 定向 49 根/88 子节点、119 叶例，0 fail、1 opt-in skip；包括撤销与派发排序、stop 回执落盘失败、重入及离线核验 |
| OCI-04 | 最终依赖守卫 1 根、两个生产入口构建、live-tag 仅编译及固定持久白名单/diff 检查通过；待独立审计 |

子节点含分组节点，叶例另计，重叠集合不相加。全集在最终补入缺 scope 零调用、sole-terminal 字段范围和错误映射测试前执行；这些后续只改测试，最终 3 根通过。守卫增强独立补跑通过。

全集 skip 为 TestOrcaLocalPipeNativeEvidence、TestOrcaLocalIPCNativeProbe、TestOrcaTerminalQueryLive、TestUsageLocalAcceptanceRepoCleanupLive、TestOrcaNativeStateLiveLifecycle、TestShortInputProbeInspect、TestOrcaWorkspaceStartupLive；消费方 skip 为 TestOrcaStoppedNativeProbe，均不计通过资格。迁移中两轮编译及首次定向遗漏旧字段/私有 helper 调用导致失败，修正后通过，原失败包和日志保留。

没有真实 Orca/模型场次；本批签收现行合同的结构等价，不授予 B5 新候选接线资格。未知结果、清理和真实活动停止仍须按 B5 的新冻结 scope/正式 Runner 执行复验。

## 独立审计确认问题及单独修复

固定 `1bcc5d4` 相对 `649c386` 独立审计发现一项 P2：基础清单 decoder 仅保存解析标记，却复用 full decoder 的 `*bool Truncated`；重复 `truncated:true` 后 `truncated:null` 会使 full 指针变 nil，旧 bool 解码仍保留 true，新 sole-terminal 消费者可能认领截断清单。

修复改为独立 BasicTerminalInventory（自己的 rows、bool 和解析标记），sole 消费完整基础值。完整清单 decoder 和原停止判据不改。补原生及本机/远端实际 sole 的 true→null 拒绝、false→null 接受反例，原字段范围仍保持；未把 null 一律当作失败来改变旧合同。

修复后定向 23 根/146 子节点、150 叶例，0 fail/skip。修复 source_digest `dc8c29d517a896586617a8a6498f07cb729c66e8a5ea0abefba88f6dd1a40686`，单独[证据](../../../../acceptance/evidence/orca-close-inventory-audit-fix-20261102/README.md)；保留本批原归档不覆盖。

修复提交 `0dfbb53` 已由原独立审计者复核关闭，无剩余确认 P0/P1/P2。原 12 文件和修复 3 文件、704 文件源码摘要及所有统计吻合；能力、原停止/workspace/owner、错误映射和依赖守卫独立复跑通过。审计者另补正式 CLI 离线两个子例通过。

收口时发现原 consumers 正则未包含 TestSRServiceOwnership；在 `0dfbb53` 上准确补跑 offline_verification、remote_offline_scope，1 根/9 子节点、8 叶例，0 fail/skip，见[单独证据](../../../../acceptance/evidence/orca-close-inventory-offline-20261102/README.md)。原 49 根已覆盖 application 离线机制，但不能替代这两个 integration 入口，故原统计不改写。无真实 Orca/模型操作，不扩大 B5 资格。
