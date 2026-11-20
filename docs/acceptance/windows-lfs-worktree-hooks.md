# Windows Git LFS worktree hooks 修复

2026-11-19，真实 308 文档维护在实施及独立验收后阻断：新 worktree 含未跟踪的 `dev/null` 四个 LFS hook，干净门禁拒绝发布。微型本地 LFS 夹具以相同创建命令复现，原证据为 `C:/SteerlineDebug/wt-9ed5-308-docs/diagnostics/lfs-hooks-01/result.json`。

`internal/execution/workspace/prepare.go` 的临时 `core.hooksPath=/dev/null` 改为 `core.hooksPath=` 加 `os.DevNull`；Windows 使用 `NUL`，Unix 保持 `/dev/null`。不持久改项目 Git 配置，不禁止 LFS 内容过滤，不清理用户文件，也不放宽工作区干净门禁。

新增 Windows 真实 Git LFS 回归检查实际 LFS 内容物化、固定 HEAD、工作区干净、无 `dev/`，以及既有 checkout hook 未执行、原字节未变化。Git LFS 是显式前置，已在测试指南声明；微型对象完全本地，无模型或外部网络。正常本机权限完整 workspace owner 包 14 个顶层测试通过，原 JSON 为 `C:/SteerlineDebug/wt-9ed5-308-docs/diagnostics/runner-workspace-fixed-01/tests.jsonl`。此前沙箱中的 junction 权限拒绝及 MSYS/LFS clean 中断没有改断言或计为通过。

独立只读审核 `/root/review_308_fixes` 通过此生产差异及真实 LFS 回归；同时审核 SteerLine 的 nullable question 契约修复。本轮不改 Runner 提示词、Schema、原生协议或初始化锁。

固定 EXE 为 `C:/SteerlineDebug/wt-9ed5-308-docs/artifacts/steerrunner-windows-lfs.exe`，source_commit `8ebbc717776eb2ad85371060566776c4402773fa`、source_dirty=true，包含此前启动索引复用修复。生产源码摘要 `75824ff79737b01e5fd974f4ea91dcc098a4095f059f6a8f6d757d0ed53de9d3`，SHA256 `31153b1f865364fc1c6a0ddef84e2596f2d67222a3ac25c904af795f853dd10f`。未部署。

修复后，使用全新 308 克隆/home、真实 Codex、正式环境检测和原项目时限，完成同一处过期文档的选题、实施、验收、draft PR 发布及独立评审。原始选题再次返回 question:null，经同时修复的 SteerLine 契约成功接受；实施 worktree 干净，无 dev/，仅修改 `docs/Code/persistence-ownership.md`。提交 `13394163a6d5e5d7935d07a4d1d0fc62099a39cb` 已交付到 [308 draft PR #463](http://192.168.0.66:3000/zymobile/merge-h-308-code/pulls/463)，目标 AIRefactor，未合并。最终独立评审建议 insufficient_evidence：缺少源码/路径核验材料，不宣称推荐合入。

真实链原证据为 `C:/SteerlineDebug/wt-9ed5-308-docs/packages/run-04/evidence/test-result.json`、`final-view.json`、`pr-verified.json`、原 Runner inspect、`verify-stop.json` 和 `cleanup-summary.json`。六个 Run 的资源结算已验证，隔离服务停止，认证副本移除，PR 留作审阅。原安装任务、原 308 工作区和安装二进制均未修改。

这是缺陷定向与真实消费方集成验证，不冒充 Runner 正式完整 L2/L3；此前[索引复用专项](codex-startup-index-reuse.md)中的完整 L1/Controlled 失败仍保留，不改报完整资格。
