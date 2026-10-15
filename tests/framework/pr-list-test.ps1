param([Parameter(Mandatory=$true)][string]$Source)
$ErrorActionPreference = 'Stop'
. $Source
$taskEmpty = @(Convert-SRPRList -JSON '[]')
if ($taskEmpty.Count -ne 0) { throw 'Empty PR array was treated as an existing PR' }
$taskOne = @(Convert-SRPRList -JSON '[{"url":"https://github.com/erranliu/SteerlineTest/pull/1","state":"OPEN"}]')
if ($taskOne.Count -ne 1 -or $taskOne[0].state -ne 'OPEN') { throw 'Existing PR was lost' }
$taskTwo = @(Convert-SRPRList -JSON '[{"state":"OPEN"},{"state":"CLOSED"}]')
if ($taskTwo.Count -ne 2) { throw 'Conflicting PRs were hidden' }
Write-Output 'PR list zero/one/many cases passed under Windows PowerShell'
