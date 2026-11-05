# 开发首轮提示词精简验收

普通 development/2 的新实施输入包含 Spec、Plan、实施验证指令、原结果识别需要的末行约定和运行文件绑定。静态参考资料、项目策略和测试入口由 Agent 从固定 worktree 查找。新渲染选项随状态定义冻结，旧 Run 继续使用原定义。审计开发继续使用原阶段契约。

SteerLine 提交前核验固定材料和正式写入器，首轮测试上下文只提供安排 SHA-256。SteerlineTest PR #771 的 Native 101 Plan 调用统一脚本，负责校验安排、接入绑定、清零进度、调用原入口并核验五项完成状态。

2026-11-05 合并前验证基于 Runner a8284d13 加本 PR 改动。Workflow、application、Orca transport 定向 Go 测试通过，覆盖旧冻结定义、首轮正文、文件变量和预算校验。这些结果属于定向验证。

配对的 SteerLine 基线为 2610963f。使用 SteerlineTest 固定提交 199803191462a128577af246a3ef70d27bc751ed 的远端 Orca Run eac715ac29d6d5c78da32c2b9e2f7389 通过，Attempt 为 c77ca7c25825dc9364acf221d023c2f7。首轮从原阻断记录的 2,153 B 降至实际下发的 573 B。Agent 原实施回复已验证，报告统一入口退出 0，Runner 独立观察到五项进度完成和 retry 的 true → false → true。停止后封存的 111 字节结果与提交的安排完全一致。服务退出和 Orca 会话资源结算已由 service verify-stopped 确认。

该次 Runner EXE SHA-256 为 70e25fcb73fc446cc30c8df8d522319be124f6c8d4764290fc1e721b201e26b1。测试安排 SHA-256 为 b9a819296f6f3c6b0e9a3ff9ce4059f89a111a9a7633ed73baa373d0475f6cc4。现场验收覆盖实施、测试进度和结果封存，在发布阶段开始时停止。脚本化接入覆盖 Native 101。

证据位于本次独立目录 native101-scripted，包含 first-prompt.txt、native-evidence.jsonl、steerrunner.exe.build.json 和 service-verification.json。
