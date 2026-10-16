$ErrorActionPreference='Stop'
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class SRKeyringProbe {
 [DllImport("advapi32.dll",CharSet=CharSet.Unicode,SetLastError=true)]
 public static extern bool CredRead(string target,uint type,uint flags,out IntPtr credential);
 [DllImport("advapi32.dll")] public static extern void CredFree(IntPtr pointer);
}
'@
$results=@()
foreach($target in @('gh:github.com:erranliu','gh:github.com')) {
 $pointer=[IntPtr]::Zero
 $ok=[SRKeyringProbe]::CredRead($target,1,0,[ref]$pointer)
 $code=if($ok){0}else{[Runtime.InteropServices.Marshal]::GetLastWin32Error()}
 # Do not inspect or copy credential fields/blob. Free the native buffer.
 if($ok){[SRKeyringProbe]::CredFree($pointer)}
 $results+=@{reference=$target;read_succeeded=[bool]$ok;win32_error=$code}
}
ConvertTo-Json -InputObject $results -Compress
