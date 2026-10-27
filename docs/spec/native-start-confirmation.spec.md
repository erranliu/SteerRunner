# Native Codex input readiness and delayed start confirmation

Status: implementation under the human request of 2026-10-27. [Plan](native-start-confirmation.plan.md).

The post428 Task18 Run accepted its only implementation submission, but its original native session contained only initialization READY. The first send receipt had no verified start; native hook errors were visible but their causal relationship to delivery was not established. Preserve that negative original and its pinned runtime; do not claim a historical repair or retransmit its input.

1. Before the sole remote native Codex send, require the original terminal's bounded `tui-idle` result and validate its runtime/incarnation/workspace on both sides. Native READY metadata remains required. Idle is input readiness, never final evidence. Failure, permission or timeout produces zero task sends.
2. A bounded receipt without `turn_started` remains unverified. A later independent canonical completion query may establish the missing start only when the original accepted input is uniquely recovered, its exact text, saved baseline, process, pane, provider and completion all match. No working-only, READY-only, screen, duplicate, altered input, stale authority, interruption or replacement may supply that proof.
3. The recovery must use existing read-only native queries and perform no `terminal send`, bare Enter, resume, replacement resource or new business input. It must survive observer reconstruction without inventing continuous or streamed working evidence.
4. Keep existing progress/report/publication checks and cleanup requirements. Controlled tests establish implementation behavior; only fresh native rounds qualify end-to-end progress. The old stalled Run and unrelated historical failures remain unchanged.
