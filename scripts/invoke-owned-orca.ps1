param([Parameter(Mandatory)][string]$RequestFile)
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
. (Join-Path $PSScriptRoot 'orca-cli-lock.ps1')
$taskRequest = Get-Content -LiteralPath $RequestFile -Raw | ConvertFrom-Json
$taskReply = Invoke-SRLockedOrca -Executable $taskRequest.executable -Arguments @($taskRequest.arguments)
[Console]::Out.Write(($taskReply.output -join "`n"))
exit $taskReply.exit_code
