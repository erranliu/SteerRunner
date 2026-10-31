# Test-only stage/provenance contract. Digest helpers remain parseable in Windows PowerShell 5.1.
. (Join-Path $PSScriptRoot 'delivery-contract.ps1')

function Write-SRPrerequisiteJSON([string]$Path, $Value) {
    $temporary = $Path + '.tmp'
    [IO.File]::WriteAllText($temporary, ($Value | ConvertTo-Json -Depth 80), [Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $temporary -Destination $Path -Force
}

function Get-SRPrerequisiteFiles([string]$Root, [switch]$Native) {
    $paths = @('cmd','internal','scripts','tests','go.mod','go.sum','.gitignore','.gitattributes','docs/workflows/generate-repository.example.json')
    $names = @(& git -c core.quotepath=false -C $Root ls-files --cached --others --exclude-standard -- @paths)
    if ($LASTEXITCODE -ne 0 -or -not $names.Count) { throw 'Cannot enumerate prerequisite inputs' }
    $unique = [Collections.Generic.SortedDictionary[string,string]]::new([StringComparer]::Ordinal)
    foreach ($name in $names) {
        $name = $name.Replace('\','/')
        if (-not $Native -and $name.StartsWith('tests/native/', [StringComparison]::Ordinal)) { continue }
        $path = Join-Path $Root $name
        if(Test-Path -LiteralPath $path) {
            $item=Get-Item -LiteralPath $path -Force
            if($item.Attributes -band [IO.FileAttributes]::ReparsePoint -or $item.PSIsContainer){throw "Unsupported prerequisite input: $name"}
        }
        $unique[$name] = if ([IO.File]::Exists($path)) { Get-SRFileDigest $path } else { 'missing' }
    }
    @($unique.GetEnumerator() | ForEach-Object { @{path=$_.Key;sha256=$_.Value} })
}

function Get-SRPrerequisiteSnapshot([string]$Root, [switch]$Native) {
    $rootPath = (Resolve-Path -LiteralPath $Root).Path
    $files = @(Get-SRPrerequisiteFiles $rootPath -Native:$Native)
    $sourceDigest = Get-SRTextDigest (($files | ForEach-Object { $_.path + ':' + $_.sha256 }) -join "`n")
    $environment = [ordered]@{root=$rootPath;machine=[Environment]::MachineName;sid=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value;os=[Environment]::OSVersion.VersionString}
    foreach ($name in @('go','node','git','powershell.exe')) {
        $command = (Get-Command $name -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
        $version = switch ($name) {
            'go' { (& $command version | Out-String).Trim() }
            'powershell.exe' { (& $command -NoProfile -Command '$PSVersionTable.PSVersion.ToString()' | Out-String).Trim() }
            default { (& $command --version | Out-String).Trim() }
        }
        if ($LASTEXITCODE -ne 0) { throw "Cannot identify prerequisite tool: $name" }
        $environment[$name] = @{path=$command;version=$version}
    }
    $environment['shell'] = @{path=(Get-Process -Id $PID).Path;version=$PSVersionTable.PSVersion.ToString()}
    $issues = @()
    $environment['NODE_OPTIONS']=[string]$env:NODE_OPTIONS
    if($env:NODE_OPTIONS){$issues += 'Node preloads/options are not covered by prerequisite reuse'}
    Push-Location $rootPath
    try {
        $goEnvironment = (& go env -json GOOS GOARCH GOAMD64 GOVERSION GOROOT CGO_ENABLED GOFLAGS GOWORK GOTOOLCHAIN GOEXPERIMENT GODEBUG | Out-String) | ConvertFrom-Json
        if ($LASTEXITCODE -ne 0) { throw 'Cannot identify Go build settings' }
        foreach ($key in @('GOOS','GOARCH','GOAMD64','GOVERSION','GOROOT','CGO_ENABLED','GOFLAGS','GOWORK','GOTOOLCHAIN','GOEXPERIMENT','GODEBUG')) { $environment[$key] = [string]$goEnvironment.$key }
        if ($goEnvironment.GOFLAGS -or $goEnvironment.GOEXPERIMENT -or ($goEnvironment.GOWORK -and $goEnvironment.GOWORK -ne 'off')) { $issues += 'Non-default Go flags/workspace/experiments require fresh prerequisites' }
        $module = (& go mod edit -json | Out-String) | ConvertFrom-Json
        if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect Go module inputs' }
        if ($module.Replace | Where-Object { $_ -and (-not $_.New.PSObject.Properties['Version'] -or -not $_.New.Version) }) { $issues += 'Local module replacements cannot be reused' }
        if(Test-Path -LiteralPath (Join-Path $rootPath 'vendor')){$issues += 'Vendor inputs require fresh execution; prerequisite reuse is unsupported'}
        if($goEnvironment.CGO_ENABLED -ne '0'){$issues += 'CGO compiler inputs are not covered by prerequisite reuse'}
    } finally { Pop-Location }
    $ignored = @(& git -C $rootPath ls-files --others --ignored --exclude-standard -- cmd internal scripts tests)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect ignored prerequisite inputs' }
    if ($ignored | Where-Object { $Native -or $_ -notlike 'tests/native/*' }) { $issues += 'Ignored source inputs cannot be reused' }
    $environmentLines = foreach ($key in $environment.Keys) {
        $value = $environment[$key]
        if ($value -is [Collections.IDictionary]) { $value = $value.path + ':' + $value.version }
        $key + ':' + $value
    }
    $environmentDigest = Get-SRTextDigest ($environmentLines -join "`n")
    @{scope_version=1;source_digest=$sourceDigest;files=$files;environment=$environment;environment_digest=$environmentDigest;fingerprint=(Get-SRTextDigest ($sourceDigest+':'+$environmentDigest));supported=($issues.Count -eq 0);issues=$issues;source_commit=(& git -C $rootPath rev-parse HEAD | Out-String).Trim();source_dirty=(-not [string]::IsNullOrWhiteSpace((& git -C $rootPath status --porcelain | Out-String)))}
}

function Get-SRPrerequisiteSnapshotOrUnavailable([string]$Root, [switch]$Native) {
    try { Get-SRPrerequisiteSnapshot $Root -Native:$Native } catch { @{supported=$false;issues=@($_.Exception.Message);fingerprint='unavailable'} }
}

function Get-SRPrerequisitePackages([string]$Root, $Catalog) {
    Push-Location $Root
    try {
        $packages = @(& go list @($Catalog.l1_packages))
        if ($LASTEXITCODE -ne 0 -or -not $packages.Count) { throw 'Cannot expand L1 packages' }
        $packages
    } finally { Pop-Location }
}

function Assert-SRPrerequisiteSet([object[]]$Actual, [object[]]$Expected, [string]$Name) {
    [string[]]$a = @($Actual); [string[]]$e = @($Expected)
    [Array]::Sort($a, [StringComparer]::Ordinal); [Array]::Sort($e, [StringComparer]::Ordinal)
    if (-not $a.Count -or ($a -join "`n") -cne ($e -join "`n") -or @($a | Select-Object -Unique).Count -ne $a.Count) { throw "Incomplete or duplicate $Name coverage" }
}

function Get-SRL1AllowedSkips {
    @('TestShortInputProbeInspect','TestProcessHostCommandHelper','TestServiceStopWorkerHelper','TestOrcaLocalPipeNativeEvidence','TestOrcaLocalIPCNativeProbe','TestOrcaNativeStateLiveLifecycle','TestOrcaWorkspaceStartupLive','TestOrcaStoppedNativeProbe',
      'TestUsageLocalAcceptanceRepoCleanupLive','TestUsageExistingNativeEvidence','TestUsageRemoteCodexCleanupLive','TestUsageRemoteCodexLive','TestRunUsageLocalCodexPublicLive','TestRunUsageLocalPiPublicLive','TestRunUsageLocalDSHPublicLive','TestRunUsageRemotePublicLive')
}

function Assert-SRNoLiveOptIn {
    foreach ($key in @('STEERRUNNER_SHORT_INPUT_INSPECT','STEERRUNNER_ORCA_STATE_PROBE_MODE','STEERRUNNER_ORCA_STATE_PROBE_ROOT','STEERRUNNER_ORCA_WORKSPACE_PROBE_ROOT','STEERRUNNER_ORCA_LOCAL_PIPE_EVIDENCE','STEERRUNNER_ORCA_IPC_PROBE_ROOT','STEERRUNNER_ORCA_STOP_PROBE_ROOT','SR_USAGE_EXISTING_STATE','SR_USAGE_LIVE_STATE','SR_USAGE_LIVE_RUN','SR_USAGE_LIVE_EVIDENCE','SR_USAGE_LIVE_PUBLIC','SR_USAGE_LIVE_CLEANUP_STATE','SR_USAGE_LOCAL_LIVE','SR_USAGE_LOCAL_STATE','SR_USAGE_LOCAL_RUN','SR_USAGE_LOCAL_EVIDENCE','SR_USAGE_LOCAL_REPO_CLEANUP','SR_USAGE_LOCAL_CLEANUP_ROOT')) {
        if ([Environment]::GetEnvironmentVariable($key)) { throw "Unexpected live opt-in in L1: $key" }
    }
}

function Assert-SRL1Evidence([string]$Directory, $Record, [string[]]$Packages) {
    $events = @(Get-Content -LiteralPath (Join-Path $Directory 'l1.jsonl') | ForEach-Object { $_ | ConvertFrom-Json })
    $completed = @($events | Where-Object { -not $_.PSObject.Properties['Test'] -and $_.Action -in @('pass','skip') } | ForEach-Object { $_.Package })
    Assert-SRPrerequisiteSet $completed $Packages 'L1 package'
    $passed = @($events | Where-Object { $_.Action -eq 'pass' -and $_.PSObject.Properties['Test'] -and $_.Test -notmatch '/' }).Count
    $skips = @($events | Where-Object { $_.Action -eq 'skip' -and $_.PSObject.Properties['Test'] } | ForEach-Object { $_.Test })
    $tap = Get-Content -LiteralPath (Join-Path $Directory 'terminal-extensions.tap') -Raw
    $framework = Get-Content -LiteralPath (Join-Path $Directory 'framework/result.json') -Raw | ConvertFrom-Json
    if ($Record.exit_code -ne 0 -or $passed -eq 0 -or $Record.test_count -ne $passed -or @($events | Where-Object {$_.Action -eq 'fail'}).Count -or @($skips | Where-Object {$_ -notin @(Get-SRL1AllowedSkips)}).Count -or
        $Record.terminal_extension_exit -ne 0 -or $Record.terminal_extension_test_count -lt 1 -or -not $Record.terminal_extensions_passed -or $tap -notmatch '(?m)^# fail 0\s*$' -or $tap -notmatch '(?m)^# skipped 0\s*$' -or
        $tap -notmatch ('(?m)^# tests '+$Record.terminal_extension_test_count+'\s*$') -or $framework.exit_code -ne 0 -or $framework.test_count -lt 1 -or $framework.test_count -ne @($framework.tests).Count -or @($framework.tests | Where-Object {-not $_.passed}).Count) { throw 'Invalid L1 prerequisite evidence' }
    @{packages=$completed;test_count=$passed;skipped_tests=$skips}
}

function Test-SRControlledHarnessComplete([string]$Directory) {
    $identity = Get-Content -LiteralPath (Join-Path $Directory 'source-identity.json') -Raw | ConvertFrom-Json
    $child = Get-Process -Id $identity.pid -ErrorAction SilentlyContinue
    if (-not $child) { return $true }
    if ($identity.PSObject.Properties['process_started_at'] -and $identity.process_started_at) {
        # PowerShell 7 may already deserialize ISO timestamps as DateTime.
        # Parse(DateTime) round-trips through a timezone-less string and can
        # mistake a live child for a reused PID on non-UTC hosts.
        return [Math]::Abs(($child.StartTime.ToUniversalTime() - ([DateTime]$identity.process_started_at).ToUniversalTime()).TotalSeconds) -gt 1
    }
    return $false
}

function Assert-SRControlledEvidence([string]$Directory, [string[]]$Expected) {
    $result = Get-Content -LiteralPath (Join-Path $Directory 'result.json') -Raw | ConvertFrom-Json
    $log = Get-Content -LiteralPath (Join-Path $Directory 'integration.log') -Raw
    $passed = @([regex]::Matches($log, '(?m)^--- PASS: (Test\w+) \(') | ForEach-Object { $_.Groups[1].Value })
    Assert-SRPrerequisiteSet $passed $Expected 'Controlled'
    if ($result.exit_code -ne 0 -or $result.scenario_count -ne $Expected.Count -or $log -match '--- (SKIP|FAIL):' -or -not (Test-SRControlledHarnessComplete $Directory)) { throw 'Controlled prerequisite failed, skipped or is still running' }
    $identity = Get-Content -LiteralPath (Join-Path $Directory 'source-identity.json') -Raw | ConvertFrom-Json
    foreach ($artifact in @(@{name='steerrunner.exe';field='runner_sha256'},@{name='integration.test.exe';field='fixture_sha256'},@{name='native-fixture.exe';field='native_fixture_sha256'})) {
        if ((Get-SRFileDigest (Join-Path $Directory $artifact.name)) -ine $identity.($artifact.field)) { throw 'Controlled artifact identity changed' }
    }
    if ([IO.Path]::GetFullPath($result.exe) -ine (Join-Path $Directory 'steerrunner.exe')) { throw 'Controlled result tested a different Runner' }
    @{tests=$passed;scenario_count=$result.scenario_count}
}

function Wait-SRControlledResult([string]$Directory,[DateTime]$Until) {
    $path=Join-Path $Directory 'result.json'
    # TestMain publishes result before exit. A visible file can still be partial;
    # wait for the original child to exit before parsing it, within its old budget.
    while(-not(Test-Path -LiteralPath $path) -or -not(Test-SRControlledHarnessComplete $Directory)) {
        if([DateTime]::UtcNow -gt $Until){throw 'Controlled harness timed out; inspect integration.log'}
        Start-Sleep -Milliseconds 200
    }
    Get-Content -LiteralPath $path -Raw | ConvertFrom-Json
}

function Get-SRPrerequisiteEvidencePaths([string]$Output,[string]$Name) {
    if($Name -eq 'l1-core') {
        @('l1.jsonl','l1-stderr.log','terminal-extensions.tap','terminal-extensions-stderr.log','coverage.out') | ForEach-Object {Join-Path $Output $_}
        Get-ChildItem -LiteralPath (Join-Path $Output 'framework') -File -Recurse | Where-Object {$_.FullName -notlike '*\native-zero-match\*'} | ForEach-Object {$_.FullName}
    } else {
        @('steerrunner.exe','steerrunner.exe.build.json','integration.test.exe','native-fixture.exe','unit-foundation.log','integration.log','result.json','source-identity.json') | ForEach-Object {Join-Path (Join-Path $Output 'controlled') $_}
    }
}

function Save-SRPrerequisiteStage([string]$Root, [string]$Output, [string]$Name, $Start, $Record, [string[]]$Packages=@(), [bool]$DefaultCatalog=$true) {
    $finish = Get-SRPrerequisiteSnapshotOrUnavailable $Root
    $stage = @{kind='executed';direct=$true;status='unavailable';started_at=$Record.started_at;finished_at=[DateTime]::UtcNow.ToString('o');duration_seconds=([DateTime]::UtcNow-[DateTime]::Parse($Record.started_at).ToUniversalTime()).TotalSeconds;input_start=$Start;input_finish=$finish;record=$Record;default_catalog=$DefaultCatalog}
    try {
        if (-not $DefaultCatalog -or -not $Start.supported -or -not $finish.supported -or $Start.fingerprint -cne $finish.fingerprint) { throw 'Prerequisite input identity is unsupported or changed' }
        $catalog = Get-Content -LiteralPath (Join-Path $Root 'tests/catalog.json') -Raw | ConvertFrom-Json
        if ($Name -eq 'l1-core') {
            $stage['coverage'] = Assert-SRL1Evidence $Output $Record $Packages
        } else {
            $directory = Join-Path $Output 'controlled'
            $stage['coverage'] = Assert-SRControlledEvidence $directory @($catalog.controlled_scenarios)
            $stage['runner_exe'] = Join-Path $directory 'steerrunner.exe'
            $build = Get-SRBuildRecord $stage.runner_exe 'runner'
            $currentBuild = Get-SRSourceIdentity $Root @('cmd/steerrunner','internal','go.mod','go.sum','scripts/build.ps1','scripts/delivery-contract.ps1')
            if ($build.source_digest -cne $currentBuild.source_digest) { throw 'Controlled Runner build does not match current source' }
        }
        $paths=@(Get-SRPrerequisiteEvidencePaths $Output $Name)
        $stage['files'] = @($paths | ForEach-Object { @{path=[IO.Path]::GetFullPath($_);sha256=(Get-SRFileDigest $_)} })
        $stage.status = 'passed'
    } catch { $stage['reason'] = $_.Exception.Message }
    $path = Join-Path $Output 'prerequisites.json'
    $manifest = if (Test-Path -LiteralPath $path) { Get-Content -LiteralPath $path -Raw | ConvertFrom-Json -AsHashtable } else { @{schema='steerrunner-test-prerequisites/v1';scope_version=1;stages=@{}} }
    $manifest.stages[$Name] = $stage
    Write-SRPrerequisiteJSON $path $manifest
    $stage
}

function Read-SRPrerequisites([string]$Root, [string]$Directory) {
    $path = Join-Path ([IO.Path]::GetFullPath($Directory)) 'prerequisites.json'
    $manifest = Get-Content -LiteralPath $path -Raw | ConvertFrom-Json -AsHashtable
    if ($manifest.schema -cne 'steerrunner-test-prerequisites/v1' -or $manifest.scope_version -ne 1) { throw 'Unsupported prerequisite proof; generate fresh prerequisites' }
    $snapshot = Get-SRPrerequisiteSnapshot $Root
    if (-not $snapshot.supported) { throw ('Prerequisite reuse unsupported: '+($snapshot.issues -join '; ')) }
    $catalog = Get-Content -LiteralPath (Join-Path $Root 'tests/catalog.json') -Raw | ConvertFrom-Json
    foreach ($name in @('l1-core','controlled')) {
        $stage = $manifest.stages[$name]
        if (-not $stage -or $stage.status -cne 'passed' -or $stage.kind -cne 'executed' -or $stage.direct -ne $true -or $stage.default_catalog -ne $true -or -not $stage.finished_at -or $stage.input_start.fingerprint -cne $snapshot.fingerprint -or $stage.input_finish.fingerprint -cne $snapshot.fingerprint -or -not $stage.input_start.supported -or -not $stage.input_finish.supported) { throw "Prerequisite $name is missing, changed or not a direct complete success; generate fresh prerequisites" }
        if (-not $stage.files.Count) { throw "Missing $name evidence inventory" }
        $expectedDirectory=if($name -eq 'l1-core'){[IO.Path]::GetFullPath($Directory)}else{Join-Path ([IO.Path]::GetFullPath($Directory)) 'controlled'}
        if($stage.record.evidence -ine $expectedDirectory){throw 'Prerequisite evidence directory does not match its source'}
        Assert-SRPrerequisiteSet @($stage.files | ForEach-Object {$_.path}) @(Get-SRPrerequisiteEvidencePaths ([IO.Path]::GetFullPath($Directory)) $name) "$name evidence inventory"
        foreach ($file in $stage.files) { if ((Get-SRFileDigest $file.path) -cne $file.sha256) { throw "Prerequisite evidence changed: $($file.path)" } }
        if ($name -eq 'l1-core') {
            $packages = @(Get-SRPrerequisitePackages $Root $catalog)
            Assert-SRPrerequisiteSet @($stage.coverage.packages) $packages 'recorded L1 package'
            $null = Assert-SRL1Evidence $stage.record.evidence $stage.record $packages
        } else {
            $null = Assert-SRControlledEvidence $stage.record.evidence @($catalog.controlled_scenarios)
            if ($stage.runner_exe -ine (Join-Path $stage.record.evidence 'steerrunner.exe')) { throw 'Prerequisite Runner path changed' }
            $build = Get-SRBuildRecord $stage.runner_exe 'runner'
            $current = Get-SRSourceIdentity $Root @('cmd/steerrunner','internal','go.mod','go.sum','scripts/build.ps1','scripts/delivery-contract.ps1')
            if ($build.source_digest -cne $current.source_digest) { throw 'Prerequisite Runner source changed' }
        }
    }
    @{manifest=$manifest;manifest_path=$path;manifest_sha256=(Get-SRFileDigest $path);snapshot=$snapshot;runner_exe=$manifest.stages.controlled.runner_exe}
}

function Assert-SRFrozenBatchFiles([string]$Config,[string]$Catalog,[string]$Runner,[string]$Helper,$Summary) {
    foreach($artifact in @(@{path=$Config;sha=$Summary.configuration_sha256},@{path=$Catalog;sha=$Summary.catalog_sha256},@{path=$Runner;sha=$Summary.runner_sha256},@{path=$Helper;sha=$Summary.helper_sha256})) {
        if((Get-SRFileDigest $artifact.path) -cne $artifact.sha){throw "Frozen batch input changed: $($artifact.path)"}
    }
}

function Invoke-SRL1Core([string]$Root, [string]$Output, $Catalog, [bool]$DefaultCatalog=$true) {
    $taskRepo=$Root; $taskOut=$Output; $taskCatalog=$Catalog
    $srStarted=[DateTime]::UtcNow.ToString('o')
    Assert-SRNoLiveOptIn
    $srStart=Get-SRPrerequisiteSnapshotOrUnavailable $Root
    $srPackages=@()
    try { $srPackages=@(Get-SRPrerequisitePackages $Root $Catalog) } catch { $srStart.supported=$false; $srStart.issues+=@($_.Exception.Message) }
    Write-Host 'Running L1'
    $taskFramework = Join-Path $taskOut 'framework'
    & (Join-Path $PSScriptRoot 'test-framework.ps1') -Evidence $taskFramework | Out-Host
    $taskCoverage = Join-Path $taskOut 'coverage.out'
    & go test @($taskCatalog.l1_packages) -count=1 -timeout=12m -json "-coverprofile=$taskCoverage" 1> (Join-Path $taskOut 'l1.jsonl') 2> (Join-Path $taskOut 'l1-stderr.log')
    $taskUnitCode = $LASTEXITCODE
    # The two formal native extensions are part of L1, including on machines
    # without model credentials or a running Orca instance.
    $taskExtensionLog = Join-Path $taskOut 'terminal-extensions.tap'
    & node --test --test-reporter=tap internal/execution/adapters/orcatransport/extensions/pi-evidence.test.mjs internal/execution/adapters/orcatransport/extensions/dsh-managed.test.mjs 1> $taskExtensionLog 2> (Join-Path $taskOut 'terminal-extensions-stderr.log')
    $taskExtensionCode = $LASTEXITCODE
    $taskExtensionResult = Get-Content -LiteralPath $taskExtensionLog -Raw
    $taskExtensionCount = if ($taskExtensionResult -match '(?m)^# tests (\d+)\s*$') { [int]$Matches[1] } else { 0 }
    $taskExtensionsPassed = $taskExtensionCode -eq 0 -and $taskExtensionCount -gt 0 -and $taskExtensionResult -match '(?m)^# fail 0\s*$' -and $taskExtensionResult -match '(?m)^# skipped 0\s*$'
    $taskEvents = @(Get-Content -LiteralPath (Join-Path $taskOut 'l1.jsonl') | ForEach-Object { $_ | ConvertFrom-Json })
    $taskPassed = @($taskEvents | Where-Object { $_.Action -eq 'pass' -and $_.PSObject.Properties['Test'] -and $_.Test -notmatch '/' }).Count
    $taskSkips = @($taskEvents | Where-Object { $_.Action -eq 'skip' -and $_.PSObject.Properties['Test'] })
    # Existing usage acceptance/cleanup probes are explicit native opt-ins;
    # default skips are not native passes. Their live environment guards above remain mandatory.
    $taskAllowedSkips = @(Get-SRL1AllowedSkips)
    $taskUnexpected = @($taskSkips | Where-Object { $_.Test -notin $taskAllowedSkips })
    $srRecord=@{level='L1';exit_code=$taskUnitCode;test_count=$taskPassed;skipped_tests=@($taskSkips | ForEach-Object {$_.Test});evidence=$taskOut;framework_evidence=$taskFramework;terminal_extension_exit=$taskExtensionCode;terminal_extension_test_count=$taskExtensionCount;terminal_extensions_passed=$taskExtensionsPassed;started_at=$srStarted}
    $srStage=Save-SRPrerequisiteStage $Root $Output 'l1-core' $srStart $srRecord $srPackages $DefaultCatalog
    @{record=$srRecord;stage=$srStage;passed=($taskUnitCode -eq 0 -and $taskPassed -gt 0 -and $taskUnexpected.Count -eq 0 -and $taskExtensionsPassed)}
}

function Invoke-SRNativePreflight([string]$Root, [string]$Output, [switch]$RequireStable) {
    $taskOut=$Output; $taskFramework=Join-Path $Output 'framework'
    $srStarted=[DateTime]::UtcNow.ToString('o')
    $srStart=Get-SRPrerequisiteSnapshotOrUnavailable $Root -Native
    if($RequireStable -and -not $srStart.supported){throw 'Native preflight inputs cannot be identified'}
    $taskDriver = Join-Path $taskOut 'native.test.exe'
    & go test -tags=live -c -o $taskDriver ./tests/native 1> (Join-Path $taskOut 'native-compile.log') 2> (Join-Path $taskOut 'native-compile-stderr.log')
    $taskCompileCode = $LASTEXITCODE
    $taskZeroGuard = $false
    if ($taskCompileCode -eq 0) {
        $taskZeroEvidence = Join-Path $taskFramework 'native-zero-match'
        New-Item -ItemType Directory -Path $taskZeroEvidence | Out-Null
        & $taskDriver '-test.run=^NoSuchScenario$' -evidence-dir $taskZeroEvidence
        $taskGuardCode = $LASTEXITCODE
        $taskGuardResult = Get-Content -LiteralPath (Join-Path $taskZeroEvidence 'result.json') -Raw | ConvertFrom-Json
        $taskZeroGuard = $taskGuardCode -eq 1 -and $taskGuardResult.exit_code -eq 1 -and $taskGuardResult.scenario_count -eq 0
    }
    $srFinish=Get-SRPrerequisiteSnapshotOrUnavailable $Root -Native
    if($RequireStable -and (-not $srFinish.supported -or $srStart.fingerprint -cne $srFinish.fingerprint)){throw 'Native preflight inputs changed'}
    @{kind='executed';level='NativePreflight';started_at=$srStarted;finished_at=[DateTime]::UtcNow.ToString('o');duration_seconds=([DateTime]::UtcNow-[DateTime]::Parse($srStarted).ToUniversalTime()).TotalSeconds;compile_exit=$taskCompileCode;zero_match_guard=$taskZeroGuard;driver_path=$taskDriver;driver_sha256=$(if($taskCompileCode -eq 0){Get-SRFileDigest $taskDriver}else{''});input_start=$srStart;input_finish=$srFinish}
}

function Invoke-SRControlledStage([string]$Root, [string]$Output, $Catalog, [bool]$DefaultCatalog=$true) {
    $srStarted=[DateTime]::UtcNow.ToString('o')
    $srStart=Get-SRPrerequisiteSnapshotOrUnavailable $Root
    Write-Host 'Running controlled integration prerequisite'
    $directory=Join-Path $Output 'controlled'
    $pattern='^('+(($Catalog.controlled_scenarios | ForEach-Object {[regex]::Escape($_)}) -join '|')+')$'
    & (Join-Path $PSScriptRoot 'test-foundation.ps1') -OutDir $directory -Pattern $pattern -TimeoutMinutes 45 | Out-Host
    $until=[DateTime]::UtcNow.AddMinutes(45).AddSeconds(30)
    $result=Wait-SRControlledResult $directory $until
    $record=@{level='Controlled';exit_code=$result.exit_code;scenario_count=$result.scenario_count;evidence=$directory;started_at=$srStarted}
    $passed=$result.exit_code -eq 0 -and $result.scenario_count -eq @($Catalog.controlled_scenarios).Count -and -not (Select-String -LiteralPath (Join-Path $directory 'integration.log') -Pattern '--- SKIP:' -Quiet)
    $stage=Save-SRPrerequisiteStage $Root $Output 'controlled' $srStart $record @() $DefaultCatalog
    @{record=$record;stage=$stage;passed=$passed}
}
