# Shared by source builds, packaging, installation and delivery qualification.
# Keep this file compatible with Windows PowerShell 5.1.
function Get-SRFileDigest([string]$Path) {
    (Get-FileHash -LiteralPath $Path -Algorithm SHA256 -ErrorAction Stop).Hash.ToLowerInvariant()
}

function Get-SRTextDigest([string]$Text) {
    $srHasher = [Security.Cryptography.SHA256]::Create()
    try { ([BitConverter]::ToString($srHasher.ComputeHash([Text.Encoding]::UTF8.GetBytes($Text)))).Replace('-','').ToLowerInvariant() }
    finally { $srHasher.Dispose() }
}

function Get-SRSourceIdentity([string]$Root, [string[]]$Paths) {
    $srCommit = (& git -C $Root rev-parse HEAD | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $srCommit -notmatch '^[0-9a-f]{40}$') { throw 'Cannot identify source commit.' }
    $srFiles = @(& git -c core.quotepath=false -C $Root ls-files --cached --others --exclude-standard -- @Paths)
    if ($LASTEXITCODE -ne 0 -or $srFiles.Count -eq 0) { throw 'Cannot enumerate build source.' }
    [string[]]$srNames = @($srFiles)
    [Array]::Sort($srNames,[StringComparer]::Ordinal)
    $srLines = @($srNames | ForEach-Object {
        $srFile = Join-Path $Root $_
        $srHash = if ([IO.File]::Exists($srFile)) { Get-SRFileDigest $srFile } else { 'missing' }
        $_.Replace('\','/') + ':' + $srHash
    })
    $srStatus = & git -C $Root status --porcelain
    if ($LASTEXITCODE -ne 0) { throw 'Cannot read source status.' }
    @{source_commit=$srCommit;source_dirty=(-not [string]::IsNullOrWhiteSpace(($srStatus | Out-String)));source_digest=(Get-SRTextDigest ($srLines -join "`n"))}
}

function Get-SRBuildRecord([string]$Executable, [string]$Component) {
    $srRecord = Get-Content -LiteralPath ($Executable + '.build.json') -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($srRecord.schema -cne 'steerrunner-build/v1' -or $srRecord.component -cne $Component -or
        $srRecord.platform -cne 'windows/amd64' -or $srRecord.source_commit -cnotmatch '^[0-9a-f]{40}$' -or
        $srRecord.source_digest -cnotmatch '^[0-9a-f]{64}$' -or $srRecord.source_dirty -isnot [bool] -or
        $srRecord.sha256 -cne (Get-SRFileDigest $Executable)) { throw "Invalid or stale $Component build record; rebuild the component." }
    $srRecord
}

function Get-SRPackageInventory([string]$Root) {
    # Reject links before descending: package identity must describe owned bytes.
    $srPending = New-Object 'Collections.Generic.Queue[string]'
    $srPending.Enqueue($Root)
    $srEntries = [Collections.Generic.SortedDictionary[string,object]]::new([StringComparer]::Ordinal)
    while ($srPending.Count) {
        foreach ($srItem in @(Get-ChildItem -LiteralPath $srPending.Dequeue() -Force)) {
            if ($srItem.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Package links are unsupported.' }
            if ($srItem.PSIsContainer) { $srPending.Enqueue($srItem.FullName); continue }
            $srRelative = $srItem.FullName.Substring($Root.Length + 1).Replace('\','/')
            if ($srRelative -ceq 'manifest.json') { continue }
            $srEntries.Add($srRelative,@{path=$srRelative;sha256=(Get-SRFileDigest $srItem.FullName)})
        }
    }
    @($srEntries.Values)
}

function Test-SRClientPackage([string]$PackageDir) {
    $srRoot = [IO.Path]::GetFullPath($PackageDir).TrimEnd('\','/')
    if ((Get-Item -LiteralPath $srRoot).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Package links are unsupported.' }
    $srActual = @(Get-SRPackageInventory $srRoot)
    $srManifestPath = Join-Path $srRoot 'manifest.json'
    if ((Get-Item -LiteralPath $srManifestPath).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Package links are unsupported.' }
    $srManifest = Get-Content -LiteralPath $srManifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($srManifest.version -ne 2 -or $srManifest.snapshot_schema_version -ne 1 -or
        $srManifest.provenance.schema -cne 'steerrunner-package-source/v1') { throw 'Unsupported package manifest or missing provenance.' }
    $srListed = @{}
    foreach ($srEntry in @($srManifest.files)) {
        $srPath = [string]$srEntry.path
        if ($srPath -notmatch '^[^\\:]+$' -or $srPath.StartsWith('/') -or
            @($srPath.Split('/') | Where-Object { $_ -in @('','.', '..') }).Count -or
            $srPath -ieq 'manifest.json' -or $srListed.ContainsKey($srPath) -or
            $srEntry.sha256 -cnotmatch '^[0-9a-f]{64}$') { throw 'Invalid or duplicate package manifest path/digest.' }
        $srListed[$srPath] = $srEntry.sha256
    }
    if ($srListed.Count -ne $srActual.Count) { throw 'Package inventory mismatch.' }
    foreach ($srEntry in $srActual) {
        if (-not $srListed.ContainsKey($srEntry.path) -or $srListed[$srEntry.path] -cne $srEntry.sha256) { throw 'Package content digest mismatch.' }
    }
    foreach ($srRequired in @('steerrunner.exe','steerrunner.exe.build.json','steerrunner-viewer.exe','steerrunner-viewer.exe.build.json',
        'client/steerrunner-client.ps1','skills/steerrunner-configure/SKILL.md','skills/steerrunner-run/SKILL.md',
        'scripts/install-client.ps1','scripts/delivery-contract.ps1','scripts/verify-client-package.ps1','LICENSE','NOTICE','THIRD_PARTY_NOTICES.txt')) {
        if (-not $srListed.ContainsKey($srRequired)) { throw "Incomplete package: $srRequired" }
    }
    $srRunner = Get-SRBuildRecord (Join-Path $srRoot 'steerrunner.exe') 'runner'
    $srViewer = Get-SRBuildRecord (Join-Path $srRoot 'steerrunner-viewer.exe') 'viewer'
    foreach ($srPair in @(@($srRunner,$srManifest.components.runner.build),@($srViewer,$srManifest.components.viewer.build))) {
        foreach ($srField in @('source_commit','source_dirty','source_digest','sha256','platform','schema','component')) {
            if ($srPair[0].$srField -cne $srPair[1].$srField) { throw 'Component provenance mismatch.' }
        }
    }
    $srSource = $srManifest.provenance
    if ($srSource.source_commit -cnotmatch '^[0-9a-f]{40}$' -or $srSource.source_digest -cnotmatch '^[0-9a-f]{64}$' -or $srSource.source_dirty -isnot [bool]) { throw 'Invalid package source identity.' }
    @{manifest_sha256=(Get-SRFileDigest $srManifestPath);payload_sha256=(Get-SRTextDigest (($srActual | ForEach-Object { $_.path + ':' + $_.sha256 }) -join "`n"));file_count=$srActual.Count}
}

function Test-SRDeliveryQualification([string]$PackageDir, [string]$ReportPath) {
    $srIdentity = Test-SRClientPackage $PackageDir
    $srReportPath = [IO.Path]::GetFullPath($ReportPath)
    $srReport = Get-Content -LiteralPath $srReportPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($srReport.schema -cne 'steerrunner-delivery-qualification/v1' -or $srReport.status -cne 'passed' -or
        $srReport.scope -cne 'controlled-client-delivery' -or $srReport.manifest_sha256 -cne $srIdentity.manifest_sha256 -or
        $srReport.payload_sha256 -cne $srIdentity.payload_sha256) { throw 'Delivery qualification does not match this package.' }
    $srChecks = @($srReport.checks)
    $srExpected = @('installation','client','viewer','progress_transport')
    if ($srChecks.Count -ne $srExpected.Count) { throw 'Incomplete delivery qualification.' }
    foreach ($srKind in $srExpected) {
        $srMatches = @($srChecks | Where-Object { $_.kind -ceq $srKind })
        if ($srMatches.Count -ne 1 -or $srMatches[0].status -cne 'passed') { throw "Missing delivery check: $srKind" }
        $srCheck = $srMatches[0]
        $srEvidence = [IO.Path]::GetFullPath((Join-Path (Split-Path -Parent $srReportPath) $srCheck.report))
        if (-not $srEvidence.StartsWith((Split-Path -Parent $srReportPath) + [IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase) -or
            $srCheck.sha256 -cne (Get-SRFileDigest $srEvidence)) { throw 'Delivery evidence changed or escaped its report directory.' }
        if ($srKind -ceq 'progress_transport') {
            $srEvidenceFiles = @($srCheck.evidence)
            foreach ($srRequired in @('progress/source-identity.json','progress/integration.log','progress/progress-public-evidence.json')) {
                $srItems = @($srEvidenceFiles | Where-Object { $_.path -ceq $srRequired })
                if ($srItems.Count -ne 1 -or $srItems[0].sha256 -cne (Get-SRFileDigest (Join-Path (Split-Path -Parent $srReportPath) $srRequired))) { throw 'Progress identity, log or raw snapshot evidence changed or is missing.' }
            }
        }
    }
    $srIdentity
}
