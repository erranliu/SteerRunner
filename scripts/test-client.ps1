param([Parameter(Mandatory=$true)][string]$RunnerExe,[Parameter(Mandatory=$true)][string]$OutDir,[string]$ViewerExe='',[string]$ClientHelper='')
$ErrorActionPreference='Stop'
Set-StrictMode -Version 2
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$taskOut=[IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'Use a new isolated OutDir.' }
[IO.Directory]::CreateDirectory($taskOut) | Out-Null
$taskTools=Join-Path $taskOut 'tools';[IO.Directory]::CreateDirectory($taskTools) | Out-Null
$taskRunner=[IO.Path]::GetFullPath($RunnerExe)
$taskFake=Join-Path $taskTools 'steerrunner.exe'
Push-Location $taskRepo
try { & go build -o $taskFake ./tests/client/fixture; if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed' } } finally { Pop-Location }
foreach ($taskName in @('codex.exe','gh.exe','orca.exe')) { Copy-Item -LiteralPath $taskFake -Destination (Join-Path $taskTools $taskName) }
$taskGit=(Get-Command git.exe).Source
$taskProject=Join-Path $taskOut 'project';$taskBare=Join-Path $taskOut 'remote.git'
& $taskGit init --bare --initial-branch=main $taskBare | Out-Null
& $taskGit init --initial-branch=main $taskProject | Out-Null
& $taskGit -C $taskProject config user.email fixture@example.invalid
& $taskGit -C $taskProject config user.name Fixture
& $taskGit -C $taskProject config core.autocrlf true
[IO.File]::WriteAllText((Join-Path $taskProject 'spec.md'),"fixture spec`n")
[IO.File]::WriteAllText((Join-Path $taskProject 'plan.md'),'fixture plan')
& $taskGit -C $taskProject add .
& $taskGit -C $taskProject commit -m fixture | Out-Null
$taskURL='https://github.com/example/fixture.git'
& $taskGit -C $taskProject config ('url.'+$taskBare.Replace('\','/')+'.insteadOf') $taskURL
& $taskGit -C $taskProject remote add origin $taskURL
& $taskGit -C $taskProject push -u origin main | Out-Null
$taskRoot=Join-Path $taskOut 'client'
$taskHelper=if ($ClientHelper) { (Resolve-Path -LiteralPath $ClientHelper).Path } else { Join-Path $taskRepo 'client/steerrunner-client.ps1' }
$taskEnvironment=@{};foreach ($taskKey in @('PATH','APPDATA','SRCLI_TEST_ROOT','SRCLI_REAL_RUNNER','SRCLI_TEST_URL','SRCLI_TEST_LOST_RESPONSE','SRCLI_TEST_INSTANCE','SRCLI_TEST_SCENARIO','SRCLI_TEST_NATIVE_VERSION','ORCA_CLI_COMMAND','ORCA_DEV_REPO_ROOT')) { $taskEnvironment[$taskKey]=[Environment]::GetEnvironmentVariable($taskKey) }
$taskPassed=New-Object Collections.Generic.List[string]
function Invoke-Client([string]$Action,$InputValue,[int]$Expected=0,[switch]$NonGit) {
    $taskInput=Join-Path $taskOut ('input-'+[Guid]::NewGuid().ToString('N')+'.json')
    $taskArgs=@('-NoProfile','-File',$taskHelper,'-Action',$Action,'-ClientRoot',$taskRoot,'-ProjectPath',$taskProject)
    if ($NonGit) { $taskArgs[-1]=$env:SystemRoot }
    if ($null -ne $InputValue) { [IO.File]::WriteAllText($taskInput,($InputValue | ConvertTo-Json -Depth 50),(New-Object Text.UTF8Encoding($false)));$taskArgs+=@('-InputFile',$taskInput) }
    $taskText= & powershell.exe @taskArgs
    $taskExit=$LASTEXITCODE
    if ($taskExit -ne $Expected) { throw "Unexpected client exit for $Action : $taskExit / $taskText" }
    $taskResult=$taskText | ConvertFrom-Json
    return $taskResult
}
function Assert([bool]$Condition,[string]$Name) { if (-not $Condition) { throw "Assertion failed: $Name" };$taskPassed.Add($Name) }
try {
    $env:APPDATA=Join-Path $taskOut 'appdata';[IO.Directory]::CreateDirectory($env:APPDATA) | Out-Null
    $env:PATH=$taskTools+';'+(Split-Path -Parent $taskGit)+';'+$env:SystemRoot+'\System32;'+$env:SystemRoot+'\System32\WindowsPowerShell\v1.0'
    $env:SRCLI_TEST_ROOT=$taskOut;$env:SRCLI_REAL_RUNNER=$taskRunner;$env:SRCLI_TEST_URL=$taskURL;$env:ORCA_CLI_COMMAND=Join-Path $taskTools 'orca.exe';$env:ORCA_DEV_REPO_ROOT=$null
    $r=Invoke-Client Configure @{runner_executable=$taskFake;state_dir=(Join-Path $taskOut 'state')}
    Assert ($r.data.configuration.default_profile -eq 'local-codex') 'native default and production run-configuration validation'
    Assert ($r.data.discovery.project_binding.repository.url -eq $taskURL) 'project binding and publication template discovery'
    Assert (-not (Test-Path -LiteralPath (Join-Path $taskOut 'writes.log'))) 'configure sends zero business writes'
    foreach ($taskCase in @(
        @{version='0.159.2+build.7';eligible=$true},@{version='0.159.10';eligible=$true},@{version='0.163.0-alpha.10.2';eligible=$true},@{version='1.0.0';eligible=$true},
        @{version='0.159.1';eligible=$false},@{version='0.159.2-rc.1';eligible=$false},@{version='0.160.0-alpha.01';eligible=$false},@{version='0.0160.0';eligible=$false},@{version='0.160.0.1';eligible=$false}
    )) {
        $env:SRCLI_TEST_NATIVE_VERSION='codex-cli '+$taskCase.version;$r=Invoke-Client Discover $null
        $taskEligible=@($r.data.discovery.candidates | Where-Object { $_.id -eq 'local-codex' }).Count -eq 1
        Assert ($taskEligible -eq $taskCase.eligible) ('Runner-declared native minimum: '+$taskCase.version)
    }
    $env:SRCLI_TEST_NATIVE_VERSION=$null
    $r=Invoke-Client Configure @{default_profile='orca-local'}
    $r=Invoke-Client Configure $null
    Assert ($r.data.configuration.default_profile -eq 'orca-local') 'repeat discovery preserves explicit choice'
    $null=Invoke-Client Configure @{default_profile='local-codex'}
    [IO.File]::WriteAllText((Join-Path $taskProject 'spec.md'),"fixture spec`r`n")
    $submission=@{submission_id='submit-original';title='fixture 中文 "quoted" \ tail';spec=(Join-Path $taskProject 'spec.md');plan='plan.md'}
    $env:SRCLI_TEST_LOST_RESPONSE='1'
    $r=Invoke-Client Submit $submission 6
    Assert ($r.data.operation_id -eq 'submit-original') 'lost response preserves original command handle'
    $saved=Get-Content -LiteralPath (Join-Path $taskRoot 'submissions/submit-original/request.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    Assert ($saved.input.title -ceq $submission.title -and $saved.input.documents.spec.path -ceq 'spec.md') 'UTF-8 and quoted argv with an absolute CRLF material path'
    $env:SRCLI_TEST_LOST_RESPONSE=$null;$env:SRCLI_TEST_INSTANCE='replacement'
    $r=Invoke-Client Reconcile @{operation_id='submit-original'}
    Assert $r.data.reconciled 'service replacement reconciles original request'
    $r=Invoke-Client Submit $submission
    Assert (@(Get-Content -LiteralPath (Join-Path $taskOut 'writes.log')).Count -eq 1) 'same submission never creates a second run'
    $r=Invoke-Client Observe @{submission_id='submit-original'}
    Assert ($r.data.cli.response.data.event_page.next_cursor -eq 'cursor-1') 'same-snapshot observe cursor'
    $r=Invoke-Client Reply @{submission_id='submit-original';operation_id='answer-original';text='answer';question_id='old-question'} 2
    Assert ($r.error.code -eq 'stale_question') 'old question sends zero input'
    $env:SRCLI_TEST_LOST_RESPONSE='1'
    $r=Invoke-Client Reply @{submission_id='submit-original';operation_id='answer-original';text='answer';question_id='question-1'} 6
    $env:SRCLI_TEST_LOST_RESPONSE=$null
    $r=Invoke-Client Reconcile @{operation_id='answer-original'}
    Assert $r.data.reconciled 'reply lost response reconciles without new answer'
    $r=Invoke-Client Resume @{submission_id='submit-original';operation_id='resume-original'}
    $r=Invoke-Client Stop @{submission_id='submit-original';operation_id='stop-original'}
    Assert (@(Get-Content -LiteralPath (Join-Path $taskOut 'writes.log')).Count -eq 4) 'submit reply resume stop each dispatched once'
    $env:SRCLI_TEST_SCENARIO='no-question'
    $r=Invoke-Client Reply @{submission_id='submit-original';operation_id='feedback-without-question';text='explicit feedback'}
    Assert $r.ok 'feedback handles an omitted optional question field'
    $env:SRCLI_TEST_SCENARIO=$null
    $r=Invoke-Client Configure $null -NonGit
    Assert ($null -eq $r.data.discovery.project) 'non-Git invocation still discovers machine routes'
    $env:SRCLI_TEST_SCENARIO='orca-empty';$r=Invoke-Client Discover $null
    Assert ($r.data.discovery.gaps -contains 'orca_no_remote_environment') 'empty remote environment list is distinct from missing Orca'
    $env:SRCLI_TEST_SCENARIO='orca-auth';$r=Invoke-Client Discover $null
    Assert ($r.data.discovery.gaps -contains 'orca_environment_query_unverified' -and $r.data.discovery.gaps -notcontains 'orca_no_remote_environment' -and $r.data.discovery.orca.query_error -eq 'authentication_failed') 'failed authentication is not treated as an empty environment list'
    $env:SRCLI_TEST_SCENARIO='orca-offline';$r=Invoke-Client Discover $null
    Assert ($r.data.discovery.orca.installed -and -not $r.data.discovery.orca.environments[0].connected) 'offline Orca preserves environment and does not claim missing installation'
    $env:SRCLI_TEST_SCENARIO=$null;$env:ORCA_CLI_COMMAND=$null
    Move-Item -LiteralPath (Join-Path $taskTools 'orca.exe') -Destination (Join-Path $taskTools 'orca-disabled.exe')
    try { $r=Invoke-Client Discover $null; Assert ($r.data.discovery.gaps -contains 'orca_missing' -and $r.data.discovery.orca.hint.Length -gt 0) 'missing Orca proactively explains remote installation requirement' }
    finally { Move-Item -LiteralPath (Join-Path $taskTools 'orca-disabled.exe') -Destination (Join-Path $taskTools 'orca.exe');$env:ORCA_CLI_COMMAND=Join-Path $taskTools 'orca.exe' }
    $taskConcurrentInput=Join-Path $taskOut 'concurrent-input.json'
    [IO.File]::WriteAllText($taskConcurrentInput,(@{submission_id='submit-original';operation_id='concurrent-stop'} | ConvertTo-Json))
    $taskChildArguments=@('-NoProfile','-File',('"'+$taskHelper+'"'),'-Action','Stop','-ClientRoot',('"'+$taskRoot+'"'),'-ProjectPath',('"'+$taskProject+'"'),'-InputFile',('"'+$taskConcurrentInput+'"'))
    $taskFirst=Start-Process powershell.exe -ArgumentList $taskChildArguments -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $taskOut 'concurrent-first.txt')
    $taskSecond=Start-Process powershell.exe -ArgumentList $taskChildArguments -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $taskOut 'concurrent-second.txt')
    try { if (-not $taskFirst.WaitForExit(20000) -or -not $taskSecond.WaitForExit(20000)) { throw 'Concurrent fixture timed out' }; Assert (@(Get-Content -LiteralPath (Join-Path $taskOut 'writes.log') | Where-Object { $_ -eq 'stop concurrent-stop' }).Count -eq 1) 'concurrent helpers serialize one original command' } finally { $taskFirst.Dispose();$taskSecond.Dispose() }
    [IO.File]::AppendAllText((Join-Path $taskRoot 'submissions/submit-original/request.json'),' ')
    $r=Invoke-Client Reconcile @{operation_id='submit-original';retry_same_request=$true} 2
    Assert ($r.error.code -eq 'damaged_request_record') 'damaged request never rerenders or resends'
    [IO.File]::AppendAllText((Join-Path $taskProject 'spec.md'),' changed')
    $r=Invoke-Client Submit @{submission_id='changed-material';title='bad';spec='spec.md';plan='plan.md'} 2
    Assert ($r.error.code -eq 'material_not_in_base') 'working material mismatch blocks before submit'
    $r=Invoke-Client Configure @{runner_executable=(Join-Path $taskOut 'missing.exe')} 2
    Assert ($r.error.code -eq 'configured_runner_missing') 'explicit missing Runner has no fallback'
    [IO.File]::WriteAllText((Join-Path $taskRoot 'client.json'),'{"version":1,"version":1}')
    $r=Invoke-Client Configure $null 2
    Assert ($r.error.code -eq 'invalid_json') 'duplicate configuration key rejected without overwrite'
    $taskReport=@{controlled=$true;passed=@($taskPassed);runner_sha256=(Get-FileHash -LiteralPath $taskRunner -Algorithm SHA256).Hash;client_helper_sha256=(Get-FileHash -LiteralPath $taskHelper -Algorithm SHA256).Hash;live_execution=$false}
    [IO.File]::WriteAllText((Join-Path $taskOut 'result.json'),($taskReport | ConvertTo-Json -Depth 8),(New-Object Text.UTF8Encoding($false)))
    Write-Output ($taskReport | ConvertTo-Json -Depth 8)
} finally { foreach ($taskKey in $taskEnvironment.Keys) { [Environment]::SetEnvironmentVariable($taskKey,$taskEnvironment[$taskKey]) } }
if ($ViewerExe) { & (Join-Path $PSScriptRoot 'test-viewer.ps1') -ViewerExe $ViewerExe -OutDir (Join-Path $taskOut 'viewer') }
