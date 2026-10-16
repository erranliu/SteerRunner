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
    $taskCaseIDs = @($taskDefinition.cases | ForEach-Object { $_.id })
    if ($Entry.PSObject.Properties['cases']) {
        if (($Entry.cases -join ',') -cne ($taskCaseIDs -join ',')) { throw "Catalog has missing, duplicate or unknown cases for $($Entry.id)" }
    } elseif ($taskCaseIDs.Count -ne 1 -or $taskCaseIDs[0] -cne 'default') { throw "Scenario requires all registered cases: $($Entry.id)" }
    return $taskDefinition.cases
}
function Test-SRAdminCase([string]$Scenario,[string]$Case) {
    return $Scenario -in @('orca-remote.connection-reconnect','orca-remote.publication-unknown') -or $Case -in @('graceful-boundary','kill-boundary','boundary','unreachable')
}
