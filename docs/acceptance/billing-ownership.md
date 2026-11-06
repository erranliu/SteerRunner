# Billing responsibility and remote usage discovery

2026-11-05: implementation in an independent clone, no installation deployment.

Runner no longer exposes estimate-run-cost, price policy schemas or an internal estimator. run-usage/v1 and run-usage-events remain the immutable public usage surface; capability publishes same_snapshot, events_schema, max_events and access_requirements.

The background collector may discover a missing remote authorization through paired host identity, the original structured session, and verified directory/link metadata. It publishes source_ref/environment/host_id/session_root/observed_at in access_requirements. Discovery does not register a folder or read log contents. Only an explicitly matching temporary grant permits acquisition. Requirements persist with their snapshot and checkpoint; a new policy/instance reopens incomplete observations under the original bounded rules. Queries only read committed projections.

Price calculation, currency, rates and policy versions belong to SteerLine. Historical acceptance documents containing cost results describe their original builds and do not represent the new public surface.

Verification (2026-11-05):

- Fixed runtime build 0.2.0-dev.billing2, SHA256 78bb5f1aa0400a137322ef1501101227c017507fbc35f009db44c3c3330e2236; base f6a389cfd4ae7827d7c722555a85e94f1e1ba708 with local production changes. Later additions are tagged fixture/evidence documentation only.
- Focused affected owner selection: 265 tests passed, zero skips; the combined command exits 1 because the unrelated resources.TestSRWorkflowSelection frozen-prompt assertion fails. This assertion also fails in the original source checkout; no prompt assets, goldens or behavior wiring were changed to waive it. Do not report Runner L1/Controlled as passed.
- Fixed public CLI lifecycle/full-binding/retired commands passed from SteerLine, including immutable event capability and absence of run_cost. Runner's TestRunUsagePublicExecutable also passed for a 120-token request-time fixture and rejection of the retired command.
- Tagged billingacceptance TestUsageSteerLineFixtureHandoff creates an explicit create-only Runner-owned ledger/checkpoint fixture, not native usage evidence. SteerLine consumes it only through the fixed CLI: 100 input + 20 output -> USD 0.000140000000000000 under synthetic rates; completion-time-only fixture remains unpriced. Both services stopped/settled. Never use these seeded records as proof of native collection.
- A new isolated real remote Codex Run d5abd273c190810af91f6f4b643403ac first returned missing permission, then returned 1 response / 16,166 tokens under an exact verified leaf grant after formal restart. Repeated reads were idempotent; withdrawal preserved history; temporary registration revoked and original Run/service settlement verified. Time basis is completion, so SteerLine correctly keeps cost unavailable.

Raw evidence lives under C:/SteerlineDebug/wt-1148-billing. The detailed source/binary bindings, commands, original failures, public fixture, browser and live records are indexed by SteerLine's SteerlineFlow/testing/acceptance/billing-ownership.md. No installation was deployed. No full Runner L1/Controlled qualification is claimed.

## Pre-PR quality audit (2026-11-05)

The audited runtime head is 3bac937bfb62c3a4a74889c3b7b46627f6a36dcf, based on main a66038c8. Clear the cached access requirement before every actual re-observation: a failed observation must not allow an old requirement to reappear through a cached checkpoint. TestUsageFailedReobservationCannotReviveOldAccessRequirement failed before the fix and passed after it. Retain the main branch native-catalog command and all original documentation tails.

Audit evidence root: C:/SteerlineDebug/wt-1148-billing/audit/20261105T102227Z. Fixed clean runtime build 0.2.0-dev.billing-audit, SHA256 125124b9f8f835fc886c7bd1cab025da12b67d9a2242ea0d8bbf20e017a4c55b, source digest f378f808e8f8ff807c2f9445de661f58db95cbcaf354452134a3637b9a5f7252. Subsequent commits update acceptance documentation only.

- runner-selection.json explicitly selects 96 roots by package. runner-test-summary.json reports 245 root/subtest passes, zero skips/failures, 114.99 seconds. runner-vet.txt exits 0. This selection does not claim full Runner L1/Controlled; the previously established frozen-prompt baseline failure remains separately recorded, without changing prompt assets or golden expectations.
- SteerLine runner-contract passes all three public lifecycle/binding/capability roots against this exact build. The tagged Runner-owned published-fixture and SteerLine published consumer pass the synthetic USD 0.00014 case and completion-only unknown-cost case; these records are not native collection proof.
- remote/summary.json is a fresh formal live pass, 45.51 seconds. Same original Run 56da21db97db98a07d6485c0e4816343 and immutable input complete missing permission, verified minimal grant, formal restart, original binding rejection and same-run re-observation. Revision 7 has 1/1 verified source and 14,862 tokens (14,851 input, including 3,075 uncached + 11,776 cache read; 11 output). Repeat reads do not duplicate usage. Completion-only time remains unpriceable under SteerLine's request-time rule.
- Withdrawal preserves verified history, revokes temporary access, and stops/settles the original run and service. live/cleanup.json verifies cleanup; live/orca-workspace-cleanup.json confirms removal of the exact probe workspace. No installed service or Task 101 was modified.

Before PR creation the target was refreshed to a78d87da05120b7e6b99981b396d6cecd4d60d92 (#118 capability transport reuse). git merge-tree --write-tree HEAD github/main exits 0 with no content conflict. Per repository rules, the validated history was not rewritten solely for a rebase UI state. Above runtime tests qualify the explicit tested head, not an unexecuted post-merge tree. Squash merge is preferred. Detailed SteerLine owner, Linux, HTTP, browser and formal suite evidence is in its paired acceptance document. Neither PR deploys an installation.
