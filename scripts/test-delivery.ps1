param([Parameter(Mandatory=$true)][string]$PackageDir,[Parameter(Mandatory=$true)][string]$OutDir)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2
. (Join-Path $PSScriptRoot 'delivery-contract.ps1')
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$taskPackage = (Resolve-Path -LiteralPath $PackageDir).Path.TrimEnd('\','/')
$taskOut = [IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'Delivery evidence directory must be new.' }
if ($taskOut.StartsWith($taskPackage + [IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)) { throw 'Delivery evidence must be outside the immutable package.' }
[IO.Directory]::CreateDirectory($taskOut) | Out-Null
$taskShell = (Get-Process -Id $PID).Path
$taskReport = [ordered]@{schema='steerrunner-delivery-qualification/v1';scope='controlled-client-delivery';status='running';started_at=[DateTime]::UtcNow.ToString('o');checks=@()}
$taskExit = 1
function Save-DeliveryReport {
    $taskTemporary = Join-Path $taskOut 'qualification.json.tmp'
    [IO.File]::WriteAllText($taskTemporary,($taskReport | ConvertTo-Json -Depth 12),[Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $taskTemporary -Destination (Join-Path $taskOut 'qualification.json') -Force
}
function Add-DeliveryCheck([string]$Kind,[string]$Report,[string[]]$EvidenceFiles=@()) {
    $taskEvidence = @($EvidenceFiles | ForEach-Object { @{path=$_;sha256=(Get-SRFileDigest (Join-Path $taskOut $_))} })
    $taskReport.checks += @{kind=$Kind;status='passed';report=$Report;sha256=(Get-SRFileDigest (Join-Path $taskOut $Report));evidence=$taskEvidence}
    Save-DeliveryReport
}
Save-DeliveryReport
try {
    $taskIdentity = Test-SRClientPackage $taskPackage
    $taskReport['manifest_sha256'] = $taskIdentity.manifest_sha256
    $taskReport['payload_sha256'] = $taskIdentity.payload_sha256
    $taskSourcePaths = @('scripts','tests','internal','go.mod','go.sum')
    $taskHarness = Get-SRSourceIdentity $taskRepo $taskSourcePaths
    $taskReport['harness_source'] = $taskHarness
    Save-DeliveryReport
    Write-Host 'Testing isolated package installation'
    & $taskShell -NoProfile -File (Join-Path $PSScriptRoot 'test-client-package.ps1') -PackageDir $taskPackage -OutDir (Join-Path $taskOut 'installation') *> (Join-Path $taskOut 'installation.log')
    if ($LASTEXITCODE -ne 0) { throw 'Package installation checks failed; inspect installation.log.' }
    $taskInstalled = Get-Content -LiteralPath (Join-Path $taskOut 'installation/result.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    if (-not $taskInstalled.installed_configure -or @($taskInstalled.passed).Count -eq 0 -or $taskInstalled.manifest_sha256.ToLowerInvariant() -cne $taskIdentity.manifest_sha256) { throw 'Installation evidence is incomplete.' }
    Add-DeliveryCheck 'installation' 'installation/result.json'

    Write-Host 'Testing the packaged client and Viewer'
    $taskRunner = Join-Path $taskPackage 'steerrunner.exe'
    $taskViewer = Join-Path $taskPackage 'steerrunner-viewer.exe'
    $taskClientHelper = Join-Path $taskPackage 'client/steerrunner-client.ps1'
    & $taskShell -NoProfile -File (Join-Path $PSScriptRoot 'test-client.ps1') -RunnerExe $taskRunner -ViewerExe $taskViewer -ClientHelper $taskClientHelper -OutDir (Join-Path $taskOut 'client') *> (Join-Path $taskOut 'client.log')
    if ($LASTEXITCODE -ne 0) { throw 'Client or Viewer checks failed; inspect client.log.' }
    $taskClient = Get-Content -LiteralPath (Join-Path $taskOut 'client/result.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    if (@($taskClient.passed).Count -eq 0 -or $taskClient.runner_sha256.ToLowerInvariant() -cne (Get-SRFileDigest $taskRunner) -or
        $taskClient.client_helper_sha256.ToLowerInvariant() -cne (Get-SRFileDigest $taskClientHelper)) { throw 'Client evidence does not identify the packaged Runner and helper.' }
    Add-DeliveryCheck 'client' 'client/result.json'
    $taskViewerResult = Get-Content -LiteralPath (Join-Path $taskOut 'client/viewer/result.txt') -Raw -Encoding UTF8 | ConvertFrom-Json
    if (@($taskViewerResult.passed).Count -eq 0 -or $taskViewerResult.viewer_sha256.ToLowerInvariant() -cne (Get-SRFileDigest $taskViewer)) { throw 'Viewer evidence does not identify the packaged Viewer.' }
    Add-DeliveryCheck 'viewer' 'client/viewer/result.txt'

    Write-Host 'Testing progress bytes, restart, isolation and finalization with the packaged Runner'
    $taskProgress = Join-Path $taskOut 'progress'
    & $taskShell -NoProfile -File (Join-Path $PSScriptRoot 'test-foundation.ps1') -RunnerExe $taskRunner -OutDir $taskProgress -Pattern '^TestSRProgressFile$' -TimeoutMinutes 5 *> (Join-Path $taskOut 'progress-launch.log')
    if ($LASTEXITCODE -ne 0) { throw 'Progress harness preparation failed; inspect progress-launch.log.' }
    $taskUntil = [DateTime]::UtcNow.AddMinutes(6)
    while (-not (Test-Path -LiteralPath (Join-Path $taskProgress 'result.json'))) {
        if ([DateTime]::UtcNow -gt $taskUntil) { throw 'Progress harness timed out; inspect progress/integration.log and its owned process.' }
        Start-Sleep -Milliseconds 500
    }
    $taskResult = Get-Content -LiteralPath (Join-Path $taskProgress 'result.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    $taskNative = Get-Content -LiteralPath (Join-Path $taskProgress 'source-identity.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($taskResult.exit_code -ne 0 -or $taskResult.scenario_count -ne 1 -or $taskResult.exe -ine $taskRunner -or
        $taskNative.runner_sha256.ToLowerInvariant() -cne (Get-SRFileDigest $taskRunner) -or
        (Select-String -LiteralPath (Join-Path $taskProgress 'integration.log') -Pattern '--- SKIP:' -Quiet)) { throw 'Progress transport qualification failed, skipped or tested a different Runner.' }
    Add-DeliveryCheck 'progress_transport' 'progress/result.json' @('progress/source-identity.json','progress/integration.log','progress/progress-public-evidence.json')

    $taskAfter = Test-SRClientPackage $taskPackage
    $taskHarnessAfter = Get-SRSourceIdentity $taskRepo $taskSourcePaths
    if ($taskAfter.manifest_sha256 -cne $taskIdentity.manifest_sha256 -or $taskAfter.payload_sha256 -cne $taskIdentity.payload_sha256) { throw 'Package changed during qualification.' }
    if ($taskHarnessAfter.source_commit -cne $taskHarness.source_commit -or $taskHarnessAfter.source_digest -cne $taskHarness.source_digest) { throw 'Test harness source changed during qualification.' }
    $taskLegacyText = & powershell.exe -NoProfile -File (Join-Path $PSScriptRoot 'verify-client-package.ps1') -PackageDir $taskPackage 2> (Join-Path $taskOut 'windows-powershell-verification-stderr.log')
    if ($LASTEXITCODE -ne 0) { throw 'Windows PowerShell package verification failed.' }
    $taskLegacy = $taskLegacyText | ConvertFrom-Json
    if ($taskLegacy.package.manifest_sha256 -cne $taskIdentity.manifest_sha256 -or $taskLegacy.package.payload_sha256 -cne $taskIdentity.payload_sha256) { throw 'Package identity differs between PowerShell runtimes.' }
    $taskReport.status = 'passed'
    $taskExit = 0
} catch {
    $taskReport.status = 'failed'
    $taskReport['error'] = $_.Exception.Message
    Write-Host $_.Exception.Message -ForegroundColor Red
} finally {
    $taskReport['finished_at'] = [DateTime]::UtcNow.ToString('o')
    Save-DeliveryReport
}
Write-Output (Join-Path $taskOut 'qualification.json')
exit $taskExit
