param(
    [string]$Target = '',
    [Parameter(Mandatory)][string]$OutDir,
    [ValidateSet('long-input','controlled')][string]$Mode = 'long-input',
    [ValidateRange(1,30)][int]$Repeat = 1
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
. (Join-Path $PSScriptRoot 'delivery-contract.ps1')
$taskSourcePaths = @('internal','cmd/steerrunner-agent','go.mod','go.sum','scripts/test-native-input.ps1','scripts/delivery-contract.ps1')
$taskIdentity = Get-SRSourceIdentity $taskRepo $taskSourcePaths
if ($Mode -ne 'controlled' -and (-not $Target -or $Repeat -ne 1)) { throw 'Real probe requires a fixed target and one fresh independent session per invocation' }
$taskTarget = if ($Mode -eq 'controlled') { $null } else { Get-Content -LiteralPath $Target -Raw | ConvertFrom-Json }
$taskOut = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'Probe output must be new; never replay an attempted root' }
New-Item -ItemType Directory -Path $taskOut | Out-Null
if ($Target) { Copy-Item -LiteralPath $Target -Destination (Join-Path $taskOut 'target.json') }
$taskSource = [ordered]@{source_commit=(& git -C $taskRepo rev-parse HEAD).Trim();source_dirty=-not [string]::IsNullOrWhiteSpace((& git -C $taskRepo status --porcelain | Out-String));mode=$Mode;target_sha256=$(if($Target){(Get-FileHash -LiteralPath $Target).Hash}else{''});passed=$false}
$taskSource['source_digest']=$taskIdentity.source_digest
if ($Mode -eq 'controlled') {
    Push-Location $taskRepo
    try {
        $taskExe = Join-Path $taskOut 'native-input.test.exe'
        & go test -c -o $taskExe ./internal/execution/adapters/orcatransport 1> (Join-Path $taskOut 'compile.log') 2> (Join-Path $taskOut 'compile-stderr.log')
        if ($LASTEXITCODE -ne 0) { throw 'Controlled native input compilation failed' }
        $taskPattern = '^(TestNativePrompt.*|TestLocalNativeProbe.*|TestNativeCRLFPastePreservesOriginalInputAndCompletion|TestNativeFollowupAfterCompletedToolTurn|TestNativeUnstartedReceiptRecoversOnlyCompletedOriginalInput|TestOrcaNativeInputProofRejectsUnboundHistory|TestOrcaNativeStateBinding|TestRecoveryRejectsProviderReplacementInsideOriginalTerminal|TestNativeComposerGatePreventsPrematureInput)$'
        Push-Location (Join-Path $taskRepo 'internal/execution/adapters/orcatransport')
        try {
            & go tool test2json -t -p github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport $taskExe "-test.run=$taskPattern" "-test.count=$Repeat" '-test.timeout=8m' '-test.v' 1> (Join-Path $taskOut 'tests.jsonl') 2> (Join-Path $taskOut 'test-stderr.log')
            $taskExit = $LASTEXITCODE
        } finally { Pop-Location }
        $taskEvents = @(Get-Content -LiteralPath (Join-Path $taskOut 'tests.jsonl') | ForEach-Object { $_ | ConvertFrom-Json })
        $taskTestNames = @($taskEvents | Where-Object { $_.PSObject.Properties.Name -contains 'Test' } | ForEach-Object {$_.Test} | Sort-Object -Unique)
        $taskRequiredRoots = @('TestLocalNativeProbeTargetAndBusinessScope','TestLocalNativeProbeCensusKeepsOriginalHostScope','TestNativePromptSubmission','TestNativePromptReceiptFaults','TestNativePromptProcessingOrder','TestNativePromptControlCharactersRejected','TestNativePromptSharedQueueWithStopAndCancellation','TestNativeCRLFPastePreservesOriginalInputAndCompletion','TestNativeFollowupAfterCompletedToolTurn','TestNativeUnstartedReceiptRecoversOnlyCompletedOriginalInput','TestOrcaNativeInputProofRejectsUnboundHistory','TestOrcaNativeStateBinding','TestRecoveryRejectsProviderReplacementInsideOriginalTerminal','TestNativeComposerGatePreventsPrematureInput')
        $taskMissingRoots = @($taskRequiredRoots | Where-Object { $_ -cnotin $taskTestNames })
        $taskLeaves = @($taskTestNames | Where-Object { $taskName = $_; @($taskTestNames | Where-Object { $_.StartsWith($taskName + '/') }).Count -eq 0 })
        $taskCases = @($taskLeaves | ForEach-Object {
            $taskName = $_
            $taskTerminal = @($taskEvents | Where-Object { $_.PSObject.Properties.Name -contains 'Test' -and $_.Test -ceq $taskName -and $_.Action -in @('pass','fail','skip') })
            [ordered]@{id=$taskName;executed=$taskTerminal.Count;passed=@($taskTerminal|Where-Object Action -eq 'pass').Count;failed=@($taskTerminal|Where-Object Action -eq 'fail').Count;skipped=@($taskTerminal|Where-Object Action -eq 'skip').Count}
        })
        $taskReport = [ordered]@{schema='steerrunner-native-input-controlled/v1';repeat=$Repeat;cases=$taskCases;executed=0;passed=0;failed=0;skipped=0;exit_code=$taskExit;source=$taskSource;required_roots=$taskRequiredRoots;missing_roots=$taskMissingRoots}
        foreach ($taskCase in $taskCases) { foreach ($taskField in @('executed','passed','failed','skipped')) { $taskReport[$taskField] += $taskCase[$taskField] } }
        [IO.File]::WriteAllText((Join-Path $taskOut 'controlled-summary.json'),($taskReport|ConvertTo-Json -Depth 15),[Text.UTF8Encoding]::new($false))
        if ($taskExit -ne 0 -or $taskMissingRoots.Count -gt 0 -or $taskCases.Count -eq 0 -or $taskReport.failed -ne 0 -or $taskReport.skipped -ne 0 -or @($taskCases|Where-Object {$_.executed -ne $Repeat}).Count -ne 0) { throw 'Controlled input regression failed, skipped or did not run each leaf exactly Repeat times' }
        $taskAfter=Get-SRSourceIdentity $taskRepo $taskSourcePaths
        if ($taskAfter.source_digest -cne $taskIdentity.source_digest) { throw 'Controlled probe source changed during execution' }
        $taskSource.passed=$true
    } finally {
        Pop-Location
        $taskSource['finished_at']=[DateTime]::UtcNow.ToString('o')
        [IO.File]::WriteAllText((Join-Path $taskOut 'result.json'),($taskSource|ConvertTo-Json -Depth 10),[Text.UTF8Encoding]::new($false))
    }
    Write-Output $taskOut
    return
}
$taskSource['orca_cli_sha256']=(Get-FileHash -LiteralPath $taskTarget.CLI).Hash
if (-not $taskTarget.Environment) {
    if (-not $taskTarget.Launch.runtime_executable -or -not $taskTarget.Launch.repository_url) { throw 'Local native probe needs the existing agent helper and original repository URL' }
    $taskSource['agent_sha256']=(Get-FileHash -LiteralPath $taskTarget.Launch.runtime_executable).Hash
}
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
    & $taskExe '-test.run=^TestOrcaNativeStateLiveLifecycle$' '-test.count=1' '-test.timeout=10m' '-test.v' 1> (Join-Path $taskOut 'test.log') 2> (Join-Path $taskOut 'test-stderr.log')
    $taskSource['exit_code']=$LASTEXITCODE
    $taskLog=Get-Content -LiteralPath (Join-Path $taskOut 'test.log') -Raw
    if ($LASTEXITCODE -ne 0 -or $taskLog -notmatch '(?m)^--- PASS: TestOrcaNativeStateLiveLifecycle ' -or $taskLog -match '(?m)^--- SKIP:') { throw 'Native input probe failed, skipped or did not execute' }
    $taskResult=Get-Content -LiteralPath (Join-Path $taskOut 'native-result.json') -Raw | ConvertFrom-Json
    if (-not $taskResult.completed -or -not $taskResult.cleaned_up) { throw 'Missing original completion or confirmed cleanup' }
    if ($taskResult.submit_calls -ne 1 -or $taskResult.text_only_calls -ne 0 -or $taskResult.bare_enter_calls -ne 0 -or $taskResult.screen_reads -ne 0 -or $taskResult.business_inputs -ne 1) { throw 'Expected one generic submit, zero split inputs/screen reads and one original input' }
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
