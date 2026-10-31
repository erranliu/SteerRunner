# Native Codex input readiness and delayed start confirmation

Status: implementation under the human request of 2026-10-27. [Plan](native-start-confirmation.plan.md).

Current submission and start-evidence rules are superseded by [Orca native submission](orca-native-submit.spec.md), including its 2026-10-31 bridge repair. Historical acceptance records retain their original outcomes.

The post428 Task18 Run accepted its only implementation submission, but its original native session contained only initialization READY. The first send receipt had no verified start; native hook errors were visible but their causal relationship to delivery was not established. Preserve that negative original and its pinned runtime; do not claim a historical repair or retransmit its input.

1. Before the sole remote native Codex send, require the original terminal's bounded `tui-idle` result and validate its runtime/incarnation/workspace on both sides. Native READY metadata remains required. Idle is input readiness, never final evidence. Failure, permission or timeout produces zero task sends.
2. A Codex send receipt, including `turn_started`, does not establish original business start. Existing native history or an independent canonical completion query may establish start only when the original accepted input is uniquely recovered, its exact text, saved baseline, process, pane, provider and turn all match. No working-only, READY-only, screen, duplicate, altered input, stale authority, interruption or replacement may supply that proof. Positive replacement evidence remains durable even when the supplemental read returns an error.
3. The recovery must use existing read-only native queries and perform no `terminal send`, bare Enter, resume, replacement resource or new business input. It must survive observer reconstruction without inventing continuous or streamed working evidence.
4. Keep existing progress/report/publication checks and cleanup requirements. Controlled tests establish implementation behavior; only fresh native rounds qualify end-to-end progress. The old stalled Run and unrelated historical failures remain unchanged.

5. Single-line and multiline Codex inputs use the same existing Orca text-plus-Enter request bound to the original runtime/terminal/incarnation/request. Runner does not paste separately, read or stabilize drafts, resize a viewport, or append Enter. Unknown outcomes never authorize replay.
6. Stable canonical done records may retain an input preview with thin or fully corroborated metadata. A preview does not establish the input: require the same process-fenced, uniquely recovered complete original text and unchanged before/after main/provider/body record. Partial metadata, changed previews and all existing conflicting identities remain rejected.

7. Collapsed elements, visible fragments and display wrapping are not input integrity or submission evidence. Frozen logical wire, model-bridge integrity checks and formal original-input/turn attribution retain their respective authority.

8. Existing input-budget validation rejects malformed UTF-8. There is no draft-matching fast path. Controlled scalar-count cases do not qualify actual non-BMP transport. Preserve the pinned native negative result.
9. Minimum native probes use the existing lifecycle test and focused catalog. Each owns a fresh terminal in an empty isolated worktree, rejects replay of an attempted evidence root, records the sole generic submission and formal original input/final, and requires confirmed close plus a complete zero-terminal census. Retired split-paste shape probes are historical evidence only; no generic probe qualifies SteerLine business progress.
