# Orca checkout 操作的重启检查

2026-11-16，正式安装 #167 后，部署的停机和升级预检均通过，Runner 的 `service start` 拒绝原状态目录并返回 `legacy_orca_state`。Orca 的 `checkout_probe` 和 `checkout_fast_forward` 从 #161 起使用 `local_process` 保存过程及结果。旧启动检查按 adapter 判定，错误地将这些记录归到已退役的 Orca agent terminal。

启动检查现在要求 checkout 操作来自 Orca backend，使用 `local_process`，并带有该流程的 `EnvironmentInvocation`。普通 Orca Run 仍沿原协议门禁。重启使用原持久记录，不迁移状态，也无需清除检测结果。

Windows 定向 host 测试通过。覆盖正式 checkout probe、fast-forward、普通 Orca `local_process`、缺调用记录、backend 不符、退役 terminal 协议，以及原独立验收保留的退役协议拒绝和离线证据。

独立真机验收状态目录实际重启新版 Runner，读取原材料核验、快进和检测 Run，三条都保留 stopped。随后正常停掉独立服务并核对资源结算。
