param([Parameter(Mandatory)][string]$Report)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$reportPath = (Resolve-Path -LiteralPath $Report).Path
$summary = Get-Content (Join-Path $reportPath 'summary.json') -Raw | ConvertFrom-Json
$required = @('local-codex.progress-file','orca-local.progress-file','orca-remote.progress-file','local-codex.progress-multirun','local-codex.progress-recovery')
$cases = @($summary.results | Where-Object { $_.PSObject.Properties['scenario'] -and $_.scenario -in $required })
$owners = @{}
$conflicts = @()
$windows = @()
$identities = @()
$issues = @()
if ($summary.status -ne 'passed' -or $summary.max_parallel -lt 2) { $issues += 'Official batch did not pass with parallel scheduling' }
foreach ($id in $required) {
    $selected = @($cases | Where-Object { $_.scenario -eq $id })
    if ($selected.Count -ne 1 -or $selected[0].status -ne 'passed') { $issues += "Required case did not pass exactly once: $id"; continue }
    $case = $selected[0]
    $identities += $case.identity
    foreach ($resource in $case.resources) {
        $key = $resource.ToLowerInvariant()
        if ($owners.ContainsKey($key) -and $owners[$key] -ne $id) { $conflicts += @{resource=$resource;first=$owners[$key];second=$id} }
        $owners[$key] = $id
    }
    $record = @(Get-ChildItem -LiteralPath $case.evidence -Filter 'progress-*-observation-window.json')
    if ($record.Count -ne 1) { $issues += "Missing independent native observation window: $id"; continue }
    $window = Get-Content -LiteralPath $record[0].FullName -Raw | ConvertFrom-Json
    $missingWindowFields = @(@('contract','collection_healthy_at_end','collection_gap_budget_seconds','collection_gap_samples') | Where-Object { -not $window.PSObject.Properties[$_] })
    if ($missingWindowFields.Count -or $window.contract -ne 'steerrunner-progress-observation-window/v2' -or -not $window.collection_healthy_at_end -or $window.collection_gap_budget_seconds -ne 20 -or $window.collection_gap_samples -lt 0) { $issues += "Native observation did not prove bounded collection recovery: $id"; continue }
    $start = [datetimeoffset]$window.start
    $end = [datetimeoffset]$window.end
    if (($end-$start).TotalSeconds -lt 120 -or $window.run_ids.Count -lt 1) { $issues += "Invalid observation window: $id" }
    $windows += @{scenario=$id;start=$start;end=$end;run_ids=@($window.run_ids);evidence=$record[0].FullName;collection_gap_samples=$window.collection_gap_samples;collection_gap_budget_seconds=$window.collection_gap_budget_seconds;collection_healthy_at_end=$window.collection_healthy_at_end}
}
foreach ($field in @('runner_sha256','driver_sha256','source_commit')) {
    $values = @($identities | ForEach-Object { $_.$field } | Select-Object -Unique)
    if ($values.Count -ne 1 -or -not $values[0]) { $issues += "Mixed or missing build identity: $field" }
}
$overlaps = @()
$covered = @{}
for ($i=0; $i -lt $windows.Count; $i++) {
    for ($j=$i+1; $j -lt $windows.Count; $j++) {
        $a=$windows[$i]; $b=$windows[$j]
        $start=if($a.start -gt $b.start){$a.start}else{$b.start}
        $end=if($a.end -lt $b.end){$a.end}else{$b.end}
        if ($end -gt $start) {
            $overlaps += @{first=$a.scenario;second=$b.scenario;from=$start.ToUniversalTime().ToString('o');until=$end.ToUniversalTime().ToString('o');seconds=($end-$start).TotalSeconds}
            $covered[$a.scenario]=$true; $covered[$b.scenario]=$true
        }
    }
}
foreach ($id in $required) { if (-not $covered.ContainsKey($id)) { $issues += "No actual native observation overlap with another passing case: $id" } }
if ($conflicts.Count) { $issues += 'Native fixture resources collided' }
$result = [ordered]@{schema='steerrunner-progress-parallel-evidence/v3';report=$reportPath;max_parallel=$summary.max_parallel;cases=$cases.Count;runner_sha256=@($identities | ForEach-Object { $_.runner_sha256 } | Select-Object -Unique);driver_sha256=@($identities | ForEach-Object { $_.driver_sha256 } | Select-Object -Unique);resource_conflicts=$conflicts;windows=$windows;native_observation_overlaps=$overlaps;issues=$issues;passed=($issues.Count -eq 0)}
$result | ConvertTo-Json -Depth 15 | Set-Content (Join-Path $reportPath 'progress-parallel-verification.json') -Encoding utf8
$result | ConvertTo-Json -Depth 15
if (-not $result.passed) { throw 'Current build real parallel qualification failed; inspect progress-parallel-verification.json' }
