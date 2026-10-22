$ErrorActionPreference = 'Stop'
function Get-SRCaseRegistry {
    $taskRegistryPath = Join-Path $PSScriptRoot '../internal/testkit/livetest/cases.json'
    return (Get-Content -LiteralPath $taskRegistryPath -Raw | ConvertFrom-Json).scenarios
}
function Get-SRLiveCases($Entry) {
    $taskRegistered = @(Get-SRCaseRegistry | Where-Object { $_.id -ceq $Entry.id })
    if ($taskRegistered.Count -ne 1) { throw "Unregistered live scenario: $($Entry.id)" }
    $taskDefinition = $taskRegistered[0]
    foreach ($taskField in @('level','driver','target','timeout_minutes')) {
        if ($Entry.$taskField -cne $taskDefinition.$taskField) { throw "Catalog conflicts with registered $taskField for $($Entry.id)" }
    }
    if ($taskDefinition.PSObject.Properties['agent'] -and (-not $Entry.PSObject.Properties['agent'] -or $Entry.agent -cne $taskDefinition.agent)) { throw "Catalog conflicts with registered agent for $($Entry.id)" }
    $taskCaseIDs = @($taskDefinition.cases | ForEach-Object { $_.id })
    if ($Entry.PSObject.Properties['cases']) {
        if (($Entry.cases -join ',') -cne ($taskCaseIDs -join ',')) { throw "Catalog has missing, duplicate or unknown cases for $($Entry.id)" }
    } elseif ($taskCaseIDs.Count -ne 1 -or $taskCaseIDs[0] -cne 'default') { throw "Scenario requires all registered cases: $($Entry.id)" }
    return $taskDefinition.cases
}
function Get-SRTestCatalog([string]$Path) {
    $taskCatalog = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
    if ($taskCatalog.schema -ne 'steerrunner-tests/v1') { throw 'Unsupported catalog' }
    $taskRegistry = @(Get-SRCaseRegistry)
    $taskIDs = @{}
    $taskCatalog.scenarios = @(foreach ($taskEntry in $taskCatalog.scenarios) {
        $taskID = if ($taskEntry -is [string]) { $taskEntry } else { $taskEntry.id }
        if ($taskID -notmatch '^[a-z][a-z0-9.-]+$' -or $taskIDs.ContainsKey($taskID)) { throw 'Invalid or duplicate live catalog entry' }
        $taskIDs[$taskID] = $true
        if ($taskEntry -is [string]) {
            $taskDefinition = @($taskRegistry | Where-Object { $_.id -ceq $taskID })
            if ($taskDefinition.Count -ne 1) { throw "Unregistered live scenario: $taskID" }
            $taskDefinition = $taskDefinition[0]
            $taskExpanded = [ordered]@{}
            foreach ($taskField in @('id','level','driver','target','agent','timeout_minutes')) {
                if ($taskDefinition.PSObject.Properties[$taskField]) { $taskExpanded[$taskField] = $taskDefinition.$taskField }
            }
            $taskExpanded['cases'] = @($taskDefinition.cases | ForEach-Object { $_.id })
            [pscustomobject]$taskExpanded
        } else {
            # Existing custom and frozen catalogs retain their metadata and
            # must still match the registry before any execution is prepared.
            $null = @(Get-SRLiveCases $taskEntry)
            $taskEntry
        }
    })
    return $taskCatalog
}
function Test-SRAdminCase([string]$Scenario,[string]$Case) {
    $taskDefinition = @(Get-SRCaseRegistry | Where-Object { $_.id -ceq $Scenario })
    if ($taskDefinition.Count -ne 1) { throw 'Unknown scenario in requirements lookup' }
    $taskCase = @($taskDefinition[0].cases | Where-Object { $_.id -ceq $Case })
    if ($taskCase.Count -ne 1) { throw 'Unknown case in requirements lookup' }
    return $null -ne $taskCase[0].PSObject.Properties['requirements'] -and [bool]$taskCase[0].requirements.admin
}
