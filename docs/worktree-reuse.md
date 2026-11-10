# 已停止 Run 的受管 worktree 复用

2026-11-10。新 `execute/1` 请求可设置 `reuse_worktree_from_run_id`，引用同一 state-dir 中最后持有该工作区的 Run。它仍只接受一次输入、执行一次，并按原契约停止；不增加 maintenance Workflow、可写目录入口或旧会话恢复。

## 请求与来源

提交完整的新 Run 请求，使用新的 `request_id`、标题、执行配置和 `workflow.prompt_overrides.execute`，并增加：

```json
"reuse_worktree_from_run_id": "<最后持有工作区的已停止 Run ID>"
```

仓库描述（包括 URL、local_path、remote、base_branch、base_revision）、adapter 和 target 必须与来源相同。Orca transport 的 executable 和 args 也须保持，避免把原工作区身份绑定到另一条传输路由。执行模型可重新明确配置；模型省略仍表示原生默认，不从来源继承模型。其他 execute/1 字段和校验保持。能力入口为 `capabilities.worktree_reuse`，字段也在公开 run_input schema 中。

新 Run 使用原 worktree、分支和仓库管理目录。已有提交、索引、未提交修改、未跟踪文件及 merge/rebase/cherry-pick 等 Git 状态保留，不 reset、stash、clean、切换分支或重新创建目录。本机 rebase 暂时 detached HEAD 时，只接受其 head-name 仍指向原分支的状态。工作区已删除、重建、重定向或身份冲突时拒绝。

`inspect` 发布 `reuse_worktree_from_run_id`、`workspace_origin_run_id` 和 `workspace_identity`。新 Run 的请求摘要、Attempt、resource generation、Session、命令、结果和用量身份独立；来源 Run 保持终态，其原请求、结果和命令不改写。新 Run 受理事务记录 `worktree_reused` 事件。

## 接管与停止

来源须已撤销写入授权，停止已核实，原业务操作及控制命令无未知或未完成义务，已启动的内部 parser 已退出；启用了 result-file 时还须先完成原文件封存。活动 Run、未知执行、未知停止或未封存结果返回 `workspace_not_settled`。先通过原 request_id 查询并沿原 stop/resume 对账机制结算；复用请求不重发原业务输入或另发 close，不把未知改为成功。

可选 progress/result 初始化还须结算物理写入及通道关闭。初始化超时不延长原业务或停止期限，但新 Run 不能在旧写入仍可能继续时接管。写入派发前持久登记 pending，本机同步写入及通道完整退出后才登记 settled；Orca 写入丢失回执时，随后读到文件也不能证明写者退出。重启保留未核实义务；本次没有新增远端文件写入对账接口，无法核实者明确拒绝复用。缺少该结算证明的历史初始化同样拒绝。停止发生于首次输入前，或原输入已有绑定正确且无接受/启动证据的明确拒绝回执时，可以结算原 submit 为 failed；未知输入仍须对账。

工作区 incarnation 在原创建回执提交时绑定：本机使用原 checkout/Git admin 目录的文件身份和仓库身份；Orca 使用冻结 host、runtime、worktree ID 与 instance ID，并只读核对原 worktree。未保存该创建身份的历史 Run 返回 `workspace_identity_unavailable`，不能从一个当前目录补造历史创建证明。

现有服务单写者 lease、Run gate 和受理事务共同排他：同一工作区形成串行来源链，转交后即使新持有者已经停止，也只能引用该最新 Run。并发不同请求最多受理一个接管；同 request_id 同正文回放原回执，换正文冲突。丢失受理响应时先查原 request_id。旧 Run 的新控制请求返回 `workspace_transferred`；原已接受请求的重试仍回放原结果。旧 worker 不再派发、再次停止或清理转交后的工作区，重启保持该边界。上述状态/身份错误使用 exit 5。

停止仍按冻结范围解释。本机 Codex 要求原 owner/root 的已有停止证明；Orca 要求原 session close/未创建的已有证据，保留 `orca_agent_session` 的范围限制。该能力排他管理 Runner 的受管写者，不提供 OS 隔离，也不保证发现调用方或 Agent 脱离受管范围的任意后台写者。未核实的原生停止不提供接管资格。

## 用量与调用方职责

沿原 `run-usage` / `run-usage-events` 采集、绑定和去重。新 Run 只归属新会话和新命令，不能复制来源的 Token 或事件。跨 Run 关系由来源字段表达，金额汇总、价格和预算继续由 SteerLine 负责。

Runner 自己调用的独立 parser 使用同一不可变事件账本，发布 `scope=runner_internal`、每次物理调用唯一的 call_id、llm_parser 来源、原 command/result/step、配置模型和推理强度。原生启动前保存 pending 调用；采到原 CLI turn 用量后为 covered；未取得可信用量为 uncovered，Token 保持 null。崩溃前已登记但未收齐结果的调用保持 pending，不算零。一次允许的物理重试产生另一 call_id。

`run-usage.scope=main_session`、其 Token、状态和 coverage 继续只描述原主会话；内部调用不进入该合计。`model_call_coverage` 单独公布内部已登记、covered、uncovered、pending 数量，以及未覆盖的 subagents、tool_internal_models、unobserved_provider_calls。这些数量是已登记调用数，不是完整 Run 所有模型调用数。事件未提供 scope 的历史记录沿原 main_session 解释。SteerLine 对同一 snapshot_revision 翻页并按 event_id 去重，按 scope 区分，再仅对 decision=counted 的已覆盖用量按约定配置模型估价；未覆盖不代表零。详见[模型用量交接](model-usage.md)。

SteerAgent 继续只读，通过提示词和 Runner 的公开 submit/inspect/stop 驱动执行。多轮重构、停止后收尾和 PR 冲突处理均由新 execute 提示词表达，不增加调度、发布或审批工作流。

验证范围及当前缺口见[本次验收](acceptance/worktree-reuse.md)。
