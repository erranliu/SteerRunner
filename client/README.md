# SteerRunner Windows client

From the extracted package root, install the two skills and one shared helper:

```powershell
powershell.exe -NoProfile -File .\scripts\install-client.ps1
```

The default bundle location is `%LOCALAPPDATA%\Programs\SteerRunner`. Skills go
to `$CODEX_HOME\skills`, or `%USERPROFILE%\.codex\skills` when unset. Existing
installations or colliding skills are rejected before copying. To stage another
installation, provide explicit new `-InstallDir` and `-SkillsDir`; never replace
an active Runner EXE. Keep the extracted package for later installation.

In the intended repository, invoke `$steerrunner-configure` without arguments.
It discovers existing local and Orca routes. Discovery does not install tools,
start execution, or prove tools/login/publication readiness. Git and the chosen
execution tools must already be available. Orca remote execution additionally
needs an existing connected environment and registered execution-host project.

Then ask `$steerrunner-run` to execute your committed Spec and Plan. The skill
also supports progress, answers, explicit resume/stop, and opening the Viewer.
Keep the original submission handle when continuing from another chat. Unknown
results must be reconciled with that handle, never replaced by a new submission.

Caller choices and request records live in `%LOCALAPPDATA%\SteerRunnerClient`;
Runner state defaults to `%LOCALAPPDATA%\SteerRunner`. They are separate from the
installation and are not deleted by installation. If the existing local Codex
authentication socket path limit blocks a deep state directory, explicitly
choose a shorter absolute state directory for future Runs. Existing Runs retain
their original directory and handle.

Run `steerrunner-viewer.exe` directly for the independent read-only window. It
requires only Windows, reads `%LOCALAPPDATA%\SteerRunnerView\snapshots`, and can
be copied on its own. An optional `--snapshot-dir <absolute-local-directory>`
selects another directory. Only JSON files are read; Runner/CLI/source code and
development runtimes are unnecessary. For viewing another machine, arrange file
delivery separately; the Viewer does not fetch files or connect to Orca.

Runner samples every second and emits a five-second heartbeat. Viewer reads
every two seconds. A snapshot expires fifteen seconds after its captured time;
rereading it does not extend freshness. Broken, missing or expired sources keep
their last known rows, visibly marked. PR links open only HTTP(S) URLs. Closing
the window leaves Runner execution unchanged. The window has no task controls.

The helper runs on Windows PowerShell 5.1; no Go, Node or Python is needed to run
the delivered bundle. Native agent versions and supported combinations come
from the bundled Runner capabilities. `manifest.json` records component identity
and file SHA-256 values; these verify package integrity, not execution readiness.

Client contract version 2 removes the Check action and its request routing.
Runner state is v5 only; settle all older resources with their original build
before using a fresh state directory. Existing Check clients require matching
consumer changes and must not be pointed directly at this version.
