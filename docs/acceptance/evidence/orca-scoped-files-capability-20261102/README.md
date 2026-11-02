# Orca B4c 原始证据

`records.json.gz` 保存 14 个 UTF-8 文件；files 数组含 path、bytes、sha256、utf8，可还原原字节。归档 SHA256 `fc46bbc0ad965938d98e461c3edb784579e897c2ed7146d94320ec4858d671fd`。

- commands.json/source.json：实际命令、先暂存后的源码身份及受控范围。
- new.jsonl：初次 map 访问编译失败，1 个失败包；new-final.jsonl 为修正后的 5 根/21 子节点通过。
- new-scope-final.jsonl：补路径规范化拒绝后的最终 5 根/24 子节点、26 叶例通过。
- regression.jsonl：52 根/131 子节点、159 叶例通过；transport.jsonl：2 根/9 子节点、10 叶例通过。
- dependency.jsonl：1 根守卫；build.log：两个生产入口构建通过。
- compile.log、live-compile.log、live-compile-final.log：初始 tag/编译/测试入口失败记录；live-build.log、live-build-final.log 为改用正确的仅编译命令通过记录。空构建日志只表示命令成功，无场景资格；不归档 EXE。

没有认证帧、业务正文或真实模型操作；结论和历史失败原因见[验收](../../orca-scoped-files-capability.md)。
