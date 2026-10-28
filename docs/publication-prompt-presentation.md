# 发布提示词的 Source facts 呈现

`workflow.omit_publish_source_facts` 是可选布尔值，默认 `false`。
development/2 和 development-audited/2 可使用；内容流程不接受该字段，
`null`、字符串及其他类型均拒绝。

设为 `true` 仅省略 `publish.execute` 提示词中的 `Source facts` 标签及 JSON。
实施输入、报告输入、固定成功声明、发布策略、PR 目标和 caller-owned 文件说明保持原样。
默认渲染与旧版本逐字一致。

完整结果、attempts、fields、文档和 source bindings 继续保存在 Run，
来源资格、唯一原文、进程/provider/main 及接受、启动、完成回执核验均不变。
该字段冻结在新 Run 请求中，不改变已经冻结的输入，不允许据此重发未知输入。

它控制模型看到的文字，不能用来宣称发布或业务验证成功。原生长度对照须使用同一构建，
按实际最终输入记录字符数、UTF-8 字节数和摘要，并分别保留首任务、发布任务及清理证据。
