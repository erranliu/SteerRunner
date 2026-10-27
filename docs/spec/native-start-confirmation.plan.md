# Plan: native input readiness and delayed start confirmation

[Spec](native-start-confirmation.spec.md). Qualification source starts from the immutable original Runner 7c0df6cafb050f294e7066dfdeb896424fe78230, isolated from all live homes. Main integration must retain an equivalent product patch and be tested separately when its baseline differs.

- Gate the original remote Codex composer immediately before its first and only send; verify the same terminal before and after waiting. Keep Pi, managed-terminal and local-native paths unchanged.
- Remove the observation dead end for accepted, unstarted receipts. Query original native state, then independently read the unique original input only for a qualifying canonical done record. Commit start and completion only after existing exact attribution and process fences pass. Do not replay the sending command or relax the existing strict matcher.
- Add named-pipe RPC/lifecycle regressions for accepted/unstarted completion, observer reconstruction, duplicate/changed input, READY/working-only, stale or conflicting metadata, interrupted results and a different process; assert exactly one native send and one sending CLI call. Add zero-send composer busy/permission/timeout cases.
- Preserve a baseline negative run through an overlay of unchanged production files. Run the full adapter/protocol owners and formal L1 (including extensions, native-driver compile and zero-match guard). Build a clean isolated executable only after tests and commit metadata are fixed.
- Retain original stalled evidence and stop/settle its exact Run before future business rounds. Fresh qualification pins the new source and executable under the latest human repair request; no old home or accepted pin is changed. Continue the original two consecutive full progress/report/API/UI/publication/cleanup rounds after the repair qualifies.

Actual output paths and source/binary identities are recorded in [acceptance](../acceptance/native-start-confirmation-20261027.md). A READY-only original cannot establish that the composer race caused its historical delivery gap.
