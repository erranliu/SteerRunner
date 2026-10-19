$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$taskRoot=Join-Path $PWD '.evidence/rebase-audit-20261018'
$taskLimit=[DateTime]::UtcNow.AddMinutes(35)
while(-not (Test-Path (Join-Path $taskRoot 'full-final/summary.json'))){Start-Sleep -Seconds 2}
do {
    $taskPrerequisite=Get-Content (Join-Path $taskRoot 'full-final/summary.json') -Raw|ConvertFrom-Json
    if($taskPrerequisite.status -eq 'failed'){throw 'Current full prerequisite failed; no native call permitted'}
    if([DateTime]::UtcNow -gt $taskLimit){throw 'Prerequisite deadline expired'}
    if($taskPrerequisite.status -eq 'running'){Start-Sleep -Seconds 2}
}while($taskPrerequisite.status -eq 'running')
if($taskPrerequisite.status -ne 'passed'){throw 'Current full prerequisite did not pass'}
. ./scripts/live-catalog.ps1
. ./scripts/test-scheduler.ps1
$taskOut=Join-Path $taskRoot 'native'
$null=New-Item -ItemType Directory -Path $taskOut
# The standard L1/Controlled gate above is reused; no token-consuming case starts
# before its matching current-source prerequisite passes.
$taskConfig=Join-Path $taskRoot 'live-config.json'
$taskConfigObject=Get-Content $taskConfig -Raw|ConvertFrom-Json
& go test -tags=live ./internal/execution/adapters/codex -run '^TestLinkedGitNativeCommit$' -count=1 -timeout=3m -v -args -sandbox-auth-native $taskConfigObject.codex_executable *> (Join-Path $taskOut 'l2-git-boundary.log')
if($LASTEXITCODE -ne 0){throw 'L2 Git prerequisite failed'}
$taskCatalogPath=(Resolve-Path tests/catalog.json).Path
$taskCatalog=Get-Content $taskCatalogPath -Raw|ConvertFrom-Json
$taskSelected=@('local-codex.smoke','local-codex.result-acquisition','local-codex.audited','orca-local.codex','orca-local.pi','orca-local.dsh','orca-local.mixed')
$taskCases=@()
foreach($taskID in $taskSelected){
    $taskEntry=@($taskCatalog.scenarios|Where-Object {$_.id -eq $taskID})
    if($taskEntry.Count -ne 1){throw 'Missing exact native scenario'}
    foreach($taskCase in @(Get-SRLiveCases $taskEntry[0])){$taskCases+=@{entry=$taskEntry[0];case=$taskCase;repeat_index=1;excluded=$false}}
}
$taskSummary=@{source_commit=(& git rev-parse HEAD).Trim();prerequisite='../full-final/summary.json';planned_case_count=$taskCases.Count;scope='affected-native-subset';max_parallel=2;status='running';results=@()}
Invoke-SRCaseBatch -Executions $taskCases -Output $taskOut -Config $taskConfig -Catalog $taskCatalogPath -FixtureParent 'C:\srt' -MaxParallel 2 -Checkpoint {
    param($Records)
    $taskSummary.results=@($Records)
    [IO.File]::WriteAllText((Join-Path $taskOut 'summary.json'),($taskSummary|ConvertTo-Json -Depth 40),[Text.UTF8Encoding]::new($false))
}
$taskSummary.status=if(@($taskSummary.results|Where-Object {$_.status -ne 'passed'}).Count){'failed'}else{'passed'}
$taskSummary.finished_at=[DateTime]::UtcNow.ToString('o')
[IO.File]::WriteAllText((Join-Path $taskOut 'summary.json'),($taskSummary|ConvertTo-Json -Depth 40),[Text.UTF8Encoding]::new($false))
if($taskSummary.status -ne 'passed'){exit 1}
