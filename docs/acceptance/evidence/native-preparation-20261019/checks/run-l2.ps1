$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$taskRoot=Join-Path $PWD '.evidence/native-preparation-fix-20261019'
$taskOriginal='361589fdd890ed0bb1edb5cbc4be8cfea50e486a'
$taskMain=Join-Path $taskRoot 'live-qualified/summary.json'
do {
  if(Test-Path $taskMain){$taskPrerequisite=Get-Content $taskMain -Raw|ConvertFrom-Json}else{$taskPrerequisite=$null}
  if($null -eq $taskPrerequisite -or $taskPrerequisite.status -eq 'running'){Start-Sleep -Seconds 10}
}while($null -eq $taskPrerequisite -or $taskPrerequisite.status -eq 'running')
foreach($taskLevel in @('L1','Controlled')) {
  $taskPass=@($taskPrerequisite.results|Where-Object {$_.level -eq $taskLevel -and $_.exit_code -eq 0})
  if($taskPass.Count -ne 1){throw "Missing exact final-source prerequisite: $taskLevel"}
}
if($taskPrerequisite.source_commit -ne $taskOriginal -or (& git rev-parse HEAD).Trim() -ne $taskOriginal -or -not [string]::IsNullOrWhiteSpace((& git status --porcelain|Out-String))){throw 'Source changed after prerequisite; no L2 model starts'}
. ./scripts/live-catalog.ps1
. ./scripts/test-scheduler.ps1
$taskConfig=Join-Path $taskRoot 'live-config-qualified.json'
$taskConfigObject=Get-Content $taskConfig -Raw|ConvertFrom-Json
$taskOut=Join-Path $taskRoot 'live-l2'
$null=New-Item -ItemType Directory -Path $taskOut
& go test -tags=live ./internal/execution/adapters/codex -run '^TestLinkedGitNativeCommit$' -count=1 -timeout=3m -v -args -sandbox-auth-native $taskConfigObject.codex_executable *> (Join-Path $taskOut 'l2-git-boundary.log')
if($LASTEXITCODE -ne 0){throw 'Native Git prerequisite failed'}
$taskCatalogPath=(Resolve-Path tests/catalog.json).Path
$taskCatalog=Get-Content $taskCatalogPath -Raw|ConvertFrom-Json
$taskCases=@()
foreach($taskID in @('local-codex.smoke','local-codex.result-acquisition')){
  $taskEntry=@($taskCatalog.scenarios|Where-Object {$_.id -eq $taskID})
  if($taskEntry.Count -ne 1){throw 'Missing exact L2 scenario'}
  foreach($taskCase in @(Get-SRLiveCases $taskEntry[0])){$taskCases+=@{entry=$taskEntry[0];case=$taskCase;repeat_index=1;excluded=$false}}
}
$taskSummary=@{source_commit=$taskOriginal;source_dirty=$false;prerequisite='../live-qualified/summary.json';planned_case_count=$taskCases.Count;scope='two-affected-l2';max_parallel=2;status='running';results=@()}
Invoke-SRCaseBatch -Executions $taskCases -Output $taskOut -Config $taskConfig -Catalog $taskCatalogPath -FixtureParent 'C:\srt' -MaxParallel 2 -Checkpoint {
  param($Records)
  $taskSummary.results=@($Records)
  [IO.File]::WriteAllText((Join-Path $taskOut 'summary.json'),($taskSummary|ConvertTo-Json -Depth 40),[Text.UTF8Encoding]::new($false))
}
$taskSummary.status=if(@($taskSummary.results|Where-Object {$_.status -ne 'passed'}).Count){'failed'}else{'passed'}
$taskSummary.finished_at=[DateTime]::UtcNow.ToString('o')
[IO.File]::WriteAllText((Join-Path $taskOut 'summary.json'),($taskSummary|ConvertTo-Json -Depth 40),[Text.UTF8Encoding]::new($false))
if($taskSummary.status -ne 'passed'){exit 1}
