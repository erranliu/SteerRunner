# 宿主认证修改审计与 L2

日期：2026-10-31。审计比较 `cf8df21..55b1358`，产品源码固定于 `55b13585230824c04ca5910020274a8dd1b20806`。测试消费者修正为 `d864a04bb2ee7179f1d8ad4a3f5db5d08179933c`；只有 tests/native 两个文件变化，生产源码摘要不变。后续文档提交不作为新的产品构建。

## 审计结论

本会话审阅了认证删除调用链、开发环境组装、read_only/原生远端分版本编码、helper metadata 前置、控制 checkpoint/reader 结算和精确停止、PR provider 查询认证及状态目录切换。未确认新的生产阻断缺陷。内容/parser 的只读权限与原生远端 profile 4/state 7 保留；初始化缺少 writer 证明时仍维持 unknown，不以 PTY 缺席伪造树退出。

真实 L2 确认一项 P1 测试契约遗漏：`auditWorkerSandbox` 仍强制读取已删除的 sandbox-readiness.json 并验证到期时间，正常 worker 到达 pr_ready 后被旧测试拒绝。修正为将真实 turn-start 回执的 native turn ID 绑定到同一 session/workspace 的原生 history；保留原权限、审批、提权、bypass 和实际命令错误审计。产物改名 native-turn-audit.json，multirun 消费者同步。没有补造认证证明、忽略命令失败或放宽实际权限审计。

## 固定构建与前置

Runner SHA256 `0d554cd5a94ce6b5c772515e9b88d27b055fa7c8268d7f5b83c40ecbabe727a8`，干净产品提交 `55b1358`，源码摘要 `bf0c745aba28e40947173d3cfd36718071212add6187edf8eb33d98a71b3db37`。[构建](evidence/host-native-auth-l2-20261031/runner-build.json)。

正式 `.\scripts\test.ps1 -Level L2 -Config .\.evidence\host-native-auth-l2-config.json -OutDir .\.evidence\host-native-auth-l2` 的完整前置通过：L1 717 个 Go 顶层测试、29 个扩展测试，原生 driver 编译和零匹配 guard；Controlled 精确 25/25、零跳过。[原批次](evidence/host-native-auth-l2-20261031/initial-summary.json)保留后续 L2 审计失败状态，没有改写成通过。

修正仅影响 native test driver，故复用已通过的同一 Runner、配置/helper 摘要和前置。新 driver 编译及零匹配 guard 重新验证；按原 `scripts/run-test-case.ps1 -Request <冻结 case job>` 入口，在全新现场重跑唯一注册 L2 case local-codex.smoke/default。未新增跳过前置开关，未修改框架资格规则。源摘要及具体来源见[资格汇总](evidence/host-native-auth-l2-20261031/qualification.json)、[不变产品摘要](evidence/host-native-auth-l2-20261031/unchanged-product.json)、[新 driver 身份](evidence/host-native-auth-l2-20261031/final-source-identity.json)。

## 真实 L2 结果

使用既有专用 SteerlineTest 配置的独立副本，仅将退役 sandbox 改为 full_access；Codex 0.159.2、gpt-6.1-sol/medium，生产默认提示，无 prompt overrides、无固定 publisher。使用宿主已有登录，没有登录/登出、提取 token 或修改 credential helper。

- 首轮现场 C:/srt/7785b029a677：worker 完成实际提交、push、测试 PR [#721](https://github.com/erranliu/SteerlineTest/pull/721)，在旧准备证明审计失败。Run/服务已停，PR 已关闭、未合入。[失败](evidence/host-native-auth-l2-20261031/initial-native.log)。
- 最终现场 C:/srt/3068b813b9d0，Run `e8bd5542b5dd97abaf4c045785e19e3b`：1/1 L2 case 通过，无缺项或跳过。[case](evidence/host-native-auth-l2-20261031/final-case.json)、[结果](evidence/host-native-auth-l2-20261031/final-result.json)、[收尾](evidence/host-native-auth-l2-20261031/final-scenario.json)。
- 真实测试 PR [#722](https://github.com/erranliu/SteerlineTest/pull/722)，head `7d39afbc93decc0eaa06e9aaade9292a666ffa75`，merge `60e78558f9237208c6ea57084c268958423a303d`，仅合入专用 `codex/srtest/3068b813b9d0/base`。原反馈拒绝零新增输入，合入后 Git 内容及 checker 再核验通过。GitHub 连接器另核实 merged=true。
- 实际命令 24 条、权限错误 0；原生 history 2 contexts/15 tools，提权、审批、bypass 均 0；2 个业务 turn 启动回执与原 history 一致。[命令](evidence/host-native-auth-l2-20261031/command-audit.json)、[权限](evidence/host-native-auth-l2-20261031/native-permission-audit.json)、[回执](evidence/host-native-auth-l2-20261031/native-turn-audit.json)。
- Run 与服务均已停止；宿主 gh 账号仍为 erranliu。保留现场、分支及失败记录供审查，不将失败轮次追认为成功。

该 L2 证明本机 Codex 的真实 Git/gh 登录、实现、发布、核验与停止闭环。Orca local Codex/Pi/DSH、原生远端、多模型及其他 L3 真机组合未在本轮执行，不扩大其资格声明。
