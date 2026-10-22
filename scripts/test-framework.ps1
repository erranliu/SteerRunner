param([Parameter(Mandatory=$true)][string]$Evidence)
$ErrorActionPreference = 'Stop'
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$taskEvidence = [IO.Path]::GetFullPath($Evidence)
New-Item -ItemType Directory -Path $taskEvidence -ErrorAction Stop | Out-Null
$taskShell = (Get-Process -Id $PID).Path
. (Join-Path $PSScriptRoot 'live-catalog.ps1')
$taskCatalog = Get-SRTestCatalog (Join-Path $taskRepo 'tests/catalog.json')
$taskCatalog.scenarios[1].id = $taskCatalog.scenarios[0].id
$taskBadCatalog = Join-Path $taskEvidence 'duplicate-catalog.json'
[IO.File]::WriteAllText($taskBadCatalog,($taskCatalog|ConvertTo-Json -Depth 20),[Text.UTF8Encoding]::new($false))
$taskMissingCatalog = Join-Path $taskEvidence 'missing-level-catalog.json'
$taskMissing = Get-SRTestCatalog (Join-Path $taskRepo 'tests/catalog.json')
$taskMissing.scenarios = @($taskMissing.scenarios | Where-Object { $_.id -cne 'orca-local.codex' })
[IO.File]::WriteAllText($taskMissingCatalog,($taskMissing|ConvertTo-Json -Depth 20),[Text.UTF8Encoding]::new($false))
$taskCases = @(
    @{name='zero-match';args=@('-Level','L3','-Scenario','missing.scenario');error='No scenario matched|not selected'},
    @{name='duplicate-catalog';args=@('-Level','L3','-Catalog',$taskBadCatalog);error='Invalid or duplicate'},
    @{name='missing-level-catalog';args=@('-Level','L3','-Catalog',$taskMissingCatalog);error='Complete level catalog is missing registered scenario'},
    @{name='repeat-zero';args=@('-Level','L3','-Repeat','0');error='Repeat must be'},
    @{name='repeat-non-live';args=@('-Level','L1','-Repeat','3');error='Repeat requires a live level'},
    @{name='parallel-zero';args=@('-Level','L3','-MaxParallel','0');error='MaxParallel must be'},
    @{name='parallel-excess';args=@('-Level','L4','-MaxParallel','9');error='MaxParallel must be'}
)
$taskResults = @()
& (Join-Path $taskRepo 'tests/framework/catalog-test.ps1') -Repository $taskRepo -Evidence $taskEvidence
$taskResults += @{name='catalog-expansion-compatibility';passed=$true}
& (Join-Path $taskRepo 'tests/framework/configuration-test.ps1') -Repository $taskRepo -Evidence $taskEvidence -CatalogPaths @((Join-Path $taskEvidence 'catalog/legacy-catalog.json'),(Join-Path $taskEvidence 'catalog/frozen-catalog.json'))
$taskResults += @{name='independent-configuration';passed=$true}
& (Join-Path $taskRepo 'tests/framework/runtime-cache-test.ps1') -Repository $taskRepo -Evidence $taskEvidence
$taskResults += @{name='parallel-runtime-cache';passed=$true}
foreach ($taskCase in $taskCases) {
    $taskOut = Join-Path $taskEvidence $taskCase.name
    $taskArgs = @('-NoProfile','-File',(Join-Path $PSScriptRoot 'test.ps1'),'-OutDir',$taskOut) + $taskCase.args
    & $taskShell @taskArgs 2>&1 | Out-File -LiteralPath (Join-Path $taskEvidence ($taskCase.name + '.log')) -Encoding utf8
    if ($LASTEXITCODE -ne 2) { throw "Framework guard did not reject invalid arguments: $($taskCase.name)" }
    $taskSummary = Get-Content -LiteralPath (Join-Path $taskOut 'summary.json') -Raw | ConvertFrom-Json
    if ($taskSummary.status -ne 'failed' -or @($taskSummary.results).Count -ne 0 -or $taskSummary.error -notmatch $taskCase.error) { throw "Guard performed work or failed for the wrong reason: $($taskCase.name)" }
    $taskResults += @{name=$taskCase.name;passed=$true}
}
# Retired auth fields must be rejected before native/Orca/Git fixture effects.
foreach ($taskRetiredField in @('github_auth','remote_github_auth')) {
    $taskRetiredConfig = Join-Path $taskEvidence ($taskRetiredField + '.json')
    $taskRetiredInput = @{schema='steerrunner-live-config/v1'}
    $taskRetiredInput[$taskRetiredField] = $null
    [IO.File]::WriteAllText($taskRetiredConfig,($taskRetiredInput|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
    $taskRetiredRoot = Join-Path $taskEvidence ($taskRetiredField + '-fixtures')
    $taskRetiredLog = Join-Path $taskEvidence ($taskRetiredField + '.log')
    & $taskShell -NoProfile -File (Join-Path $PSScriptRoot 'new-test-fixture.ps1') -Scenario 'orca-remote.smoke' -Config $taskRetiredConfig -FixtureParent $taskRetiredRoot *> $taskRetiredLog
    if ($LASTEXITCODE -ne 1 -or (Test-Path -LiteralPath $taskRetiredRoot) -or -not (Select-String -LiteralPath $taskRetiredLog -Pattern 'GitHub App authentication is unsupported' -Quiet)) { throw 'Legacy App input was ignored or reached fixture effects' }
    $taskResults += @{name=('retired-auth-' + $taskRetiredField);passed=$true}
}
$taskCheckRoot = Join-Path $taskEvidence 'checker'
New-Item -ItemType Directory -Path (Join-Path $taskCheckRoot 'evidence') | Out-Null
$taskFile = Join-Path $taskCheckRoot 'evidence/native-101.md'
$taskBody = "# Evidence sample 101`n- sample_id: 101`n- generated_at: " + [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ') + "`n- " + [char]0x6a21 + [char]0x62df + [char]0x4efb + [char]0x52a1 + "`n- " + [char]0x672a + [char]0x4fee + [char]0x6539 + [char]0x4e1a + [char]0x52a1 + [char]0x4ee3 + [char]0x7801 + "`n"
[IO.File]::WriteAllText($taskFile,$taskBody,[Text.UTF8Encoding]::new($false))
$taskCheckScript = Join-Path $taskCheckRoot 'check.ps1'
[IO.File]::WriteAllText($taskCheckScript,[IO.File]::ReadAllText((Join-Path $taskRepo 'tests/fixtures/evidence-smoke/check.ps1')),[Text.UTF8Encoding]::new($true))
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $taskCheckScript -Root $taskCheckRoot 2>&1 | Out-File -LiteralPath (Join-Path $taskEvidence 'checker-positive.log') -Encoding utf8
if ($LASTEXITCODE -ne 0) { throw 'Windows PowerShell checker rejected valid evidence' }
[IO.File]::AppendAllText($taskFile,'extra')
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $taskCheckScript -Root $taskCheckRoot 2>&1 | Out-File -LiteralPath (Join-Path $taskEvidence 'checker-negative.log') -Encoding utf8
if ($LASTEXITCODE -eq 0) { throw 'Windows PowerShell checker accepted invalid evidence' }
$taskResults += @{name='checker-positive-negative';passed=$true}
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $taskRepo 'tests/framework/pr-list-test.ps1') -Source (Join-Path $taskRepo 'tests/fixtures/evidence-smoke/pr-list.ps1') 2>&1 | Out-File -LiteralPath (Join-Path $taskEvidence 'pr-list.log') -Encoding utf8
if ($LASTEXITCODE -ne 0) { throw 'Windows PowerShell PR list normalization failed' }
$taskResults += @{name='pr-list-zero-one-many';passed=$true}
foreach ($taskCase in @('empty','existing','conflict','query-failure')) {
    $taskMock = Join-Path $taskEvidence ('publisher-' + $taskCase)
    New-Item -ItemType Directory -Path (Join-Path $taskMock 'sr-smoke'),(Join-Path $taskMock 'evidence') | Out-Null
    [IO.File]::WriteAllText((Join-Path $taskMock 'evidence/native-101.md'),$taskBody,[Text.UTF8Encoding]::new($false))
    foreach ($taskName in @('check.ps1','publish.ps1','pr-list.ps1')) {
        [IO.File]::WriteAllText((Join-Path $taskMock ('sr-smoke/' + $taskName)),[IO.File]::ReadAllText((Join-Path $taskRepo ('tests/fixtures/evidence-smoke/' + $taskName))),[Text.UTF8Encoding]::new($true))
    }
    [IO.File]::WriteAllText((Join-Path $taskMock 'sr-smoke/publication.json'),'{"fixture_id":"012345abcdef","base_branch":"codex/srtest/012345abcdef/base"}')
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $taskRepo 'tests/framework/publish-test.ps1') -FixtureRoot $taskMock -Case $taskCase 2>&1 | Out-File -LiteralPath (Join-Path $taskMock 'output.log') -Encoding utf8
    $taskExpected = if ($taskCase -in @('empty','existing')) { 0 } else { 1 }
    if ($LASTEXITCODE -ne $taskExpected) { throw "Publisher case failed: $taskCase" }
    $taskCalls = @(Get-Content -LiteralPath (Join-Path $taskMock 'calls.log'))
    $taskCreates = @($taskCalls | Where-Object { $_ -like 'gh pr create *' }).Count
    if ($taskCreates -ne [int]($taskCase -eq 'empty')) { throw "Publisher created the wrong number of PRs: $taskCase" }
    $taskResults += @{name=('publisher-' + $taskCase);passed=$true}
}
& $taskShell -NoProfile -File (Join-Path $taskRepo 'tests/framework/stability-test.ps1') -Repository $taskRepo -Evidence $taskEvidence *> (Join-Path $taskEvidence 'stability-framework.log')
if ($LASTEXITCODE -ne 0) { throw 'Stability framework guards failed; inspect stability-framework.log' }
$taskResults += @{name='stability-registry-gate-scope';passed=$true}
& $taskShell -NoProfile -File (Join-Path $taskRepo 'tests/framework/scheduler-test.ps1') -Repository $taskRepo -Evidence $taskEvidence *> (Join-Path $taskEvidence 'scheduler-framework.log')
if ($LASTEXITCODE -ne 0) { throw 'Parallel scheduler checks failed; inspect scheduler-framework.log' }
$taskResults += @{name='parallel-scheduler-isolation';passed=$true}
& $taskShell -NoProfile -File (Join-Path $taskRepo 'tests/framework/native-result-test.ps1') -Repository $taskRepo -Evidence (Join-Path $taskEvidence 'native-result') *> (Join-Path $taskEvidence 'native-result-framework.log')
if ($LASTEXITCODE -ne 0) { throw 'Native result contract checks failed; inspect native-result-framework.log' }
$taskResults += @{name='native-result-contracts';passed=$true}
[IO.File]::WriteAllText((Join-Path $taskEvidence 'result.json'),(@{exit_code=0;test_count=$taskResults.Count;tests=$taskResults}|ConvertTo-Json -Depth 10),[Text.UTF8Encoding]::new($false))
Write-Host 'Framework guards and Windows PowerShell checker passed'
