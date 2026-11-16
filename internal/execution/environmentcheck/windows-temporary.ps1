function New-EnvironmentScratch([string]$root, [string]$prefix) {
    $tempRoot = (Get-Item -LiteralPath ([IO.Path]::GetTempPath()) -Force).FullName
    if ($tempRoot -eq $root -or $tempRoot.StartsWith($root.TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'temporary directory is inside the project' }
    for ($ancestor = Get-Item -LiteralPath $tempRoot -Force; $null -ne $ancestor; $ancestor = $ancestor.Parent) {
        if ($ancestor.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'temporary directory is redirected' }
    }
    return [IO.Directory]::CreateDirectory((Join-Path $tempRoot ($prefix + [Guid]::NewGuid().ToString('N')))).FullName
}
function Remove-EnvironmentInvocation([string]$directory) {
    if ($directory -and [IO.Directory]::Exists($directory)) {
        [IO.File]::Delete((Join-Path $directory 'input.json'))
        [IO.File]::Delete((Join-Path $directory 'invoke.ps1'))
        [IO.Directory]::Delete($directory)
    }
}
