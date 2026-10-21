# Worktree name prefix

Implement the caller-owned optional Run field `name_prefix`, independently of opaque correlation metadata. SteerLine supplies `sl-<task number>` for C1 and `sl-<task number>-c<candidate number>` from C2 onward; other callers may supply any valid prefix. Prefixes contain 1–48 lowercase ASCII letters/digits with single interior hyphens (`^[a-z0-9]+(-[a-z0-9]+)*$`). Explicit empty/null or unsafe values are rejected before resource creation. The field is included in the frozen request/input digest; managed worktree execution freezes its effective value in the execution snapshot, defaulting to `sr` when omitted. Content workflows accept the common field but do not create a managed worktree.

Orca worktree creation and receipt validation share `<effective prefix>-<full Runner Run ID>`. Names do not establish ownership or recovery eligibility. Existing exact resource/request bindings and creation receipts remain authoritative; this change does not add resource discovery, recreation, retry, migration or branch renaming. SteerLine owns the transaction assigning its candidate number and final submit bytes; Runner does not parse Task/Candidate identifiers or correlation fields.

## Implementation

1. Publish the field in RunRequest and the development/content input schemas, with matching admission rules.
2. Freeze the effective prefix in SessionContext and use one naming function for Orca creation and receipt validation.
3. Verify the public contract, freeze boundary and local/environment Orca paths; record evidence in [acceptance](../acceptance/worktree-name-prefix.md).

## Test plan

All selected tests are focused L1 on Windows/amd64, after implementation, using isolated Git/SQLite and controlled native fixtures. Build the CLI into the external evidence directory, never an installed runtime.

| Acceptance | Command / scenarios | Passing condition / evidence |
| --- | --- | --- |
| WNP-A1 | `go test -json ./internal/contracts ./internal/resources ./internal/application ./internal/execution/adapters/orcatransport -run '^(TestSRCLIContract\|TestRunNamePrefix\|TestSRFreezeOrcaAgent\|TestSRPublishedRunConfiguration\|TestContentPublishedSchemas\|TestWorkspaceNamePrefix\|TestWorkspaceCreation\|TestWorkspaceTerminalSurfaceReadiness\|TestCreationOwner\|TestAgentLaunchSingleTerminal)' -count=1` | Public schema and production admission agree on omitted, valid, boundary and invalid prefixes for all input variants; `runner-tests.jsonl` |
| WNP-A2 | Same command: `TestRunNamePrefixFreeze`, existing `TestSRFreezeOrcaAgent` | Requested/default effective prefix frozen independently of correlation; request digest remains consistent; same log |
| WNP-A3 | Same command: `TestWorkspaceNamePrefixCreationAndRecovery`, existing creation/receipt recovery tests | Local/environment create argv and verified receipt use the exact frozen prefix/full Run ID; mismatches rejected, readonly reconciliation, no duplicate create; same log |
| WNP-A4 | `go build -o <evidence>/steerrunner.exe ./cmd/steerrunner`; caller runs `TestRunnerPresetFreeze/published-runner-name-prefix` with `STEERRUNNER_EXE` set to that EXE | Actual caller-reserved submit validates through public CLI with ok/valid true, no resident service or native worker; caller `steerline-tests.jsonl` and EXE SHA256 |

L2/L3/L4/full and native probes are not selected: this is a naming-only change; controlled owner tests cover actual creation command compilation and receipt reconciliation without repeating execution/permission/delivery matrices. Actual results and evidence paths belong to acceptance, not this plan.
