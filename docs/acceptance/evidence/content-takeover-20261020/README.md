# 2026-10-20 内容流程接管证据

[完整验收](../../content-takeover.md)登记根因、命令、固定构建及限制。

- build.json / version-response.json：干净源码 4ad8cd8 的固定构建。
- targeted-pass.log、workflow-acquisition.log：最终通过，早期失败日志另行保留。
- controlled/：两个受控场景及三条同 EXE 接管响应。
- native/：一个真实 Codex generate 的原信封、公开结果、stop、结算及接管响应；caller-commands.jsonl 保存实际 stdout/stderr。
- original/：保留现场原失败/修复响应、绑定比较、181 个文件前后摘要；未启动服务或改写记录。

EXE 留在工作区 `.evidence/content-takeover-20261020/controlled/steerrunner.exe`，不入 Git。
SHA256 为 `bb4e189db25d730050d5bd3b5dbf1e93eba4872b923687ff8b2e00c96dec35e2`。
resource digest 为 `a48bfb1aa7250009b21b4170532d877be01c23375ac386cb39e269794263ef91`。
manifest.json 记录证据文件摘要，不包含自身。
