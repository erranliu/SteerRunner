$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$taskRoot=$PSScriptRoot
$taskConfig=Get-Content (Join-Path $taskRoot 'live-config-r2.json') -Raw|ConvertFrom-Json
$taskIdentity=Get-Content (Join-Path $taskRoot 'final-latest/controlled/steerrunner.exe.build.json') -Raw|ConvertFrom-Json
$taskEquivalence=Get-Content (Join-Path $taskRoot 'code-equivalence.json') -Raw|ConvertFrom-Json
$taskTestSource=(&git rev-parse HEAD).Trim()
if($taskIdentity.source_commit -ne $taskEquivalence.test_execution_source_commit -or $taskIdentity.source_dirty -or $taskTestSource -ne $taskEquivalence.capture_test_source_commit -or -not $taskEquivalence.production_and_controlled_sources_identical){throw 'Frozen probe EXE or test source mismatch'}
$taskRunner=Join-Path $taskRoot 'final-latest/controlled/steerrunner.exe'
$taskLocalRepo='C:/Projects/SteerlineTest'
$taskCases=@(
 @{name='local';repo='faae3ad4-da24-4d6d-9698-d8931b893789';environment='';helper=(Join-Path $taskRoot 'steerrunner-agent.exe');native=$taskConfig.codex_executable;base=(&git -C $taskLocalRepo rev-parse HEAD).Trim()},
 @{name='remote';repo='903fb315-7d5e-4246-8913-816e1d3bdf03';environment=$taskConfig.orca_environment;helper=$taskConfig.remote_runtime_executable;native=$taskConfig.remote_agent;base='cebdd5ab552150c49743e40593c836689ecf789b'}
)
$taskResults=@()
try {
 foreach($taskCase in $taskCases){
  $taskDirectory='C:/SteerlineDebug/sr-rebase-control-20261019-'+$taskCase.name+'-'+$taskTestSource.Substring(0,8)
  if(Test-Path -LiteralPath $taskDirectory){throw 'Fresh native stop probe directory required'}
  $taskParameters=@{ROOT=$taskDirectory;CLI=$taskConfig.orca_executable;RUNNER=$taskRunner;REPO=$taskCase.repo;BASE=$taskCase.base;REPOSITORY=$taskLocalRepo;ENVIRONMENT=$taskCase.environment;HELPER=$taskCase.helper;HELPER_SHA256=$taskConfig.remote_runtime_sha256;NATIVE=$taskCase.native;MODEL=$taskConfig.remote_model}
  foreach($taskPair in $taskParameters.GetEnumerator()){[Environment]::SetEnvironmentVariable('STEERRUNNER_ORCA_STOP_PROBE_'+$taskPair.Key,$taskPair.Value)}
  $taskBegin=[DateTime]::UtcNow.ToString('o')
  & go test ./internal/host -run '^TestOrcaStoppedNativeProbe$' -count=1 -timeout=8m -v > (Join-Path $taskRoot ('probe-'+$taskCase.name+'.log')) 2>&1
  $taskExit=$LASTEXITCODE
  $taskResults+=@{name=$taskCase.name;exit_code=$taskExit;started_at=$taskBegin;finished_at=[DateTime]::UtcNow.ToString('o');parameters=$taskParameters;source_commit=$taskIdentity.source_commit;test_source_commit=$taskTestSource;evidence=$taskDirectory}
  $taskResults|ConvertTo-Json -Depth 14|Set-Content -Encoding utf8NoBOM (Join-Path $taskRoot 'probes.json')
  if($taskExit -ne 0){throw ('Native stop probe failed: '+$taskCase.name)}
 }
} finally {
 foreach($taskKey in @('ROOT','CLI','RUNNER','REPO','BASE','REPOSITORY','ENVIRONMENT','HELPER','HELPER_SHA256','NATIVE','MODEL')){[Environment]::SetEnvironmentVariable('STEERRUNNER_ORCA_STOP_PROBE_'+$taskKey,$null)}
}
