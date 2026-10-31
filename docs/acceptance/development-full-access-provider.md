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

## 授权后的网络环境修复

用户随后明确授权设置远端 Orca 的代理环境并重启复测。为远端 `liuming` 用户设置 `HTTP_PROXY/HTTPS_PROXY=http://127.0.0.1:7897`，并设置 `NO_PROXY=localhost,127.0.0.1,::1,192.168.1.151`。原用户环境值备份到本次专用工作树的 `proxy-env-backup.json`，未读取或修改登录凭据，模型和 effort 不变。这些是授权后保留的用户环境设置，不是产品自动迁移。

先按监听 6768 的 PID、路径及创建时间重启配对主应用；新终端仍未继承代理。进一步通过本次诊断 shell 的祖先链确认其父进程是原独立 terminal daemon，再按原 PID、路径及创建时间重启该 daemon 与对应主应用。未按进程名批量终止其他 Orca 实例。一次性重启任务均自行注销；新终端核验 task 不存在，HTTP(S)_PROXY 与 NO_PROXY 实际继承，ChatGPT HTTPS 握手完成（未认证 HEAD 返回 403，curl exit 0；不作为模型验收）。新 runtime 为 `c7911c80-1c69-42bb-ab2e-db16cf81e02f`。[授权变更与新终端证据](evidence/provider-metadata-20261031/network-restart.json)。

随后用原生 `codex`、同一固定 Runner、相同冻结配置 SHA `3ed9cd636c8c2fe046ada1ce4c6d269a62fbeef2c06c1989633814a0e6d38bfe`、新现场 `C:/srt/6c95d7aadfcc` 复测真实 `orca-remote.smoke/default`。未使用临时 launcher wrapper 或向测试终端额外注入环境命令；环境来自重启后的原宿主。原始协调输出在 `.evidence/provider-proxy-remote-smoke`。

## 原生启动审计消费者修正

授权重启后的[首轮完整 smoke](evidence/provider-metadata-20261031/remote-proxy-first-summary.json)通过 readiness，实际交付专用测试 PR [#717](https://github.com/erranliu/SteerlineTest/pull/717)，随后在冻结启动参数审计失败，未进入 driver 合入阶段。[现场结果](evidence/provider-metadata-20261031/remote-proxy-first-scenario.json)保留失败及 Run/服务停止事实。

原因是测试侧 `ReceiptAuditPolicy` 仅编译裸原生命令；生产冻结阶段还会添加两个精确的文件通道包装器，在通道关闭时清除继承的 `STEERLINE_PROGRESS_FILE/STEERLINE_RESULT_FILE`，退出后还原。测试预期漏掉了这些包装器，错误拒绝实际冻结命令。修正预期以包含关闭状态的完整固定包装器，继续逐字比较；没有剥离任意脚本或放宽权限、模型和环境恢复核验。

使用首轮真实 journal 中的启动命令新增回归，修复前复现，修复后整个 livetest 包通过；另验证缺失包装器、意外启用通道、恢复变量/模型/权限变更及追加命令均拒绝。原始红绿日志在 `.evidence/full-access-diagnostics/remote-audit-{reproduction,fix}.log`。

另一项遗漏由整份真实 journal 的离线复核提前发现：生产使用有边界的 bracketed paste 和单独 Enter 两步协议，测试审计把后者误当作缺少文本的业务输入。现在仅在原 native launch 策略下，把同一原终端、同一环境的一次粘贴与一次 Enter 配对；两次 intent/receipt 仍全部计入完整性，分别核验 runtime、handle、accepted、UTF-8 字节数及原参数。缺失回执或尚未提交的粘贴不能称 complete；额外 Enter、交错粘贴、错目标/环境、控制字节、参数或回执改变均拒绝。没有按“裸 Enter”一概豁免。

新回归先对基线复现失败，再用隔离 Go overlay 验证修复，待基线 L1 结束才修改源码；修复后整个 livetest 包及[原真实 journal 离线复核](evidence/provider-metadata-20261031/native-audit-recheck.json)通过。离线复核记录一个业务工作树、三段文本输入、零辅助终端和完整回执，不把已失败的 smoke 改成通过。#717 已关闭、未合入。原始日志为 `native-paste-audit-{reproduction,fix-tests}.log`。

## 当前交付状态

产品元数据竞态、远端联网前置与测试启动审计消费者均已处理。PR #74 继续保持草稿；将固定新构建，完成更新后的基础验证及新现场完整 smoke 后再转为正式 PR。

其他未执行的 L3、Pi/DSH、audited 及 read_only/parser 真机资格继续沿原审计记录登记为未验证。本机 L2 #716 为此前 `ba18cb7` 构建结果，不追认为本次 `fc20599` 的新真机运行。
