param([string]$Root=(Get-Location).Path)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'control.ps1')
$taskControl=Get-SRControl $Root
Write-SRAudit $taskControl 'checker' @{}
$taskBody=[IO.File]::ReadAllText((Join-Path $Root 'evidence/native-101.md'),[Text.UTF8Encoding]::new($false,$true)).Replace("`r`n","`n")
$taskLines=$taskBody.Split([char]10)
if($taskLines.Count -ne 6 -or $taskLines[0] -ne '# Evidence sample 101' -or $taskLines[1] -ne '- sample_id: 101' -or $taskLines[3] -ne '- 模拟任务' -or $taskLines[4] -ne '- 未修改业务代码' -or $taskLines[5] -ne ''){throw 'Artifact differs from frozen Spec'}
$taskTime=[DateTimeOffset]::ParseExact($taskLines[2].Substring(16),[string[]]@('yyyy-MM-ddTHH:mm:ssZ','yyyy-MM-ddTHH:mm:ss.FFFFFFFZ'),[Globalization.CultureInfo]::InvariantCulture,[Globalization.DateTimeStyles]::AssumeUniversal)
if([Math]::Abs(([DateTimeOffset]::UtcNow-$taskTime).TotalMinutes) -gt 60){throw 'Artifact timestamp is stale'}
Wait-SRGate $taskControl 'implementation' $Root
@{passed=$true;artifact='evidence/native-101.md';generated_at=$taskTime.ToString('o')}|ConvertTo-Json -Compress
