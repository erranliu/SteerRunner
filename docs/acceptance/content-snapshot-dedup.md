# 内容快照检查点去重

`generate.repository` 在输入组装、派发前和恢复时，各只扫描一次冻结仓库快照。派发在保存发送意图前检查，恢复以原快照重新核验，原生最终结果验收时再次独立检查。目录身份、清单、文件类型、长度、SHA-256 和上传材料的原字节检查均适用。跨检查点不复用校验结果。

## 合并前验证

`TestContentRepositoryCheckpointsRejectEqualSizeRewrite` 使用真实 Git 固定提交生成快照，再把其中的 `code.txt` 等长改写：

- 输入冻结后、派发前改写。进入派发流程后记录 `source_unverified`，没有保存发送意图，也没有调用原生端。
- 撤销授权后、处理恢复前改写。恢复请求失败，Run 仍需显式恢复授权。
- 原输入派发后、结果验收前改写。原生最终证据被处理，内容结果以 `repository_snapshot_changed` 判为失败。

上述三个分支均通过。Application 和 workspace 的内容、上传材料、快照及运行时文件通道定向回归也通过。

受控公开接口的 `TestSRContentRepository/complete` 分别用本次构建和未修改的 main `c16da57d` 运行，两边均在原生回合启动前以 `incomplete workspace intent` 结束。当前启动路径将只读内容目录送入 Git worktree 启动核验，场景未抵达快照派发检查。这个对照用于确认基线问题，公开接口集成验证未通过。
