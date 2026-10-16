# 沙箱认证证据索引

本次结论在 [专属验收](../../sandbox-authentication.md)。本目录仅导出非秘密配置、事实和摘要；
原始工具诊断、调用日志及所有失败场地保留在原位置。受控无效 canary 不是有效执行凭据。
下面原有 final/delivery 名称代表各历史时点，当前默认 gh 代理修订以 `host-broker-*`、
`broker-first-*`、`broker-compatible-*` 分别记录源码、实际只读回归、首次真实失败及兼容修复。
`broker-first-formal-summary.json` 保持 L2 失败；其 PR 对账为空、原资源已关闭，原生审计
见 `broker-first-real-worker-sandbox-audit-v2.json`，不是完整业务成功。

- `formal-summary.json`：首轮正式入口；其中 Controlled 早于最后的准备清理边界修复，不计最终源前置。
- `manifest.json`、`source-identity.json`、`scenario.json`、`result.json`、`native.log`、`cleanup-run.json`：
  首轮真实 L2 `C:/srt/02ba5748823f`，缺认证引用阻断；无业务 PR、Run/服务已停止。
- `final-build-comparison.json`、`driver-comparison.json`：当前重建与该 live EXE/driver 字节相同。
- `source-snapshot-final.json`：最终实现/测试源码的逐文件摘要，包含新增未提交文件；
  这是运行时源码取证时间点，不是后来补写验收文档的自引用快照。
- `preparation-blocker-audit.json`、`worker-pr-query.json`：公开 Run 操作、零业务输入记录及
  专属 base/head 的只读 PR/ref 对账。不是发布权限证明。
- `native-readiness.json`、`native-cleanup.json`、`native-boundary.log`：
  `C:/srt/auth-boundary-2007950559` 的生产准备路径实测；无效 canary 明确拒绝、原进程退出。
- `final-components.log`、`expiry-reconnect.log`：最终定向包/原会话到期阻断回归。
- `material-audit.json`：新样本/生成器/发布覆盖的摘要及旧审批认证修补指令检查。
- `credential-reference-inventory.json`、`app-inventory.json`：只读非秘密引用检查。
  403 不能证明账号没有 App，仅说明现有 OAuth 无法枚举 App 安装。

原始正式批次：`.evidence/sandbox-auth/formal-l2/`。
诊断原始场地：`C:/srt/authdiag-402847155`（最初写探针语法失败）、
`C:/srt/authdiag-3783072331`、`C:/srt/authdiag-3971120298`。
清理回归场地：`C:/srt/auth-boundary-3768020956`（清理证据未确认）、
`C:/srt/auth-boundary-1628824273`、`C:/srt/auth-boundary-2007950559`。
不删除或重写其历史结果。

最终固定源第二轮 `.evidence/sandbox-auth/formal-final/` 外层超时失败，没有启动 L2；
`final-before-deadline-fix-summary.json` 保留该失败。其子进程在外层期限后返回 exit 0 / 12，
见 `late-controlled-result.json` / `late-controlled-source.json` / `late-controlled-build.json`；
不将该迟到结果改成正式通过。`late-controlled-process.json` 保存其结束后的查询。
外层现已对齐子进程原有 8m 期限并仅增加 30s 结果余量，新批次位于
`.evidence/sandbox-auth/formal-deadline-fixed/`，失败及收尾见下一段。
`diagnostic-summary.json` 和 `diagnostic-process-audit.json` 分别记录原因分类与原进程退出。
L3/稳定性未执行，本次 Orca terminal 的执行主机准备/确定性沙箱能力仍缺失。

第三轮同样超时失败，见 `deadline-fixed-summary.json` / `deadline-fixed-controlled.log`；
子进程最后一个场景中断、缺 result，未启动新的 L2。`timeout-cleanup-request.json` 和
`timeout-cleanup-final-status.json` 记录原专属服务的精确身份停止及 resources_settled=true。

最终源码在该前置编译后又收口了认证引用恢复、GH_HOST 固定和安全探针文件名。
`delivery-components.log`、`delivery-native-boundary.log`、`delivery-native-readiness.json`、
`delivery-native-cleanup.json`、`delivery-native-process-audit.json` 是最新源码定向证据，
新场地 `C:/srt/auth-boundary-1736698634`；仅无效 canary 负例，无业务输入。
`delivery-build.json` / `delivery-source.json` 是最终构建和代码摘要；不是整体 L2 通过证据。

`keyring-host.json` / `keyring-sandbox.json` 直接证明相同 gh 凭据引用在宿主 CredRead 成功、
MXC 内 ACCESS_DENIED (5)。没有读取/输出 blob；原场地 `C:/srt/authdiag-2631366071` 的
停止事实见 `keyring-cleanup.json`。
