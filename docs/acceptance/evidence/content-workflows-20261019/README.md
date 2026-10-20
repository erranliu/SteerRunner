# 内容流程交付证据

唯一结论见[专属验收](../../content-workflows.md)。本目录不含 EXE、认证或原生会话文件。

- `build-manifest.json`：交付 EXE/原生 EXE 摘要、版本、capability 来源；当前生产源码重建与真实受测二进制一致。
- `capabilities.json`、`*_input.schema.json`、`generate.request.json`、`conversation.request.json`：实际公开契约和经生产纯 gate 验证的最小示例。
- `rules.log`、`content-tests.log`、`owner-tests.log`：单元与相关 owner 回归。
- `controlled-*`：交付构建三个受控场景；10 个生成分支、原会话多轮/中断/重启/关闭、问答与未知发送/中断零重发。
- `native-*`：真实 Codex 0.159.2 / gpt-6.1-sol，1 个结构化生成业务轮和 5 个原会话轮次，含中断后继续与服务重启后继续；每项保留完整公开 RunView/历史及来源。
- `service-status.json`、`verify-stopped.json`：最终公开只读资源核验，两个 Run 均已结算。
- `development-regression*`：原 development 的运行、未知结果停止和重启授权回归。
- `application-aggregate-timeout.log`、`application-core-partial.log`：额外聚合回归的原超时，不能当作 package 成功。
- `application-git.log`、`application-tail.log`：耗时 Git 子集与剩余 17 项补验通过；最终内容单元另外通过。

原完整 caller CLI 审计日志及可执行夹具保留在本机 `.evidence/content-final-receipts`。
这些测试证据不成为 SteerLine 的数据库或原生历史访问接口。正式接入只使用公开 CLI。
