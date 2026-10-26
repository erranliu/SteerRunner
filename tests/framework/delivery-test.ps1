param([Parameter(Mandatory=$true)][string]$Repository,[Parameter(Mandatory=$true)][string]$Evidence)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2
. (Join-Path $Repository 'scripts/delivery-contract.ps1')
$taskEvidence = [IO.Path]::GetFullPath($Evidence)
if (Test-Path -LiteralPath $taskEvidence) { throw 'Delivery framework evidence must be new.' }
[IO.Directory]::CreateDirectory($taskEvidence) | Out-Null
$taskBase = Join-Path $taskEvidence 'package'
[IO.Directory]::CreateDirectory($taskBase) | Out-Null
function Write-JSON([string]$Path,$Value) { [IO.File]::WriteAllText($Path,($Value | ConvertTo-Json -Depth 20),[Text.UTF8Encoding]::new($false)) }
$taskPaths = @('steerrunner.exe','steerrunner-viewer.exe','client/steerrunner-client.ps1','skills/steerrunner-configure/SKILL.md','skills/steerrunner-run/SKILL.md','scripts/install-client.ps1','scripts/delivery-contract.ps1','scripts/verify-client-package.ps1','LICENSE','NOTICE','THIRD_PARTY_NOTICES.txt')
foreach ($taskPath in $taskPaths) {
    $taskFile = Join-Path $taskBase $taskPath
    [IO.Directory]::CreateDirectory((Split-Path -Parent $taskFile)) | Out-Null
    [IO.File]::WriteAllText($taskFile,$taskPath)
}
$taskRunner = @{schema='steerrunner-build/v1';component='runner';platform='windows/amd64';source_commit=('a'*40);source_digest=('b'*64);source_dirty=$true;sha256=(Get-SRFileDigest (Join-Path $taskBase 'steerrunner.exe'))}
$taskViewer = @{schema='steerrunner-build/v1';component='viewer';platform='windows/amd64';source_commit=('c'*40);source_digest=('d'*64);source_dirty=$false;sha256=(Get-SRFileDigest (Join-Path $taskBase 'steerrunner-viewer.exe'))}
Write-JSON (Join-Path $taskBase 'steerrunner.exe.build.json') $taskRunner
Write-JSON (Join-Path $taskBase 'steerrunner-viewer.exe.build.json') $taskViewer
$taskManifest = @{version=2;snapshot_schema_version=1;provenance=@{schema='steerrunner-package-source/v1';source_commit=('e'*40);source_digest=('f'*64);source_dirty=$true};components=@{runner=@{build=$taskRunner};viewer=@{build=$taskViewer}};files=@(Get-SRPackageInventory $taskBase)}
Write-JSON (Join-Path $taskBase 'manifest.json') $taskManifest
$taskIdentity = Test-SRClientPackage $taskBase
$taskPassed = New-Object 'Collections.Generic.List[string]'
$taskPassed.Add('separate component commits retain explicit identities')
$taskLegacyText = & powershell.exe -NoProfile -File (Join-Path $Repository 'scripts/verify-client-package.ps1') -PackageDir $taskBase
if ($LASTEXITCODE -ne 0) { throw 'Windows PowerShell package verification failed.' }
$taskLegacy = $taskLegacyText | ConvertFrom-Json
if ($taskLegacy.package.manifest_sha256 -cne $taskIdentity.manifest_sha256 -or $taskLegacy.package.payload_sha256 -cne $taskIdentity.payload_sha256) { throw 'Package identity differs between PowerShell runtimes.' }
$taskPassed.Add('Windows PowerShell and host runtime use the same package identity')
function Reject-Package([string]$Name,[scriptblock]$Change) {
    $taskCopy = Join-Path $taskEvidence $Name
    Copy-Item -LiteralPath $taskBase -Destination $taskCopy -Recurse
    $taskValue = Get-Content -LiteralPath (Join-Path $taskCopy 'manifest.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    & $Change $taskCopy $taskValue
    $taskRejected = $false
    try { $null = Test-SRClientPackage $taskCopy } catch { $taskRejected=$true }
    if (-not $taskRejected) { throw "Package guard accepted: $Name" }
    $taskPassed.Add($Name)
}
Reject-Package 'changed-bytes' { param($dir,$manifest) [IO.File]::AppendAllText((Join-Path $dir 'client/steerrunner-client.ps1'),'changed') }
Reject-Package 'unlisted-file' { param($dir,$manifest) [IO.File]::WriteAllText((Join-Path $dir 'extra.ps1'),'extra') }
Reject-Package 'missing-file' { param($dir,$manifest) [IO.File]::Delete((Join-Path $dir 'LICENSE')) }
Reject-Package 'omitted-inventory-entry' { param($dir,$manifest) $manifest.files=@($manifest.files | Where-Object {$_.path -cne 'client/steerrunner-client.ps1'});Write-JSON (Join-Path $dir 'manifest.json') $manifest }
Reject-Package 'duplicate-path' { param($dir,$manifest) $manifest.files+=@{path='LICENSE';sha256=(Get-SRFileDigest (Join-Path $dir 'LICENSE'))};Write-JSON (Join-Path $dir 'manifest.json') $manifest }
Reject-Package 'case-alias' { param($dir,$manifest) $manifest.files+=@{path='license';sha256=(Get-SRFileDigest (Join-Path $dir 'LICENSE'))};Write-JSON (Join-Path $dir 'manifest.json') $manifest }
Reject-Package 'traversal-path' { param($dir,$manifest) $manifest.files[0].path='../outside';Write-JSON (Join-Path $dir 'manifest.json') $manifest }
Reject-Package 'stale-build-record' { param($dir,$manifest)
    $record=Get-Content -LiteralPath (Join-Path $dir 'steerrunner.exe.build.json') -Raw | ConvertFrom-Json
    $record.sha256='0'*64;Write-JSON (Join-Path $dir 'steerrunner.exe.build.json') $record
    $manifest.files=@(Get-SRPackageInventory $dir);Write-JSON (Join-Path $dir 'manifest.json') $manifest
}
Reject-Package 'inconsistent-component-source' { param($dir,$manifest) $manifest.components.viewer.build.source_commit='0'*40;Write-JSON (Join-Path $dir 'manifest.json') $manifest }
Reject-Package 'missing-provenance' { param($dir,$manifest) $manifest.PSObject.Properties.Remove('provenance');Write-JSON (Join-Path $dir 'manifest.json') $manifest }
Reject-Package 'legacy-package-version' { param($dir,$manifest) $manifest.version=1;Write-JSON (Join-Path $dir 'manifest.json') $manifest }
Reject-Package 'missing-required-file-even-with-valid-inventory' { param($dir,$manifest)
    [IO.File]::Delete((Join-Path $dir 'skills/steerrunner-run/SKILL.md'));$manifest.files=@(Get-SRPackageInventory $dir);Write-JSON (Join-Path $dir 'manifest.json') $manifest
}
$taskChecks = @()
foreach ($taskKind in @('installation','client','viewer','progress_transport')) {
    $taskFile = $taskKind + '.json'
    Write-JSON (Join-Path $taskEvidence $taskFile) @{fixture=$true;kind=$taskKind}
    $taskChecks += @{kind=$taskKind;status='passed';report=$taskFile;sha256=(Get-SRFileDigest (Join-Path $taskEvidence $taskFile))}
}
[IO.Directory]::CreateDirectory((Join-Path $taskEvidence 'progress')) | Out-Null
$taskChecks[3]['evidence'] = @(@('source-identity.json','integration.log','progress-public-evidence.json') | ForEach-Object {
    $file='progress/'+$_;Write-JSON (Join-Path $taskEvidence $file) @{fixture=$true}
    @{path=$file;sha256=(Get-SRFileDigest (Join-Path $taskEvidence $file))}
})
$taskQualification = @{schema='steerrunner-delivery-qualification/v1';scope='controlled-client-delivery';status='passed';manifest_sha256=$taskIdentity.manifest_sha256;payload_sha256=$taskIdentity.payload_sha256;checks=$taskChecks}
$taskReport = Join-Path $taskEvidence 'qualification-fixture.json'
Write-JSON $taskReport $taskQualification
$null = Test-SRDeliveryQualification $taskBase $taskReport
$taskPassed.Add('matching package and all evidence hashes accepted')
foreach ($taskCase in @('failed','incomplete','different-package','different-scope','changed-evidence','outside-evidence','changed-progress-log','missing-progress-snapshots')) {
    $taskValue = Get-Content -LiteralPath $taskReport -Raw -Encoding UTF8 | ConvertFrom-Json
    switch ($taskCase) {
        'failed' { $taskValue.status='failed' }
        'incomplete' { $taskValue.checks=@($taskValue.checks | Where-Object {$_.kind -ne 'viewer'}) }
        'different-package' { $taskValue.manifest_sha256='0'*64 }
        'different-scope' { $taskValue.scope='live-qualified' }
        'changed-evidence' { $taskValue.checks[0].sha256='0'*64 }
        'outside-evidence' { $taskValue.checks[0].report='../outside.json' }
        'changed-progress-log' { $taskValue.checks[3].evidence[1].sha256='0'*64 }
        'missing-progress-snapshots' { $taskValue.checks[3].evidence=@($taskValue.checks[3].evidence | Where-Object {$_.path -ne 'progress/progress-public-evidence.json'}) }
    }
    $taskFile = Join-Path $taskEvidence ($taskCase+'.json')
    Write-JSON $taskFile $taskValue
    $taskRejected=$false
    try { $null=Test-SRDeliveryQualification $taskBase $taskFile } catch { $taskRejected=$true }
    if (-not $taskRejected) { throw "Qualification guard accepted: $taskCase" }
    $taskPassed.Add($taskCase)
}
Write-JSON (Join-Path $taskEvidence 'result.json') @{exit_code=0;test_count=$taskPassed.Count;passed=@($taskPassed);controlled=$true}
Write-Output ($taskPassed -join "`n")
