$ErrorActionPreference = 'Stop'
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
Push-Location $taskRepo
try {
    $taskModules = @(& go list -deps -f '{{with .Module}}{{.Path}}|{{.Version}}|{{.Dir}}{{end}}' ./cmd/steerrunner | Where-Object { $_ } | Sort-Object -Unique)
    if ($LASTEXITCODE -ne 0) { throw 'Cannot enumerate runtime modules.' }
    $taskGoRoot = (& go env GOROOT).Trim()
    $taskText = [Text.StringBuilder]::new()
    [void]$taskText.AppendLine("Third-party runtime notices for SteerRunner`nGenerated from the Windows/amd64 executable dependency closure.`nThese are upstream notices; they do not set SteerRunner's own license.`n")
    [void]$taskText.AppendLine("=== Go runtime / standard library ===`n")
    [void]$taskText.AppendLine([IO.File]::ReadAllText((Join-Path $taskGoRoot 'LICENSE')))
    foreach ($taskModule in $taskModules) {
        $taskFields = $taskModule -split '\|', 3
        if ($taskFields[0] -eq 'github.com/erranliu/SteerRunner') { continue }
        $taskLicenseFiles = @(Get-ChildItem -LiteralPath $taskFields[2] -File | Where-Object { $_.Name -match '^(LICENSE|COPYING|NOTICE|SQLITE-LICENSE)' } | Sort-Object Name)
        if ($taskLicenseFiles.Count -eq 0) { throw "Missing upstream notices for $($taskFields[0])" }
        foreach ($taskLicenseFile in $taskLicenseFiles) {
            [void]$taskText.AppendLine("`n=== $($taskFields[0]) $($taskFields[1]) / $($taskLicenseFile.Name) ===`n")
            [void]$taskText.AppendLine([IO.File]::ReadAllText($taskLicenseFile.FullName))
        }
    }
    [IO.File]::WriteAllText((Join-Path $taskRepo 'THIRD_PARTY_NOTICES.txt'), ($taskText.ToString().Replace("`r`n", "`n").TrimEnd() + "`n"), [Text.UTF8Encoding]::new($false))
} finally { Pop-Location }
