param([Parameter(Mandatory=$true)][string]$URL)
$ErrorActionPreference = 'Stop'
if ($URL -notmatch '^https://github.com/erranliu/SteerlineTest/pull/([1-9][0-9]*)$') { throw 'Foreign PR URL' }
$taskRaw = & gh api ('repos/erranliu/SteerlineTest/pulls/' + $Matches[1])
if ($LASTEXITCODE -ne 0) { throw 'GitHub PR query failed' }
$taskPR = $taskRaw | ConvertFrom-Json
$taskState = if ($taskPR.merged) { 'merged' } elseif ($taskPR.state -eq 'open') { 'open' } else { 'closed_unmerged' }
[ordered]@{url=$taskPR.html_url; repository=$taskPR.base.repo.full_name; base_branch=$taskPR.base.ref; head_repository=$taskPR.head.repo.full_name; head_branch=$taskPR.head.ref; head_revision=$taskPR.head.sha; state=$taskState; merged=[bool]$taskPR.merged; merged_at=$taskPR.merged_at; merge_commit=$taskPR.merge_commit_sha} | ConvertTo-Json -Compress
