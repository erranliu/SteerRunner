param(
    [string]$NativeExecutable = '',
    [string]$Model = '',
    [string]$OutDir = ''
)
$ErrorActionPreference = 'Stop'
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $OutDir) { $OutDir = Join-Path $taskRepo ('.evidence/content-' + [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss')) }
$taskOutput = [IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskOutput) { throw 'Evidence directory must be new.' }
New-Item -ItemType Directory -Path $taskOutput | Out-Null
function Wait-ContentResult([string]$Directory) {
    $taskDeadline = [DateTime]::UtcNow.AddMinutes(12)
    while ([DateTime]::UtcNow -lt $taskDeadline) {
        $taskResultPath = Join-Path $Directory 'result.json'
        if (Test-Path -LiteralPath $taskResultPath) {
            $taskResult = Get-Content -LiteralPath $taskResultPath -Raw | ConvertFrom-Json
            if ($taskResult.exit_code -ne 0 -or $taskResult.scenario_count -lt 1) { throw "Tests failed: $taskResultPath" }
            return $taskResult
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Original test outcome unverified; inspect $Directory. Do not rerun its model requests."
}
Push-Location $taskRepo
try {
    & go test ./internal/contracts ./internal/resources ./internal/cli ./internal/execution/workflow ./internal/execution/reliable/... ./internal/execution/resultacquisition ./internal/execution/adapters/codex ./internal/application -run 'TestContent|TestSRPublishedRunConfiguration|TestSRValidation' -count=1 -timeout=4m 2>&1 | Tee-Object -FilePath (Join-Path $taskOutput 'rules.log')
    if ($LASTEXITCODE -ne 0) { throw 'Content rule tests failed.' }
    $taskControlled = Join-Path $taskOutput 'controlled'
    & (Join-Path $PSScriptRoot 'test-foundation.ps1') -Pattern '^TestSRContent(Generate|Conversation|QuestionAndUnknown)$' -OutDir $taskControlled
    $taskControlledResult = Wait-ContentResult $taskControlled
    if ($taskControlledResult.scenario_count -ne 3) { throw 'Controlled content scenario count mismatch.' }
    $taskSummary = [ordered]@{ controlled = 'passed'; native = 'not_requested'; evidence = $taskOutput }
    if ($NativeExecutable) {
        $taskNative = Join-Path $taskOutput 'native'
        & (Join-Path $PSScriptRoot 'test-foundation.ps1') -Pattern '^TestSRContentNative$' -OutDir $taskNative -ContentNativeExecutable $NativeExecutable -ContentNativeModel $Model
        $taskNativeResult = Wait-ContentResult $taskNative
        if ($taskNativeResult.scenario_count -ne 1) { throw 'Native content scenario count mismatch.' }
        $taskSummary.native = 'passed'
    }
    [IO.File]::WriteAllText((Join-Path $taskOutput 'summary.json'),($taskSummary|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
    $taskSummary | ConvertTo-Json
} finally { Pop-Location }
