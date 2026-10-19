param([Parameter(Mandatory=$true)][string]$Config,[string]$Scenario='orca-remote.caller-exit',[string]$Case='accepted-response-lost',[string]$RunnerExe='',[string]$ConfigHelper='',[switch]$RequireAdmin)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'test-configuration.ps1')
. (Join-Path $PSScriptRoot 'live-catalog.ps1')
$taskRepo=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if(-not $RunnerExe){$RunnerExe=Join-Path $taskRepo 'bin/steerrunner.exe'}
if(-not $ConfigHelper){$ConfigHelper=Join-Path $taskRepo 'bin/test-config.exe'}
$taskReport=Get-SRTestConfiguration -RunnerExe $RunnerExe -Helper $ConfigHelper -Config $Config
$taskCaseReport=@($taskReport.cases|Where-Object {$_.scenario_id -ceq $Scenario -and $_.case_id -ceq $Case})
if($taskCaseReport.Count -ne 1 -or $taskCaseReport[0].availability -ne 'available'){throw 'Stability case configuration is unavailable'}
$taskConfig=$taskReport.configuration
$taskDefinition=@(Get-SRCaseRegistry|Where-Object {$_.id -ceq $Scenario})[0]
$taskRequirement=@($taskDefinition.cases|Where-Object {$_.id -ceq $Case})[0].requirements
$taskExecution=$taskConfig.configurations[$taskRequirement.roles.primary.configuration].execution
$taskOrca=$taskExecution.settings.transport.executable
if (-not $taskExecution.target.environment -or -not [IO.Path]::IsPathFullyQualified($taskOrca) -or -not (Test-Path -LiteralPath $taskOrca -PathType Leaf)) { throw 'Stability requires an absolute Orca executable and fixed environment' }
if ($RequireAdmin) {
    $taskPrincipal = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
    if (-not $taskPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        if (-not $taskConfig.harness.ContainsKey('admin_host')) { throw 'admin_required: isolated dual-path network injection requires elevated firewall management' }
        $taskReady=Get-Content -LiteralPath (Join-Path $taskConfig.harness.admin_host 'ready.json') -Raw|ConvertFrom-Json
        $taskProcess=Get-Process -Id $taskReady.pid -ErrorAction Stop
        if(-not $taskReady.elevated -or $taskProcess.StartTime.ToUniversalTime().ToFileTimeUtc().ToString() -ne $taskReady.created){throw 'Elevated network host identity changed'}
    }
    if ((Get-Service MpsSvc).Status -ne 'Running') { throw 'Windows firewall must already be running' }
}
