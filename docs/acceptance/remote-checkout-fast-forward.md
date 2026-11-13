# 已有远端 checkout 快进

`checkout_fast_forward` 独立接受已登记的配对 Orca checkout、仓库 URL、remote、分支和预期提交。它核对原宿主与 Orca 占用清单、当前分支、工作区和 Git 操作状态，抓取指定分支并比较实际提交，只允许快进到预期提交。退出状态与结果通过原 Run 封存。使用方自行决定调用时机。`checkout_probe.verify_only` 只核对检测材料并返回 `matches`。

Windows 隔离 Git 测试覆盖有未跟踪改动、分支不符、远端已变化、分叉及精确快进。通过已配对 Orca 对 308 做只读检查，确认会话清单的身份字段和原 checkout 活跃终端的拒绝路径。真实配对 checkout 的写入和检测链路未运行。
