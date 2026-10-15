# SteerRunner real-environment smoke

Adapted from SteerlineTest `docs/spec/native-101.spec.md` at
`cebdd5ab552150c49743e40593c836689ecf789b`.

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
Preserve actual output and exit code. Verify the diff contains only this file,
then commit it locally. Preserve all fixture tooling and repository rules.
Do not push or create a PR during implementation.

When the separate publication turn authorizes publication, run exactly
`powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./sr-smoke/publish.ps1`.
Use its actual PR URL. The test driver owns merge; do not merge the PR yourself.
