# 运行时单文件汇报原始证据

`r2/summary.json` 是修复构建的正式总结果：failed，7 项实际执行、6 项通过。本机 Orca 的业务准备受初始终端回执缺失阻塞；它的原始失败和严格资源未核实结论均保留。

- `source-files.json`：生产、脚本、测试文件摘要；文档及诊断工具不参与。Runner/driver 的实际摘要另见 r2 构建及各场景 source-identity。
- `r2/controlled/`：23 项 Controlled 原日志和进度专项公开响应，不能代替真实环境。
- `r2/<scenario>/`：冻结 manifest、原始 native 日志/会话或 Orca CLI 记录、公开请求/快照、故障及收尾、原持久执行记录。没有部署远端 helper。
- `r2/orca-local.progress-file/manual-cleanup-*`：核验原 worktree instance 后精确关闭独有终端，清单为零；服务已退出，但严格原生绑定结算仍未核实。
- `r2/progress-parallel-verification.json`：修复构建没有成功 L3 写者窗口重叠，不作完整并行资格通过。
- `r1/`：首次构建失败、修正回归和清理；旧构建 C/D 真实写者窗口重叠 1.7607439 秒，两项通过。不能替代新构建。
- `probes/`：独立真实文件传输探针结果，包括首次失败与完整清理。无模型、不算 L3。
- `owned-process-census.json`：有限、只读的进程命令行核对；不是 native 结算证明。

`r2/files.json` 记录该摘录的逐文件摘要。完整原始批次位于任务仓库 `.evidence/progress-acceptance-r2/`，各新建现场仍保留在正式 summary 的 C:/srt 路径。详细实际结果、准确命令及未验边界见 [验收记录](../../runtime-progress.md)。
