# SteerRunner real-environment smoke

Adapted from SteerlineTest `docs/spec/native-101.spec.md` at
`cebdd5ab552150c49743e40593c836689ecf789b`.

Read all UTF-8 Markdown inputs explicitly as UTF-8. In Windows PowerShell 5 use
`Get-Content -Raw -Encoding UTF8` or .NET `ReadAllText` with UTF-8, because the
default `Get-Content` may corrupt Chinese text in files without a BOM. Do not
copy text from a corrupted display. Write the artifact as UTF-8 without a BOM.

Create or replace only `evidence/native-101.md` with exactly five UTF-8 lines
and a final newline:

```text
# Evidence sample 101
- sample_id: 101
- generated_at: <current UTC ISO 8601 timestamp ending in Z>
- 模拟任务
- 未修改业务代码
```

Read the actual current UTC time. Run
`powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./sr-smoke/check.ps1`.
Preserve actual output and exit code. Verify the diff contains only this file.
Preserve all fixture tooling and repository rules. A nonzero checker exit is a failure.
