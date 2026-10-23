package codexworker

// NativeReadinessPrompt establishes a session before the first business input.
// Its authority must expire with that one turn: long tasks can retain the
// initialization message in their compacted history.
const NativeReadinessPrompt = "For this initialization turn only, initialize this agent session and reply READY without using tools or changing files. This readiness instruction expires immediately after that reply. In all subsequent turns, including after context compaction, follow the latest user task and continue it until its requested work is complete."
