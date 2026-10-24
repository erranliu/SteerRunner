$ErrorActionPreference='Stop'
$progressPath=$env:STEERLINE_PROGRESS_FILE
if(-not $progressPath -or -not [IO.Path]::IsPathRooted($progressPath) -or -not [IO.File]::Exists($progressPath)){throw 'Injected progress file unavailable'}
$progressRoot=Split-Path -Parent $progressPath
$progressRun=Split-Path -Leaf $progressRoot
$progressGate=Join-Path (Get-Location).ProviderPath ('.sr-runtime-stage/progress-control/'+$progressRun)
function Wait-ProgressGate([int]$Step){
 $until=[DateTime]::UtcNow.AddMinutes(8)
 while(-not (Test-Path -LiteralPath (Join-Path $progressGate ([string]$Step)))){if([DateTime]::UtcNow -gt $until){throw 'Progress gate expired'};Start-Sleep -Milliseconds 150}
}
function Publish-Progress([string]$Word){
 $lock=[IO.File]::Open(($progressPath+'.lock'),[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
 try {
  # Any business merge belongs here, outside Runner. This fixture publishes
  # arbitrary binary data plus the actual path obtained from tool environment.
  $old=[IO.File]::ReadAllBytes($progressPath)
  $bytes=[Text.Encoding]::UTF8.GetBytes($Word+'|'+$progressPath)+[byte[]]@(0,255,17)
  $temp=$progressPath+'.'+[Guid]::NewGuid().ToString('N')+'.tmp'
  [IO.File]::WriteAllBytes($temp,$bytes)
  [IO.File]::Replace($temp,$progressPath,($progressPath+'.backup'))
  [IO.File]::Delete(($progressPath+'.backup'))
 } finally {$lock.Dispose()}
}
Publish-Progress 'violet'
Wait-ProgressGate 1
Publish-Progress 'copper'
Wait-ProgressGate 2
Publish-Progress 'copper'
Wait-ProgressGate 3
Publish-Progress 'silver'
Wait-ProgressGate 4
Publish-Progress 'amber'
Wait-ProgressGate 5
Write-Output 'producer-finished'
