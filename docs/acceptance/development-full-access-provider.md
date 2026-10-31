# 远端 provider 元数据启动竞态与联网诊断

日期：2026-10-31。接续[实施后审计](development-full-access-audit.md)，本记录不改写前两次远端失败，也不把诊断初始化计作真实 smoke。

## 代码修复

`QueryOriginalAgent` 将原 pane 暂时缺少 provider/session 关联报告为普通致命错误；`PrepareSession` 因而在关联异步发布前退出。现在将这一情况归类为既有的 `ErrOriginalMainAgentUnavailable`，沿原有 readiness 截止时间重试。缺失期间不发送业务输入；永久缺失仍超时拒绝；原 worktree、runtime、terminal incarnation、pane 和 provider 冲突仍立即拒绝。不增加屏幕推断、私有日志查找或身份回退。

修复前新增测试复现失败，修复后通过，覆盖迟到/永久缺失、pane/provider 改变，以及等待期间零业务输入。代码提交 `fc205994260eb5d7d85e82b76721d2b4d4f48dee`；原始红绿日志在 `.evidence/full-access-diagnostics/provider-metadata-{reproduction,fix-tests}.log`。

## 固定构建验证

[构建身份](evidence/provider-metadata-20261031/build.json)：工作树干净，源码摘要 `c4a8c66349f9fc8be22c283899a76c2dadea50d97877c77e6c8100d3ac432a60`；Runner SHA256 `74a29a03ab91461d80537159300eaf9902e1a87e495145620bbd3bef481b3b88`。后续文档提交不改变该产品源码摘要。

- [完整 L1](evidence/provider-metadata-20261031/l1-summary.json)：726 项 Go 顶层测试、29 项 Node 通过，framework、native driver 编译及零匹配 guard 通过；17 项既有 helper/opt-in allowlist skip。
- [完整 Controlled](evidence/provider-metadata-20261031/controlled-summary.json)：同一固定 Runner，results 1 项及 rest 24 项，精确注册集合 25/25，无缺项、重复或 skip。
- [真实远端 smoke 复测](evidence/provider-metadata-20261031/remote-summary.json)：仍失败，错误已变为有界 readiness 超时 `original native main-agent readiness timed out; no input was sent: context deadline exceeded`。Run `808395709cbaa4f5a511ff0c436e6ec9`，现场 `C:/srt/178033afc94e`。[收尾记录](evidence/provider-metadata-20261031/remote-scenario.json)确认 Run 与服务已停；终端停止仅为 `orca_terminal` scope，不证明宿主登录撤销或全部 OS 后代终止。

## 第二层原因与诊断边界

只在专用 `SteerlineTest` 诊断工作树运行相同 `gpt-6.1-sol/medium` 与固定无工具初始化提示。原生会话能够登记 provider/session，但远端 Codex v0.166.0 模型请求持续 `Request timed out`，主 agent 因而停在 `working`。没有据 UI 空闲外观判为就绪。

[脱敏网络与进程诊断](evidence/provider-metadata-20261031/network-diagnostic.json)记录：远端系统代理已启用 `127.0.0.1:7897`，Orca 终端未继承 HTTP_PROXY、HTTPS_PROXY、NO_PROXY。直接 HTTPS 访问 ChatGPT/OpenAI 超时（curl exit 28），GitHub 返回 200。未读取 auth.json、凭据值或远端私有 transcript 文件。

仅对专用诊断 shell 设置 HTTP(S)_PROXY 后模型可返回 READY，但本机 hooks/MCP 通信出现 HTTP 502，provider 元数据仍不完整。再在该进程添加 `NO_PROXY=localhost,127.0.0.1,::1,192.168.1.151`，使用单独发送的环境设置和完全相同的原生启动命令，已观察到原 pane 的 provider/session 与 done 状态。此诊断没有执行 Runner 真实业务或发布 PR，不能代替 smoke 通过。

曾试验的两个临时启动脚本未取得完整原生资格，已逐个核验 SHA 并删除；未改安装程序、全局配置或登录凭据。所有诊断终端按原 handle/runtime 关闭，证据仅声明 `orca_terminal` scope。原始状态与画面留在忽略的 `.evidence/provider-*-trace-*`，便携摘要不包含私有 transcript 路径。

官方[配置参考](https://learn.chatgpt.com/docs/config-file/config-reference)把 `features.network_proxy` 定义为沙箱命令联网代理；据此没有把该实验选项当作模型连接修复。本次也不为 SteerRunner 新增隐式全局代理选择。

## 当前交付状态

已修复产品的元数据启动竞态，完整基础回归通过。当前真实远端 smoke 仍未通过；原生诊断成功不改变该结论。PR #74 保持草稿，等待远端 Orca 启动环境的代理与本机绕过设置得到授权处理，再用同一固定构建、新现场复测。需要重启远端 Orca 的操作会影响现有会话，故另行请求用户授权；没有代为重启。

其他未执行的 L3、Pi/DSH、audited 及 read_only/parser 真机资格继续沿原审计记录登记为未验证。本机 L2 #716 为此前 `ba18cb7` 构建结果，不追认为本次 `fc20599` 的新真机运行。
