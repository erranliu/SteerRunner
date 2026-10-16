param(
    [Parameter(Mandatory=$true)][string]$Scenario,
    [Parameter(Mandatory=$true)][string]$Config,
    [string]$Catalog = '',
    [string]$FixtureParent = 'C:\srt',
    [string]$RecordPath = '',
    [string]$Case = 'default',
    [ValidateRange(1,10)][int]$RepeatIndex = 1
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'live-catalog.ps1')
$taskRepo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $Catalog) { $Catalog = Join-Path $taskRepo 'tests/catalog.json' }
$taskCatalog = Get-Content -LiteralPath $Catalog -Raw | ConvertFrom-Json
$taskScenarios = @($taskCatalog.scenarios | Where-Object { $_.id -eq $Scenario })
if ($taskScenarios.Count -ne 1 -or $taskScenarios[0].level -notin @('L2','L3')) { throw 'Select exactly one registered live scenario' }
$taskScenario = $taskScenarios[0]
$taskCases = @(Get-SRLiveCases $taskScenario)
$taskCase = @($taskCases | Where-Object { $_.id -ceq $Case })
if ($taskCase.Count -ne 1) { throw 'Select an exact registered case; multi-case scenarios require -Case' }
$taskStability = $taskScenario.driver -eq 'execution-stability'
if ($taskStability) { & (Join-Path $PSScriptRoot 'stability-preflight.ps1') -Config $Config -RequireAdmin:(Test-SRAdminCase $Scenario $Case) }
$taskConfig = Get-Content -LiteralPath $Config -Raw | ConvertFrom-Json
if ($taskConfig.schema -notin @('steerrunner-live-config/v1','steerrunner-live-config/v2')) { throw 'Unsupported live config' }
function Read-Setting([string]$Name, [string]$Default = '') {
    $taskProperty = $taskConfig.PSObject.Properties[$Name]
    if ($null -eq $taskProperty -or -not $taskProperty.Value) { return $Default }
    return [string]$taskProperty.Value
}
function Write-JSON([string]$Path, $Value) { [IO.File]::WriteAllText($Path, ($Value | ConvertTo-Json -Depth 30), [Text.UTF8Encoding]::new($false)) }
function Invoke-Git([string[]]$Arguments) {
    $taskOutput = & git -c core.longpaths=true @Arguments
    if ($LASTEXITCODE -ne 0) { throw ('Fixture Git failed: ' + ($Arguments -join ' ')) }
    return ($taskOutput | Out-String).Trim()
}
$taskExecution = $null
if ($taskScenario.target -eq 'local-codex') {
    $taskCodex = Read-Setting 'codex_executable'
    if (-not [IO.Path]::IsPathRooted($taskCodex) -or -not (Test-Path -LiteralPath $taskCodex -PathType Leaf)) { throw 'Configure the absolute native Codex executable' }
    $taskVersion = (& $taskCodex --version | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $taskVersion -ne 'codex-cli 0.159.2') { throw 'Local native qualification requires Codex 0.159.2' }
    $taskExecution = @{adapter='codex-app-server'; target=@{kind='local'}; settings=@{executable=$taskCodex; model=(Read-Setting 'model'); reasoning_effort=(Read-Setting 'reasoning_effort' 'medium')}}
} elseif ($taskScenario.target -eq 'orca-environment') {
    $taskOrca = Read-Setting 'orca_executable'
    $taskEnvironment = Read-Setting 'orca_environment'
    if (-not [IO.Path]::IsPathRooted($taskOrca) -or -not (Test-Path -LiteralPath $taskOrca -PathType Leaf) -or -not $taskEnvironment) { throw 'Configure a selected Orca executable and remote environment' }
    $taskRemoteModel = Read-Setting 'model'
    if ($taskConfig.PSObject.Properties['remote_model']) { $taskRemoteModel = [string]$taskConfig.remote_model }
    $taskRemoteEffort = Read-Setting 'reasoning_effort' 'medium'
    if ($taskConfig.PSObject.Properties['remote_reasoning_effort']) { $taskRemoteEffort = [string]$taskConfig.remote_reasoning_effort }
    $taskLaunch = @{executable=(Read-Setting 'remote_agent' 'codex')}
    if ($taskRemoteModel) {
        $taskLaunch['model'] = $taskRemoteModel
        if ($taskRemoteEffort) { $taskLaunch['reasoning_effort'] = $taskRemoteEffort }
    }
    if ($taskConfig.PSObject.Properties['remote_args']) {
        $taskRemoteArgs = @($taskConfig.remote_args)
        foreach ($taskArg in $taskRemoteArgs) { if ($taskArg -isnot [string] -or $taskArg -match '[\x00\r\n]') { throw 'remote_args must be a string array' } }
        $taskLaunch['args'] = $taskRemoteArgs
    }
    $taskExecution = @{adapter='orca-terminal';target=@{kind='orca_environment';environment=$taskEnvironment};settings=@{transport=@{executable=$taskOrca;wait_timeout='40s'};agent_launch=$taskLaunch}}
} else { throw 'Unregistered execution target' }
if ($taskScenario.target -eq 'local-codex' -and -not (Read-Setting 'model')) { $taskExecution.settings.Remove('model') }
& gh auth status --active --hostname github.com 2>&1 | Out-Host
if ($LASTEXITCODE -ne 0) { throw 'Host GitHub login is unavailable' }
$taskID = [Guid]::NewGuid().ToString('N').Substring(0,12)
$taskRoot = Join-Path ([IO.Path]::GetFullPath($FixtureParent)) $taskID
if (Test-Path -LiteralPath $taskRoot) { throw 'Fixture directory must be new' }
New-Item -ItemType Directory -Path $taskRoot | Out-Null
foreach ($taskDir in @('bin','evidence')) { New-Item -ItemType Directory -Path (Join-Path $taskRoot $taskDir) | Out-Null }
$taskBase = 'codex/srtest/' + $taskID + '/base'
$taskPreparation = @{schema='steerrunner-test-preparation/v1';fixture_id=$taskID;scenario=$Scenario;root=$taskRoot;base_branch=$taskBase;status='preparing'}
Write-JSON (Join-Path $taskRoot 'preparation.json') $taskPreparation
if ($RecordPath) { Write-JSON ([IO.Path]::GetFullPath($RecordPath)) @{root=$taskRoot;manifest=(Join-Path $taskRoot 'manifest.json');preparation=(Join-Path $taskRoot 'preparation.json')} }
Write-Host "Preparing $Scenario in $taskRoot"
if ($taskStability) {
    $taskHelper = Join-Path $taskRoot 'bin/stability.exe'
    Push-Location $taskRepo
    try { & go build -trimpath -o $taskHelper ./tests/helpers/stability; if ($LASTEXITCODE -ne 0) { throw 'Stability helper build failed' } } finally { Pop-Location }
    $taskNetwork = Join-Path $taskRoot 'bin/network.ps1'
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'stability-network.ps1') -Destination $taskNetwork
    # The real packaged CLI runs under a private Node executable, without copying authentication.
    $taskRuntime = Join-Path $taskRoot 'bin/orca-runtime'
    $taskRuntimeFiles = & (Join-Path $PSScriptRoot 'prepare-test-orca-runtime.ps1') -OrcaExe $taskOrca -Destination $taskRuntime
    $taskNode = Join-Path $taskRuntime 'node.exe'
    $taskCLI = Join-Path $taskRuntime 'out/cli/index.js'
    $taskProbe = & $taskNode $taskCLI repo list --environment $taskEnvironment --json | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or -not $taskProbe.ok) { throw 'Private native Orca runtime failed preflight' }
    $taskExecution.settings.transport.executable = $taskHelper
    $taskExecution.settings.transport['args'] = @('audit','-manifest',(Join-Path $taskRoot 'manifest.json'),'--')
}
$taskSourceRevision = 'cebdd5ab552150c49743e40593c836689ecf789b'
$taskRepository = Join-Path $taskRoot 'SteerlineTest'
$null = Invoke-Git @('clone','--quiet','--no-hardlinks','https://github.com/erranliu/SteerlineTest.git',$taskRepository)
$null = Invoke-Git @('-C',$taskRepository,'checkout','-b',$taskBase,$taskSourceRevision)
$taskToolDir = Join-Path $taskRepository 'sr-smoke'
New-Item -ItemType Directory -Path $taskToolDir | Out-Null
$taskTemplates = Join-Path $taskRepo 'tests/fixtures/evidence-smoke'
foreach ($taskName in @('spec.md','plan.md','check.ps1','publish.ps1','pr-list.ps1')) {
    $taskText = [IO.File]::ReadAllText((Join-Path $taskTemplates $taskName))
    # Windows PowerShell 5 requires a BOM for non-ASCII source literals.
    [IO.File]::WriteAllText((Join-Path $taskToolDir $taskName),$taskText,[Text.UTF8Encoding]::new($taskName.EndsWith('.ps1')))
}
Write-JSON (Join-Path $taskToolDir 'publication.json') @{fixture_id=$taskID;base_branch=$taskBase}
if ($taskStability) {
    foreach ($taskName in @('control.ps1','check.ps1','publish.ps1')) {
        $taskText = [IO.File]::ReadAllText((Join-Path $taskRepo ('tests/fixtures/evidence-stability/' + $taskName)))
        [IO.File]::WriteAllText((Join-Path $taskToolDir $taskName),$taskText,[Text.UTF8Encoding]::new($true))
    }
    $taskText = [IO.File]::ReadAllText((Join-Path $taskToolDir 'spec.md'))
    $taskText += "`nThe checker and publisher contain bounded test gates. Run each exact command once and wait for completion. The test controller owns gate release; never change or bypass gates, retry a pending command, or start a second checker. Keep the original terminal/session open. Gate timeout is a real source failure.`n"
    [IO.File]::WriteAllText((Join-Path $taskToolDir 'spec.md'),$taskText,[Text.UTF8Encoding]::new($false))
    $taskPlanText=[IO.File]::ReadAllText((Join-Path $taskToolDir 'plan.md'))
    $taskPlanText+="`nFor the exact fixed publication command, request the configured native automatic review BEFORE its first execution: it requires Git/GitHub network access outside the native sandbox. Wait for the review outcome. Do not execute the publisher sandboxed first, do not retry a denied review, and do not alter permissions, authentication or command scope. A denial or unavailable review remains a blocker. The publisher itself must execute only once.`n"
    [IO.File]::WriteAllText((Join-Path $taskToolDir 'plan.md'),$taskPlanText,[Text.UTF8Encoding]::new($false))
    $taskGate = if ($Scenario -eq 'orca-remote.publication-unknown') { 'publication' } elseif ($Scenario -eq 'orca-remote.caller-exit') { 'none' } else { 'implementation' }
    Write-JSON (Join-Path $taskToolDir 'stability.json') @{fixture_id=$taskID;case_id=$Case;gate=$taskGate;gate_timeout_seconds=600}
    [IO.File]::AppendAllText((Join-Path $taskRepository '.gitignore'),"`n.sr-stability-*/`n",[Text.UTF8Encoding]::new($false))
}
if (-not $taskStability -and $taskScenario.target -eq 'orca-environment') {
    $taskPlanText=[IO.File]::ReadAllText((Join-Path $taskToolDir 'plan.md'))
    $taskPlanText+="`nFor the exact fixed publication command, request the configured native automatic review before its first execution because Git/GitHub network access is required outside the native sandbox. Wait for the native decision; denial or unavailable review remains a blocker. Preserve authentication and permissions. Feedback continues on the original session, branch and PR.`n"
    [IO.File]::WriteAllText((Join-Path $taskToolDir 'plan.md'),$taskPlanText,[Text.UTF8Encoding]::new($false))
}
$taskQuery = Join-Path $taskRoot 'bin/query-pr.ps1'
Copy-Item -LiteralPath (Join-Path $taskTemplates 'query-pr.ps1') -Destination $taskQuery
if ($taskStability) {
    Copy-Item -LiteralPath $taskQuery -Destination (Join-Path $taskRoot 'bin/query-pr-real.ps1')
    $taskQuerySource = @'
param([Parameter(Mandatory=$true)][string]$URL)
$ErrorActionPreference='Stop'
$taskInjected=Test-Path -LiteralPath (Join-Path $PSScriptRoot 'query-unavailable')
$taskRecord=@{at=[DateTime]::UtcNow.ToString('o');url=$URL;injected=[bool]$taskInjected}|ConvertTo-Json -Compress
[IO.File]::AppendAllText((Join-Path (Split-Path -Parent $PSScriptRoot) 'evidence/provider-queries.jsonl'),($taskRecord+"`n"),[Text.UTF8Encoding]::new($false))
if($taskInjected){throw 'Injected PR query transport unavailable'}
& (Join-Path $PSScriptRoot 'query-pr-real.ps1') -URL $URL
'@
    [IO.File]::WriteAllText($taskQuery,$taskQuerySource,[Text.UTF8Encoding]::new($true))
}
$null = Invoke-Git @('-C',$taskRepository,'add','--','sr-smoke')
if ($taskStability) { $null = Invoke-Git @('-C',$taskRepository,'add','--','.gitignore') }
$null = Invoke-Git @('-C',$taskRepository,'-c','user.name=SteerRunner Fixture','-c','user.email=fixture@example.invalid','commit','--quiet','-m',('Prepare independent smoke ' + $taskID))
$taskBaseRevision = Invoke-Git @('-C',$taskRepository,'rev-parse','HEAD')
$taskExe = Join-Path $taskRoot 'bin/steerrunner.exe'
& (Join-Path $PSScriptRoot 'build.ps1') -OutFile $taskExe | Out-Host
$taskBuild = Get-Content -LiteralPath ($taskExe + '.build.json') -Raw | ConvertFrom-Json
$taskSpec = Join-Path $taskToolDir 'spec.md'
$taskPlan = Join-Path $taskToolDir 'plan.md'
$taskManifest = [ordered]@{
    schema='steerrunner-test/v1';id=$taskID;level=$taskScenario.level;scenario=$Scenario;driver=$taskScenario.driver;root=$taskRoot
    runner_exe=$taskExe;runner_sha256=$taskBuild.sha256.ToLowerInvariant();source_commit=$taskBuild.source_commit;source_dirty=$taskBuild.source_dirty
    repository_path=$taskRepository;source_revision=$taskSourceRevision;base_branch=$taskBase;base_revision=$taskBaseRevision
    spec_path=$taskSpec;spec_sha256=(Get-FileHash -LiteralPath $taskSpec -Algorithm SHA256).Hash.ToLowerInvariant()
    plan_path=$taskPlan;plan_sha256=(Get-FileHash -LiteralPath $taskPlan -Algorithm SHA256).Hash.ToLowerInvariant()
    execution=$taskExecution;publication=@{provider='command';repository='erranliu/SteerlineTest';base_branch=$taskBase;settings=@{base_url='https://github.com';query=@{executable='powershell.exe';args=@('-NoProfile','-ExecutionPolicy','Bypass','-File',$taskQuery,'-URL','{pr_url}')}}}
    created_at=[DateTime]::UtcNow.ToString('o')
}
if ($taskStability) {
    function Frozen-Tool([string]$Path) { return @{path=$Path;sha256=(Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()} }
    $taskManifest.schema = 'steerrunner-test/v2'
    $taskManifest['case_id'] = $Case
    $taskManifest['repeat_index'] = $RepeatIndex
    $taskManifest['stability'] = @{helper=(Frozen-Tool $taskHelper);firewall=(Frozen-Tool $taskNetwork);native_orca=(Frozen-Tool $taskNode);cli_entry=(Frozen-Tool $taskCLI);runtime_files=(Frozen-Tool $taskRuntimeFiles);diagnostic_orca=$taskOrca;expected_outcome=$taskCase[0].outcome;timeout_minutes=$taskScenario.timeout_minutes}
    if ($taskConfig.PSObject.Properties['admin_host']) { $taskManifest.stability['admin_host'] = [IO.Path]::GetFullPath($taskConfig.admin_host) }
}
# Persist the intended ref before its first external mutation; uncertain pushes
# retain this identity and are never retried under a replacement fixture name.
Write-JSON (Join-Path $taskRoot 'manifest.json') $taskManifest
$taskPreparation.status = 'pushing-base'
Write-JSON (Join-Path $taskRoot 'preparation.json') $taskPreparation
$null = Invoke-Git @('-C',$taskRepository,'push','origin',('HEAD:refs/heads/' + $taskBase))
if ($taskScenario.target -eq 'orca-environment') {
    $taskRuntime = & (Join-Path $PSScriptRoot 'prepare-native-orca.ps1') -Manifest (Join-Path $taskRoot 'manifest.json') -OrcaExe $taskOrca -Environment $taskEnvironment
    $taskManifest['remote_runtime_id'] = [string]$taskRuntime
    Write-JSON (Join-Path $taskRoot 'manifest.json') $taskManifest
}
$taskPreparation.status = 'ready'
Write-JSON (Join-Path $taskRoot 'preparation.json') $taskPreparation
Write-Output (Join-Path $taskRoot 'manifest.json')
