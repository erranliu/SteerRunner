$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$taskBase=Join-Path $PSScriptRoot 'native-r5/controlled'
$taskOut=Join-Path $PSScriptRoot 'controlled-r5-25'
if(Test-Path -LiteralPath $taskOut){throw 'Fresh controlled directory required'}
$taskIdentity=Get-Content (Join-Path $taskBase 'source-identity.json') -Raw|ConvertFrom-Json
if($taskIdentity.source_commit -ne (&git rev-parse HEAD).Trim() -or -not [string]::IsNullOrWhiteSpace((&git status --porcelain|Out-String))){throw 'Frozen controlled source changed'}
$taskHarness=Join-Path $taskBase 'integration.test.exe';$taskRunner=Join-Path $taskBase 'steerrunner.exe';$taskHelper=Join-Path $taskBase 'native-fixture.exe'
if((Get-FileHash $taskHarness -Algorithm SHA256).Hash -ne $taskIdentity.fixture_sha256 -or (Get-FileHash $taskRunner -Algorithm SHA256).Hash -ne $taskIdentity.runner_sha256){throw 'Frozen controlled binary changed'}
New-Item -ItemType Directory -Path $taskOut|Out-Null
function Quote-FixedControlArgument([string]$Argument){'"'+($Argument -replace '(\\*)"','$1$1\"' -replace '(\\+)$','$1$1')+'"'}
$taskArgs=@($taskHarness,('-test.run='+$taskIdentity.pattern),'-test.v','-test.timeout=25m','-runner-exe',$taskRunner,'-evidence-dir',$taskOut,'-helper-exe',$taskHelper)
$taskCommandLine=($taskArgs|ForEach-Object {Quote-FixedControlArgument $_}) -join ' '
$taskStartup=New-CimInstance -ClassName Win32_ProcessStartup -ClientOnly -Property @{ShowWindow=[uint16]0}
$taskCreated=Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{CommandLine=$taskCommandLine;CurrentDirectory=$PWD.ProviderPath;ProcessStartupInformation=$taskStartup}
if($taskCreated.ReturnValue -ne 0){throw 'Controlled process failed to start'}
$taskMetadata=[ordered]@{pid=$taskCreated.ProcessId;pattern=$taskIdentity.pattern;source_commit=$taskIdentity.source_commit;source_dirty=$false;runner_sha256=$taskIdentity.runner_sha256;fixture_sha256=$taskIdentity.fixture_sha256;original_frozen_binary_source=(Join-Path $taskBase 'source-identity.json');aggregate_timeout_minutes=25;scenario_deadlines_unchanged=$true;started_at=[DateTime]::UtcNow.ToString('o')}
$taskMetadata|ConvertTo-Json -Depth 10|Set-Content -Encoding utf8NoBOM (Join-Path $taskOut 'source-identity.json')
$taskUntil=[DateTime]::UtcNow.AddMinutes(25).AddSeconds(30)
while(-not (Test-Path (Join-Path $taskOut 'result.json'))){if([DateTime]::UtcNow -gt $taskUntil){throw 'Frozen controlled25 aggregate timed out'};Start-Sleep -Seconds 2}
$taskResult=Get-Content (Join-Path $taskOut 'result.json') -Raw|ConvertFrom-Json
if($taskResult.exit_code -ne 0 -or $taskResult.scenario_count -ne 16 -or (Select-String -LiteralPath (Join-Path $taskOut 'integration.log') -Pattern '--- SKIP:' -Quiet)){throw 'Controlled25 failed/skipped/count mismatch'}
Write-Output 'Frozen full Controlled16 passed under25m aggregate budget'
