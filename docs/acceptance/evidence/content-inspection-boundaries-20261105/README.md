# 内容结果锁与离线 inspect 证据

`records.json.gz`：45个UTF-8文件，解压JSON 3,702,350字节，gzip 235,952字节；SHA256 `01b41a40199f730b8ed53160a22895a3ef5946118d180e4f2b8b5340a4bbfa7e`。

格式`steerrunner-content-inspection-evidence/v1`，files各项含path/bytes/sha256/utf8。逐项将utf8重编码为无BOM UTF-8即可恢复原字节；归档生成后已解压逐项核对字节数和SHA256。

- source-before/source-audit/source-final：固定main f6a389c、原源码/351个既有内部测试摘要、持久声明和代表编码核对；最终源与正式构建摘要一致。
- A/B原始jsonl及test-statistics：A独立验证后才实施B；集合有重叠，不相加。4个原生现场opt-in跳过，不算真机通过。
- SQLite初轮断言失败、原InspectOffline overlay复现与修正后同组通过；fixture-baseline-failure在原StagedBusiness上复现主线新提示词未知阶段，fixture-fixed保留新旧分支通过。
- controlled/controlled-final：原失败/中止日志及构建身份；不计完整通过。superseded-cleanup保留最后原Run公开stop、停服和独立verify-stopped的结算结果。
- controlled-qualified：最终同EXE的3项受控、重启4case、result.json exit0、原生调用记录、命令及进程/fixture身份。Runner SHA e40b074f…，source_digest 7407e052…；完整值见[验收](../../content-inspection-boundaries.md)。

不包含EXE、数据库或认证材料。完整L1/L2/L3和真实模型未运行，当前变更位于工作分支供审阅。
