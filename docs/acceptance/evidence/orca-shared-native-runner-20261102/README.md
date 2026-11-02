# Orca B5 证据

`records.json.gz`：424 个 UTF-8 文件，原 JSON 22,711,534 字节，gzip 677,644 字节；SHA256 `c0db7cb0301162b2156c0bc57acf45a1ed16170b9f49b28a8a251c4b46cb5c7a`。格式 steerrunner-b5-evidence/v1，files 每项 path/bytes/sha256/utf8；UTF-8 重新编码可复核每个文件。

- source-final、build sidecar、commands：最终 ac361dd 的源码/EXE/命令身份；EXE 不归档。
- test-statistics 与各 jsonl：初始失败、最终受控回归和所有 skip 分开，零匹配与编译不计真实资格。
- scene-ledger、各 Manifest/准备及 evidence：最终 09e7cd2cbd8d、4e43581c5513、fb72b0846670、d7ca2e402fc3 四场；历史成功、失败和仅准备场次另列。
- 最终 active-original、accepted/feedback、GitHub/精确 Git 对象 checker、online-stop/service-stop/offline、原清单和 call-audit 均保留。测试 PR #754/#755 未合并；历史 #752/#753 不平移资格。
- public/derivations：CLI repo-list 中无关注册仓库行已裁除，记录原始源 SHA256 和可复核公开副本；不修改相关操作/请求/原资源证据。完整原始日志仍在本地 .evidence/orca-b5。适用合同和未验证边界见[验收](../../orca-shared-native-runner.md)。

无凭据、认证帧、EXE 或 Runner 数据库。各测试集有重叠，不相加。
