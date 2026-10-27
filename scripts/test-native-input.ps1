param(
    [Parameter(Mandatory)][string]$Target,
    [Parameter(Mandatory)][string]$OutDir,
    [ValidateSet('long-input','segmented-shape')][string]$Mode = 'long-input'
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
. (Join-Path $PSScriptRoot 'delivery-contract.ps1')
$taskSourcePaths = @('internal','go.mod','go.sum','scripts/test-native-input.ps1','scripts/delivery-contract.ps1')
$taskIdentity = Get-SRSourceIdentity $taskRepo $taskSourcePaths
$taskTarget = Get-Content -LiteralPath $Target -Raw | ConvertFrom-Json
$taskOut = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'Probe output must be new; never replay an attempted root' }
New-Item -ItemType Directory -Path $taskOut | Out-Null
Copy-Item -LiteralPath $Target -Destination (Join-Path $taskOut 'target.json')
$taskSource = [ordered]@{source_commit=(& git -C $taskRepo rev-parse HEAD).Trim();source_dirty=-not [string]::IsNullOrWhiteSpace((& git -C $taskRepo status --porcelain | Out-String));mode=$Mode;target_sha256=(Get-FileHash -LiteralPath $Target).Hash;passed=$false}
$taskSource['source_digest']=$taskIdentity.source_digest
$taskSource['orca_cli_sha256']=(Get-FileHash -LiteralPath $taskTarget.CLI).Hash
$taskKeys = @('STEERRUNNER_ORCA_STATE_PROBE_ROOT','STEERRUNNER_ORCA_STATE_PROBE_MODE','STEERRUNNER_ORCA_STATE_PROBE_ENVIRONMENT')
$taskPrevious = @{}
foreach($taskKey in $taskKeys) { $taskPrevious[$taskKey]=[Environment]::GetEnvironmentVariable($taskKey) }
Push-Location $taskRepo
try {
    $taskExe = Join-Path $taskOut 'native-input.test.exe'
    & go test -c -o $taskExe ./internal/execution/adapters/orcatransport 1> (Join-Path $taskOut 'compile.log') 2> (Join-Path $taskOut 'compile-stderr.log')
    if ($LASTEXITCODE -ne 0) { throw 'Native input probe compilation failed' }
    $taskSource['test_exe_sha256']=(Get-FileHash -LiteralPath $taskExe).Hash
    [Environment]::SetEnvironmentVariable($taskKeys[0],$taskOut)
    [Environment]::SetEnvironmentVariable($taskKeys[1],$Mode)
    [Environment]::SetEnvironmentVariable($taskKeys[2],[string]$taskTarget.Environment)
    & $taskExe '-test.run=^TestOrcaNativeStateLiveLifecycle$' '-test.count=1' '-test.timeout=6m' '-test.v' 1> (Join-Path $taskOut 'test.log') 2> (Join-Path $taskOut 'test-stderr.log')
    $taskSource['exit_code']=$LASTEXITCODE
    $taskLog=Get-Content -LiteralPath (Join-Path $taskOut 'test.log') -Raw
    if ($LASTEXITCODE -ne 0 -or $taskLog -notmatch '(?m)^--- PASS: TestOrcaNativeStateLiveLifecycle ' -or $taskLog -match '(?m)^--- SKIP:') { throw 'Native input probe failed, skipped or did not execute' }
    $taskResult=Get-Content -LiteralPath (Join-Path $taskOut 'native-result.json') -Raw | ConvertFrom-Json
    if (-not $taskResult.completed -or -not $taskResult.cleaned_up) { throw 'Missing original completion or confirmed cleanup' }
    if ($Mode -eq 'long-input' -and ($taskResult.paste_calls -ne 1 -or $taskResult.enter_calls -ne 1 -or $taskResult.business_inputs -ne 1)) { throw 'Expected one production paste, one submit and one original input' }
    if ($Mode -eq 'segmented-shape' -and ($taskResult.paste_calls -ne 2 -or $taskResult.enter_calls -ne 1 -or $taskResult.logical_inputs -ne 1)) { throw 'Expected two real paste effects, one submit and one logical original input' }
    $taskAfter=Get-SRSourceIdentity $taskRepo $taskSourcePaths
    if ($taskAfter.source_digest -cne $taskIdentity.source_digest) { throw 'Probe source changed during execution' }
    $taskSource.passed=$true
} finally {
    foreach($taskKey in $taskKeys) { [Environment]::SetEnvironmentVariable($taskKey,$taskPrevious[$taskKey]) }
    Pop-Location
    $taskSource['finished_at']=[DateTime]::UtcNow.ToString('o')
    [IO.File]::WriteAllText((Join-Path $taskOut 'result.json'),($taskSource|ConvertTo-Json -Depth 10),[Text.UTF8Encoding]::new($false))
}
Write-Output $taskOut
