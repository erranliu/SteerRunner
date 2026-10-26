# GEN-REPO 实施 Plan

1. 扩展 generate 输入与公开 Schema/capability；增加程序生成的 repository_source，保留原结果接口。
2. 复用现有受管 Git 进程读取固定 commit 树和批量 blob，安全物化私有快照；保存 manifest 到现有 Record，接入原目录核验和结果验收。
3. 对 generate 最终文本统一实施 1 MiB 上限，继续原 Schema 和完整 final 检查。
4. 受控测试覆盖固定提交、脏工作树/后续提交隔离、多文档、Schema/缺失/超限/漂移失败、原请求重放、停止与重启；运行现有相关回归。
5. 尝试本机原生 Codex 调查验证，单独记录实际执行/未验范围。构建 EXE 并记录版本、SHA-256、源码 commit 和调用示例。
