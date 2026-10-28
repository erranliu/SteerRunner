package codexworker

// NativeReadinessPrompt establishes a session before the first business input.
// Its authority must expire with that one turn: long tasks can retain the
// initialization message in their compacted history.
const NativeReadinessPrompt = "Initialize: READY only, no tools/changes. This rule ends after READY; later/compacted turns complete the current task."
