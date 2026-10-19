param([Parameter(Mandatory=$true)][string]$Request)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$taskJob = Get-Content -LiteralPath $Request -Raw | ConvertFrom-Json
$taskRecord = $taskJob.record | ConvertTo-Json -Depth 30 | ConvertFrom-Json -AsHashtable
function Save-Case {
    $taskTemporary = $taskJob.result_path + '.tmp'
    [IO.File]::WriteAllText($taskTemporary,($taskRecord | ConvertTo-Json -Depth 40),[Text.UTF8Encoding]::new($false))
    [IO.File]::Move($taskTemporary,$taskJob.result_path,$true)
}
try {
    $taskRecord.started_at = [DateTime]::UtcNow.ToString('o')
    $taskRecord.status=if($taskJob.entry.driver -eq 'controlled-integration'){'running'}else{'preparing'}
    Save-Case
    if ($taskJob.entry.driver -eq 'controlled-integration') {
        $taskRecord.evidence = Join-Path $taskJob.output ($taskJob.entry.id + '-r' + $taskJob.repeat_index)
        $taskPattern = '^' + [regex]::Escape($taskJob.case.test) + '$'
        & (Join-Path $PSScriptRoot 'test-foundation.ps1') -OutDir $taskRecord.evidence -Pattern $taskPattern | Out-Host
        $taskUntil = [DateTime]::UtcNow.AddMinutes(15).AddSeconds(30)
        $taskResultPath = Join-Path $taskRecord.evidence 'result.json'
        while (-not (Test-Path -LiteralPath $taskResultPath)) {
            if ([DateTime]::UtcNow -gt $taskUntil) { throw 'Controlled case timed out; preserve original evidence' }
            Start-Sleep -Milliseconds 500
        }
        $taskResult = Get-Content -LiteralPath $taskResultPath -Raw | ConvertFrom-Json
        if ($taskResult.exit_code -ne 0 -or $taskResult.scenario_count -ne 1 -or (Select-String -LiteralPath (Join-Path $taskRecord.evidence 'integration.log') -Pattern '--- SKIP:' -Quiet)) { throw 'Controlled case failed, skipped or did not execute exactly once' }
    } else {
        foreach ($taskArtifact in @(@{identity='configuration_sha256';path='config'},@{identity='runner_sha256';path='runner_exe'},@{identity='helper_sha256';path='config_helper'})) {
            $taskExpected=$taskJob.PSObject.Properties[$taskArtifact.identity]
            if ($taskExpected -and $taskExpected.Value) {
                $taskPath=[string]$taskJob.($taskArtifact.path)
                if ((Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant() -cne $taskExpected.Value) { throw ('Frozen batch artifact changed: ' + $taskArtifact.identity) }
            }
        }
        if ($taskJob.entry.id -eq 'local-codex.smoke') {
            . (Join-Path $PSScriptRoot 'test-configuration.ps1')
            $taskBoundaryConfig=Get-SRTestConfiguration -RunnerExe $taskJob.runner_exe -Helper $taskJob.config_helper -Config $taskJob.config
            if ($taskJob.PSObject.Properties['configuration_sha256'] -and $taskJob.configuration_sha256 -and $taskBoundaryConfig.source_digest -cne $taskJob.configuration_sha256) { throw 'Frozen batch artifact changed before L2 Git boundary' }
            $taskBoundaryLog=Join-Path $taskJob.output ('local-codex-smoke-r'+$taskJob.repeat_index+'-git-boundary.log')
            $taskRecord['git_boundary']=@{evidence=$taskBoundaryLog}
            Save-Case
            Push-Location ([IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..')))
            try {
                & go test -tags=live ./internal/execution/adapters/codex -run '^TestLinkedGitNativeCommit$' -count=1 -timeout=3m -v -args -sandbox-auth-native $taskBoundaryConfig.configuration.configurations.'local-codex'.execution.settings.executable 1> $taskBoundaryLog 2> ($taskBoundaryLog+'.stderr.log')
                $taskRecord.git_boundary['exit_code']=$LASTEXITCODE
            } finally { Pop-Location }
            if ($taskRecord.git_boundary.exit_code -ne 0) { throw 'L2 native Git commit or repository isolation failed' }
        }
        $taskExpectedIdentities=@{}
        foreach ($taskIdentity in @(@{field='configuration_sha256';parameter='ConfigurationSHA256'},@{field='runner_sha256';parameter='RunnerSHA256'},@{field='helper_sha256';parameter='HelperSHA256'},@{field='requirements_sha256';parameter='RequirementsSHA256'})) {
            if ($taskJob.PSObject.Properties[$taskIdentity.field]) { $taskExpectedIdentities[$taskIdentity.parameter]=[string]$taskJob.($taskIdentity.field) }
        }
        $taskRecord.manifest = [string](& (Join-Path $PSScriptRoot 'new-test-fixture.ps1') -Scenario $taskJob.entry.id -Case $taskJob.case.id -RepeatIndex $taskJob.repeat_index -Config $taskJob.config -Catalog $taskJob.catalog -FixtureParent $taskJob.fixture_parent -RecordPath $taskRecord.fixture_record -RunnerExe $taskJob.runner_exe -ConfigHelper $taskJob.config_helper @taskExpectedIdentities)
        $taskRecord.evidence = Join-Path (Split-Path -Parent $taskRecord.manifest) 'evidence'
        $taskRecord.status = 'running'
        Save-Case
        $taskRecord.evidence = & (Join-Path $PSScriptRoot 'test-native.ps1') -Manifest $taskRecord.manifest -TimeoutMinutes $taskJob.entry.timeout_minutes
        $taskManifest = Get-Content -LiteralPath $taskRecord.manifest -Raw | ConvertFrom-Json
        $taskScenario = Get-Content -LiteralPath (Join-Path $taskRecord.evidence 'scenario.json') -Raw | ConvertFrom-Json
        . (Join-Path $PSScriptRoot 'native-result.ps1')
        if($taskManifest.driver -eq 'environment-check'){
            $taskCheck=(Get-Content -LiteralPath (Join-Path $taskRecord.evidence 'check-latest.json') -Raw|ConvertFrom-Json).data.check
            $taskView=[pscustomobject]@{run_id=$taskCheck.check_id;execution=[pscustomobject]@{workspace=$taskCheck.actual.workspace;session_id=$taskCheck.actual.session_id;branch=$taskCheck.resources.head_ref};pr=[pscustomobject]@{url=$taskCheck.resources.pr_url}}
        } else {
            $taskView=if($taskScenario.PSObject.Properties['runs']){$null}else{Get-Content -LiteralPath (Join-Path $taskRecord.evidence 'latest-run.json') -Raw|ConvertFrom-Json}
        }
        $taskRecord.resources = @(Get-SRNativeResources $taskManifest $taskScenario $taskView)
    }
    $taskRecord.identity = Get-Content -LiteralPath (Join-Path $taskRecord.evidence 'source-identity.json') -Raw | ConvertFrom-Json -AsHashtable
    foreach ($taskKey in @($taskRecord.identity.Keys)) {
        if ($taskKey -like '*_sha256') { $taskRecord.identity[$taskKey]=([string]$taskRecord.identity[$taskKey]).ToLowerInvariant() }
    }
    $taskRecord.expected_outcome=$taskJob.case.outcome
    $taskRecord.status = 'passed'
} catch {
    $taskRecord.status = 'failed'
    $taskRecord.error = $_.Exception.Message
} finally {
    $taskRecord.finished_at = [DateTime]::UtcNow.ToString('o')
    Save-Case
}
if ($taskRecord.status -ne 'passed') { exit 1 }
