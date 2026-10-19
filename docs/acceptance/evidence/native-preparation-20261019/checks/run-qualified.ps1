$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$taskRoot=Join-Path $PWD '.evidence/native-preparation-fix-20261019'
$taskConfig=Get-Content .evidence/rebase-audit-20261018/live-config-final.json -Raw | ConvertFrom-Json
foreach($taskMode in @('host-broker','invalid-canary')) {
  $taskFlags=@('-sandbox-auth-native',$taskConfig.codex_executable)
  if($taskMode -eq 'host-broker'){$taskFlags+='-sandbox-auth-host-broker'}
  & go test ./internal/execution/adapters/codex -run '^TestAuthenticationNativeToolBoundary$' -count=1 -timeout=3m -v -args @taskFlags *> (Join-Path $taskRoot ('native-qualified-'+$taskMode+'.log'))
  if($LASTEXITCODE -ne 0){throw "Native prerequisite failed: $taskMode"}
}
$taskHelper=Join-Path $taskRoot 'steerrunner-agent-qualified.exe'
& go build -trimpath -o $taskHelper ./cmd/steerrunner-agent
if($LASTEXITCODE -ne 0){throw 'Helper build failed'}
$taskMeta=(& $taskHelper --terminal-metadata | ConvertFrom-Json)
if($LASTEXITCODE -ne 0){throw 'Helper metadata failed'}
$taskSHA=(Get-FileHash -LiteralPath $taskHelper -Algorithm SHA256).Hash
$taskBuild=[ordered]@{source_commit=(& git rev-parse HEAD).Trim();source_dirty=-not [string]::IsNullOrWhiteSpace((& git status --porcelain|Out-String));sha256=$taskSHA;metadata=$taskMeta}
$taskBuild | ConvertTo-Json -Depth 10 | Set-Content -Encoding utf8NoBOM (Join-Path $taskRoot 'helper-build-qualified.json')
$taskConfig.orca_local_runtime_executable=$taskHelper
$taskConfig | ConvertTo-Json -Depth 20 | Set-Content -Encoding utf8NoBOM (Join-Path $taskRoot 'live-config-qualified.json')
& ./scripts/test.ps1 -Level L3 -Scenario @('orca-local.codex','orca-local.pi','orca-local.dsh') -Config (Join-Path $taskRoot 'live-config-qualified.json') -OutDir (Join-Path $taskRoot 'live-qualified') -MaxParallel 2
exit $LASTEXITCODE
