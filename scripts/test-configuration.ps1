# Test-only configuration consumer. Production never imports this script.
function Get-SRTestConfiguration {
    param([Parameter(Mandatory=$true)][string]$RunnerExe,[Parameter(Mandatory=$true)][string]$Helper,[string]$Config='')
    # CLI JSON is UTF-8 even when a scheduler child inherits an OEM console.
    # Preserve non-ASCII workflow prompts before passing the same bytes to the helper.
    $taskPriorConsoleEncoding = [Console]::OutputEncoding
    $taskPriorPipeEncoding = $OutputEncoding
    try {
        [Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
        $OutputEncoding = [Text.UTF8Encoding]::new($false)
        foreach ($taskParameter in @('RunnerExe','Helper','Config')) {
            $taskValue = Get-Variable -Name $taskParameter -ValueOnly
            if ($taskValue) { Set-Variable -Name $taskParameter -Value $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($taskValue) }
        }
        if (-not (Test-Path -LiteralPath $RunnerExe -PathType Leaf) -or -not (Test-Path -LiteralPath $Helper -PathType Leaf)) { throw 'Build the selected Runner and test configuration helper before querying availability.' }
        $taskCapabilitiesJSON = (& $RunnerExe capabilities --offline --json | Out-String)
        if ($LASTEXITCODE -ne 0) { throw 'Cannot query the selected Runner capability contract.' }
        $taskCapabilities = $taskCapabilitiesJSON | ConvertFrom-Json -AsHashtable
        $taskActualSHA = (Get-FileHash -LiteralPath $RunnerExe -Algorithm SHA256).Hash.ToLowerInvariant()
        if (-not $taskCapabilities.ok -or $taskCapabilities.data.capability_source.kind -cne 'executable' -or $taskCapabilities.data.capability_source.executable_sha256 -cne $taskActualSHA) { throw 'Selected Runner capability identity changed.' }
        $taskResolved = if ($Config) { [IO.Path]::GetFullPath($Config) } else { '' }
        $taskAssessmentJSON = ($taskCapabilitiesJSON | & $Helper --test-config $taskResolved - | Out-String)
        if ($LASTEXITCODE -ne 0) { throw 'Invalid strict test configuration or mismatched Runner/helper contract; migrate old live-config files.' }
        $taskAssessment = $taskAssessmentJSON | ConvertFrom-Json -AsHashtable
        $taskRegistrySHA = (Get-FileHash -LiteralPath (Join-Path $PSScriptRoot '../internal/testkit/livetest/cases.json') -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($taskAssessment.requirements_digest -cne $taskRegistrySHA) { throw 'Selected test helper and case requirements do not match.' }
        $taskAssessment['runner_exe'] = [IO.Path]::GetFullPath($RunnerExe)
        $taskAssessment['runner_sha256'] = $taskActualSHA
        $taskAssessment['helper_sha256'] = (Get-FileHash -LiteralPath $Helper -Algorithm SHA256).Hash.ToLowerInvariant()
        return $taskAssessment
    } finally {
        [Console]::OutputEncoding = $taskPriorConsoleEncoding
        $OutputEncoding = $taskPriorPipeEncoding
    }
}

function Merge-SRCaseRecords {
    param([object[]]$Executions,[object[]]$Records)
    foreach ($taskExecution in $Executions) {
        $taskMatch = @($Records | Where-Object { $_.ContainsKey('case_id') -and $_.scenario -ceq $taskExecution.entry.id -and $_.case_id -ceq $taskExecution.case.id -and $_.repeat_index -eq $taskExecution.repeat_index })
        if ($taskMatch.Count) { $taskMatch; continue }
        @{level=$taskExecution.entry.level;scenario=$taskExecution.entry.id;case_id=$taskExecution.case.id;repeat_index=$taskExecution.repeat_index;status=$(if($taskExecution.excluded){'excluded'}elseif($taskExecution.unavailable){'unavailable'}else{'queued'});availability=$taskExecution.availability}
    }
}

function Set-SRLevelCoverage {
    param([Collections.IDictionary]$Summary,[object[]]$Registered,[object[]]$Records,[string[]]$Levels)
    $taskCoverage = @{}
    foreach ($taskLevel in @($Levels | Where-Object { $_ -in @('L2','L3','L4') })) {
        $taskExpected = @($Registered | Where-Object { $_.level -eq $taskLevel } | ForEach-Object { $taskEntry=$_; foreach ($taskCase in $taskEntry.cases) { $taskEntry.id+'/'+$taskCase.id } })
        $taskActual = @($Records | Where-Object { $_.level -eq $taskLevel -and $_.status -eq 'passed' } | ForEach-Object { $_.scenario+'/'+$_.case_id } | Select-Object -Unique)
        $taskMissing = @($taskExpected | Where-Object { $_ -notin $taskActual })
        $taskIncomplete = @($Records | Where-Object { $_.level -eq $taskLevel -and $_.status -ne 'passed' })
        $taskSets=@{}
        foreach($taskSet in @('requested','excluded','unavailable','executed')) {
            $taskSets[$taskSet]=@($Records | Where-Object { $_.level -eq $taskLevel -and ($taskSet -eq 'requested' -or ($taskSet -eq 'executed' -and $_.status -in @('preparing','running','passed','failed')) -or $_.status -eq $taskSet) } | ForEach-Object { $_.scenario+'/'+$_.case_id } | Select-Object -Unique)
        }
        $taskCoverage[$taskLevel] = @{complete_level_passed=($taskExpected.Count -gt 0 -and $taskMissing.Count -eq 0 -and $taskIncomplete.Count -eq 0);registered=$taskExpected;requested=$taskSets.requested;excluded=$taskSets.excluded;unavailable=$taskSets.unavailable;executed=$taskSets.executed;passed=$taskActual;missing=$taskMissing}
    }
    $Summary['levels']=$taskCoverage
}

function Get-SRBatchOutcome {
    param([object[]]$Records,[int]$SelectedScenarioCount,[int]$AuthorizedExclusions)
    if (@($Records | Where-Object { $_.ContainsKey('status') -and $_.status -eq 'failed' }).Count) { return 'failed' }
    $taskUnavailable=@($Records | Where-Object { $_.ContainsKey('status') -and $_.status -eq 'unavailable' }).Count
    $taskExecuted=@($Records | Where-Object { $_.ContainsKey('case_id') -and $_.status -in @('preparing','running','passed','failed') }).Count
    $taskPending=@($Records | Where-Object { $_.ContainsKey('status') -and $_.status -in @('queued','preparing','running') }).Count
    if ($taskUnavailable -or $taskPending -or ($SelectedScenarioCount -gt 0 -and $taskExecuted -eq 0)) { return 'incomplete' }
    if ($AuthorizedExclusions) { return 'passed_with_exclusions' }
    return 'passed'
}

# Case sets remain diagnostic; qualification also requires every selected repeat
# and a successful final batch gate. Never resurrect qualification in finally.
function Set-SRQualificationCoverage {
    param([Collections.IDictionary]$Summary,[object[]]$Executions,[object[]]$Records)
    foreach($level in $Summary.levels.Keys) {
        $complete=$Summary.status -in @('passed','passed_with_exclusions')
        foreach($execution in @($Executions | Where-Object {$_.entry.level -eq $level})) {
            $matches=@($Records | Where-Object {$_.level -eq $level -and $_.scenario -ceq $execution.entry.id -and $_.case_id -ceq $execution.case.id -and $_.repeat_index -eq $execution.repeat_index})
            if($matches.Count -ne 1 -or $matches[0].status -ne 'passed'){$complete=$false}
        }
        $Summary.levels[$level].complete_level_passed=$Summary.levels[$level].complete_level_passed -and $complete
    }
    if($Summary.Contains('baseline') -and $Summary.levels.ContainsKey('L2')){$Summary.baseline.l2_verified=$Summary.levels.L2.complete_level_passed}
}
function Test-SRCodexVersion {
    param([string]$Version)
    $taskMatch = [regex]::Match($Version.Trim(), '^codex-cli (0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$')
    if (-not $taskMatch.Success) { return $false }
    foreach ($taskIdentifier in $taskMatch.Groups[4].Value.Split('.')) {
        if ($taskIdentifier -cmatch '^0[0-9]+$') { return $false }
    }
    $taskMinimum = @('0', '159', '2')
    for ($taskIndex = 0; $taskIndex -lt 3; $taskIndex++) {
        $taskActual = $taskMatch.Groups[$taskIndex + 1].Value
        $taskExpected = $taskMinimum[$taskIndex]
        $taskOrder = [string]::CompareOrdinal($taskActual, $taskExpected)
        if ($taskActual.Length -gt $taskExpected.Length -or ($taskActual.Length -eq $taskExpected.Length -and $taskOrder -gt 0)) { return $true }
        if ($taskActual.Length -lt $taskExpected.Length -or $taskOrder -lt 0) { return $false }
    }
    return $taskMatch.Groups[4].Value -eq ''
}
