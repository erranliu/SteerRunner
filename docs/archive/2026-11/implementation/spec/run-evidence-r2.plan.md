# R2 最小 Plan

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

1. 已读：产品 Spec 的 CLI/状态目录/停止与清理、reliable-modularization Spec、development workflow、现有 workspace Inspect/Verify、Record/Commit 和 CLI/IPC 接线。根 AGENTS.md 不存在，上级 .codex/AGENTS.md 为空。
2. contracts 定义路径、收集输入、产物与 Git 观察；workspace 增加双样本只读观察及受限发现/读取；application 沿原 gate/Commit 保存记录；CLI/host 增加命令。
3. 发布内嵌 schema/capabilities、帮助、README 和最小请求响应示例。
4. 仅跑相关 owner 定向测试和真实 EXE→IPC→隔离 Git/SQLite 的 R2 场景。证据写 docs/acceptance/evidence/run-evidence-r2-20261019；不调用真实模型、不做真 PR、不改 SteerLine。
5. 完成后记录验收/未验范围，提交实现，再从实际 commit 构建 EXE，保存 version/SHA256/resource_digest；不将 dirty 来源冒充最终 commit。

实施完成：contracts/CLI/host/application/workspace/Record 最小接线及内嵌契约已落地。
owner 六包与 R2 application/instance 定向通过；EXE 受控场景已验证两 Run、Git、证据和 stop/清理。
验收与迭代问题见 ../acceptance/run-evidence-r2.md；最终构建身份在 .evidence/r2-release。
