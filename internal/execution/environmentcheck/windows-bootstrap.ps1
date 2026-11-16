$ErrorActionPreference = 'Stop'
@@TEMPORARY@@
function Write-InvocationFile([string]$name, [int]$count, [string]$destination) {
    $payload = [Text.StringBuilder]::new()
    for ($i = 0; $i -lt $count; $i++) {
        $key = '@@PREFIX@@' + $name + '_' + $i
        $chunk = [Environment]::GetEnvironmentVariable($key)
        if (!$chunk) { throw 'environment invocation transport chunk is missing' }
        [void]$payload.Append($chunk)
        [Environment]::SetEnvironmentVariable($key, $null)
    }
    [IO.File]::WriteAllBytes($destination, [Convert]::FromBase64String($payload.ToString()))
}
$directory = $null
$code = 125
$failure = ''
try {
    $root = (Get-Item -LiteralPath . -Force).FullName
    $directory = New-EnvironmentScratch $root 'steerrunner-env-invocation-'
    $script = Join-Path $directory 'invoke.ps1'
    Write-InvocationFile 'INPUT' @@INPUT_CHUNKS@@ (Join-Path $directory 'input.json')
    Write-InvocationFile 'SCRIPT' @@SCRIPT_CHUNKS@@ $script
    [Environment]::SetEnvironmentVariable('STEERRUNNER_ENV_BOOTSTRAP', $null)
    $lines = @(& powershell.exe -NoProfile -NonInteractive -File $script)
    $code = $LASTEXITCODE
} catch {
    $failure = $_.Exception.Message
} finally {
    try { Remove-EnvironmentInvocation $directory } catch { $failure = 'environment invocation file cleanup failed' }
}
if ($failure) {
    $code = 125
    $diagnostic = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($failure))
    [Console]::WriteLine('SLENV-@@ID@@-end:125:' + $diagnostic)
} else {
    foreach ($line in $lines) { [Console]::WriteLine($line) }
}
Start-Sleep -Seconds @@SETTLE@@
exit $code
