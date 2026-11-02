# Orca B6 最终证据

`records.json.gz`：164 个 UTF-8 文件，JSON 8,775,038 字节，gzip 271,009 字节；SHA256 `03c1429105a5df9ceb231d15f66fe52eea776488e54f42363917aec7dee54232`。格式 steerrunner-b6-evidence/v1，files 每项 path/bytes/sha256/utf8；UTF-8 重编码复核原字节。

- source-final/build、integration：最终 e30fc82 / source_digest cdd04570... / EXE 8af60c55...；集成 main 7f6c538，不平移 B5 旧构建资格。
- 5组 jsonl/test-statistics：受控原始集合、merge 后 application、新公开合同、升级/离线与 skip 分列。
- scene-ledger/Manifest/原正式回执及 evidence：baf0fb631927、d066f33fe011、0b74a34ce9b4、6386ff2b3347 四场；最后两场模型真实开发与 PR #756/#757 未合并。
- 原 Started/working hook metadata、唯一 close/撤销前后send、完整原清单、Run stop/service stop/真实 offline 都保留；准备终端独立收尾。
- public/derivations 为去掉无关注册仓库行的公开副本，记录原始文件 SHA；相关原操作、资源、请求和结果保持，完整原始日志仍在本地 .evidence/orca-b6。

无凭据/认证帧/EXE/数据库。集合有重叠不相加，编译和 skip 不计真实场次；适用合同与保留例外见[验收](../../orca-native-rollout.md)。
