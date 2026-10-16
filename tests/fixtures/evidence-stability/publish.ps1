$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'control.ps1')
. (Join-Path $PSScriptRoot 'pr-list.ps1')
$taskRoot=(Get-Location).Path
$taskControl=Get-SRControl $taskRoot
Write-SRAudit $taskControl 'publisher' @{}
$taskFixture=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'publication.json') -Raw | ConvertFrom-Json
$taskRemote=(& git remote get-url origin | Out-String).Trim()
$taskBranch=(& git branch --show-current | Out-String).Trim()
if($LASTEXITCODE -ne 0 -or $taskRemote -ne 'https://github.com/erranliu/SteerlineTest.git' -or -not $taskBranch -or $taskFixture.base_branch -notmatch '^codex/srtest/[a-f0-9]{12}/base$' -or $taskBranch -eq $taskFixture.base_branch){throw 'Publication identity differs from dedicated fixture'}
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'check.ps1')
if($LASTEXITCODE -ne 0){throw 'Artifact check failed'}
$taskDirty=(& git status --porcelain | Out-String).Trim()
if($LASTEXITCODE -ne 0 -or $taskDirty){throw 'Publication requires committed work'}
Write-SRAudit $taskControl 'push' @{branch=$taskBranch}
& git push origin ('HEAD:refs/heads/'+$taskBranch)
if($LASTEXITCODE -ne 0){throw 'Original push failed'}
$taskRaw=& gh pr list --repo erranliu/SteerlineTest --head $taskBranch --base $taskFixture.base_branch --state all --json url,state
if($LASTEXITCODE -ne 0){throw 'Original PR query failed; no replacement permitted'}
$taskExisting=@(Convert-SRPRList -JSON ($taskRaw -join "`n"))
if($taskExisting.Count -eq 1 -and $taskExisting[0].state -eq 'OPEN'){$taskURL=$taskExisting[0].url;Write-SRAudit $taskControl 'reuse' @{url=$taskURL}}
elseif($taskExisting.Count -ne 0){throw 'Original PR conflicts; do not recreate'}
else{
 Write-SRAudit $taskControl 'create' @{branch=$taskBranch;base=$taskFixture.base_branch}
 $taskURL=(& gh pr create --repo erranliu/SteerlineTest --base $taskFixture.base_branch --head $taskBranch --title ('SteerRunner stability '+$taskFixture.fixture_id) --body 'Real execution stability test; dedicated test base. Driver owns merge.' | Out-String).Trim()
 if($LASTEXITCODE -ne 0){throw 'PR creation outcome unknown; query original before any retry'}
}
if($taskURL -notmatch '^https://github.com/erranliu/SteerlineTest/pull/[1-9][0-9]*$'){throw 'Invalid real PR URL'}
$taskProof=& gh pr view $taskURL --json url,baseRefName,headRefName,headRefOid
if($LASTEXITCODE -ne 0){throw 'Cannot verify published PR'}
[IO.File]::WriteAllText((Join-Path $taskControl.path 'published.json'),($taskProof -join "`n"),[Text.UTF8Encoding]::new($false))
Write-SRAudit $taskControl 'published' ($taskProof|ConvertFrom-Json)
Wait-SRGate $taskControl 'publication' $taskRoot
Write-Output $taskURL
