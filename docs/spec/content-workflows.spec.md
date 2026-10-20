# conversation/1 与 generate/1

日期：2026-10-19。普通开发授权；不修改 SteerLine、不部署、不进入 roadmap。

## 边界与调查

仓库及父目录未发现 AGENTS.md。基线为 development/2、development-audited/2 和 state/v3。
当前 RunRequest 强制仓库、Spec/Plan、publication；freeze 创建 Git 副本；queueInput 注入开发材料；
Route 负责补报/发布。不能将这些开发前置用于内容流程。现有 Record/Commit、operation、
reliable.Next/NormalizeResult、Codex ExecutionLifecycle、原生问答路由、VerifyManualRecovery/
BindRecovery、owned Job 停止是复用边界，不另建运行时、账本或调度器。

## 输入与公开面

沿 submit v1 信封，通过 workflow id/version 区分互斥输入。共用 title、correlation、execution；
conversation 包含 instruction、materials、directory（path、access=read_only）；generate 包含
instruction、materials、可选 directory 和 output_schema。新流程拒绝 repository/documents/publication、
审计及提示词覆盖。首版仅 local codex-app-server、read_only；原生 never 审批不变。
材料使用现有 Document inline 快照结构；不带 Git 仓库时拒绝 git 材料，避免伪造仓库。
纯材料生成使用 Run 私有空目录；明确绑定原目录时核验规范绝对目录身份，恢复重新核对。
read_only 是原生只读文件权限，不承诺限制读取到单一目录；目录是明确的工作范围。
本次原生初始化查询 account/read，并以同一只读权限执行无模型目录读取探针；
这些检查不创建业务轮次，不调用 Git/发布检查，也不沿用历史开发检查资格。

inspect 提供结果、轮次和操作可用性；history 提供真实提交消息、交互、原生 final 及来源。
message（expected_revision、text）只在 conversation 空闲且资格有效时接受；reply answer
复用原问题 ID 和原执行范围绑定；无范围绑定的旧 epoch 输入仍需人工确认。interrupt 只中断当前轮，stop 继续关闭整个 Run；close 为显式
会话关闭操作。所有写操作沿 request_id 去重/CAS/持久意图，未知只查原对象。

## 流程与完成

generate：固定输入 → 单一 execute → 完整原生 final → 程序格式校验 → 结果固定 → 停止/结算。
无 schema 返回文本；有 schema 必须是单一 JSON 值并通过自包含 JSON Schema（本地 fragment 引用可用，
外部引用不加载）。不推断业务完成、不解析业务 ID，不补报/修复/独立 parser/第二模型轮次。
失败、取消、缺 final、JSON/schema 错误均非成功；人工问答直接失败并结算。
结果状态与 cleanup/stop 状态独立；清理失败不得显示全部完成。

conversation：execute → input，显式 message → 同一原生 thread execute；无自动开发阶段。
失败/取消仍保留轮次事实，确认原轮终止后才可继续；interrupt 不关闭会话。close/stop 一经接受
永久拒绝后续消息。一个 Run 至多一个活动轮，原问答只答原轮。
内容流程不冻结或使用开发结果 parser 配置，接管不要求该不适用的配置；原材料、workflow 执行语义、implementation codec 和原生身份校验继续适用。开发流程仍要求有效的原结果获取配置。

兼容服务换代沿[共同恢复契约](service-generation-recovery.spec.md)保留原授权与原 owner/native，
空闲仍等待显式 message；已接受但尚未冻结的 message 绑定原执行范围，核验原会话/目录后仅冻结一次。
已建立副作用意图的输入只对账，不创建替代 thread 或重放历史；无法核实即未核实。
原暂停/撤销门槛保持，适用的显式 resume 仍须核验原会话及旧 writer。close/stop 结算尚未派发的请求。
历史同时关联调用命令与实际原生 command/turn，避免混淆接受和发送。

2026-10-20 接管校正：创建与接管按冻结 workflow 的实际能力决定开发结果获取配置是否必需。
final-only 的 generate/1、conversation/1 不消费开发结果解析器，合法零值 AcquisitionConfig
必须可接管；不读取当前可变配置、不补默认值、不改写原记录。含开发结果解释、report 或 parse
能力的流程仍严格校验冻结配置；任何已存在的 parser 意图仍校验配置、解析契约和原 owner
恢复能力。workflow/version、执行记录、codec、冻结材料、原 Run/input/request 绑定检查不变。
verify-takeover 是只读资格判断，不启动 Agent、不发送输入；compatible 不是恢复完成或资源结算证明。

## 数据职责

Runner 保存固定输入、实际指令、请求/轮次身份、问答与结果、诊断和资源事实。
SteerLine 保存业务关联、构造指令、校验业务内容、保存正式文档。新流程无 TaskDelivery、Git
写入、推送、PR 权限或结算。开发流程的准入、stop 和恢复语义保持。
