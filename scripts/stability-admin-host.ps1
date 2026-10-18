param([Parameter(Mandatory=$true)][string]$Directory,[string]$FixtureParent='C:/srt')
$ErrorActionPreference='Stop'
$taskPrincipal=[Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
if(-not $taskPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)){throw 'Network host requires administrator token'}
$taskQueue=[IO.Path]::GetFullPath($Directory)
New-Item -ItemType Directory -Force -Path $taskQueue|Out-Null
$taskHostID=[Guid]::NewGuid().ToString('N')
$taskReady=@{host_id=$taskHostID;elevated=$true;pid=$PID;created=(Get-Process -Id $PID).StartTime.ToUniversalTime().ToFileTimeUtc().ToString();fixture_parent=[IO.Path]::GetFullPath($FixtureParent);at=[DateTime]::UtcNow.ToString('o')}
[IO.File]::WriteAllText((Join-Path $taskQueue 'ready.json'),($taskReady|ConvertTo-Json))
$taskLimit=[DateTime]::UtcNow.AddHours(12)
try {
 while([DateTime]::UtcNow -lt $taskLimit -and -not (Test-Path -LiteralPath (Join-Path $taskQueue 'stop'))){
  foreach($taskRequestFile in @(Get-ChildItem -LiteralPath $taskQueue -Filter '*.request.json' -File)){
   $taskResult=@{ok=$false;host_id=$taskHostID;at=[DateTime]::UtcNow.ToString('o')}
   try {
    $taskRequest=Get-Content -LiteralPath $taskRequestFile.FullName -Raw|ConvertFrom-Json
    if($taskRequest.host_id -ne $taskHostID -or $taskRequest.action -notin @('block','block-state','restore')){throw 'Invalid network host request'}
    $taskManifest=Get-Content -LiteralPath $taskRequest.manifest -Raw|ConvertFrom-Json
    $taskExpected=Join-Path $taskReady.fixture_parent $taskManifest.id
    if($taskManifest.id -notmatch '^[a-f0-9]{12}$' -or [IO.Path]::GetFullPath($taskManifest.root) -ine $taskExpected -or [IO.Path]::GetFullPath($taskRequest.manifest) -ine (Join-Path $taskExpected 'manifest.json')){throw 'Network host fixture scope mismatch'}
    if($taskManifest.stability.firewall.sha256 -ine (Get-FileHash -LiteralPath (Join-Path $PSScriptRoot 'stability-network.ps1') -Algorithm SHA256).Hash){throw 'Network injector source differs from frozen fixture'}
    # Execute only the repository injector, never a script selected by the queue request.
    $taskOutput=& (Join-Path $PSScriptRoot 'stability-network.ps1') -Manifest $taskRequest.manifest -Action $taskRequest.action 2>&1|Out-String
    $taskResult.ok=$true
    $taskResult['output']=$taskOutput
   }catch{$taskResult['error']=$_.Exception.Message}
   $taskResponse=$taskRequestFile.FullName.Replace('.request.json','.result.json')
   $taskTemporary=$taskResponse+'.tmp'
   [IO.File]::WriteAllText($taskTemporary,($taskResult|ConvertTo-Json -Depth 6))
   Move-Item -LiteralPath $taskTemporary -Destination $taskResponse
   Move-Item -LiteralPath $taskRequestFile.FullName -Destination ($taskRequestFile.FullName+'.processed')
  }
  Start-Sleep -Milliseconds 200
 }
}finally{
 [IO.File]::WriteAllText((Join-Path $taskQueue 'finished.json'),(@{host_id=$taskHostID;at=[DateTime]::UtcNow.ToString('o')}|ConvertTo-Json))
}
