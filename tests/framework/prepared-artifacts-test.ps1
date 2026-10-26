param([Parameter(Mandatory=$true)][string]$Repository,[Parameter(Mandatory=$true)][string]$Evidence)
$ErrorActionPreference='Stop'
$taskRoot=Join-Path $Evidence 'prepared-artifacts'
$taskSource=Join-Path $taskRoot 'source'
$null=New-Item -ItemType Directory -Path $taskSource
$taskIdentity=@{source_commit='guard-fixture';source_dirty=$true}
foreach($taskArtifact in @(@{name='steerrunner.exe';field='runner_sha256'},@{name='integration.test.exe';field='fixture_sha256'},@{name='native-fixture.exe';field='native_fixture_sha256'})) {
    $taskPath=Join-Path $taskSource $taskArtifact.name
    # Deliberately not executable: every negative case must fail before launch.
    [IO.File]::WriteAllText($taskPath,$taskArtifact.name)
    $taskIdentity[$taskArtifact.field]=(Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash
}
$taskVerified=@()
foreach($taskCase in @('runner_sha256','fixture_sha256','native_fixture_sha256','missing-native-hash','failed-prerequisite','skipped-prerequisite','explicit-runner-with-prepared')) {
    $taskChanged=$taskIdentity.Clone()
    $taskExit=0
    $taskLog='--- PASS: prerequisite'
    $taskError='identity changed or is missing'
    $taskArguments=@{}
    switch($taskCase) {
        'missing-native-hash'{$taskChanged.Remove('native_fixture_sha256')}
        'failed-prerequisite'{$taskExit=1;$taskError='passed prerequisite without skips'}
        'skipped-prerequisite'{$taskLog='--- SKIP: prerequisite';$taskError='passed prerequisite without skips'}
        'explicit-runner-with-prepared'{$taskArguments.RunnerExe=Join-Path $taskSource 'steerrunner.exe';$taskError='cannot be combined with RunnerExe'}
        default{$taskChanged[$taskCase]='0'*64}
    }
    [IO.File]::WriteAllText((Join-Path $taskSource 'source-identity.json'),($taskChanged|ConvertTo-Json))
    [IO.File]::WriteAllText((Join-Path $taskSource 'result.json'),(@{exit_code=$taskExit;scenario_count=1}|ConvertTo-Json))
    [IO.File]::WriteAllText((Join-Path $taskSource 'integration.log'),$taskLog)
    $taskOutput=Join-Path $taskRoot $taskCase
    $taskRejected=$false
    try { & (Join-Path $Repository 'scripts/test-foundation.ps1') -OutDir $taskOutput -PreparedArtifactsDirectory $taskSource @taskArguments | Out-Null }
    catch { $taskRejected=$_.Exception.Message -match $taskError }
    if(-not $taskRejected -or (Test-Path -LiteralPath (Join-Path $taskOutput 'source-identity.json'))){throw "Unverified shared artifacts reached execution: $taskCase"}
    $taskVerified+=$taskCase
}
[IO.File]::WriteAllText((Join-Path $taskRoot 'verified.json'),(@{rejected=$taskVerified;execution_started=$false}|ConvertTo-Json))
Write-Output 'Prepared artifact changes, missing identities, failed prerequisites and skips rejected before launch'
