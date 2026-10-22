param([Parameter(Mandatory=$true)][string]$Repository,[Parameter(Mandatory=$true)][string]$Evidence)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $Repository 'scripts/live-catalog.ps1')
$taskRoot = Join-Path $Evidence 'catalog'
New-Item -ItemType Directory -Path $taskRoot | Out-Null
$taskCatalog = Get-SRTestCatalog (Join-Path $Repository 'tests/catalog.json')
$taskRegistry = @(Get-SRCaseRegistry)
if ((($taskCatalog.scenarios.id | Sort-Object) -join ',') -cne (($taskRegistry.id | Sort-Object) -join ',')) { throw 'Default catalog lost registered scenarios' }
$taskSelection = Get-Content -LiteralPath (Join-Path $Repository 'tests/catalog.json') -Raw | ConvertFrom-Json
if (($taskCatalog.scenarios.id -join ',') -cne ($taskSelection.scenarios -join ',')) { throw 'Catalog expansion reordered scenario selection' }
foreach ($taskEntry in $taskCatalog.scenarios) { $null = @(Get-SRLiveCases $taskEntry) }
# Expanded snapshots remain directly consumable as legacy custom catalogs.
$taskFrozen = Join-Path $taskRoot 'frozen-catalog.json'
$taskWire = $taskCatalog | ConvertTo-Json -Depth 60
[IO.File]::WriteAllText($taskFrozen, $taskWire, [Text.UTF8Encoding]::new($false))
$taskFrozenSHA = (Get-FileHash -LiteralPath $taskFrozen).Hash
$taskReloaded = Get-SRTestCatalog $taskFrozen | ConvertTo-Json -Depth 60
if ($taskReloaded -cne $taskWire -or (Get-FileHash -LiteralPath $taskFrozen).Hash -cne $taskFrozenSHA) { throw 'Frozen catalog changed during reload' }
# Independent v1 sample: an omitted default case, an explicit multi-case list,
# original ordering and custom metadata must survive without source rewriting.
$taskLegacy = '{"schema":"steerrunner-tests/v1","label":"legacy selection","scenarios":[{"id":"orca-local.codex","level":"L3","driver":"development-feedback","target":"orca-local","agent":"codex","timeout_minutes":30},{"id":"orca-remote.connection-reconnect","level":"L3","driver":"execution-stability","target":"orca-environment","timeout_minutes":30,"cases":["active-disconnect","pre-send-disconnect"]}]}'
$taskLegacyPath = Join-Path $taskRoot 'legacy-catalog.json'
[IO.File]::WriteAllText($taskLegacyPath, $taskLegacy, [Text.UTF8Encoding]::new($false))
$taskLegacySHA = (Get-FileHash -LiteralPath $taskLegacyPath).Hash
if ((Get-SRTestCatalog $taskLegacyPath | ConvertTo-Json -Depth 60) -cne ($taskLegacy | ConvertFrom-Json | ConvertTo-Json -Depth 60) -or (Get-FileHash -LiteralPath $taskLegacyPath).Hash -cne $taskLegacySHA) { throw 'Legacy catalog changed during reload' }
$taskRetiredConfig = Join-Path $taskRoot 'retired-auth.json'
[IO.File]::WriteAllText($taskRetiredConfig, '{"github_auth":null}')
$taskShell = (Get-Process -Id $PID).Path
foreach ($taskPath in @($taskLegacyPath, $taskFrozen)) {
    $taskLog = $taskPath + '.fixture.log'
    & $taskShell -NoProfile -File (Join-Path $Repository 'scripts/new-test-fixture.ps1') -Catalog $taskPath -Scenario 'orca-local.codex' -Config $taskRetiredConfig -FixtureParent (Join-Path $taskRoot 'unused-fixtures') *> $taskLog
    if ($LASTEXITCODE -ne 1 -or (Get-Content -LiteralPath $taskLog -Raw) -notmatch 'GitHub App authentication is unsupported' -or (Test-Path -LiteralPath (Join-Path $taskRoot 'unused-fixtures'))) { throw 'Original fixture entry point rejected a compatible catalog or reached fixture effects' }
}
$taskFailures = @(
    @{name='unknown';change={param($c) $c.scenarios = @('unknown.scenario')};error='Unregistered'},
    @{name='duplicate';change={param($c) $c.scenarios += $c.scenarios[0].id};error='Invalid or duplicate'},
    @{name='level';change={param($c) $c.scenarios[0].level = 'L4'};error='registered level'},
    @{name='driver';change={param($c) $c.scenarios[0].driver = 'wrong'};error='registered driver'},
    @{name='target';change={param($c) $c.scenarios[0].target = 'wrong'};error='registered target'},
    @{name='timeout';change={param($c) $c.scenarios[0].timeout_minutes++};error='registered timeout_minutes'},
    @{name='agent';change={param($c) ($c.scenarios | Where-Object {$_.PSObject.Properties['agent']})[0].agent = 'wrong'};error='registered agent'},
    @{name='missing-case';change={param($c) ($c.scenarios | Where-Object {$_.cases.Count -gt 1})[0].cases = @('missing')};error='missing, duplicate or unknown cases'},
    @{name='omitted-cases';change={param($c) ($c.scenarios | Where-Object {$_.cases.Count -gt 1})[0].PSObject.Properties.Remove('cases')};error='requires all registered cases'}
)
foreach ($taskFailure in $taskFailures) {
    $taskInvalid = $taskWire | ConvertFrom-Json
    & $taskFailure.change $taskInvalid
    $taskPath = Join-Path $taskRoot ($taskFailure.name + '.json')
    [IO.File]::WriteAllText($taskPath, ($taskInvalid | ConvertTo-Json -Depth 60), [Text.UTF8Encoding]::new($false))
    $taskRejected = $false
    try { $null = Get-SRTestCatalog $taskPath } catch { $taskRejected = $_.Exception.Message -match $taskFailure.error }
    if (-not $taskRejected) { throw "Catalog guard did not reject $($taskFailure.name) for the expected reason" }
}
[IO.File]::WriteAllText((Join-Path $taskRoot 'verified.json'), (@{scenarios=$taskCatalog.scenarios.Count;frozen_roundtrip=$true;legacy_default_case=$true;rejected=@($taskFailures.name)} | ConvertTo-Json), [Text.UTF8Encoding]::new($false))
Write-Output 'Catalog expansion, frozen reload, legacy compatibility and conflict guards passed'
