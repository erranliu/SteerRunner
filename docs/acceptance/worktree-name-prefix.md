# Worktree name prefix acceptance

Owned by [the focused implementation plan](../spec/worktree-name-prefix.plan.md).

| ID | Scope | Result |
| --- | --- | --- |
| WNP-A1 | Published schemas and production admission of generic name_prefix | Passed: development/generate/conversation agree on omitted, valid, 48/49-character boundaries and invalid values |
| WNP-A2 | Frozen request digest and effective prefix independent of correlation | Passed: sr default, sl-103-c2 and unrelated caller prefix; existing local/remote Orca freeze regression also passed |
| WNP-A3 | Local/environment Orca create argv, receipt identity and readonly recovery | Passed: exact prefix/full Run ID, C1 without a candidate suffix / C2 with -c2, generic prefix, mismatch rejection and readonly recovery with no duplicate creation |
| WNP-A4 | Actual SteerLine-reserved submit validated by the freshly built public CLI | Passed: caller's published-runner-name-prefix subtest executed against this build; ok=true/valid=true, no resident service |

2026-10-21 final C1-rule rerun, Windows/amd64: Runner base `8261310d5f2d1edb6fa0c81ba05fa4119b561386` and SteerLine test base `36d4f5967e9ecc2a9e02b0f124e8bbf3aff77d50`, each including this change at test time. Runner was isolated from unrelated work in the original checkout. SteerLine's base tree equals origin/main; only its own commit is moved onto that remote base before publication. Final evidence: `C:/Users/lmas1/AppData/Local/Temp/steerline-name-prefix-c1-20261021-1792550428937/` (`summary.json`, `runner-tests.jsonl`, `steerline-tests.jsonl`, `steerrunner.exe`). CLI SHA256: `df285e77318f3858ddf9d2f588be263a612ba1c03c69167bbd2fae054ee7090e`. The earlier all-candidates-suffixed run remains under `steerline-name-prefix-20261021-1792549733835/` and is not evidence for the final C1 rule.

The plan's focused command passed 19 top-level Runner tests across four packages (168 test/subtest passes, zero fail/skip); the caller command passed 26 top-level tests across three packages (45 test/subtest passes, zero fail/skip), including the explicit public CLI handoff. Both diffs passed whitespace checks. The initial new content schema fixture omitted required read_only access mode and failed; corrected fixture passed the final command. The initial failure remains in this chat's tool record, not counted as a pass.

No native Orca/model/PR run, installation deployment or existing worktree rename is claimed.
