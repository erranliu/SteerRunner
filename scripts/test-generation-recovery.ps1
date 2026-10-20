param(
    [string]$NativeExecutable = '',
    [string]$Model = '',
    [string]$OutDir = ''
)
$ErrorActionPreference = 'Stop'
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $OutDir) { $OutDir = Join-Path $taskRepo ('.evidence/recovery-' + [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss')) }
$taskOutput = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
if (Test-Path -LiteralPath $taskOutput) { throw 'Evidence directory must be new.' }
if ($NativeExecutable) { $NativeExecutable = (Resolve-Path -LiteralPath $NativeExecutable).Path }
New-Item -ItemType Directory -Path $taskOutput | Out-Null
function Wait-RecoveryResult([string]$Directory, [int]$Expected) {
    $taskDeadline = [DateTime]::UtcNow.AddMinutes(26)
    while ([DateTime]::UtcNow -lt $taskDeadline) {
        $taskResultPath = Join-Path $Directory 'result.json'
        if (Test-Path -LiteralPath $taskResultPath) {
            $taskResult = Get-Content -LiteralPath $taskResultPath -Raw | ConvertFrom-Json
            if ($taskResult.exit_code -ne 0 -or $taskResult.scenario_count -ne $Expected -or (Select-String -LiteralPath (Join-Path $Directory 'integration.log') -Pattern '--- SKIP:' -Quiet)) { throw "Recovery tests failed, skipped or did not execute every selected scenario: $taskResultPath" }
            return $taskResult
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Original test outcome unverified; inspect $Directory before any new execution."
}
Push-Location $taskRepo
try {
    & go test ./internal/application ./internal/host ./internal/storage ./internal/execution/workspace ./internal/execution/adapters/codex ./internal/execution/adapters/orcatransport -run 'TestServiceGeneration|TestWorkflowRecovery|TestLegacyRecovery|TestPendingStops|TestRecoveryUpgrade|TestExecutionOwner|TestSRPreparationQuery|TestRepositoryQuery|TestRepositoryCache|TestR2' -count=1 -timeout=4m 2>&1 | Tee-Object -FilePath (Join-Path $taskOutput 'rules.log')
    if ($LASTEXITCODE -ne 0) { throw 'Recovery rule tests failed.' }
    $taskControlled = Join-Path $taskOutput 'controlled'
    & (Join-Path $PSScriptRoot 'test-foundation.ps1') -Pattern '^TestSR(RestartPreservesAuthorization|ResultAcquisition|RunEvidenceR2|ContentGenerationRecovery)$' -OutDir $taskControlled
    $null = Wait-RecoveryResult $taskControlled 4
    $taskSummary = [ordered]@{ controlled = 'passed'; native = 'not_requested'; evidence = $taskOutput }
    if ($NativeExecutable) {
        $taskNative = Join-Path $taskOutput 'native'
        & (Join-Path $PSScriptRoot 'test-foundation.ps1') -Pattern '^TestSR(ContentNativeGenerationRecovery|NativeParserGenerationRecovery)$' -OutDir $taskNative -ContentNativeExecutable $NativeExecutable -ContentNativeModel $Model
        $null = Wait-RecoveryResult $taskNative 2
        $taskSummary.native = 'passed'
    }
    [IO.File]::WriteAllText((Join-Path $taskOutput 'summary.json'),($taskSummary | ConvertTo-Json),[Text.UTF8Encoding]::new($false))
    $taskSummary | ConvertTo-Json
} finally { Pop-Location }
