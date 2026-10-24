# Runtime byte report producer

This task only runs the frozen `sr-smoke/progress.ps1` once in the assigned
execution workspace. Use the execution environment's native shell tool, not
`node_repl`, `child_process`, GUI automation or another host's process API.

The script uses `STEERLINE_PROGRESS_FILE`, writes arbitrary bytes, and waits for
external fixture gates. Keep waiting for its original foreground process. If
the shell tool returns a running session ID, poll that same session; a running
session is not task completion. Never detach, restart or modify the producer.

There is no implementation, checker, commit, push or PR task. Do not execute
an example task or publish anything. Do not inspect or print credentials.
Preserve actual errors. The fixture driver ends this task by stopping the Run.
