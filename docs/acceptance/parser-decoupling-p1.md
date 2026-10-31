# Parser 解耦 P1：先前 API 调查（历史方向）

日期：2026-10-31。调查源码 `e78c0db6f4e9be6242c6928cfeae596c8358ca5f`；工作区包含本轮新增的 [Spec](../spec/parser-decoupling.spec.md)、[Plan](../spec/parser-decoupling.plan.md) 及文档索引。用户已追加“实施吧”，授权实施。本记录保留先前 API 方向的调查事实。用户随后明确继续使用现有 Codex 登录和 CLI，API 原型已删除；当前实现和验证见 [实施验收](parser-decoupling.md)。不再存在待提供 API 服务或密钥的前置。

## 当前结果

- 生产代码、旧 parser、执行 owner、操作账本、停止、恢复、state/v6 和客户端配置均未修改。
- 新增 当时的受控 API 传输原型（现已删除），仅用于 P1，无 Runner 调用者。
- 本机回环 HTTP 证明请求不声明工具、不会分派工具、schema 不符拒绝、取消有效、提供方错误正文不泄露；这些结果不计真实模型或完整解耦通过。

## 后端调查

当前安装 `codex-cli 0.159.2`。检查 `codex --help`、`exec --help`、`debug --help`、`debug prompt-input --help`、`app-server --help` 并导出含 experimental 字段的 JSON schema。公开方法包含 thread/turn、模型目录、账户和进程控制；本次未发现独立无工具文本推理方法或对应工具选择参数。此结论限于已安装版本的可见接口，不推导其他版本或私有接口不存在。

协议导出位于忽略目录 `.evidence/parser-decoupling-p1/protocol`，不涉及模型调用。CLI 报 Codex 临时 PATH alias 目录写入失败，但上述 help 与协议导出完成；未修改安装、登录或真实会话。

当前 parser `codexexec.Args` 已明确说明无法提供完全无工具保证。按 Spec，不能以该实现换名或仅删除事件检测作为替代后端。

[官方 Responses 接口](https://developers.openai.com/api/reference/resources/responses/methods/create) 提供 `tool_choice=none`；[结构化输出](https://developers.openai.com/api/docs/guides/structured-outputs) 支持 JSON schema。这构成一个候选纯文本推理入口。原型只发送显式文本、schema 和空工具列表，没有本地工具处理器，也不读取 Codex 登录凭据。

仅检查了当前进程 OPENAI_API_KEY/CODEX_API_KEY 等候选变量是否存在，未打印值；未发现可用 API 凭据或已指定的独立推理服务。本机公开 Codex 配置的 model 为 `gpt-6.1-sol`，未配置自定义 provider；这不是该模型可经当前账户 API 调用的证据。

## 已执行检查

| 命令 | 结果与范围 |
| --- | --- |
| `go test ./tests/experiments/textanalysis -count=1 -timeout=30s -v` | 5 个顶层测试及候选反例通过；仅本机回环假服务、合成凭据 |
| `go vet ./tests/experiments/textanalysis` | 通过 |
| `go test ./internal/execution/resultacquisition ./internal/execution/reliable/operation -count=1 -timeout=60s` | 两个原实现包基线通过；未覆盖完整 application/owner/真实审计闭环 |
| `git diff --check` | 通过 |

首批受限环境执行回环测试时请求无法到达假服务，取消测试还因夹具只等待开始屏障而超时；已修正夹具为同时接收提前失败，并在允许回环连接的执行环境重跑通过。没有通过放宽断言取得成功；本次失败保留为环境/夹具记录，不计真实后端失败或通过。

原型覆盖：显式输入不变、空工具/无持久对话请求、非法/额外字段/缺失字段/超限拒绝、动作型输出拒绝、在途取消、HTTP 错误无隐藏重试和诊断脱敏、坏输入/缺凭据零派发。没有完成并发/业务事务/停止竞争/服务重启测试，因为对应主链尚未实施。

## 后续用户决定

用户明确要求“parser 继续用现有 Codex 登录和 CLI，不要引入新的 API 服务或密钥。工具禁用和进程管理封装在 parser 内部，不耦合正常执行链路”。这一指示替代先前 P1 的强无工具 API 前置。Spec/Plan 已相应修订，原型退出实施范围；历史测试通过不能代替当前 CLI 验证。
