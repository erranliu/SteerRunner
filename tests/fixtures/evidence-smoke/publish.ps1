$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'pr-list.ps1')
$taskFixture = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'publication.json') -Raw | ConvertFrom-Json
$taskRemote = (& git remote get-url origin | Out-String).Trim()
$taskBranch = (& git branch --show-current | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $taskRemote -ne 'https://github.com/erranliu/SteerlineTest.git' -or -not $taskBranch -or $taskFixture.base_branch -notmatch '^codex/srtest/[a-f0-9]{12}/base$' -or $taskBranch -eq $taskFixture.base_branch) { throw 'Publication identity differs from dedicated test fixture' }
& powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'check.ps1')
if ($LASTEXITCODE -ne 0) { throw 'Artifact check failed' }
$taskDirty = (& git status --porcelain | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $taskDirty) { throw 'Publication requires committed work only' }
& git push origin ('HEAD:refs/heads/' + $taskBranch)
if ($LASTEXITCODE -ne 0) { throw 'Original branch push failed' }
$taskRaw = & gh pr list --repo erranliu/SteerlineTest --head $taskBranch --base $taskFixture.base_branch --state all --json url,state
if ($LASTEXITCODE -ne 0) { throw 'Original PR query failed; do not create a replacement' }
$taskExisting = @(Convert-SRPRList -JSON ($taskRaw -join "`n"))
if ($taskExisting.Count -eq 1 -and $taskExisting[0].state -eq 'OPEN') { Write-Output $taskExisting[0].url; exit 0 }
if ($taskExisting.Count -ne 0) { throw 'Original PR outcome conflicts; do not recreate' }
& gh pr create --repo erranliu/SteerlineTest --base $taskFixture.base_branch --head $taskBranch --title ('SteerRunner smoke ' + $taskFixture.fixture_id) --body 'Real SteerRunner smoke. Dedicated test base; merge and cleanup are owned by the test driver.'
if ($LASTEXITCODE -ne 0) { throw 'PR creation outcome unavailable; query the original PR before retrying' }
