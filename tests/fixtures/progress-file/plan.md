# Runtime producer plan

1. In the assigned workspace use the provided native shell execution tool to run
   `powershell.exe -NoProfile -ExecutionPolicy Bypass -File sr-smoke/progress.ps1`
   exactly once, with the original injected environment.
2. Keep the process in the foreground. If the tool yields a running session ID,
   poll that same session. Do not use `node_repl` or spawn a replacement process.
3. Wait for the driver's external gates and explicit Run stop. Do not run
   `check.ps1`, change files, commit, push, create a PR or claim early completion.
