$ErrorActionPreference = 'Stop'
function Hash([byte[]]$Bytes) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try { ([BitConverter]::ToString($sha.ComputeHash($Bytes))).Replace('-', '').ToLowerInvariant() } finally { $sha.Dispose() }
}
function Hash-Material([byte[]]$Bytes) {
    $direct = Hash $Bytes
    if ([Array]::IndexOf($Bytes, [byte]13) -lt 0) { return ,@($direct) }
    $stream = [IO.MemoryStream]::new()
    try {
        for ($index = 0; $index -lt $Bytes.Length; $index++) {
            if ($Bytes[$index] -ne 13 -or $index + 1 -ge $Bytes.Length -or $Bytes[$index + 1] -ne 10) { $stream.WriteByte($Bytes[$index]) }
        }
        return ,@($direct, (Hash $stream.ToArray()))
    } finally { $stream.Dispose() }
}
function Read-Material([string]$Relative) {
    $current = $root
    foreach ($part in $Relative.Split('/')) {
        $current = Join-Path $current $part
        $item = Get-Item -LiteralPath $current -Force
        if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'material path is redirected' }
    }
    if ($item.PSIsContainer -or $item.Length -gt 1048576) { throw 'material exceeds bounds or is not a regular file' }
    return ,([IO.File]::ReadAllBytes($current))
}
function Verify {
    $actual = & git rev-parse --show-toplevel 2>$null
    if ($LASTEXITCODE -ne 0 -or [IO.Path]::GetFullPath($actual) -ne $root) { throw 'project checkout identity differs' }
    $actual = & git config --get ('remote.' + $spec.remote + '.url') 2>$null
    if ($LASTEXITCODE -ne 0 -or !($actual -is [string])) { throw 'project checkout remote differs' }
    if ($actual -cne $spec.url) {
        if ($actual -cnotmatch '^https?://' -or $spec.url -cnotmatch '^https?://' -or ($actual -creplace '\.git$', '') -cne ($spec.url -creplace '\.git$', '')) { throw 'project checkout remote differs' }
    }
    $doc = [Text.UTF8Encoding]::new($false, $true).GetString((Read-Material 'SteerlineFlow/environment.md')).Replace([string][char]13 + [char]10, [string][char]10)
    $fence = [regex]::Escape([string][char]96 * 3)
    $matches = [regex]::Matches($doc, ('(?ms)^' + $fence + 'steerline-environment\n(.*?)\n' + $fence + '$'))
    if ($matches.Count -ne 1 -or ([regex]::Matches($doc, $fence + 'steerline-environment')).Count -ne 1 -or (Hash ([Text.Encoding]::UTF8.GetBytes($matches[0].Groups[1].Value))) -cne $spec.manifest_sha256) { throw 'environment manifest differs' }
    foreach ($entry in $spec.files.PSObject.Properties) {
        if (!( (Hash-Material (Read-Material $entry.Name)) -ccontains $entry.Value )) { throw ('environment material differs: ' + $entry.Name) }
    }
}
$code = 125
$result = ''
$diagnostic = ''
$scratch = $null
try {
    $spec = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('@@INPUT@@')) | ConvertFrom-Json
    $root = (Get-Item -LiteralPath . -Force).FullName
    if ($root -ne [IO.Path]::GetFullPath($spec.root)) { throw 'execution directory differs from the registered checkout' }
    Verify
    $tempRoot = (Get-Item -LiteralPath ([IO.Path]::GetTempPath()) -Force).FullName
    if ($tempRoot -eq $root -or $tempRoot.StartsWith($root.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'temporary directory is inside the project' }
    for ($ancestor = Get-Item -LiteralPath $tempRoot -Force; $null -ne $ancestor; $ancestor = $ancestor.Parent) {
        if ($ancestor.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'temporary directory is redirected' }
    }
    $scratch = Join-Path $tempRoot ('steerrunner-env-' + [Guid]::NewGuid().ToString('N'))
    [IO.Directory]::CreateDirectory($scratch) | Out-Null
    $resultPath = Join-Path $scratch 'result.json'
    [IO.File]::WriteAllBytes($resultPath, [Convert]::FromBase64String($spec.initial))
    $env:STEERLINE_RESULT_FILE = $resultPath
    $env:TEMP = $scratch
    $env:TMP = $scratch
    $env:TMPDIR = $scratch
    if ($spec.interpreter -ceq 'powershell.exe') {
        & powershell.exe -NoProfile -NonInteractive -File (Join-Path $root $spec.script) *> $null
    } else {
        & bash $spec.script *> $null
    }
    $code = $LASTEXITCODE
    Verify
    $item = Get-Item -LiteralPath $resultPath -Force
    if ($item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $item.Length -gt 65536) { throw 'result file is missing, redirected or exceeds bounds' }
    $result = [Convert]::ToBase64String([IO.File]::ReadAllBytes($resultPath))
} catch {
    $code = 125
    $result = ''
    $diagnostic = $_.Exception.Message
} finally {
    if ($scratch) {
        try { Remove-Item -LiteralPath $scratch -Recurse -Force -ErrorAction Stop } catch { $code = 125; $result = ''; $diagnostic = 'temporary directory cleanup failed' }
    }
}
$prefix = 'SLENV-' + '@@ID@@' + '-'
for ($offset = 0; $offset -lt $result.Length; $offset += 1024) {
    [Console]::WriteLine($prefix + 'data:' + $result.Substring($offset, [Math]::Min(1024, $result.Length - $offset)))
}
[Console]::WriteLine($prefix + 'end:' + $code + ':' + [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($diagnostic)))
if (@@SETTLE@@ -gt 0) { Start-Sleep -Seconds @@SETTLE@@ }
exit $code
