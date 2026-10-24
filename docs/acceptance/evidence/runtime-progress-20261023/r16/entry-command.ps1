$ErrorActionPreference='Stop'
$taskRepo='C:/Users/lmas1/.codex/worktrees/e60a/SteerRunner'
Set-Location -LiteralPath $taskRepo
$taskHelper=(Resolve-Path -LiteralPath '.evidence/progress-agent-r16.exe').Path
$taskConfig=Get-Content -LiteralPath '.evidence/progress-test-config-r12.json' -Raw | ConvertFrom-Json
$taskConfig.configurations.'orca-local-codex'.execution.settings.agent_launch.runtime_executable=$taskHelper.Replace('\','/')
$taskConfig.configurations.'orca-local-codex'.execution.settings.agent_launch.runtime_sha256=(Get-FileHash -LiteralPath $taskHelper -Algorithm SHA256).Hash.ToLowerInvariant()
[IO.File]::WriteAllText((Join-Path $taskRepo '.evidence/progress-test-config-r16.json'),($taskConfig|ConvertTo-Json -Depth 60),[Text.UTF8Encoding]::new($false))
& go version -m $taskHelper 1> '.evidence/progress-agent-r16-build.txt'
& './scripts/test.ps1' -Level L1,Controlled,L2,L3 -Scenario local-codex.smoke,local-codex.result-acquisition,local-codex.progress-file,orca-local.progress-file,orca-remote.progress-file,local-codex.progress-multirun,local-codex.progress-recovery -Config '.evidence/progress-test-config-r16.json' -MaxParallel 7 -OutDir '.evidence/progress-acceptance-r16' *> '.evidence/progress-integration-r16-console.log'
exit $LASTEXITCODE
