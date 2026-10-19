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
