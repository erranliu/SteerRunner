# Smoke implementation Plan

1. Read the frozen Spec with an explicit UTF-8 decoder.
2. Read current UTC and write only `evidence/native-101.md` in the exact format,
   encoded as UTF-8 without a BOM and with a final newline.
3. Run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File ./sr-smoke/check.ps1`.
4. Inspect the actual exit code and diff. Report the observed verification result.
