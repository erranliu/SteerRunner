# Focused execution is a development result, never a complete qualification.
function Assert-SRFocusedArguments {
    param([string[]]$Levels,[string[]]$Scenarios,[string]$Config,[string]$Runner,[bool]$ExplicitOutput,[bool]$DefaultCatalog,[bool]$Availability,[string]$Prerequisites)
    if ($Levels.Count -ne 1 -or $Levels[0] -notin @('L2','L3') -or -not $Scenarios.Count -or
        @($Scenarios | Where-Object { $_ -cnotmatch '^[a-z][a-z0-9.-]+$' }).Count -or
        -not $Config -or -not $Runner -or -not $ExplicitOutput -or -not $DefaultCatalog -or $Availability -or $Prerequisites) {
        throw 'Focused requires one L2/L3 level, exact Scenario IDs, Config, RunnerExe and a new explicit OutDir; Catalog, ListAvailability and PrerequisitesFrom are unsupported'
    }
    if (-not (Test-Path -LiteralPath $Config -PathType Leaf)) { throw 'Focused Config must be an existing file' }
}

function Assert-SRFocusedRunner([string]$Root,[string]$Runner) {
    $build=Get-SRBuildRecord $Runner 'runner'
    $current=Get-SRSourceIdentity $Root @('cmd/steerrunner','internal','go.mod','go.sum','scripts/build.ps1','scripts/delivery-contract.ps1')
    if ($build.source_digest -cne $current.source_digest) { throw 'Focused Runner does not match current production inputs; rebuild it' }
}

function Get-SRFocusedNativeSource([string]$Root) {
    (Get-SRSourceIdentity $Root @('internal','tests/native','go.mod','go.sum')).source_digest
}

function Assert-SRFocusedInputs([string]$Root,[string]$Runner,$Preflight,[string]$NativeSource) {
    Assert-SRFocusedRunner $Root $Runner
    if ((Get-SRFileDigest $Preflight.driver_path) -cne $Preflight.driver_sha256 -or
        (Get-SRFocusedNativeSource $Root) -cne $NativeSource) { throw 'Focused native driver or its inputs changed; rebuild and run again' }
}
