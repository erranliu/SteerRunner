param([string]$Repository,[string]$Evidence)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
. (Join-Path $Repository 'scripts/test-prerequisite-contract.ps1')
. (Join-Path $Repository 'scripts/test-configuration.ps1')
$taskRealSnapshotFunction=${function:Get-SRPrerequisiteSnapshot}
$taskRealSnapshot=Get-SRPrerequisiteSnapshotOrUnavailable $Repository
if(-not $taskRealSnapshot.ContainsKey('fingerprint')){throw 'Real snapshot failed under strict mode'}
$taskRoot=Join-Path $Evidence 'prerequisites'
if(Test-Path -LiteralPath $taskRoot){throw 'Prerequisite framework evidence must be new'}
$null=New-Item -ItemType Directory -Path $taskRoot
$taskRepo=Join-Path $taskRoot 'repository'
$null=New-Item -ItemType Directory -Path $taskRepo
function Put([string]$Path,[string]$Text) {
    $null=New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path)
    [IO.File]::WriteAllText($Path,$Text,[Text.UTF8Encoding]::new($false))
}
function JSON([string]$Path,$Value){Put $Path ($Value | ConvertTo-Json -Depth 80)}
$taskCatalog=@{l1_packages=@('./internal/...');controlled_scenarios=@('TestOne','TestTwo')}
foreach($name in @('cmd/steerrunner/main.go','internal/fixture/input.go','scripts/build.ps1','scripts/delivery-contract.ps1','tests/native/driver.go','tests/integration/input.go','tests/helpers/input.go','tests/framework/input.ps1','internal/extensions/test.mjs','go.sum','.gitattributes','docs/workflows/generate-repository.example.json')) {Put (Join-Path $taskRepo $name) 'original'}
Put (Join-Path $taskRepo 'go.mod') "module example.invalid/prerequisites`ngo 1.24.2`n"
Put (Join-Path $taskRepo '.gitignore') '.evidence/'
JSON (Join-Path $taskRepo 'tests/catalog.json') $taskCatalog
& git -C $taskRepo init -q
if($LASTEXITCODE -ne 0){throw 'Cannot initialize isolated prerequisite repository'}
& git -C $taskRepo add .
if($LASTEXITCODE -ne 0){throw 'Cannot register isolated inputs'}
& git -C $taskRepo -c user.name=Fixture -c user.email=fixture@example.invalid -c commit.gpgsign=false commit -qm fixture
if($LASTEXITCODE -ne 0){throw 'Cannot commit isolated prerequisite inputs'}
$taskIsolatedSnapshot=Get-SRPrerequisiteSnapshot $taskRepo
if(-not $taskIsolatedSnapshot.ContainsKey('fingerprint')){throw 'Isolated real snapshot failed under strict mode'}
# Environment/package doubles are confined to this test process. Production has no injection switch.
$script:taskEnvironment='same-tools';$script:taskUnsupported=$false;$script:taskPackages=@('example.invalid/prerequisites/internal/fixture')
function Get-SRPrerequisiteSnapshot([string]$Root,[switch]$Native) {
    $files=@(Get-SRPrerequisiteFiles $Root -Native:$Native)
    $digest=Get-SRTextDigest (($files | ForEach-Object {$_.path+':'+$_.sha256}) -join "`n")
    @{supported=(-not $script:taskUnsupported);issues=@();fingerprint=(Get-SRTextDigest ($digest+':'+$script:taskEnvironment));files=$files;source_digest=$digest;environment_digest=$script:taskEnvironment}
}
function Get-SRPrerequisitePackages([string]$Root,$Catalog){$script:taskPackages}
$taskOut=Join-Path $taskRepo '.evidence/source'
$taskControlled=Join-Path $taskOut 'controlled'
Put (Join-Path $taskOut 'l1.jsonl') ((@(
    @{Action='pass';Package=$script:taskPackages[0];Test='TestOne'},
    @{Action='skip';Package=$script:taskPackages[0];Test='TestRunUsageLocalCodexPublicLive'},
    @{Action='pass';Package=$script:taskPackages[0]}
) | ForEach-Object {$_ | ConvertTo-Json -Compress}) -join "`n")
Put (Join-Path $taskOut 'terminal-extensions.tap') "# tests 1`n# fail 0`n# skipped 0`n"
foreach($name in @('l1-stderr.log','terminal-extensions-stderr.log','coverage.out')){Put (Join-Path $taskOut $name) ''}
JSON (Join-Path $taskOut 'framework/result.json') @{exit_code=0;test_count=1;tests=@(@{name='isolated-framework';passed=$true})}
foreach($name in @('steerrunner.exe','integration.test.exe','native-fixture.exe')){Put (Join-Path $taskControlled $name) $name}
$taskRunner=Join-Path $taskControlled 'steerrunner.exe'
$taskBuild=Get-SRSourceIdentity $taskRepo @('cmd/steerrunner','internal','go.mod','go.sum','scripts/build.ps1','scripts/delivery-contract.ps1')
JSON ($taskRunner+'.build.json') @{schema='steerrunner-build/v1';component='runner';platform='windows/amd64';source_commit=('a'*40);source_digest=$taskBuild.source_digest;source_dirty=$true;sha256=(Get-SRFileDigest $taskRunner)}
JSON (Join-Path $taskControlled 'source-identity.json') @{pid=2147483646;runner_sha256=(Get-SRFileDigest $taskRunner);fixture_sha256=(Get-SRFileDigest (Join-Path $taskControlled 'integration.test.exe'));native_fixture_sha256=(Get-SRFileDigest (Join-Path $taskControlled 'native-fixture.exe'))}
JSON (Join-Path $taskControlled 'result.json') @{exit_code=0;scenario_count=2;exe=$taskRunner}
Put (Join-Path $taskControlled 'integration.log') "--- PASS: TestOne (0.01s)`n    --- PASS: TestOne/child (0.01s)`n--- PASS: TestTwo (0.01s)`n"
Put (Join-Path $taskControlled 'unit-foundation.log') 'PASS'
$taskStart=Get-SRPrerequisiteSnapshot $taskRepo
$taskRecord=@{level='L1';started_at=[DateTime]::UtcNow.ToString('o');exit_code=0;test_count=1;evidence=$taskOut;framework_evidence=(Join-Path $taskOut 'framework');terminal_extension_exit=0;terminal_extension_test_count=1;terminal_extensions_passed=$true;skipped_tests=@('TestRunUsageLocalCodexPublicLive')}
$taskCore=Save-SRPrerequisiteStage $taskRepo $taskOut 'l1-core' $taskStart $taskRecord $script:taskPackages
$taskControlledRecord=@{level='Controlled';started_at=[DateTime]::UtcNow.ToString('o');exit_code=0;scenario_count=2;evidence=$taskControlled}
$taskControl=Save-SRPrerequisiteStage $taskRepo $taskOut 'controlled' $taskStart $taskControlledRecord
if($taskCore.status -ne 'passed' -or $taskControl.status -ne 'passed'){throw ('Synthetic direct proof failed: '+($taskCore['reason'],$taskControl['reason'] -join '; '))}
$null=Read-SRPrerequisites $taskRepo $taskOut
$taskManifestPath=Join-Path $taskOut 'prerequisites.json'
$taskManifest=[IO.File]::ReadAllText($taskManifestPath)
$taskChecks=[Collections.Generic.List[string]]::new()
$taskChecks.Add('real-snapshot-strict-mode')
function Reject([string]$Name,[scriptblock]$Action) {
    $rejected=$false
    try {& $Action} catch {$rejected=$true}
    if(-not $rejected){throw "Prerequisite guard accepted $Name"}
    $taskChecks.Add($Name)
}
foreach($name in @('cmd/steerrunner/main.go','internal/fixture/input.go','scripts/build.ps1','tests/integration/input.go','tests/helpers/input.go','tests/framework/input.ps1','internal/extensions/test.mjs','go.mod','tests/catalog.json','docs/workflows/generate-repository.example.json','.gitignore')) {
    $path=Join-Path $taskRepo $name;$original=[IO.File]::ReadAllText($path)
    try {Put $path ($original+'changed');Reject ('source-'+$name) {Read-SRPrerequisites $taskRepo $taskOut | Out-Null}} finally {Put $path $original}
}
foreach($name in @('tests/native/driver.go','docs/new-notes.md')) {
    $path=Join-Path $taskRepo $name;$exists=Test-Path -LiteralPath $path;$original=if($exists){[IO.File]::ReadAllText($path)}else{''}
    $beforeNative=Get-SRPrerequisiteSnapshot $taskRepo -Native
    try {
        Put $path ($original+'changed');$null=Read-SRPrerequisites $taskRepo $taskOut
        if($name -like 'tests/native/*' -and (Get-SRPrerequisiteSnapshot $taskRepo -Native).fingerprint -ceq $beforeNative.fingerprint){throw 'Native change did not invalidate preflight input identity'}
        $taskChecks.Add('accepted-'+$name)
    } finally {if($exists){Put $path $original}else{Remove-Item -LiteralPath $path}}
}
$path=Join-Path $taskRepo 'internal/fixture/new.go'
try {Put $path 'new input';Reject 'untracked-input' {Read-SRPrerequisites $taskRepo $taskOut | Out-Null}}finally{Remove-Item -LiteralPath $path}
$path=Join-Path $taskRepo 'tests/helpers/input.go';$original=[IO.File]::ReadAllText($path)
try{Remove-Item -LiteralPath $path;Reject 'deleted-input' {Read-SRPrerequisites $taskRepo $taskOut | Out-Null}}finally{Put $path $original}
$script:taskEnvironment='changed-git';Reject 'tool-identity' {Read-SRPrerequisites $taskRepo $taskOut | Out-Null};$script:taskEnvironment='same-tools'
$script:taskUnsupported=$true;Reject 'unsupported-inputs' {Read-SRPrerequisites $taskRepo $taskOut | Out-Null};$script:taskUnsupported=$false
$vendor=Join-Path $taskRepo 'vendor/modules.txt'
try {
    Put $vendor 'vendor inputs'
    $actual=& $taskRealSnapshotFunction $taskRepo
    if($actual.supported -or ($actual.issues -join '; ') -notmatch 'Vendor'){throw 'Vendor mode was not excluded from reuse'}
    $taskChecks.Add('real-vendor-unsupported')
}finally{Remove-Item -LiteralPath $vendor;Remove-Item -LiteralPath (Split-Path -Parent $vendor)}
$path=Join-Path $taskRepo 'go.mod';$original=[IO.File]::ReadAllText($path)
try {
    Put $path ($original+"`nreplace example.invalid/dep => ../outside`n")
    $actual=& $taskRealSnapshotFunction $taskRepo
    if($actual.supported -or ($actual.issues -join '; ') -notmatch 'Local module'){throw 'Local module replacement was not excluded from reuse'}
    $taskChecks.Add('real-local-replace-unsupported')
}finally{Put $path $original}
$exclude=Join-Path $taskRepo '.git/info/exclude';$originalExclude=[IO.File]::ReadAllText($exclude)
foreach($extension in @('txt','bin')) {
    $ignoredPath=Join-Path $taskRepo ('internal/fixture/ignored.'+$extension)
    try {
        Put $exclude ($originalExclude+"`ninternal/fixture/ignored.$extension`n")
        Put $ignoredPath 'unrecorded input'
        $actual=& $taskRealSnapshotFunction $taskRepo
        if($actual.supported -or ($actual.issues -join '; ') -notmatch 'Ignored'){throw 'Non-code ignored input remained reusable'}
        $taskChecks.Add('ignored-input-'+$extension)
    } finally {Put $exclude $originalExclude;Remove-Item -LiteralPath $ignoredPath}
}
$directoryInput=Join-Path $taskRepo 'internal/fixture/link'
$null=New-Item -ItemType Directory -Path $directoryInput
$gitlinkCommit=(& git -C $taskRepo rev-parse HEAD).Trim()
try {
    & git -C $taskRepo update-index --add --cacheinfo "160000,$gitlinkCommit,internal/fixture/link"
    if($LASTEXITCODE -ne 0){throw 'Cannot create isolated directory input'}
    Reject 'directory-or-gitlink-input' {Get-SRPrerequisiteFiles $taskRepo | Out-Null}
    $unavailable=Get-SRPrerequisiteSnapshotOrUnavailable $taskRepo
    if($unavailable.supported -or -not $unavailable.ContainsKey('fingerprint')){throw 'Unsupported source blocked default snapshot fallback'}
    $taskChecks.Add('unsupported-source-default-fallback')
} finally {& git -C $taskRepo update-index --force-remove -- internal/fixture/link;Remove-Item -LiteralPath $directoryInput}
foreach($setting in @(@{name='GOAMD64';first='v1';second='v2'},@{name='GODEBUG';first='x509sha1=0';second='x509sha1=1'})) {
    $previous=[Environment]::GetEnvironmentVariable($setting.name)
    try {
        [Environment]::SetEnvironmentVariable($setting.name,$setting.first);$first=& $taskRealSnapshotFunction $taskRepo
        [Environment]::SetEnvironmentVariable($setting.name,$setting.second);$second=& $taskRealSnapshotFunction $taskRepo
        if($first.environment_digest -ceq $second.environment_digest){throw 'Go setting change did not invalidate identity'}
        $taskChecks.Add('go-setting-'+$setting.name)
    } finally {[Environment]::SetEnvironmentVariable($setting.name,$previous)}
}
$previous=$env:NODE_OPTIONS
try {$env:NODE_OPTIONS='--no-warnings';$actual=& $taskRealSnapshotFunction $taskRepo;if($actual.supported -or ($actual.issues -join '; ') -notmatch 'Node'){throw 'Node options remained reusable'};$taskChecks.Add('node-options-unsupported')}finally{$env:NODE_OPTIONS=$previous}
$script:taskPackages+=@('missing-package');Reject 'package-expansion' {Read-SRPrerequisites $taskRepo $taskOut | Out-Null};$script:taskPackages=@($script:taskPackages[0])
foreach($name in @('steerrunner.exe','steerrunner.exe.build.json','integration.test.exe','native-fixture.exe','integration.log','result.json','source-identity.json','unit-foundation.log')) {
    $path=Join-Path $taskControlled $name;$original=[IO.File]::ReadAllBytes($path)
    try {[IO.File]::AppendAllText($path,'changed');Reject ('artifact-'+$name) {Read-SRPrerequisites $taskRepo $taskOut | Out-Null};Remove-Item -LiteralPath $path;Reject ('missing-'+$name) {Read-SRPrerequisites $taskRepo $taskOut | Out-Null}}finally{[IO.File]::WriteAllBytes($path,$original)}
}
foreach($variant in @('failed','running','recursive','wrong-stage-input','missing-inventory','partial-inventory','old-schema')) {
    $manifest=$taskManifest | ConvertFrom-Json -AsHashtable
    switch($variant) {
        'failed' {$manifest.stages.controlled.status='failed'}
        'running' {$manifest.stages.controlled.status='running'}
        'recursive' {$manifest.stages.controlled.kind='reused'}
        'wrong-stage-input' {$manifest.stages['l1-core'].input_start.fingerprint='different'}
        'missing-inventory' {$manifest.stages.controlled.files=@()}
        'partial-inventory' {$manifest.stages.controlled.files=@($manifest.stages.controlled.files | Select-Object -Skip 1)}
        'old-schema' {$manifest.schema='legacy'}
    }
    try{JSON $taskManifestPath $manifest;Reject $variant {Read-SRPrerequisites $taskRepo $taskOut | Out-Null}}finally{Put $taskManifestPath $taskManifest}
}
$path=Join-Path $taskControlled 'integration.log';$original=[IO.File]::ReadAllText($path)
foreach($text in @("--- PASS: TestOne (0s)`n--- PASS: TestOther (0s)","--- PASS: TestOne (0s)`n--- PASS: TestOne (0s)","--- PASS: TestOne (0s)`n--- SKIP: TestTwo (0s)","--- PASS: TestOne/child (0s)`n--- PASS: TestTwo (0s)")) {
    try {Put $path $text;Reject 'controlled-set' {Assert-SRControlledEvidence $taskControlled $taskCatalog.controlled_scenarios | Out-Null}}finally{Put $path $original}
}
$path=Join-Path $taskControlled 'source-identity.json';$original=[IO.File]::ReadAllText($path);$identity=$original | ConvertFrom-Json -AsHashtable
try{$identity.pid=$PID;JSON $path $identity;Reject 'result-before-exit' {Assert-SRControlledEvidence $taskControlled $taskCatalog.controlled_scenarios | Out-Null}}finally{Put $path $original}
JSON (Join-Path $taskOut 'summary.json') @{status='failed';error='later native failure'}
$null=Read-SRPrerequisites $taskRepo $taskOut
$taskChecks.Add('failed-native-parent-retains-direct-proof')
$path=Join-Path $taskOut 'l1.jsonl';$original=[IO.File]::ReadAllText($path)
try {
    Put $path ($original+"`n"+(@{Action='skip';Package=$script:taskPackages[0];Test='TestUnexpected'}|ConvertTo-Json -Compress))
    Reject 'unexpected-L1-skip' {Assert-SRL1Evidence $taskOut $taskRecord $script:taskPackages | Out-Null}
    Put $path (@{Action='pass';Package=$script:taskPackages[0]}|ConvertTo-Json -Compress)
    Reject 'zero-L1-tests' {Assert-SRL1Evidence $taskOut $taskRecord $script:taskPackages | Out-Null}
} finally {Put $path $original}
$path=Join-Path $taskOut 'terminal-extensions.tap';$original=[IO.File]::ReadAllText($path)
try {Put $path "# tests 1`n# fail 0`n# skipped 1`n";Reject 'Node-skip' {Assert-SRL1Evidence $taskOut $taskRecord $script:taskPackages | Out-Null}}finally{Put $path $original}
$unavailable=Save-SRPrerequisiteStage $taskRepo $taskOut 'controlled' $taskStart $taskControlledRecord @() $false
if($unavailable.status -ne 'unavailable'){throw 'Custom catalog acquired default prerequisite eligibility'}
Put $taskManifestPath $taskManifest
# Final coverage must not resurrect failed qualification, or lose repeat identity.
$execution=@{entry=@{level='L2';id='smoke'};case=@{id='default'};repeat_index=1}
$records=@(@{level='L2';scenario='smoke';case_id='default';repeat_index=1;status='passed'})
foreach($status in @('failed','passed')) {
    $summary=@{status=$status;baseline=@{l2_verified=$true};levels=@{L2=@{complete_level_passed=$true}}}
    $executions=@($execution)
    if($status -eq 'passed'){$other=$execution.Clone();$other.repeat_index=2;$executions+=@($other)}
    Set-SRQualificationCoverage $summary $executions $records
    if($summary.levels.L2.complete_level_passed -or $summary.baseline.l2_verified){throw 'Final qualification gate was resurrected or omitted a repeat'}
    $taskChecks.Add('qualification-'+$status)
}
# Reuse argument failures happen before prerequisites, fixture creation or model use.
$shell=(Get-Process -Id $PID).Path
# A real child exposes malformed JSON while alive, then publishes the complete
# result. The wrapper must wait for exit instead of racing TestMain's write.
$waitDirectory=Join-Path $taskRoot 'result-publication'
$null=New-Item -ItemType Directory -Path $waitDirectory
$writer=Join-Path $taskRoot 'result-writer.ps1'
Put $writer @'
param([string]$Result)
[IO.File]::WriteAllText($Result,'{')
Start-Sleep -Milliseconds 600
[IO.File]::WriteAllText($Result,'{"exit_code":0,"scenario_count":2}')
Start-Sleep -Milliseconds 300
'@
$start=[Diagnostics.ProcessStartInfo]::new($shell);$start.UseShellExecute=$false;$start.CreateNoWindow=$true
foreach($argument in @('-NoProfile','-File',$writer,'-Result',(Join-Path $waitDirectory 'result.json'))){$start.ArgumentList.Add($argument)}
$writerProcess=[Diagnostics.Process]::Start($start)
try {
    JSON (Join-Path $waitDirectory 'source-identity.json') @{pid=$writerProcess.Id;process_started_at=$writerProcess.StartTime.ToUniversalTime().ToString('o')}
    $result=Wait-SRControlledResult $waitDirectory ([DateTime]::UtcNow.AddSeconds(15))
    if($result.exit_code -ne 0 -or $result.scenario_count -ne 2 -or -not $writerProcess.HasExited){throw 'Result was consumed before harness completion'}
    $taskChecks.Add('real-result-publication-after-exit')
}finally{$writerProcess.WaitForExit(15000)|Out-Null;$writerProcess.Dispose()}
JSON (Join-Path $waitDirectory 'source-identity.json') @{pid=$PID}
Reject 'harness-completion-deadline' {Wait-SRControlledResult $waitDirectory ([DateTime]::UtcNow.AddSeconds(-1)) | Out-Null}
# Legacy workers omit both new fields; partial or stale identities must fail
# before the L2 Git boundary, fixture preparation, or native process startup.
foreach($variant in @('partial','wrong-hash','wrong-level')) {
    $recordPath=Join-Path $taskRoot ($variant+'-case.json')
    $job=@{entry=@{id='local-codex.smoke';level='L2'};record=@{status='queued';started_at=$null;finished_at=$null};result_path=$recordPath;native_driver=$taskRunner}
    if($variant -ne 'partial'){$job['native_driver_sha256']=('0'*64)}
    if($variant -eq 'wrong-level'){$job.entry.level='L3';$job.native_driver_sha256=Get-SRFileDigest $taskRunner}
    $request=Join-Path $taskRoot ($variant+'-request.json');JSON $request $job
    & $shell -NoProfile -File (Join-Path $Repository 'scripts/run-test-case.ps1') -Request $request *> (Join-Path $taskRoot ($variant+'-worker.log'))
    $code=$LASTEXITCODE;$result=Get-Content -LiteralPath $recordPath -Raw | ConvertFrom-Json
    if($code -ne 1 -or $result.status -ne 'failed' -or $result.error -notmatch 'native driver' -or $result.PSObject.Properties['git_boundary']){throw 'Unverified native driver reached fixture effects'}
    $taskChecks.Add('driver-'+$variant)
}
foreach($level in @('L1','Controlled','L3','L4')) {
    $output=Join-Path $taskRoot ('invalid-'+$level)
    & $shell -NoProfile -File (Join-Path $Repository 'scripts/test.ps1') -Level $level -PrerequisitesFrom $taskOut -OutDir $output *> (Join-Path $taskRoot ($level+'.log'))
    $summary=Get-Content -LiteralPath (Join-Path $output 'summary.json') -Raw | ConvertFrom-Json
    if($LASTEXITCODE -ne 2 -or $summary.status -ne 'failed' -or $summary.results.Count -ne 0 -or $summary.error -notmatch 'only supports -Level L2' -or (Test-Path (Join-Path $output 'framework'))){throw 'Unsupported reuse level reached execution'}
    $taskChecks.Add('reject-level-'+$level)
}
# Exercise the actual independent entry dispatcher with process-local stage
# doubles; no production switches, model process or qualifying proof is created.
$entryText=Get-Content -LiteralPath (Join-Path $Repository 'scripts/test-prerequisites.ps1') -Raw
$entryText=$entryText.Replace(". (Join-Path `$PSScriptRoot 'test-prerequisite-contract.ps1')",'')
$entryText=$entryText.Replace("`$taskRoot=[IO.Path]::GetFullPath((Join-Path `$PSScriptRoot '..'))",("`$taskRoot='"+$taskRepo.Replace("'","''")+"'"))
$entryCopy=Join-Path $taskRoot 'isolated-entry.ps1';Put $entryCopy $entryText
$dispatcher=Join-Path $taskRoot 'dispatch.ps1'
Put $dispatcher @'
param([string]$Library,[string]$Entry,[string]$Stage,[string]$Output)
$ErrorActionPreference='Stop'
. $Library
function Mark($Output,$Name){[IO.File]::AppendAllText((Join-Path $Output 'calls.txt'),$Name+"`n")}
function Invoke-SRL1Core($Root,$Output,$Catalog){Mark $Output 'L1';@{passed=$true;stage=@{name='l1-core';kind='test-double'}}}
function Invoke-SRNativePreflight($Root,$Output){Mark $Output 'NativePreflight';@{compile_exit=0;zero_match_guard=$true;kind='test-double'}}
function Invoke-SRControlledStage($Root,$Output,$Catalog){Mark $Output 'Controlled';@{passed=$true;stage=@{name='controlled';kind='test-double'}}}
& $Entry -Stage $Stage -OutDir $Output
'@
foreach($stage in @('L1','Controlled','All')) {
    $output=Join-Path $taskRoot ('dispatch-'+$stage)
    & $shell -NoProfile -File $dispatcher -Library (Join-Path $Repository 'scripts/test-prerequisite-contract.ps1') -Entry $entryCopy -Stage $stage -Output $output *> (Join-Path $taskRoot ('dispatch-'+$stage+'.log'))
    $code=$LASTEXITCODE;$report=Get-Content -LiteralPath (Join-Path $output 'stages.json') -Raw | ConvertFrom-Json
    $expected=switch($stage){'L1'{'L1,NativePreflight'};'Controlled'{'Controlled'};'All'{'L1,NativePreflight,Controlled'}}
    if($code -ne 0 -or $report.status -ne 'passed' -or ((Get-Content (Join-Path $output 'calls.txt')) -join ',') -cne $expected -or (Test-Path (Join-Path $output 'prerequisites.json'))){throw 'Independent stage entry dispatched incorrect stages'}
    $taskChecks.Add('independent-entry-'+$stage)
}
$hashScript=Join-Path $taskRoot 'hash51.ps1'
Put $hashScript "param([string]`$Library,[string]`$Root)`n. `$Library`n`$files=@(Get-SRPrerequisiteFiles `$Root)`nGet-SRTextDigest ((`$files|ForEach-Object{`$_.path+':'+`$_.sha256}) -join [char]10)"
$legacy=& powershell.exe -NoProfile -ExecutionPolicy Bypass -File $hashScript -Library (Join-Path $Repository 'scripts/test-prerequisite-contract.ps1') -Root $taskRepo
if($LASTEXITCODE -ne 0 -or ($legacy | Out-String).Trim() -cne (Get-SRPrerequisiteSnapshot $taskRepo).source_digest){throw 'Prerequisite file digest differs under Windows PowerShell 5.1'}
$taskChecks.Add('windows-powershell-digest')
JSON (Join-Path $taskRoot 'verified.json') @{checks=$taskChecks.ToArray();native_qualification=$false}
Write-Host "Prerequisite contract checks passed: $($taskChecks.Count)"
