# Background usage consoles and Orca final identity

Date: 2026-11-03. Base: `69a4d4b24625a8ac74a454c74d8fcdac70191144`.
Worktree: `C:\Project\SteerRunner-worktrees\console-final-identity`.

## Problems and changes

The usage collector ticks every two seconds. Its runtime-status and cleanup terminal-census commands launched the console-subsystem Orca CLI without Windows console suppression. Both now share a command helper that preserves CommandContext cancellation and the existing CLI serialization while using HideWindow and CREATE_NO_WINDOW on Windows. Other platforms retain the original command behavior.

Final evidence carried native provider session/turn IDs, while Orca's execution route uses a terminal identity and command ID. The final-contract check compared those different namespaces and kept completed execute/1 runs in final_source_unverified. Evidence now carries an explicit normalized final session/turn binding alongside unchanged native provenance. Codex and verified Orca history/extension producers populate that binding after their original-source checks. The actor checks the normalized final against the active execution route and still rejects absent native provenance, wrong final routes, stale execution evidence, unknown outcomes and successful turns without verified finals. No source identity or verification gate is inferred from display text.

The change does not deploy, restart, stop or rewrite the installed service or its historical Run. It does not change prompts, workflow inputs, native receipt verification, CLI queueing or mutation retries. Old evidence is not upgraded or fabricated; an original source read must produce a complete binding.

## Regression evidence

Output root: `C:\SteerlineDebug\runner-console-final-20261103`.

- `focused/tests.jsonl` and `focused/stderr.txt`: initial focused regression passed, including the Windows real-process console check, single execution, final-route negatives and fixed-control receipt recovery.
- `baseline/tests.jsonl`: both defects reproduced with the two old production behaviors restored temporarily; the console test and the valid Orca-final closure test failed as expected. Fixed source was restored unconditionally before further testing. The console regression uses a hidden parent console and an isolated executable fixture; it does not launch the real Orca CLI or flash a visible console.
- The Windows regression exercises both actual collector entry points, repeats status queries, checks that child processes have no console, and retains pre-cancelled query behavior.
- Single-execution regression covers distinct normalized/native IDs through close, original stop obligation and result sealing; foreign session/turn, missing normalized binding and missing native provenance remain rejected. Original extension and control-receipt tests check the producer mappings and retain foreign-owner/input/replacement failures.

## Final validation

All commands ran from the isolated worktree with fresh output directories under the root above.

| Entry | Evidence | Result |
| --- | --- | --- |
| `./scripts/test-prerequisites.ps1 -Stage L1 -OutDir <root>/L1` | `L1/stages.json`, `L1/l1.jsonl`, `L1/framework/result.json` | Passed: complete default L1, 27 framework checks, configured Go package set, native compile and zero-match guard. Go/application completed in 851.5 seconds; the complete L1 core took 1122.5 seconds. |
| `./scripts/test-controlled.ps1 -Combination @('content-flow','execute-stop') -OutDir <root>/controlled-flows` | `controlled-flows/summary.json`, `controlled-flows/controlled/result.json`, `controlled-flows/controlled/integration.log` | Passed: exactly 10 selected roots; includes content flows, native unknown/stop, manual reconciliation, final-seal's four combinations, four authorization-restart cases and single-execution cold stop. |
| `./scripts/test-controlled.ps1 -Group @('orca-cli') -RunnerExe <root>/controlled-flows/controlled/steerrunner.exe -OutDir <root>/controlled-orca-cli` | `controlled-orca-cli/summary.json`, `controlled-orca-cli/controlled/integration.log` | Passed: one selected root and all seven CLI queue/read-retry/restart/stop child cases, using the same Runner EXE. |

The earlier attempt to combine Group and Combination was rejected by the selection guard before execution; the two valid selections above were run separately. `git diff --check` passed, and the original main checkout remained clean.

No live model/Orca session, installation repair or full native L2/L3 qualification is claimed.
