# Run configuration publication

Date: 2026-10-17. Implemented locally; not published or deployed.

This increment implements batch A of SteerLine's runner collaboration Spec/Plan. The authoritative caller documents are in the active SteerLine worktree at C:/Users/lmas1/.codex/worktrees/62d3/SteerLine/docs/spec/runner-collaboration.spec.md and runner-collaboration.plan.md; caller results belong to docs/acceptance/runner-collaboration.md there. SteerLine baseline is 48c5671926466fe83c23de7fee5377bc61f87188 with current uncommitted implementation. This does not close historical WA/SA/SREC acceptance items.

## Contract and ownership

- capabilities.configuration_schemas.run_configuration is a self-contained JSON schema for {execution,workflow}. It reuses published adapter settings, workflow schema and the production workflow catalog. No external schema resolution or production example/fixture protocol is added.
- contracts.ValidateRunConfiguration is the production pure configuration gate; ValidateRun calls it using original raw execution/workflow JSON, preserving explicit null rejection. Schema and admission share workflow choice/support/override sources. The generated schema participates in resource_digest.
- development/1 and development-audited/1 remain Runner-owned. Audited local Codex needs explicit execution and audit models. No caller state machine, migration or native storage rewrite is introduced.
- Acceptance freezes normalized parameters, materials and templates. Final prompt/model/source are persisted before each turn; restoration reuses those exact inputs. Native identity remains bound at its own lifecycle point.

## Minimal test plan and results

| Item | Level / command | Result / evidence |
| --- | --- | --- |
| SRCFG-A01 | L1: go test ./internal/resources -run '^TestSRPublishedRunConfiguration$' -count=1 -v | Passed: 14 positive/negative subtests compare schema and the production pure gate; no models, Git freeze or skips |
| SRCFG-A02 | L1: go test ./internal/application -run '^TestSRFrozenInputRestoration$' -count=1 -v | Passed: audit_answer, business_feedback, automatic_repair, report_retry; frozen final input restored without rerender |
| SRCFG-A03 | Targeted regression: go test ./internal/contracts ./internal/resources -count=1 | Passed; build script completed |

Evidence directory: C:/Project/SteerRunner/.evidence/collaboration-1792198113555. configuration-extra.log (final 14 cases), configuration.log (initial 11), input-restoration.log, build.log and steerrunner.exe.build.json retain commands/results and source identity. The first schema compile exposed a control-character regex incompatibility in the existing public workflow schema; the published regex was corrected and the final tests passed.

Source baseline: 529b6c360d9abb6ee225c82cc4393a9cb29d685e, dirty=true. Version 0.1.0-dev.1; steerrunner-cli/v1; Windows/amd64. Actual EXE SHA-256: 0a89bc55f7a745a764d53484d1e74f17dc47897088f46a5aa713036d8e978393. Caller I1 captured real capabilities and performed idle schema/input-encoding verification against this exact build; product Run/audit/materials integration is separately verified by caller N1.

No installation directory was changed. This evidence is for this source/build only, not a formal release or broad native regression.

## Review fixes (2026-10-17)

The previous evidence remains historical. The review found a caller status/business race, missing startup original-Run validation and a plain Codex padded-model schema mismatch. This increment adds the public `--expected-instance-id` precondition (advertised by `instance_binding.expected_instance_id`), checks it before connecting and again at service admission, and disables implicit service startup for bound calls. Service stop retains its original input-body identity. SteerLine owns the startup verification fix; no Runner workflow/recovery state machine was added.

- R1/SRCFG-A01: 18 schema/production cases now include trimmed, padded, blank and newline plain Codex models. Model whitespace is rejected in the published schema, without caller normalization.
- R3/SRCFG-A04: `go test ./internal/cli ./internal/host ./internal/resources ./internal/contracts -count=1` passed all four packages. `TestSRExpectedInstanceDoesNotStartService` proves no state directory/service creation; `TestSRExpectedInstanceFencesReplacement` covers submit/reply/resume/check/inspect and server-side enforcement. No native/model execution in these tests.
- Fixed executable: `.evidence/collaboration-fix-7f82fb0599514709af3590f77c1ca661/steerrunner.exe` plus `.build.json`, SHA-256 `74a3ab9129f287fd362696ba8ba862a88701c46aa443c22b4d33879605696992`, resource digest `2cd6bac5029ab42d939fce9b25c97316a5dda404056e9a000616ee24726de1f5`; baseline 529b6c3 with implementation changes, dirty=true. No installation or previous build was overwritten.
- Caller I1 passed with this EXE: matched-instance capabilities/schema and input encoding, padded plain model rejection and a direct stale expected-instance CLI rejection. Raw capabilities: `C:/Users/lmas1/AppData/Local/Temp/steerline-collaboration-fix-18e97677397a434891e2688601444622/capabilities.json`; test report under `cli-report/i1/summary.json` in that directory. It also captures the selected build/resource identity and original-instance stop/settlement.

The fix test plan is limited to the above R1/R3 plus caller U1/U3/I1 and the audited product chain. R2 frozen-turn semantics are unchanged and retain their earlier evidence. Cross-repository PRs must land Runner before enabling the new caller with this contract; version text alone cannot substitute for selecting the new SHA/EXE.

