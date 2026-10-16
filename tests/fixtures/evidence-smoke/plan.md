# Smoke implementation Plan

Read every UTF-8 Spec/Plan with an explicit UTF-8 decoder, including files
pointed to by the frozen input. On Windows PowerShell 5, use
`Get-Content -Raw -Encoding UTF8`; default `Get-Content` can display Chinese
incorrectly. Preserve the exact Unicode text and write the artifact as UTF-8
without a BOM. Verify the actual checker exit code before committing.

1. Read the frozen Spec and verify the checkout, current branch and base.
2. Read current UTC and write only `evidence/native-101.md` in the exact format.
3. Run the Spec's PowerShell checker, inspect the diff and commit that file.
4. Report the observed check and commit results. A source failure stays a failure.
5. On the separate publication input, run the exact fixed publish.ps1 command.
   It pushes this branch and creates or reuses its PR on the dedicated test base.
   Report its actual output and exit code. The Runner independently queries and
   verifies the returned PR. Do not run extra ad-hoc GitHub verification commands
   after a successful publisher: a different sandbox authentication context can
   misleadingly return 404 for a PR just created by the approved command.
   A failed or uncertain publisher stays failed or uncertain.
6. A later feedback turn asks for a newer UTC sample. Recheck, commit and publish
   on the original branch and PR, using the original session.

Use the execution host's native permission policy. On local Windows use the
configured shell with login=false. Host-side GitHub login is checked before
submission. If the exact publication command returns HTTP 401 inside the native
sandbox, request native automatic review once for that same command, as described
by the Runner worker setup. Preserve a denial or unavailable review as a blocker.
Before any publication retry, query the original branch/PR outcome. Preserve
authentication and permission settings; do not copy credentials or export tokens.
