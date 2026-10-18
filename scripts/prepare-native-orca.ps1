param(
    [Parameter(Mandatory=$true)][string]$Manifest,
    [Parameter(Mandatory=$true)][string]$OrcaExe,
    [string]$Environment = '',
    [ValidateSet('local','orca_environment')][string]$Target = 'orca_environment',
    [string]$RepositoryPath = ''
)
$ErrorActionPreference = 'Stop'
if (($Target -eq 'orca_environment' -and -not $Environment) -or ($Target -eq 'local' -and $Environment)) { throw 'Use an explicit local target or a fixed remote environment' }
$taskInput = Get-Content -LiteralPath $Manifest -Raw | ConvertFrom-Json
$taskRoot = [IO.Path]::GetFullPath($taskInput.root).TrimEnd('\')
if ($taskInput.schema -notin @('steerrunner-test/v1','steerrunner-test/v2') -or $taskInput.base_branch -cne ('codex/srtest/' + $taskInput.id + '/base') -or $taskInput.id -notmatch '^[a-f0-9]{12}$' -or $taskInput.base_revision -notmatch '^[0-9a-f]{40}$' -or $taskInput.publication.repository -ne 'erranliu/SteerlineTest') { throw 'Dedicated Runner fixture provenance is incomplete' }
if (-not [IO.Path]::IsPathRooted($OrcaExe) -or -not (Test-Path -LiteralPath $OrcaExe -PathType Leaf)) { throw 'Use the selected absolute Orca executable' }
$taskEvidence = Join-Path $taskRoot ('evidence/orca-preparation-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $taskEvidence | Out-Null
$taskCallCount = 0
function Invoke-TaskOrca([string[]]$Arguments) {
    $taskRoute = @()
    if ($Target -eq 'orca_environment') { $taskRoute=@('--environment',$Environment) }
    $taskRaw = & $OrcaExe @Arguments @taskRoute --json
    $taskCode = $LASTEXITCODE
    $taskValue = $taskRaw | ConvertFrom-Json
    $script:taskCallCount++
    $taskRecord = @{at=[DateTime]::UtcNow.ToString('o');arguments=$Arguments;exit_code=$taskCode;response=$taskValue}
    [IO.File]::WriteAllText((Join-Path $taskEvidence (('call-{0:D3}.json' -f $script:taskCallCount))),($taskRecord|ConvertTo-Json -Depth 20),[Text.UTF8Encoding]::new($false))
    if ($taskCode -ne 0 -or -not $taskValue.ok) { throw "Orca preparation call failed: $($taskValue.error.message)" }
    return $taskValue
}
$taskListing = Invoke-TaskOrca @('repo','list')
$taskRepos = @($taskListing.result.repos | Where-Object { $_.kind -eq 'git' -and [IO.Path]::GetFileName($_.path.TrimEnd('/','\')) -eq 'SteerlineTest' -and $_.gitRemoteIdentity.remoteUrl -eq 'https://github.com/erranliu/SteerlineTest.git' })
if ($RepositoryPath) {
    if (-not [IO.Path]::IsPathFullyQualified($RepositoryPath)) { throw 'Registered repository selection must be an absolute path' }
    $taskRepos = @($taskRepos | Where-Object { $_.path.Replace('\','/').TrimEnd('/') -ieq $RepositoryPath.Replace('\','/').TrimEnd('/') })
}
if ($taskRepos.Count -ne 1 -or -not $taskListing._meta.runtimeId) { throw 'The selected host must have one unambiguous registered SteerlineTest Git repository' }
$taskRepo = $taskRepos[0]
$taskWorktrees = Invoke-TaskOrca @('worktree','list','--repo',('id:'+$taskRepo.id))
$taskMains = @($taskWorktrees.result.worktrees | Where-Object { $_.isMainWorktree -and ($_.path.Replace('\','/').TrimEnd('/') -eq $taskRepo.path.Replace('\','/').TrimEnd('/')) })
if ($taskMains.Count -ne 1 -or $taskWorktrees._meta.runtimeId -ne $taskListing._meta.runtimeId) { throw 'Original registered main worktree identity is unavailable' }
$taskRepoQuoted = "'" + $taskRepo.path.Replace("'","''") + "'"
$taskMarker = 'SR_BASE_' + [Guid]::NewGuid().ToString('N') + ':'
# This is an isolated fixture preparation terminal, never a business agent.
# Split the marker in the command so command echo cannot prove completion.
$taskCommand = '& git -C ' + $taskRepoQuoted + ' fetch --no-tags origin +refs/heads/' + $taskInput.base_branch + ':refs/remotes/origin/' + $taskInput.base_branch + '; if ($LASTEXITCODE -eq 0) { & git -C ' + $taskRepoQuoted + " cat-file -e '" + $taskInput.base_revision + "^{commit}'; if (`$LASTEXITCODE -eq 0) { Write-Output ('SR_' + '" + $taskMarker.Substring(3) + "' + '" + $taskInput.base_revision + "') } }"
$taskCreated = Invoke-TaskOrca @('terminal','create','--worktree',('id:'+$taskMains[0].id),'--title','SteerRunner native base preparation','--command',$taskCommand)
$taskHandle = $taskCreated.result.terminal.handle
if (-not $taskHandle -or $taskCreated._meta.runtimeId -ne $taskListing._meta.runtimeId) { throw 'Preparation terminal creation is unverified; do not recreate it' }
try {
    $taskUntil = [DateTime]::UtcNow.AddSeconds(45)
    $taskVerified = $false
    do {
        $taskShown = Invoke-TaskOrca @('terminal','show','--terminal',$taskHandle)
        if ($taskShown._meta.runtimeId -ne $taskListing._meta.runtimeId -or $taskShown.result.terminal.incarnationId -ne $taskCreated.result.terminal.incarnationId) { throw 'Preparation terminal changed identity' }
        $taskVerified = ([string]$taskShown.result.terminal.preview).Contains($taskMarker + $taskInput.base_revision)
        if ($taskVerified) { break }
        Start-Sleep -Milliseconds 750
    } while ([DateTime]::UtcNow -lt $taskUntil)
    if (-not $taskVerified) { throw 'Native Git fetch/cat-file completion is unverified; preserve the preparation evidence' }
    [IO.File]::WriteAllText((Join-Path $taskEvidence 'result.json'),(@{status='base_verified';base_revision=$taskInput.base_revision;runtime_id=$taskListing._meta.runtimeId;repository_id=$taskRepo.id;terminal_handle=$taskHandle}|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
} finally {
    $taskClosed = Invoke-TaskOrca @('terminal','close','--terminal',$taskHandle)
    if ($taskClosed._meta.runtimeId -ne $taskListing._meta.runtimeId -or $taskClosed.result.close.handle -ne $taskHandle -or -not $taskClosed.result.close.ptyKilled) { throw 'Preparation terminal stop was not confirmed' }
}
[IO.File]::WriteAllText((Join-Path $taskEvidence 'result.json'),(@{status='prepared';base_revision=$taskInput.base_revision;runtime_id=$taskListing._meta.runtimeId;repository_id=$taskRepo.id;terminal_handle=$taskHandle;terminal_stopped=$true}|ConvertTo-Json),[Text.UTF8Encoding]::new($false))
Write-Host "Native base prepared through the selected Orca host; evidence $taskEvidence"
if ($Target -eq 'local') { Write-Output (@{runtime_id=$taskListing._meta.runtimeId;repository_path=$taskRepo.path} | ConvertTo-Json -Compress) }
else { Write-Output $taskListing._meta.runtimeId }
