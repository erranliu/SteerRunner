# 固定版本只读 generate（Roadmap 内部代号 GEN-REPO）

扩展 generate/1，可选 `generate.repository={local_path,commit}`；commit 必须是本地仓库可解析的完整提交对象 ID，不能与 directory 并用，且必须提供 output_schema。不引入任务、发布、业务文档格式或新生命周期。

接受时只从 Git 对象读取树和 blob，在 Run 私有目录物化无 .git 的精确快照，不读工作树文件、不修改源仓库。拒绝链接、子模块和不安全/冲突路径；固定上限 4096 文件、64 MiB blob 总量。记录 commit、tree 和每文件 blob ID、SHA-256、大小；派发、恢复和接受成功结果前核验快照。原有 Codex read_only 权限与完整 final 取证保持适用；只读权限不是读取范围隔离。

结果仍通过 inspect/history 的 content_result.json 交付，完全由调用方 output_schema 定义多文档路径/正文、数量、必填内容和业务规则。Runner 不另加业务包络；调用方应在 Schema 中精确约束预期路径和非空完整正文。新增 content_result.repository_source（和 Run repository_source），提供独立程序生成的版本及文件依据。最终文本限 1 MiB UTF-8，缺完整 final、坏 JSON、Schema 不符、超限和快照变化不能成功；不自动修复或重新生成。语义截断只能由调用方约束和验收判定。

沿用原请求账本、原命令、原会话、停止和重启对账；未知结果不重发。交付 capability/schema、示例、限制、受控/真实验证记录和可追溯 EXE。版本 manifest 编码后额外限 1 MiB，保证原 inspect/history 传输可完整容纳结果和来源。
