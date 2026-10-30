# Orca 原生输入可靠提交 Plan

日期：2026-10-30。基线及规范见 [Spec](orca-native-submit.spec.md)。
状态：P0 调查完成；P1 独立方案审计通过；P2 被 Orca 契约缺口阻断。后续阶段不能绕过前置。

## P0 当前代码与能力调查

核实五个指定文件、`local_codex_history.go`、`Turn/NativeProof`、state codec、正式测试目录与真实输入入口。记录当前本机/远端 runtime、版本和能力，查 CLI schema、handler、host native 提交实现和请求恢复语义。保留安装包 SHA-256 和函数定位，不发布凭据、配置或共享终端正文。

对当前安装包做隔离函数行为核验：静默 render gate、缺消费确认仍受理、正文截断/改字节/混入、补 Enter、开始观察不等于全文。用虚拟时钟/受控信号；实际调用原安装函数，明确替换了哪些依赖。每个反例 30 次仅为契约证据稳定性，**不是 Runner L3 或真实故障复现**。

退出标准：明确既有接口能否满足 Spec §5。不能满足则出具接口缺口及证据，停止生产实施；可以交独立 draft PR 保存 Spec/Plan/审计结果，不能将任务标已修复。

## P1 独立方案审计

新独立只读审计者检查 Spec/Plan/证据与当前源码：五层事实、提交前正确性、旧路径删除清单、本机/远端一致性、超时零重发、并发围栏和测试映射。主会话修正确认的问题，记录审计者、范围和结论。审计通过设计不表示 Orca gate 已通过。

## P2 Orca 前置契约核验（阻断）

要求 Orca 提供 Spec §5 的严格 `terminal.send` 扩展/等效既有接口、能力标识、只读请求查询和可核验的 provider 消费/排他提交保证。复用 mutation journal；上游证明重复、重启、未知窗不产生额外写入/CR。只接受两执行目标相同契约。

不在 Runner 仓库修改/升级安装中的 Orca，不编造响应字段，不通过项目材料或脚本绕过。若无法满足，应由 Orca 修复其 native delivery/provider 接口并提供源码/测试证据后再继续本 Plan。缺接口不是用户缺少授权，不要求用户确认不安全切换。

## P3 Runner 实施（P1、P2 通过后）

1. 在现有 `protocol/orca` 调用/查询上增加强提交验证；沿用目标选择与原 requestId，写入前冻结参数与身份。能力缺失明确拒绝，无 legacy 回退。
2. `Lifecycle.submit` 普通原生 Codex 单行/多行、本机/远端调用同一函数；只调用一次业务提交。同步将本机 Codex `prepareInput` 的 native-command/model wire 选择及 `observeOnce -> observeRecorded` 完成选择迁入同一正文编译、原输入/final 核验链路，清理仅供其使用的 helper reader/receipt 调用。保留既有执行准备、认证、模型参数和停止保证，不能以此次统一为由收缩；P2 必须核实这些耦合可安全解除。仅保存返回的权威 proof，按既有 Turn/operation 状态推进；accepted 不设 started/completed。unknown 只读原请求，不再 send/Enter。
3. 保留输入预算、NUL/ESC 安全检查、规范化、prepare/readiness、完整原输入和 final 的独立证明。原请求 started receipt 必须证明原归属；否则继续原查询规则，不借普通 activity 认领。
4. 按 Spec §6 完整删除草稿/重绘实现、状态、fixture、专用模式和当前文档门禁。全仓搜索所有符号/配置及引用，确保没有隐藏旧分支或新旧两套路径；历史 evidence 原字节保留。
5. 如需 codec 变更，仅为存储强提交 proof；旧未知记录不自动恢复提交，原停止和查询继续按原身份。无新增公开配置开关。

## P4 确定性行为回归（规划，均未实现）

扩展现有 `orcatransport` fixture 与 `tests/integration` 的 CLI/SQLite/服务重启入口；协议使用真实 IPC/WS fixture，不伪造“已经可靠提交”的上游保证。新增 provider 测试端点仅在 test fixture 内，通道可逐段放行 consumed-input 与 receipts，无生产故障模式。

| 需求 ID | 入口与拟映射 | 条件与独有断言 | 证据 |
| --- | --- | --- | --- |
| NS-T01 | `orcatransport` 共同提交表；focused native `native-input` | 多行、中文、内部空行、CRLF、长输入；逻辑冻结不变，规范化 wire 字节/摘要完全一致 | request、长度、SHA、provider 消费全文；1 submit |
| NS-T02 | 同表、现有 `TestOrcaNativeStateLiveLifecycle` | 屏幕折行、宽度变化、折叠标签、投影缺 LF、屏幕不可读；不调用 read/resize，不影响提交/正式判定 | read/resize=0、原 wire digest、原结果 |
| NS-T03 | 共同提交表 + native input query 反例 | 缺失、截断、单字节篡改、额外输入；消费边界前拒绝、0执行；恶意/冲突 proof 后正式核验也拒绝 | consumed bytes、commit=0、execution=0；冲突原因 |
| NS-T04 | 共同提交表/provider channel | 正文分段到达、消费确认延迟；最后一段及校验前绝不提交；无固定 sleep | channel 步序和实际提交 ordinal |
| NS-T05 | 既有 native start/completion tests | accepted 入队，随后 committed，尚未 started；都不能完成；独立主回合/全文证据后才推进 | 每一边界的 Turn facts/事件及 final |
| NS-T06 | 扩展 `TestSRStabilityLostSubmitResponse`、native tests | paste/consume、commit、start、final 各窗口丢回执/超时；只读查询可恢复原证据，未知时不补写 | 原业务请求尝试=1；实际 commit/执行按注入边界为0或1且≤1，补 CR=0 |
| NS-T07 | 共同提交表与协议身份反例 | runtime/terminal/incarnation/pane/provider/boundary 变化、原草稿和并发人工写者；提交前竞争使其无执行地失败 | 原因、拒绝/unknown、0额外写入；不能用屏幕正文判 |
| NS-T08 | 扩展 `TestSRRestartPreservesAuthorization` 和原 unknown/reconnect 入口 | 重复请求、断线重连、commit 前后服务重启、receipt 丢失/过期；旧 identity 不重投 | 原 operation journal、查询计数、实际 execution=0或1 |
| NS-T09 | 全部共同表参数化 local IPC/remote WS | 相同输入/故障矩阵，不允许本机 helper/凭据/完成旁路 | 两目标结果逐 case 相同、传输绑定证据 |
| NS-T10 | 既有 original input/final、stop/reconcile tests | 原全文唯一性、完整原 final、冲突/工具输出/旧 done 拒绝，原资源停止/清理不退化 | 原证明链和完整 stop census |

NS-T03–T08 的每个叶子故障、每个目标 **30 次**；报告用例清单、执行/失败/跳过，保留首次失败和后续修复的新批次。不要把一次快照回放 100 次列为此矩阵。

P3 时将 NS-T01–T10 绑定实际测试名与已登记 case；扩展 `tests/catalog.json.focused_native_scenarios`，使既有 `scripts/test-native-input.ps1` 支持 local/remote 相同目标格式，删除目前强制 Environment 非空和 segmented-shape 旧门禁。通用 L3 的 ID 不因文档规划而预先注册；实装时同步 `cases.json`、目录/manifest 校验、`docs/testing.md`、`test-catalog.md`。不为达到计数增加业务模型任务。

## P5 项目检查及真实验证（P3、P4 后）

按仓库 `scripts/test.ps1 -Level L1`、受影响 Controlled/L3 的正式入口完成检查，执行框架目录一致性校验。仅在新变更或失败提出新疑问时增加检查，避免无目的重复。

真实输入沿扩展后的既有 `scripts/test-native-input.ps1`，每目标三次独立调用/新目录/原生会话，使用同一个无副作用最小提示词（只返回 nonce；不要求写项目文件、Git 或 PR）。覆盖多行、中文、空行/CRLF及长输入；记录真实 logical/wire 原文与摘要。

每个现场在发送前记录源码/EXE/CLI 摘要、完整目标/资源清单、native input baseline；唯一 requestId 与提交意图先持久化。调用一次强提交，保留底层 write/submit ordinal、accepted/committed/started/completed 原凭据，正式原输入/原 final 查询。最后仅精确关闭本次资源，按完整清单核验清理，记录六个会话的结果、失败、未验与清理。缺少底层实际计数不能用 Runner 调用次数代替。

不能重投历史未知 Run、补 Enter、重跑 Unity、创建业务 PR 或改配置以求通过。缺真实路径前提记未验证；受控通过不能代替本机/远端三个独立原生会话。Orca 强契约缺失时这六场均阻断，不在旧实现上跑后声称完成新方案真实验收。

## P6 独立复审与 PR

最终 diff、实际测试、真实证据及清理范围交新的独立只读审计者复审；修正已确认问题，报告保留项和未验边界。单独分支/PR，不合并、不部署。若 P2 仍阻断，只提交设计/缺口证据的 draft PR；标题/正文明确“未实施”，最终报告不得声称屏幕依赖已移除或可靠提交已保证。
