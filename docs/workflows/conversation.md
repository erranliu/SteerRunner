# conversation/1

一个 Run 是一个持续的原生 Codex 会话。首版仅本机 `codex-app-server`，复用 Run/Record、
可靠执行与原生问答/恢复；没有第二运行时或规划/发布阶段。

```text
execute → input → execute（仅显式 message）
execute/input → closed（显式 close 或 stop）
```

执行结束后 `status=waiting_input`、`step=input`；轮次完成不会关闭 Run。
`controls.can_message` 表示当前可以接受下一条消息；每个会话一次只允许一个活动轮次。
`interrupt` 通过已有 Codex turn/interrupt 中断本轮，持久化原请求与操作；待原生终态核实后
回到 input。`close` 和原 `stop` 都关闭整个会话，接受后永久拒绝新消息、回答和 resume。
close 的资源结算与原轮结果分别返回，不把接收关闭命令当作已停止。

最小请求（目录须已存在；UTF-8，经 `submit --input - --json`）：

```json
{
  "protocol": "steerrunner-cli/v1",
  "request_id": "conversation-001",
  "input": {
    "title": "规划讨论",
    "correlation": {"planning_session": "caller-owned-id"},
    "workflow": {"id": "conversation", "version": 1},
    "execution": {
      "adapter": "codex-app-server",
      "target": {"kind": "local"},
      "settings": {"access_mode": "read_only", "executable": "C:/Tools/codex.exe"}
    },
    "conversation": {
      "instruction": "根据材料讨论测试范围，先列出待澄清问题。",
      "materials": [{"kind": "inline", "path": "context.txt", "content": "需要覆盖创建、轮次中断和关闭。"}],
      "directory": {"path": "C:/Projects/PlanningMaterials", "access": "read_only"}
    }
  }
}
```

下条输入使用 `message --run-id R --input -`，信封 input 为：

```json
{"expected_revision": 12, "text": "先讨论中断后的继续条件。"}
```

`interrupt` / `close` / `stop` / `resume` 的 input 为 `{"expected_revision":12}`。
所有写操作都使用新的持久 request_id，重复同 ID 同正文返回原回执；正文变化冲突。
旧 revision、并发 message、活动轮次输入和过期回答拒绝。发送结果未知时查询原 request_id，
不要换身份重试。轮内原生问题沿 `reply` 的 `kind=answer`、原 question_id、完整问题集答复；
问题已结束或正在中断时不能继续答复。`reply feedback` 不用于 conversation。

`inspect` 返回当前 session/turn、结果与 controls，`history --run-id R --after <cursor> --limit 200`
分页返回原指令、后续输入、问题、回答和各轮完整 final/诊断。每个结果有 command/attempt/session/
turn、原生 outcome、final item/source 和 verified。历史不收集完整工具正文。

重启只观察/对账原执行，撤销新输入授权。空闲会话显式 resume 核验原 thread、目录和原 writer，
再显式 message；必要时创建新的 owned app-server 进程并 resume 原 thread，不新建 thread 或
重放历史。活动轮、未知发送/回答/中断不能靠 resume 解除未知；无证据明确未核实。
目录及原生只读权限边界沿 [generate](generate.md)。正式规划文档由调用方保存。
