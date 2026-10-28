# 项目策略阅读示例

这是内置 workflow 行为的说明，不是可加载配置。SteerLine 继续提交原 Run 请求，无需新增槽位或路径字段。

假设固定 base commit 中存在：

```text
SteerlineFlow/runner-policies/git.md
SteerlineFlow/runner-policies/testing.md
SteerlineFlow/environment.md
```

`SteerlineFlow/runner-policies/command.md` 缺失。Runner 固定三槽位的存在情况与存在文件的 SHA-256，不把正文发送给模型。

development/2 的 `implement.execute` 及 development-audited/2 的 `implement.execute`、`audit.code.execute` 在原有提示词后追加：

```text
阅读项目策略：SteerlineFlow/runner-policies/git.md。
阅读项目策略：SteerlineFlow/runner-policies/testing.md。
```

两种开发流程的 `publish.execute` 只追加：

```text
阅读项目策略：SteerlineFlow/runner-policies/git.md。
```

补报、解析和其他步骤均不追加。缺失的 `command` 沿原 workflow 规则；独立的环境文档不会成为策略引用。三个文件都缺失时，原提示词没有策略相关增量。文件存在但不可读时明确报错，不改用其他目录的同名文件。

原始阅读模板为 `\n阅读项目策略：%s。`。示例中的每行及换行均计入 Runner 提示词预算。文档配置不等于环境就绪；项目声明和检测环境能力，SteerLine 执行准入，Runner 保持原权限、发布授权与通过标准。
