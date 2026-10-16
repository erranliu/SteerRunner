param([Parameter(Mandatory=$true)][string]$Manifest,[ValidateSet('block','restore','watchdog')][string]$Action='block')
$ErrorActionPreference='Stop'
$taskManifestPath=[IO.Path]::GetFullPath($Manifest)
$taskM=Get-Content -LiteralPath $taskManifestPath -Raw | ConvertFrom-Json
if ($taskM.schema -ne 'steerrunner-test/v2' -or $taskM.id -notmatch '^[a-f0-9]{12}$') { throw 'Invalid stability network identity' }
$taskRoot=[IO.Path]::GetFullPath($taskM.root).TrimEnd('\')
$taskPrograms=@($taskM.runner_exe,$taskM.stability.native_orca.path,$taskM.stability.helper.path)
$taskGroup='SteerRunner-L3-'+$taskM.id
$taskPrincipal=[Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
$taskIntent=Join-Path $taskRoot 'evidence/network-intent.json'
foreach($taskProgram in $taskPrograms){$taskResolved=(Resolve-Path -LiteralPath $taskProgram).Path;if(-not $taskResolved.StartsWith($taskRoot+'\',[StringComparison]::OrdinalIgnoreCase)){throw 'Network scope escapes fixture'}}
if(-not $taskPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)){
 if(-not $taskM.stability.admin_host){throw 'admin_required: no verified elevated network host'}
 $taskQueue=$taskM.stability.admin_host
 $taskReady=Get-Content -LiteralPath (Join-Path $taskQueue 'ready.json') -Raw|ConvertFrom-Json
 $taskProcess=Get-Process -Id $taskReady.pid -ErrorAction Stop
 if(-not $taskReady.elevated -or $taskProcess.StartTime.ToUniversalTime().ToFileTimeUtc().ToString() -ne $taskReady.created){throw 'Elevated network host identity changed'}
 $taskID=[Guid]::NewGuid().ToString('N')
 $taskRequest=Join-Path $taskQueue ($taskID+'.request.json')
 $taskTemporary=$taskRequest+'.tmp'
 [IO.File]::WriteAllText($taskTemporary,(@{host_id=$taskReady.host_id;manifest=$taskManifestPath;action=$Action}|ConvertTo-Json))
 Move-Item -LiteralPath $taskTemporary -Destination $taskRequest
 $taskResponse=Join-Path $taskQueue ($taskID+'.result.json')
 $taskUntil=[DateTime]::UtcNow.AddSeconds(90)
 while(-not (Test-Path -LiteralPath $taskResponse)){if([DateTime]::UtcNow -gt $taskUntil){throw 'Elevated network action outcome unknown; retain original request for reconciliation'};Start-Sleep -Milliseconds 200}
 $taskResult=Get-Content -LiteralPath $taskResponse -Raw|ConvertFrom-Json
 if(-not $taskResult.ok -or $taskResult.host_id -ne $taskReady.host_id){throw ('Elevated network action failed: '+$taskResult.error)}
 exit 0
}
function Restore-SRNetwork {
 foreach($taskIndex in 0..($taskPrograms.Count-1)){
  $taskName=$taskGroup+'-'+$taskIndex
  $taskRules=@(Get-NetFirewallRule -Name $taskName -ErrorAction SilentlyContinue)
  foreach($taskRule in $taskRules){
   if($taskRule.Group -ne $taskGroup -or (Get-NetFirewallApplicationFilter -AssociatedNetFirewallRule $taskRule).Program -ine $taskPrograms[$taskIndex]){throw 'Network rule identity changed'}
   $taskRule | Remove-NetFirewallRule
  }
 }
 if(@(Get-NetFirewallRule -Group $taskGroup -ErrorAction SilentlyContinue).Count){throw 'Network rules remain'}
 [IO.File]::WriteAllText((Join-Path $taskRoot 'evidence/network-restored.json'),(@{restored=$true;at=[DateTime]::UtcNow.ToString('o')}|ConvertTo-Json))
}
if($Action -eq 'restore'){Restore-SRNetwork;exit 0}
if($Action -eq 'watchdog'){
 $taskLimit=[DateTime]::UtcNow.AddMinutes(10)
 while([DateTime]::UtcNow -lt $taskLimit){if(Test-Path -LiteralPath (Join-Path $taskRoot 'evidence/network-restored.json')){exit 0};Start-Sleep -Seconds 2}
 Restore-SRNetwork
 [IO.File]::WriteAllText((Join-Path $taskRoot 'evidence/network-watchdog-expired.json'),'true')
 exit 1
}
$taskPrincipal=[Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
if(-not $taskPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)){throw 'admin_required'}
[IO.File]::WriteAllText($taskIntent,(@{id=$taskM.id;programs=$taskPrograms;at=[DateTime]::UtcNow.ToString('o')}|ConvertTo-Json))
$taskRestored=Join-Path $taskRoot 'evidence/network-restored.json'
if(Test-Path -LiteralPath $taskRestored){Remove-Item -LiteralPath $taskRestored}
$taskShell=(Get-Process -Id $PID).Path
$taskStartup=New-CimInstance -ClassName Win32_ProcessStartup -ClientOnly -Property @{ShowWindow=[uint16]0}
$taskWatch=Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{CommandLine=('"'+$taskShell+'" -NoProfile -File "'+$PSCommandPath+'" -Manifest "'+$taskManifestPath+'" -Action watchdog');ProcessStartupInformation=$taskStartup}
if($taskWatch.ReturnValue -ne 0){throw 'Network recovery watchdog failed to start'}
foreach($taskIndex in 0..($taskPrograms.Count-1)){
 New-NetFirewallRule -Name ($taskGroup+'-'+$taskIndex) -Group $taskGroup -DisplayName ($taskGroup+'-'+$taskIndex) -Direction Outbound -Action Block -Profile Any -Program $taskPrograms[$taskIndex] | Out-Null
}
# Delete only this fixture's existing IPv4 TCBs; documented SetTcpEntry DELETE_TCB.
if(-not ('SRCloseTcp' -as [type])){ Add-Type -TypeDefinition @'
using System; using System.Runtime.InteropServices; using System.Net;
public class SRCloseTcp {
 [StructLayout(LayoutKind.Sequential)] public struct Row {public uint state,local,localPort,remote,remotePort;}
 [DllImport("iphlpapi.dll")] static extern uint SetTcpEntry(ref Row row);
 public static uint Close(string a,int p,string b,int q){
 byte[] ap={(byte)(p>>8),(byte)p,0,0},bp={(byte)(q>>8),(byte)q,0,0};
 Row row=new Row{state=12,local=BitConverter.ToUInt32(IPAddress.Parse(a).GetAddressBytes(),0),localPort=BitConverter.ToUInt32(ap,0),remote=BitConverter.ToUInt32(IPAddress.Parse(b).GetAddressBytes(),0),remotePort=BitConverter.ToUInt32(bp,0)};
 return SetTcpEntry(ref row);
 }
}
'@
}
$taskClosed=@()
$taskProcesses=@(Get-CimInstance Win32_Process | Where-Object {$_.ExecutablePath -in $taskPrograms})
foreach($taskProcess in $taskProcesses){
 $taskConnections=@(Get-NetTCPConnection -OwningProcess $taskProcess.ProcessId -State Established -ErrorAction SilentlyContinue)
 foreach($taskConnection in $taskConnections){
  if($taskConnection.LocalAddress.Contains(':')){throw 'IPv6 active connection cannot be safely closed by this injector'}
  $taskCurrent=Get-CimInstance Win32_Process -Filter ('ProcessId = '+$taskProcess.ProcessId)
  if($taskCurrent -and ($taskCurrent.ExecutablePath -ine $taskProcess.ExecutablePath -or $taskCurrent.CreationDate -ne $taskProcess.CreationDate)){throw 'TCP owner process identity changed'}
  $taskCode=$null
  if($taskCurrent){$taskCode=[SRCloseTcp]::Close($taskConnection.LocalAddress,$taskConnection.LocalPort,$taskConnection.RemoteAddress,$taskConnection.RemotePort)}
  # Enumeration races with normal socket/process exit after firewall activation.
  # A nonzero API result is acceptable only if a fresh OS read proves the exact
  # original tuple is already absent; never infer absence from the return code.
  $taskRemaining=@(Get-NetTCPConnection -ErrorAction Stop|Where-Object {$_.OwningProcess -eq $taskProcess.ProcessId -and $_.State -eq 'Established' -and $_.LocalAddress -eq $taskConnection.LocalAddress -and $_.LocalPort -eq $taskConnection.LocalPort -and $_.RemoteAddress -eq $taskConnection.RemoteAddress -and $_.RemotePort -eq $taskConnection.RemotePort})
  $taskFact=@{pid=$taskProcess.ProcessId;creation_time=$taskProcess.CreationDate;program=$taskProcess.ExecutablePath;local=$taskConnection.LocalAddress;local_port=$taskConnection.LocalPort;remote=$taskConnection.RemoteAddress;remote_port=$taskConnection.RemotePort;exit_code=$taskCode;api_called=[bool]$taskCurrent;exact_connection_remaining=($taskRemaining.Count -gt 0);elevated=$true;at=[DateTime]::UtcNow.ToString('o')}
  $taskClosed+=$taskFact
  [IO.File]::AppendAllText((Join-Path $taskRoot 'evidence/network-close-attempts.jsonl'),(($taskFact|ConvertTo-Json -Compress)+"`n"),[Text.UTF8Encoding]::new($false))
  if($taskCode -ne 0 -and $taskRemaining.Count -gt 0){throw "TCB close failed: $taskCode; elevated=True; exact original connection still present"}
 }
}
[IO.File]::WriteAllText((Join-Path $taskRoot 'evidence/network-blocked.json'),(@{rules=$taskGroup;closed=$taskClosed;watchdog_pid=$taskWatch.ProcessId;at=[DateTime]::UtcNow.ToString('o')}|ConvertTo-Json -Depth 6))
