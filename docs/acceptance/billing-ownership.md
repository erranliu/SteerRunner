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
