param([Parameter(Mandatory=$true)][string]$Repository)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $Repository 'scripts/test-configuration.ps1')
$taskCases = @(
    @{version='codex-cli 0.159.2';accepted=$true},
    @{version='codex-cli 0.159.2+build.7';accepted=$true},
    @{version='codex-cli 0.159.3';accepted=$true},
    @{version='codex-cli 0.159.10';accepted=$true},
    @{version='codex-cli 0.160.0';accepted=$true},
    @{version='codex-cli 0.163.0-alpha.10.2';accepted=$true},
    @{version='codex-cli 1.0.0';accepted=$true},
    @{version='codex-cli 10.0.0-rc.1+build.2';accepted=$true},
    @{version='codex-cli 0.159.1';accepted=$false},
    @{version='codex-cli 0.158.99';accepted=$false},
    @{version='codex-cli 0.99.999';accepted=$false},
    @{version='codex-cli 0.159.2-alpha.1';accepted=$false},
    @{version='codex-cli 0.0160.0';accepted=$false},
    @{version='codex-cli 0.160.0-alpha.01';accepted=$false},
    @{version='codex-cli 0.160';accepted=$false},
    @{version='codex-cli 0.160.0.1';accepted=$false},
    @{version='codex-cli 0.160.0-alpha..1';accepted=$false},
    @{version='codex-cli 0.160.0-';accepted=$false},
    @{version='codex-cli v0.160.0';accepted=$false},
    @{version='codex 0.160.0';accepted=$false},
    @{version='';accepted=$false}
)
foreach ($taskCase in $taskCases) {
    if ((Test-SRCodexVersion $taskCase.version) -ne $taskCase.accepted) {
        throw "Incorrect native Codex qualification: $($taskCase.version)"
    }
}
Write-Host "Codex minimum-version checks passed ($($taskCases.Count) cases)."
