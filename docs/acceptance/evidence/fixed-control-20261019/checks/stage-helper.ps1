param([string]$Round="r3")
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$taskRoot=$PSScriptRoot
if(-not [string]::IsNullOrWhiteSpace((& git status --porcelain|Out-String))){throw 'Freeze clean source first'}
& go build -trimpath -o (Join-Path $taskRoot 'steerrunner-agent.exe') ./cmd/steerrunner-agent
if($LASTEXITCODE -ne 0){throw 'Helper build failed'}
$taskSHA=(Get-FileHash (Join-Path $taskRoot 'steerrunner-agent.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
$taskIdentity=[ordered]@{commit=(&git rev-parse HEAD).Trim();dirty=$false;sha256=$taskSHA;at=[DateTime]::UtcNow.ToString('o')}
$taskIdentity|ConvertTo-Json|Set-Content -Encoding utf8NoBOM (Join-Path $taskRoot ('helper-build-'+$Round+'.json'))
& node (Join-Path $taskRoot 'upload-helper.cjs') *> (Join-Path $taskRoot ('upload-helper-'+$Round+'.log'))
if($LASTEXITCODE -ne 0){throw 'Helper upload failed'}
$taskUpload=Get-Content (Join-Path $taskRoot 'uploaded-helper.json') -Raw|ConvertFrom-Json
$taskCommand=@'
$ErrorActionPreference='Stop'; $src='{0}'; $dst='{1}'; $sha='{2}'; if ((Get-FileHash -LiteralPath $src -Algorithm SHA256).Hash.ToLowerInvariant() -ne $sha) {{ throw 'source hash mismatch' }}; if (Test-Path -LiteralPath $dst) {{ if ((Get-FileHash -LiteralPath $dst -Algorithm SHA256).Hash.ToLowerInvariant() -ne $sha) {{ throw 'immutable destination differs' }} }} else {{ New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dst)|Out-Null; Copy-Item -LiteralPath $src -Destination $dst }}; & $dst --terminal-metadata; if ($LASTEXITCODE -ne 0) {{ throw 'metadata failed' }}; Write-Output ('SR_'+'FIXED_HELPER_OK:'+$sha)
'@ -f $taskUpload.source,$taskUpload.destination,$taskUpload.sha256
$taskCommand|Set-Content -Encoding utf8NoBOM (Join-Path $taskRoot ('helper-command-'+$Round+'.txt'))
& 'C:/Users/lmas1/AppData/Local/Programs/orca/resources/bin/orca.exe' terminal create --worktree ('id:'+$taskUpload.worktree) --title 'SteerRunner fixed control r2 test helper verification' --command $taskCommand --environment e68f4edf-8a83-42c2-a9da-6d404d3aa3d8 --json > (Join-Path $taskRoot ('helper-created-'+$Round+'.json'))
if($LASTEXITCODE -ne 0){throw 'Helper verification creation unresolved'}
$taskCreated=Get-Content (Join-Path $taskRoot ('helper-created-'+$Round+'.json')) -Raw|ConvertFrom-Json
if($taskCreated._meta.runtimeId -ne $taskUpload.runtime -or -not $taskCreated.result.terminal.incarnationId){throw 'Verification owner differs'}
$taskVerified=$false
for($taskTry=0;$taskTry -lt 5;$taskTry++){
 & 'C:/Users/lmas1/AppData/Local/Programs/orca/resources/bin/orca.exe' terminal read --terminal $taskCreated.result.terminal.handle --limit 120 --environment e68f4edf-8a83-42c2-a9da-6d404d3aa3d8 --json > (Join-Path $taskRoot ('helper-read-'+$Round+'.json'))
 $taskRead=Get-Content (Join-Path $taskRoot ('helper-read-'+$Round+'.json')) -Raw|ConvertFrom-Json
 if(($taskRead.result.terminal.tail -join "`n") -match ('SR_FIXED_HELPER_OK:'+[regex]::Escape($taskUpload.sha256))){$taskVerified=$true;break}
 Start-Sleep -Seconds 1
}
& 'C:/Users/lmas1/AppData/Local/Programs/orca/resources/bin/orca.exe' terminal show --terminal $taskCreated.result.terminal.handle --environment e68f4edf-8a83-42c2-a9da-6d404d3aa3d8 --json > (Join-Path $taskRoot ('helper-show-'+$Round+'.json'))
$taskShow=Get-Content (Join-Path $taskRoot ('helper-show-'+$Round+'.json')) -Raw|ConvertFrom-Json
if($taskShow.result.terminal.incarnationId -ne $taskCreated.result.terminal.incarnationId -or $taskShow._meta.runtimeId -ne $taskUpload.runtime){throw 'Verification incarnation changed'}
& 'C:/Users/lmas1/AppData/Local/Programs/orca/resources/bin/orca.exe' terminal close --terminal $taskCreated.result.terminal.handle --environment e68f4edf-8a83-42c2-a9da-6d404d3aa3d8 --json > (Join-Path $taskRoot ('helper-closed-'+$Round+'.json'))
if($LASTEXITCODE -ne 0 -or -not $taskVerified){throw 'Helper verification or close failed'}
$taskConfig=Get-Content (Join-Path $taskRoot 'live-config.json') -Raw|ConvertFrom-Json
$taskConfig.remote_runtime_executable=$taskUpload.destination;$taskConfig.remote_runtime_sha256=$taskUpload.sha256
$taskConfig|ConvertTo-Json -Depth 15|Set-Content -Encoding utf8NoBOM (Join-Path $taskRoot ('live-config-'+$Round+'.json'))
$taskIdentity|ConvertTo-Json
