$ErrorActionPreference='Stop'
function Get-SRControl([string]$Root) {
    $taskConfig=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'stability.json') -Raw | ConvertFrom-Json
    $taskAdmin=(& git -C $Root rev-parse --absolute-git-dir | Out-String).Trim()
    if($LASTEXITCODE -ne 0 -or -not [IO.Path]::IsPathRooted($taskAdmin)){throw 'Test control requires the exact Git admin directory'}
    $taskControl=Join-Path $Root ('.sr-stability-'+$taskConfig.fixture_id)
    New-Item -ItemType Directory -Force -Path $taskControl | Out-Null
    return @{config=$taskConfig;path=$taskControl}
}
function Write-SRAudit($Control,[string]$Kind,$Data) {
    $taskEntry=@{at=[DateTime]::UtcNow.ToString('o');kind=$Kind;data=$Data;pid=$PID}
    [IO.File]::AppendAllText((Join-Path $Control.path 'audit.jsonl'),(($taskEntry|ConvertTo-Json -Depth 8 -Compress)+"`n"),[Text.UTF8Encoding]::new($false))
}
function Wait-SRGate($Control,[string]$Stage,[string]$Root) {
    $taskBranch=(& git -C $Root branch --show-current | Out-String).Trim()
    if($LASTEXITCODE -ne 0){throw 'Cannot identify checker branch'}
    if(-not $taskBranch -or $Control.config.gate -ne $Stage){return}
    $taskRelease=Join-Path $Control.path ($Stage+'.released')
    if(Test-Path -LiteralPath $taskRelease){return}
    [IO.File]::WriteAllText((Join-Path $Control.path ($Stage+'.entered')),([DateTime]::UtcNow.ToString('o')))
    Write-SRAudit $Control 'gate-entered' @{stage=$Stage;branch=$taskBranch}
    $taskUntil=[DateTime]::UtcNow.AddSeconds($Control.config.gate_timeout_seconds)
    while(-not (Test-Path -LiteralPath $taskRelease)){
        if([DateTime]::UtcNow -ge $taskUntil){Write-SRAudit $Control 'gate-timeout' @{stage=$Stage};throw 'Bounded test gate timed out; source failed'}
        Start-Sleep -Milliseconds 500
    }
    Write-SRAudit $Control 'gate-released' @{stage=$Stage}
}
