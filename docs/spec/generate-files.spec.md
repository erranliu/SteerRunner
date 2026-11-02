# generate/1 冻结文件材料

2026-11-02，由 SteerLine Roadmap 材料自动准备需求授权。复用现有内容材料持久化和生命周期，generate.materials 新增 kind=file；conversation 与开发文档类型不扩展。正文写入原 Run 私有输入文件，提示只给读取位置和摘要。独立文件 manifest 不混入 Git 来源；严格路径、原字节、上限和恢复核验，不重建丢失文件，不重复未知请求。公开能力/schema/Run/结果都可核验，旧固定请求不重渲染。

内容结果采用的新增材料检查只作用于内容 workflow，不影响 execute/1 原生 final 和冻结输入预算规则。组件、受控、真实读取及停止证据见 [验收](../acceptance/generate-files.md)，接口见 [generate](../workflows/generate.md)。
