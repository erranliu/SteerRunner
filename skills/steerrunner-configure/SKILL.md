---
name: steerrunner-configure
description: Configure SteerRunner execution routes and project publication bindings. Invoke directly without arguments to discover local and existing Orca routes. Use for SteerRunner setup or changing its execution defaults, not for ordinary coding or executing a Spec and Plan.
---

Read `assets/client-location.json` in this installed skill and use its absolute `helper_path`. Run the shared helper with Windows PowerShell:

```powershell
powershell.exe -NoProfile -File <helper_path>
```

No arguments or preparatory questions are needed for the default configure action. The helper discovers routes, preserves existing choices, and saves what it can determine. Present its candidates, missing dependencies, project binding and remaining choices; discovery does not prove execution or publication readiness.

When the user supplies choices, write a UTF-8 JSON object to a temporary input file and rerun with `-InputFile <path>`. Its optional fields are `runner_executable`, absolute `state_dir`, `default_profile`, `profiles`, and `project`. Profiles use Runner's existing `{execution,workflow}` objects. `project` accepts `profile`, `repository` and `publication`; preserve its existing values when making a narrow change. Read the current configuration returned by the helper to construct the complete object being replaced. Pass `-ProjectPath` only when the intended project differs from the current directory. Never add secrets or pairing codes.

Only ask about gaps the helper cannot determine, such as multiple targets, missing remote project path, publication target, or an explicitly selected audit model. A missing Orca prompts installation for remote execution; installed-but-stopped, no remote environment, and connection failures require their respective remedies. Do not install, launch, pair or register anything just to complete discovery.

Development profiles require explicit `execution.settings.access_mode=full_access`. Reject sandbox, omitted, empty and null modes with a migration explanation; preserve read_only content profiles and the existing authentication chain. Do not silently rewrite saved profiles or frozen submissions. Change the editable profile only when authorized, then submit a new request identifier; reconcile existing submissions using their original identifier and body. Full access has the execution account's permissions outside the worktree. Retired Runs may still have live native processes; use original-resource stop and preserve unknown cleanup facts.
