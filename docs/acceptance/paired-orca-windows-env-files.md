# 配对 Orca Windows 环境检测文件执行

正式 Run `d63b4aa1dbe776d7d4435da2dd2a748d` 在项目入口启动前触发 Windows 命令长度限制。本机和配对 Windows 现在共用 `invoke.ps1` 与 `input.json`，均由 Windows PowerShell 的 `-File` 入口执行。配对终端只在命令行传启动脚本，检测数据通过每段至多 8 KiB 的环境值送达。启动脚本在仓库外落文件，清除子进程环境里的传输变量，包装脚本读取输入后在项目检查前清除启动文件。Runner 仍沿原终端、退出码和封存结果对账。

2026-11-16，台式机64G 的配对 Orca 以独立 Runner 与独立 Git 仓库处理 49,190 字节的初始 JSON。夹具入口核对全部 48 KiB 数据、尾部标记、传输变量和暂存文件清理，然后写回结果。

| 真机 Run | 保存结果 | 终态 |
| --- | --- | --- |
| `8303c4f41e8551f38da02663aa8cbcc2` | `{"matches":true}` | completed / stopped |
| `31d656ee4538dfa1da939f4b4605afe9` | `{"windows_files":"pass","seed_bytes":49152}` | completed / stopped |

`go test ./internal/execution/environmentcheck ./internal/host -count=1` 通过。Windows 回归覆盖本机与终端启动、同一大输入、材料错误和仓库外临时文件清理。真实 Runner 构建 SHA-256 为 `32d19482a667383bc7313e9f802168357b81b101c7b68c345299d44a35c9844f`。独立工作树 `.evidence/orca-windows-env-files-live/` 保留构建回执、请求、原 Run、结果及独立服务的停机回执。
