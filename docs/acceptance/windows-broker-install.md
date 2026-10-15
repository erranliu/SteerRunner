# Windows 安装启动修复验收

本次只修复 WMI 独立启动时额外启用调用者原有特权导致的拒绝，不改变任务流程、原生会话或模型调用。

| ID | 自动验收 | 方式及通过条件 | 结果 |
| --- | --- | --- | --- |
| WB-A01 | 保持调用者授权 | Windows 真实 token duplicate：额外启用后还原，缺失特权拒绝，缺少启用权限时拒绝且不启用，原 token 未改变 | 定向测试通过，含 does-not-enable-child 反例 |
| WB-A02 | 原启动意图保护 | 现有 TestSRStartupIntent，未核实原启动不允许重试 | 定向测试通过 |
| WB-A03 | 真实独立服务 | 实际 EXE 在独立 `.evidence/token-diagnostic/state` 上 service start/status/stop；ready、detached=true，原实例退出 | 通过：ready/detached=true，后续查询仍 ready；原实例停止，状态 not_running |
| WB-A04 | SteerLine 三项目集成 | 安装中的实际 EXE；三个项目 ready、各自 Runner ready；正常 stop 后原实例退出，再部署启动 | 通过：实际 BAT 更新退出 0；clean 停机、5 个原进程按创建时间核实退出，新宿主/Web/三项目和 3 Runner ready，detached=true；任务和 profiles 摘要不变 |

证据：`.evidence/token-diagnostic/result.log` 原始权限差异、定向测试命令输出；SteerLine `.evidence/install-migration/runner-probe-ready.json` 及安装目录 `migration/20261015-task-definitions/`。实际用户/组/特权、完整性、restricted/elevated、EXE 身份仍须通过原核验；只在挂起的原返回进程句柄上禁用额外启用特权，不启用、不添加、不去掉授权组。访问历史位 USED_FOR_ACCESS 不作为授权差异。

本次没有执行 Codex 任务、PR 或恢复验收，既有失败/待验结论不变。

实际 EXE SHA256：`fea55559847e7c460c5afeb9d07b5607a5c9f473254ace9df407da0aa98906e8`。安装时构建为 `af545563` 加当时尚未提交的修复，版本记录如实带 `-dirty`；后续提交不改写该构建身份，也不构成 Release 发布。最终证据：SteerLine 安装目录 `migration/20261015-task-definitions/deployment-verification.json`、`verification.json`、`oneclick-deploy-final.log`；隔离探针停止证据为 SteerLine `.evidence/install-migration/runner-probe-stopped.json`。


## 提 PR 前复验（2026-10-15）

命令：`go test ./internal/host -count=1 -v`，Windows/amd64，独立 token 与测试临时目录。输出为 2 个顶层测试、1 个子场景，全部 PASS、零 SKIP；包结果 `ok .../internal/host 1.504s`。证据 `.evidence/token-diagnostic/pr-host-tests.log`。

新增 `does-not-enable-child` 把调用者原已启用的一个权限在复制 token 中禁用，要求拒绝不同授权且保持子进程该权限禁用；原 token 仍与测试前一致。已移除权限同样拒绝；WMI 额外启用的已有禁用权限可以还原。现有未核实启动意图保护同时通过。

`git diff --check` 通过。本轮未改宿主实现之外的执行代码，未重复 Codex/PR/live/R1，WB-A03/A04 保留原来实际 EXE/安装包的验证版本，不追认新的业务闭环。
