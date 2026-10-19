# 个人多机执行器与 gh 单仓库认证

日期：2026-10-19。用户要求按个人多机执行器边界移除 GitHub App 支持，保留 gh 单仓库 broker 和 Run 隔离，并更新产品定位、契约、文档和测试。

## 交付范围

- 在本人管理和信任的预配置主机上执行冻结 Spec/Plan；调用方/SteerLine 选择本机或 Orca environment，组织任务与 DAG。
- 工具安装、仓库准备、gh 登录由执行主机用户负责。Runner 管理环境检查、Run、反馈、显式恢复、发布对账、停止和清理。
- 移除 App 引用、私钥/JWT/installation token 签发与撤销、App 特有就绪/停止证明和测试；不提供团队、多租户、集中密钥托管或自动跨机迁移。
- 公共 Run/Check、冻结 Codex profile、远端 helper 配置严格拒绝旧 github_auth（包括 null）；测试 fixture 同样拒绝 github_auth/remote_github_auth，不能静默改用个人身份。
- 升级前用原版本停止并核实已有 App Run/Check 的认证/进程清理；新版本不迁移旧身份，不接管旧 App 凭据。历史记录和 JSON 保留原结论。

保留每 Run/Check 独立、有限期的 gh 单仓库 capability、固定操作白名单、原身份校验、未知投递零重发及 broker/socket 清理。宿主凭据不进入 worker；停止不退出宿主登录，也不影响其他 Run。远端双模型审计、同服务多 Run、Pi/DSH 及显式 sandbox/full_access 保持各自契约。

## 验证要求

组件覆盖旧字段拒绝（对象/空对象/null）、内部 profile/helper 配置拒绝、缺登录零 native 输入、授权到期、跨仓库/跨 Run capability 拒绝、停止幂等和另一 Run 继续、原远端停止 owner 及秘密不进入 argv/持久化事实。正式 L1 包含 PowerShell fixture 旧配置零副作用保护、native driver 编译和零匹配保护；Controlled 保留审计和多 Run。

远端正式复测使用新独立现场和冻结的新 helper，覆盖可运行的非管理员范围：sandbox/full_access 闭环、原会话反馈/PR、双模型审计、同服务多 Run、活动停止、两种权限环境检查及调用方/服务换代。八项管理员故障沿历史缺口登记；不重复发起此前取消的 UAC，不把授权排除或组件通过记为真机通过。

正式构建、命令、结果和有限证据完成后追加于本文。历史验收不继承为本构建通过。
