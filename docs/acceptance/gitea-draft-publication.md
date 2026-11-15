# Gitea draft publication

2026-11-15. Base source was cf3da565. The independent build used the working tree with the Gitea changes.

The Gitea provider accepts a draft intent and the instance's work-in-progress title prefix. The publication stage receives those two values and the original repository, remote and base. The worker uses the fixed Gitea publication asset. Runner requires a platform-confirmed open draft with the original PR identity and the same full SHA as the candidate's remote branch before it records initial proof. Later Ready transitions retain that proof. Gitea's PR reference can substitute for its deleted source ref in the API response. The original source branch comes from the label for that exact PR reference.

The independent review approved the actual prompt change. The Gitea asset is 331 UTF-8 bytes including its leading and trailing line feeds. The GitHub asset stays 141 bytes, byte-for-byte. The prompt contains no authentication settings or query command.

The contracts, resources, publication and workflow package tests passed. The focused application tests covered first draft qualification, Ready after prior proof, publication concurrency and stale publication results. A preceding unrestricted package run reached the default Go test deadline. The independent build was 0.2.0-dev.4-gitea-draft with SHA-256 41db7c72b07c2426a91b727e98447dc198966967e8eff1abf0073d8b5c1997fe.

The isolated SteerLine live test passed with Gitea 1.27.1, Codex and Edge. It verified draft PR #1, a candidate to-do, pinned launch and results at two distinct Heads, stale-result rejection, archive after source deletion, and clean settlement. Raw summary: C:/SteerlineDebug/wt-4bd2-gitea-manual/e2e-37eea05d559940d6bd521fa060571b3e/summary.json.
