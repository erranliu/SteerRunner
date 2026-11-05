# 内容结果锁与离线读取再审计证据

`records.json.gz`：8个UTF-8文件，解压JSON 744226字节，gzip 52009字节；SHA256 `83ee46cf42707eb0fe963f3445b37daec7bd9ad36f8b564cb777d95e3f878684`。

格式`steerrunner-content-inspection-reaudit/v1`，files各项含path/bytes/sha256/utf8。UTF-8重新编码可还原原字节；归档后已解压逐项验证长度和SHA256。

- public-stop/application/components：新增真实停止入口回归、46项应用定向、74项组件的原始日志；4个既有现场opt-in跳过，集合有重叠不相加。
- audit：351个既有内部测试、原持久格式/代表编码及Runner/driver/native-fixture的逐项身份比对。三项产物均与[初轮受控](../content-inspection-boundaries-20261105/README.md)相同；沿用该次三项受控及重启四case证据，不将其记为本次重跑。
- wire/source-final/build：148565字节编码与原基线相同；源码和本次正式构建匹配。新增内容仅为内部回归，无生产实现修复。

不含EXE、数据库或认证材料。资格范围及提交授权见[再审计报告](../../content-inspection-reaudit.md)，不代表完整L1/L2/L3或真实模型通过。
