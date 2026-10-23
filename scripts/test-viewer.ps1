param([Parameter(Mandatory=$true)][string]$ViewerExe,[Parameter(Mandatory=$true)][string]$OutDir)
$ErrorActionPreference='Stop'
$taskOut=[IO.Path]::GetFullPath($OutDir)
if (Test-Path -LiteralPath $taskOut) { throw 'Use a new isolated OutDir' }
[IO.Directory]::CreateDirectory($taskOut) | Out-Null
Add-Type @'
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class ViewerProbe {
  public delegate bool Visitor(IntPtr h,IntPtr p);
  [DllImport("user32.dll")]public static extern bool EnumWindows(Visitor v,IntPtr p);
  [DllImport("user32.dll")]public static extern uint GetWindowThreadProcessId(IntPtr h,out uint p);
  [DllImport("user32.dll",CharSet=CharSet.Unicode)]public static extern IntPtr FindWindowEx(IntPtr h,IntPtr after,string name,string title);
  [DllImport("user32.dll")]public static extern IntPtr SendMessage(IntPtr h,uint m,IntPtr w,IntPtr l);
  [DllImport("user32.dll",CharSet=CharSet.Unicode)]public static extern int GetWindowText(IntPtr h,StringBuilder text,int count);
  [DllImport("user32.dll",EntryPoint="SendMessageW",CharSet=CharSet.Unicode)]public static extern IntPtr TextMessage(IntPtr h,uint m,IntPtr size,StringBuilder text);
  [DllImport("user32.dll",CharSet=CharSet.Unicode)]public static extern int GetClassName(IntPtr h,StringBuilder text,int count);
  public static IntPtr ForProcess(uint pid){IntPtr found=IntPtr.Zero;EnumWindows((h,p)=>{uint id;GetWindowThreadProcessId(h,out id);var name=new StringBuilder(256);GetClassName(h,name,256);if(id==pid && name.ToString()=="SteerRunnerReadOnlyViewer")found=h;return true;},IntPtr.Zero);return found;}
}
'@
$taskSource='a'*64;$taskPath=Join-Path $taskOut ($taskSource+'.json')
$taskSnapshot=@{schema_version=1;source_id=$taskSource;instance_id='fixture';sequence=1;generated_at=[DateTime]::UtcNow.ToString('o');runs=@(@{run_id='run';title='Viewer controlled fixture';workflow=@{id='development';version=2};step='implement';status='running';pr=$null})}
function Publish { [IO.File]::WriteAllText($taskPath,($taskSnapshot | ConvertTo-Json -Depth 8),(New-Object Text.UTF8Encoding($false))) }
function StatusText($h) { $text=New-Object Text.StringBuilder 4096;[ViewerProbe]::TextMessage($h,0x000D,[IntPtr]4096,$text) | Out-Null;return $text.ToString() }
Publish
$taskOriginalExe=[IO.Path]::GetFullPath($ViewerExe)
$taskExe=Join-Path $taskOut 'standalone-viewer.exe'
Copy-Item -LiteralPath $taskOriginalExe -Destination $taskExe
# Owned native-window integration test. It starts hidden and closes its own HWND.
$taskProcess=Start-Process -FilePath $taskExe -ArgumentList @('--snapshot-dir',('"'+$taskOut+'"')) -WindowStyle Hidden -PassThru -RedirectStandardError (Join-Path $taskOut 'stderr.txt') -RedirectStandardOutput (Join-Path $taskOut 'stdout.txt')
try {
    $taskUntil=[DateTime]::UtcNow.AddSeconds(12);$taskWindow=[IntPtr]::Zero
    do { Start-Sleep -Milliseconds 100;$taskWindow=[ViewerProbe]::ForProcess($taskProcess.Id) } while ($taskWindow -eq [IntPtr]::Zero -and [DateTime]::UtcNow -lt $taskUntil)
    if ($taskWindow -eq [IntPtr]::Zero) { throw 'Viewer did not create its native window' }
    do { $taskList=[ViewerProbe]::FindWindowEx($taskWindow,[IntPtr]::Zero,'SysListView32',[NullString]::Value);if ($taskList -eq [IntPtr]::Zero) { Start-Sleep -Milliseconds 100 } } while ($taskList -eq [IntPtr]::Zero -and [DateTime]::UtcNow -lt $taskUntil)
    $taskStatus=[ViewerProbe]::FindWindowEx($taskWindow,[IntPtr]::Zero,'STATIC',[NullString]::Value)
    if ($taskStatus -eq [IntPtr]::Zero) { throw 'Viewer status control was not found' }
    $taskHeader=[ViewerProbe]::SendMessage($taskList,0x101F,[IntPtr]::Zero,[IntPtr]::Zero)
    if ([ViewerProbe]::SendMessage($taskHeader,0x1200,[IntPtr]::Zero,[IntPtr]::Zero).ToInt64() -ne 4) { throw 'Viewer does not have exactly four columns' }
    do { Start-Sleep -Milliseconds 100;$taskCount=[ViewerProbe]::SendMessage($taskList,0x1004,[IntPtr]::Zero,[IntPtr]::Zero).ToInt64() } while ($taskCount -ne 1 -and [DateTime]::UtcNow -lt $taskUntil)
    if ($taskCount -ne 1) { throw 'Viewer did not render the Run from a standalone JSON fixture' }
    $taskSnapshot.sequence=2;$taskSnapshot.generated_at=[DateTime]::UtcNow.AddSeconds(-30).ToString('o');Publish
    $taskUntil=[DateTime]::UtcNow.AddSeconds(8)
    do { Start-Sleep -Milliseconds 200;$taskText=StatusText $taskStatus } while ($taskText -notmatch 'expired' -and [DateTime]::UtcNow -lt $taskUntil)
    if ($taskText -notmatch 'expired') { throw ('Viewer did not mark a newly read old snapshot as expired: '+$taskText) }
    [IO.File]::WriteAllText($taskPath,'broken')
    $taskUntil=[DateTime]::UtcNow.AddSeconds(8)
    do { Start-Sleep -Milliseconds 200;$taskText=StatusText $taskStatus } while ($taskText -notmatch 'cannot read valid' -and [DateTime]::UtcNow -lt $taskUntil)
    if ($taskText -notmatch 'cannot read valid' -or [ViewerProbe]::SendMessage($taskList,0x1004,[IntPtr]::Zero,[IntPtr]::Zero).ToInt64() -ne 1) { throw 'Viewer lost the last good row after a broken snapshot' }
    $taskOtherSource='b'*64;$taskOtherPath=Join-Path $taskOut ($taskOtherSource+'.json')
    $taskOther=@{schema_version=1;source_id=$taskOtherSource;instance_id='second';sequence=1;generated_at=[DateTime]::UtcNow.ToString('o');runs=@(@{run_id='run';title='Same ID in another source';workflow=@{id='future';version=1};step='new';status='future-status';pr=$null})}
    [IO.File]::WriteAllText($taskOtherPath,($taskOther | ConvertTo-Json -Depth 8))
    $taskUntil=[DateTime]::UtcNow.AddSeconds(8)
    do { Start-Sleep -Milliseconds 100;$taskCount=[ViewerProbe]::SendMessage($taskList,0x1004,[IntPtr]::Zero,[IntPtr]::Zero).ToInt64() } while ($taskCount -ne 2 -and [DateTime]::UtcNow -lt $taskUntil)
    if ($taskCount -ne 2) { throw 'One failed source cleared or masked the other source' }
    $taskSnapshot.sequence=3;$taskSnapshot.generated_at=[DateTime]::UtcNow.ToString('o');$taskSnapshot.runs=@();Publish
    $taskUntil=[DateTime]::UtcNow.AddSeconds(8)
    do { Start-Sleep -Milliseconds 100;$taskCount=[ViewerProbe]::SendMessage($taskList,0x1004,[IntPtr]::Zero,[IntPtr]::Zero).ToInt64() } while ($taskCount -ne 1 -and [DateTime]::UtcNow -lt $taskUntil)
    if ($taskCount -ne 1) { throw 'A valid complete snapshot did not remove its missing Run' }
    [IO.File]::Delete($taskOtherPath)
    $taskUntil=[DateTime]::UtcNow.AddSeconds(8)
    do { Start-Sleep -Milliseconds 100;$taskText=StatusText $taskStatus } while ($taskText -notmatch 'snapshot missing' -and [DateTime]::UtcNow -lt $taskUntil)
    if ($taskText -notmatch 'snapshot missing' -or [ViewerProbe]::SendMessage($taskList,0x1004,[IntPtr]::Zero,[IntPtr]::Zero).ToInt64() -ne 1) { throw 'A deleted source lost its last good Run' }
    [ViewerProbe]::SendMessage($taskWindow,0x0010,[IntPtr]::Zero,[IntPtr]::Zero) | Out-Null
    if (-not $taskProcess.WaitForExit(5000) -or $taskProcess.ExitCode -ne 0) { throw 'Viewer did not settle and exit cleanly after close' }
    $taskWindow=[IntPtr]::Zero
    $taskResult=@{passed=@('native window','four columns','copied standalone EXE and snapshot row','expired old snapshot','last good row retained after parse failure','independent sources with the same Run ID','complete snapshot removes only its missing Run','deleted source retains last good row','close settles reads and exits');viewer_sha256=(Get-FileHash -LiteralPath $taskExe -Algorithm SHA256).Hash}
    [IO.File]::WriteAllText((Join-Path $taskOut 'result.txt'),($taskResult | ConvertTo-Json -Depth 5))
    Write-Output ($taskResult | ConvertTo-Json -Depth 5)
} catch { Write-Output $_.Exception.Message; throw } finally {
    if ($taskWindow -ne [IntPtr]::Zero) { [ViewerProbe]::SendMessage($taskWindow,0x0010,[IntPtr]::Zero,[IntPtr]::Zero) | Out-Null }
    if (-not $taskProcess.WaitForExit(5000)) { $taskProcess.Kill();Write-Output 'Owned Viewer required forced cleanup after test failure.' }
    $taskProcess.Dispose()
}
