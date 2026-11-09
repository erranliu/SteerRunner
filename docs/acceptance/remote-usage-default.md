# Default remote usage collection

Date: 2026-11-09. User-authorized follow-up to remove per-host/per-date log grants. Based on 023dbd482f077008856ce7d278f864477cdddc21 plus this worktree's changes, branch codex/default-remote-usage. No installation or original Run changes.

The service usage_policy now contains only enabled (default true). The collector automatically accepts the narrow log leaf of the verified original Run/session. Pairing fingerprint, original structured session, CodexHome/path/link verification, per-instance collection permit, source identity/checkpoints, bounded reads and journaled temporary registration cleanup remain enforced. Old remote_log_access fields are rejected, not migrated. The obsolete grant-requirement snapshot projection and capability are removed; the schema and CLI contract match the new policy.

Tests cover default access across 2026/11/06 and 2026/11/09, disabled collection, invalid fingerprints/environments and unsafe roots. Existing identity, file transport, lease, parser, immutable usage and public query/stop tests remain. The live diagnostic fixtures now use the default policy instead of explicit grants, and the disabled negative case remains.

Evidence root: C:/SteerlineDebug/wt-ae18-remote-default/.

- Initial `go test ./internal/usage/... ./internal/contracts ./internal/resources -count=1` passed (tool output).
- Extended focused run `go test ./internal/usage/... ./internal/contracts ./internal/resources ./internal/cli ./internal/execution/adapters/protocol/orca ./internal/storage -run 'Usage|RunUsage' -count=1 -json`: 88 tests passed; 7 existing opt-in live/evidence diagnostics skipped. The storage test package failed to compile after removing its retired grant-requirement test because of an unused import. Original logs retained under runner-usage-20261109T043903461Z; the whole command is not marked passed.
- After removing that unused import, all storage Usage tests passed, runner-storage-20261109T044008678Z.
- First complete L1 failed in configuration framework before the product package set: atomic record replacement reported access denied instead of the expected L2 native Git failure (runner-l1-20261109T043614910Z/summary.json). A separately initialized clean-baseline configuration framework passed (runner-baseline-config-20261109T044019129Z/framework.log); this is not a proven fixed baseline failure. The new complete L1 run passed: runner-l1-recheck-20261109T044318903Z/summary.json (998.4 seconds), 751 Go top-level tests, 21 extension tests and native driver compile/zero-match guard. Twelve pre-existing helper/live-opt-in skips are explicitly allowed by the formal L1 entry and are not live qualification. The first failed report remains preserved.
- SteerLine consumer L1, L2, web.unit, go.http, production frontend build, application usage/collection focused tests and tagged acceptance compilation passed; detailed reports are in the SteerLine acceptance document.

Live qualification and deployment remain unverified. The default-collection remote execute acceptance driver was updated, but no live model request was run in this change. Historical explicit-grant live evidence does not qualify this new build. No fixed prompt body or prompt wiring changed.

The matched two-repository build also passed via SteerLine build.ps1 -Web -RunnerRepo this-worktree. Build timings: C:/SteerlineDebug/wt-ae18-remote-default/diagnostics/build-20261109T044855822Z.json. The new Runner offline capabilities publish remote_log_access_default=verified_session. Binaries were built in the two source worktrees only; no service was started or deployed.
