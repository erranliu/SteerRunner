# generate/1

一次性内容生成，仅本机 `codex-app-server`。共用现有请求信封、Run、材料快照、
可靠执行、原生会话初始化、完整 final 取证和 owned Job 结算。

```text
execute → closed
```

只发送一次业务输入。轮内可使用原生只读工具；没有补报、独立解析、修复、重新生成或发布。
接受时固定 instruction、inline materials、execution 和 output_schema。材料正文随原输入提交，
后续不读取调用方移动文档。无需 Git、Spec/Plan、Task 或 publication。

`execution.settings.access_mode` 必须明确为 `read_only`；沿 Codex 0.159.2、Windows MXC、
`approvalPolicy=never`、`approvalsReviewer=user`。外部 MCP、Apps、hooks 和子 Agent 关闭。
可选 directory 为已有绝对目录和 `access=read_only`；省略时使用 Run 私有空目录。
初始化通过原生 account/read 查询认证，并使用同一只读权限执行无模型目录读取探针，
之后才允许业务输入；缺认证或目录权限时不启动业务轮次。
目录在接受、派发、恢复时核验原目录身份。原生 read-only 限制写入，不承诺单目录读取隔离。

无 output_schema：`content_result.text` 为完整最终文本。
有 output_schema：必须返回单一 JSON 值，程序校验后保存至 `content_result.json`。
首版固定 JSON Schema 2020-12，允许本资源 `#` 引用；拒绝 `$id`、其他 dialect 和外部引用，
无文件/网络 loader。Runner 仅校验格式；业务含义由调用方校验。

坏 JSON/schema、原生失败/取消、缺完整 final、人工问答均非成功；人工问答诊断为
`human_input_required` 并关闭。原生权限审批沿已有拒绝规则。已发送但未知只对账原请求，
不能以新 request_id 重发。`content_result.status` 是结果事实；`stop_status` 是资源结算。
结果已成功而 cleanup 未核实仍为 settling/reconciling，不显示 completed。

最小请求（UTF-8，经 `submit --input - --json`）：

```json
{
  "protocol": "steerrunner-cli/v1",
  "request_id": "generate-001",
  "input": {
    "title": "生成测试安排",
    "workflow": {"id": "generate", "version": 1},
    "execution": {
      "adapter": "codex-app-server",
      "target": {"kind": "local"},
      "settings": {"access_mode": "read_only", "executable": "C:/Tools/codex.exe"}
    },
    "generate": {
      "instruction": "根据材料返回测试条目列表。只返回 JSON。",
      "materials": [{"kind": "inline", "path": "requirements.txt", "content": "验证创建和关闭会话。"}],
      "output_schema": {"type": "array", "items": {"type": "string"}}
    }
  }
}
```

用 `inspect --request-id generate-001` 找回原 Run/command；随后 `inspect --run-id R` 与
`history --run-id R` 读取结果及来源。不要从原生会话文件或 Runner 数据库取得结果。
`stop` 可显式关闭/结算；generate 不提供 message/reply/interrupt。

## 固定仓库版本调查

可选 `generate.repository = {"local_path":"C:/Repos/Product","commit":"完整 commit ID"}`，
与 directory 互斥，且必须提供 output_schema。仅支持已有本地 Git 仓库（含本地 bare/linked
worktree），对象必须已存在；不联网 clone/fetch、不更新源索引/工作树/refs。

Runner 仅从指定 commit 的 Git 对象读取完整文件树与 blob，关闭 replace objects 并移除环境中的
Git 路由覆盖。Run 私有目录只有这些普通文件，没有 .git，不运行 checkout filter、export-subst、
export-ignore 或目标仓库 hooks。源工作树的未提交文件、暂存区和之后的新提交均不参与快照。
链接、子模块、不安全路径、Windows 大小写冲突均拒绝接受。快照保存在原 state-dir，恢复时
只核验原快照，丢失/漂移不会重新物化或另起模型请求。

公开 `capabilities.data.generate` 与 `configuration_schemas.generate_input` 描述实际接口。
`run.repository_source` 和 `content_result.repository_source` 返回程序生成的 local_path、commit、
tree、files；每个文件包含 path、blob、sha256、size_bytes。可用源仓库
`git --no-replace-objects show COMMIT:PATH` 核对 blob 字节及 SHA-256。manifest 证明固定版本
可供调查的文件；它不是“模型读过每个文件”或业务推论正确的证明。文档应引用相关文件/版本，
调用方负责验收引用与结论。原生只读权限不提供单目录读取隔离。

最终文本（整个 JSON，含转义、路径和所有正文）最多 **1,048,576 UTF-8 字节**；请求信封最多
8,388,608 字节；快照最多 4096 文件、67,108,864 blob 字节；编码后的版本 manifest 最多
1,048,576 字节。受管 Git 单次命令 30 秒、仓库冻结总预算 2 分钟。超限不成功，不截取正文
后标成功。原生失败、缺完整 final、JSON 截断、Schema 不符、快照漂移同样不能成功。
超限结果仅保留来源身份与失败诊断，不能通过 history 当成完整文档读取。

多文档 JSON 的字段、数量、路径与业务内容由调用方 output_schema 决定。Runner 不另加业务
包络、不落盘目标文档。示例使用 documents/path/content，并用 const 固定相对路径、精确数组
数量和非空正文；调用方应提供所有预期文档的约束。Schema 太宽会接受缺少业务内容的 JSON；
合法字符串在业务意义上未写完，无法仅靠 Runner 检测，需 SteerLine 的格式/验收规则。

最小完整示例见 [generate-repository.example.json](generate-repository.example.json)，替换
local_path、完整 commit、Codex EXE 和业务材料后：

```powershell
$runner = 'C:/Tools/steerrunner.exe'
$state = 'C:/RunnerState/spec-plan'
& $runner service start --state-dir $state --json
& $runner submit --state-dir $state --input ./docs/workflows/generate-repository.example.json --json
& $runner inspect --state-dir $state --request-id roadmap-R7-spec-plan-001 --json
& $runner inspect --state-dir $state --run-id R --json
& $runner history --state-dir $state --run-id R --json
```

从 inspect 的 `data.run.content_result.json` 或 history 的 `entries[].result.json` 读取完整结果。
`ok=true` 仅表示查询成功，接受结果需 `run.status=completed`、`content_result.status=succeeded`、
`source.verified=true`、`stop_status=stopped`；随后执行 SteerLine 业务验收。
超时或连接丢失先 inspect 原 request_id；如需重放，必须原 request_id + 原始请求，不能换 ID。
`stop` 仍使用原 expected_revision，`resume` 沿用既有可恢复状态限制。generate 关闭后不重新生成。
重启必须保留 state-dir，并使用支持此输入及源 manifest 的 Runner 版本。
