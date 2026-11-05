param(
    [string]$Target = '',
    [Parameter(Mandatory)][string]$OutDir,
    [ValidateSet('long-input','controlled','capability-baseline')][string]$Mode = 'long-input',
    [ValidateRange(1,30)][int]$Repeat = 1
)
$ErrorActionPreference = 'Stop'
# The legacy TUI execution producer and its probes are retired in this build.
# Keep the old command explicit: no build, output directory or Orca effect occurs.
throw 'Legacy Orca terminal input probes are retired. Use the original Runner build for historical qualification; current Orca Agent Runs use structured Agent Session tests. See docs/acceptance/orca-reader-execution-boundary.md.'
