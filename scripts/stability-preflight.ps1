param([Parameter(Mandatory=$true)][string]$Config,[switch]$RequireAdmin)
$ErrorActionPreference = 'Stop'
$taskConfig = Get-Content -LiteralPath $Config -Raw | ConvertFrom-Json
if ($taskConfig.schema -ne 'steerrunner-live-config/v2') { throw 'Stability cases require steerrunner-live-config/v2' }
$taskAllowed = @('schema','codex_executable','model','reasoning_effort','access_mode','audit_model','audit_reasoning_effort','feedback_qualification','orca_executable','orca_environment','remote_agent','remote_model','remote_reasoning_effort','remote_args','remote_runtime_executable','remote_runtime_sha256','remote_github_auth','admin_host')
foreach ($taskProperty in $taskConfig.PSObject.Properties.Name) { if ($taskProperty -notin $taskAllowed) { throw "Unknown stability config field: $taskProperty" } }
if (-not $taskConfig.orca_environment -or -not [IO.Path]::IsPathFullyQualified($taskConfig.orca_executable) -or -not (Test-Path -LiteralPath $taskConfig.orca_executable -PathType Leaf)) { throw 'Stability requires an absolute Orca executable and fixed environment' }
if ($RequireAdmin) {
    $taskPrincipal = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
    if (-not $taskPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        if (-not $taskConfig.PSObject.Properties['admin_host']) { throw 'admin_required: isolated dual-path network injection requires elevated firewall management' }
        $taskReady=Get-Content -LiteralPath (Join-Path $taskConfig.admin_host 'ready.json') -Raw|ConvertFrom-Json
        $taskProcess=Get-Process -Id $taskReady.pid -ErrorAction Stop
        if(-not $taskReady.elevated -or $taskProcess.StartTime.ToUniversalTime().ToFileTimeUtc().ToString() -ne $taskReady.created){throw 'Elevated network host identity changed'}
    }
    if ((Get-Service MpsSvc).Status -ne 'Running') { throw 'Windows firewall must already be running' }
}
