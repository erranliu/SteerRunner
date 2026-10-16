param([Parameter(Mandatory=$true)][string]$Repository,[Parameter(Mandatory=$true)][string]$Evidence)
$ErrorActionPreference='Stop'
. (Join-Path $Repository 'scripts/live-catalog.ps1')
$taskCatalog=Get-Content -LiteralPath (Join-Path $Repository 'tests/catalog.json') -Raw | ConvertFrom-Json
$taskCount=0
foreach($taskEntry in @($taskCatalog.scenarios|Where-Object {$_.level -eq 'L3'})){$taskCount+=@(Get-SRLiveCases $taskEntry).Count}
if($taskCount -ne 20){throw 'L3 registry did not expand to twenty cases'}
$taskEnvironmentCount=0
foreach($taskEntry in @($taskCatalog.scenarios|Where-Object {$_.driver -eq 'environment-check'})){$taskEnvironmentCount+=@(Get-SRLiveCases $taskEntry).Count}
if($taskEnvironmentCount -ne 4){throw 'Environment check registry must include both local and Orca access modes'}
$taskEntry=$taskCatalog.scenarios|Where-Object {$_.id -eq 'orca-remote.service-restart'}
$taskEntry.cases=@('graceful-active')
$taskRejected=$false
try{Get-SRLiveCases $taskEntry|Out-Null}catch{$taskRejected=$true}
if(-not $taskRejected){throw 'Missing mandatory cases were accepted'}
$taskRoot=Join-Path $Evidence 'stability-gate'
New-Item -ItemType Directory -Force -Path $taskRoot|Out-Null
$taskPreflightConfig=Get-Content -LiteralPath (Join-Path $Repository 'tests/live-stability-config.example.json') -Raw|ConvertFrom-Json
$taskPreflightConfig.orca_executable=(Get-Process -Id $PID).Path
$taskPreflightPath=Join-Path $taskRoot 'config.json'
[IO.File]::WriteAllText($taskPreflightPath,($taskPreflightConfig|ConvertTo-Json -Depth 20))
& (Join-Path $Repository 'scripts/stability-preflight.ps1') -Config $taskPreflightPath
$taskPreflightConfig|Add-Member NoteProperty unknown_configuration $true
[IO.File]::WriteAllText($taskPreflightPath,($taskPreflightConfig|ConvertTo-Json -Depth 20))
$taskRejected=$false;try{& (Join-Path $Repository 'scripts/stability-preflight.ps1') -Config $taskPreflightPath}catch{$taskRejected=$_.Exception.Message -like 'Unknown stability config field:*'}
if(-not $taskRejected){throw 'Unknown configuration was accepted by stability preflight'}
New-Item -ItemType Directory -Force -Path (Join-Path $taskRoot 'sr-smoke'),(Join-Path $taskRoot 'evidence')|Out-Null
& git -C $taskRoot init --quiet
if($LASTEXITCODE -ne 0){throw 'Gate fixture git init failed'}
foreach($taskName in @('check.ps1','control.ps1')){[IO.File]::WriteAllText((Join-Path $taskRoot ('sr-smoke/'+$taskName)),[IO.File]::ReadAllText((Join-Path $Repository ('tests/fixtures/evidence-stability/'+$taskName))),[Text.UTF8Encoding]::new($true))}
[IO.File]::WriteAllText((Join-Path $taskRoot 'sr-smoke/stability.json'),'{"fixture_id":"012345abcdef","case_id":"active","gate":"implementation","gate_timeout_seconds":2}')
$taskBody="# Evidence sample 101`n- sample_id: 101`n- generated_at: "+[DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')+"`n- 模拟任务`n- 未修改业务代码`n"
[IO.File]::WriteAllText((Join-Path $taskRoot 'evidence/native-101.md'),$taskBody,[Text.UTF8Encoding]::new($false))
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $taskRoot 'sr-smoke/check.ps1') -Root $taskRoot *> (Join-Path $taskRoot 'timeout.log')
if($LASTEXITCODE -eq 0){throw 'Gate timeout silently released'}
$taskControl=Join-Path $taskRoot '.sr-stability-012345abcdef'
if(-not (Test-Path -LiteralPath (Join-Path $taskControl 'implementation.entered'))){throw 'Gate barrier was not recorded'}
[IO.File]::WriteAllText((Join-Path $taskControl 'implementation.released'),'released')
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $taskRoot 'sr-smoke/check.ps1') -Root $taskRoot *> (Join-Path $taskRoot 'released.log')
if($LASTEXITCODE -ne 0){throw 'Released gate did not complete real checker'}
$taskFake=Join-Path $taskRoot 'wrong-scope.json'
[IO.File]::WriteAllText($taskFake,(@{schema='steerrunner-test/v2';id='012345abcdef';root=$taskRoot;runner_exe=(Get-Command powershell.exe).Source;stability=@{native_orca=@{path=(Get-Command powershell.exe).Source};helper=@{path=(Get-Command powershell.exe).Source}}}|ConvertTo-Json -Depth 5))
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $Repository 'scripts/stability-network.ps1') -Manifest $taskFake -Action block *> (Join-Path $taskRoot 'wrong-scope.log')
if($LASTEXITCODE -eq 0 -or (Get-Content (Join-Path $taskRoot 'wrong-scope.log') -Raw) -notmatch 'escapes fixture'){throw 'Network scope guard failed'}
Write-Output 'Stability registry, bounded gate, real checker and network scope guard passed'
