# PR #93 完整回归修复证据

`records.json.gz`：52 个 UTF-8 文件，JSON 10,650,079 字节，gzip 911,364 字节；SHA256 `13cad7a5ea2bf1239f1db7ffb45e242486ede7cceb26379b17fcaaba1a6e8298`。格式 `steerrunner-pr93-repair-evidence/v1`；files 每项包含 path/bytes/sha256/utf8，UTF-8 重编码可逐文件核验原字节。

- 最终正式入口为 `scripts/test.ps1 -Level @('L1','Controlled')`，独立目录 `.evidence/pr93-repair-full-20261103-02`，status=passed、validation_scope=complete；完整默认清单，无资格复用。
- 源码为 `8b34c59` 加冻结的 11 文件修复，暂存 diff SHA256 `6a76066643e29a6bab034f58cbdbb6fcf5e770cb9bee4ae67f5d44aec1922839`；逐文件 SHA 记录在 frozen-repair.json，测试前后均一致。提交前执行，构建如实记录 source_dirty=true。
- L1：36 包、780 顶层测试通过，3174 父子 pass 事件；17 项明确 opt-in/helper skip，无失败或意外跳过；框架27项、终端扩展29项、原生驱动构建与零匹配拒绝守卫通过。
- Controlled：27 个顶层全部通过，零缺失、额外、重复或 skip；完整子例/timings、原始 integration.log、Runner 构建身份和正式前置资格指纹分别保存。
- 保存原完整补测失败、基线对照、修复初轮失败及最终定向/完整结果；初轮新增审计把零派发时缺少调用日志误判为失败，后以原持久 queue/calls_started=0/资源缺席及停止事实修复，不改写原失败。
- process-check.json 与 fixture-process-check.json 核查本轮固定测试 EXE 和46个原临时目录，无遗留进程。无 EXE、数据库、凭据或真实 Orca 认证帧；原完整现场文件在上述本地证据目录保留。

Go 1.26.4、Windows/amd64、GOMAXPROCS=4；未更改场景/产品期限，不计真实 L2/L3 资格。原 B5/B6 原生场次继续绑定原构建；本次范围和修复说明见[验收末节](../../orca-native-rollout.md#2026-11-03-pr-93-完整回归修复)。
