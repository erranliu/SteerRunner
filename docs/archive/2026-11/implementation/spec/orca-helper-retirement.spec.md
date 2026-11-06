# Orca Agent helper 退役

> 2026-11-06 归入历史实施材料（整理基线 `dda9a827`）。正文中的状态、日期、构建和资格按原记录时点阅读；归档不表示所述机制失效，也不把未验项改成通过。现行入口见[文档索引](../../../../README.md)；[迁移清单](../README.md)。

新 Orca Agent Run 使用同一套结构化会话机制。本机和配对远端 Codex 支持 `execute/1` 和 `development/2`。本机另支持 Codex `development-audited/2` 及 Pi 的 `execute/1` 和 `development/2`。发布判定、直接 Codex、独立 parser 和公共终端继续走各自现有入口。

旧 `agent_launch` 可执行文件、非空参数及 helper 路径和摘要在受理前拒绝。保留模型、目标、权限、仓库和发布配置。旧终端 Run 使用原构建完成或停止，在原目录确认资源、未决控制和 writer 均结清。排空后保留原目录，新构建使用独立目录。缺证据的旧执行仍须由旧构建处理。退役删除 helper 可执行入口及新 Run 的终端启动路径，保留旧冻结记录的只读证明逻辑。

本机 Pi 使用 Orca 管理的现有工具和宿主环境。放弃 Runner 原先的固定工具白名单、环境过滤和 Job 写入者证明。原会话一次关闭回执作为该会话范围的停止证据。`execute/1` 在首个业务结束点或显式 stop 采集一次结果文件快照，`development/2` 在显式 stop 采集。快照仅反映读取时刻。Pi usage 原归属尚未核验，报告不可用。远端 Pi 保持不受理。业务反馈通过原 Workflow 续轮。

本机 Codex audited 的结果、实际模型与工作区发布门禁见 [M5 Spec](orca-structured-audited.spec.md)。验收分别记录 [Pi 首轮与续轮烟测](../acceptance/orca-native-pi-smoke.md) 和 [退役验收](../acceptance/orca-helper-retirement.md)。
