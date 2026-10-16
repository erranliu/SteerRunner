param([Parameter(Mandatory=$true)][string]$OrcaExe,[Parameter(Mandatory=$true)][string]$Destination)
$ErrorActionPreference='Stop'
$taskResources=Split-Path -Parent (Split-Path -Parent $OrcaExe)
$taskSources=@()
foreach($taskTree in @('out','node_modules')){
 $taskSourceRoot=if($taskTree -eq 'out'){Join-Path $taskResources 'app.asar.unpacked/out'}else{Join-Path $taskResources 'node_modules'}
 foreach($taskFile in @(Get-ChildItem -LiteralPath $taskSourceRoot -Recurse -File)){
  $taskSources+=@{path=($taskTree+'/'+$taskFile.FullName.Substring($taskSourceRoot.Length+1).Replace('\','/'));source=$taskFile.FullName;sha256=(Get-FileHash -LiteralPath $taskFile.FullName -Algorithm SHA256).Hash.ToLowerInvariant()}
 }
}
$taskNativeNode=(Get-Command node).Source
$taskSources+=@{path='node.exe';source=$taskNativeNode;sha256=(Get-FileHash -LiteralPath $taskNativeNode -Algorithm SHA256).Hash.ToLowerInvariant()}
$taskSources=@($taskSources|Sort-Object { $_.path })
$taskIndex=@($taskSources|ForEach-Object{[ordered]@{path=$_.path;sha256=$_.sha256}})
$taskIndexJSON=([ordered]@{schema='steerrunner-native-runtime/v1';files=$taskIndex}|ConvertTo-Json -Depth 6)
$taskDigest=[Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($taskIndexJSON))).ToLowerInvariant()
$taskCache=Join-Path $PSScriptRoot ('../.evidence/native-runtime/'+$taskDigest)
New-Item -ItemType Directory -Force -Path $Destination|Out-Null
foreach($taskFile in $taskSources){
 $taskCached=Join-Path $taskCache $taskFile.path
 $taskTarget=Join-Path $Destination $taskFile.path
 if(-not (Test-Path -LiteralPath $taskCached)){
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $taskCached)|Out-Null
  Copy-Item -LiteralPath $taskFile.source -Destination $taskCached
 }
 if((Get-FileHash -LiteralPath $taskCached -Algorithm SHA256).Hash -ine $taskFile.sha256){throw 'Immutable test runtime cache changed'}
 New-Item -ItemType Directory -Force -Path (Split-Path -Parent $taskTarget)|Out-Null
 if($taskFile.path -eq 'node.exe'){Copy-Item -LiteralPath $taskCached -Destination $taskTarget}
 else{try{New-Item -ItemType HardLink -Path $taskTarget -Target $taskCached -ErrorAction Stop|Out-Null}catch{Copy-Item -LiteralPath $taskCached -Destination $taskTarget}}
}
$taskIndexPath=Join-Path (Split-Path -Parent $Destination) 'runtime-files.json'
[IO.File]::WriteAllText($taskIndexPath,$taskIndexJSON,[Text.UTF8Encoding]::new($false))
Write-Output $taskIndexPath
