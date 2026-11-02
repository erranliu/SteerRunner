# generate 文件材料验收

2026-11-02，基线 `66cc0acf50b675c75458d9d6281139e16158a3a3` 加本轮工作树。复用原 Document/Material/输入文件 owner，原 C:/Project/SteerRunner 未修改；当前独立源码在 C:/SteerlineDebug/wt-762b-roadmap-materials/runner-source。证据根为 C:/SteerlineDebug/wt-762b-roadmap-materials。

## 实际结果

- `diagnostics/runner-owner-current-20261102T155337968Z/go-test.jsonl`：contracts/resources/application 定向通过。含 file 路径/保留名/大小写/前缀、原字节/不内联、缺失/篡改/恢复、Windows junction 首次准备及同内容重定向拒绝、原 execute 冷停止。
- `test-results/runner-L1-20261102T072435495Z/summary.json`：完整 L1 保持失败，framework/native compile/零匹配守卫通过。workflow boundary 引用不存在 conversation 目录、resume 缺冻结 budget 均在未修改基线复现（diagnostics/runner-baseline-boundary-20261102T155221618Z、runner-baseline-20261102T074257673Z）；另有 Orca sample-feed 时间断言，基线定向复跑通过，仅记录为不稳定失败。未修改这些测试或放宽检查，不宣称全 L1/全 L2/发布资格。
- `diagnostics/live-platform-digests-20261102T155125711Z/summary.json`：SteerLine 正式生产调用链 + 真实 Runner/Codex，229.971s，通过。工作树草稿随机 SumInput51fbe397 未进入 Git，解析及文档实际包含该标记。原 generate.materials 的 file 请求及 Run/content_result.file_materials 保存原始证据；源码 manifest 独立。两次 generate、六份文档、隔离 Git/provider 发布合并、SQLite 两任务一依赖、正常停止资源结算通过。
- 实际解析 Run=7cfda41580f5cc643a07b91a6674b24e，文档 Run=1b83695dc0d5edf65ae68403a1ccbbbc，均 completed/stopped。evidence/runner-settlement.json 的 resources_settled=true。
- EXE `bin/steerrunner-verified.exe` SHA256=df27f15bc61767edcfdd6713b6572d5747d4889cfccaace8846d1dcef3dba9ec，resource digest=a968df6ee06ae6a16650ed9f0eee3ab878e9406a4541ab44fa6548674161c707。

## 正文与审核

既有 content frame 固定指令正文不变，不增加模型行为规则。InputContract 的公开材料描述从 `JSON array: content (escaped caller text), path, sha256 (Runner framing and facts)` 改为 `JSON array: inline content or frozen file location, path and sha256; file bodies are not expanded`（UTF-8 82→97 字节，描述发现能力）。业务接线对 file 仅发送 path/file/sha256 原事实；正文固定在原 Run 私有文件，无 inline 兜底、无二次存储或原请求重构。独立只读审核 review_roadmap_newlines 已检查正式渠道、原请求/恢复/manifest、Windows 重定向和真实调用方接线，修复复审后无剩余发现；未用样本更新代替审核。

曾有 execute/1 回归因内容 final 检查范围过宽失败，现限定为内容 workflow，原失败日志保留。真实链中调用方曾不接受 input_file、提前采用 settling、模型抄错引用摘要；调用方分别修复来源类型、等待正常停止、由平台核验原 Git 后填写摘要，未放宽 Runner 原始字节检查。每次新现场独立验证，没有重发未知请求。

当前仅交付配套源码与隔离验证。未部署安装版，未以本机隔离 provider 代替真实外部 GitHub/Gitea PR 或浏览器资格。
