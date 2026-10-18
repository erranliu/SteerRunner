param(
    [Parameter(Mandatory=$true)][string]$Scenario,
    [Parameter(Mandatory=$true)][string]$Config,
    [string]$Catalog = '',
    [string]$FixtureParent = 'C:\srt',
    [string]$RecordPath = '',
    [string]$FeedbackQualification = '',
    [string]$Case = 'default',
    [string]$RunnerExe = '',
    [string]$ConfigHelper = '',
    [string]$ConfigurationSHA256 = '',
    [string]$RunnerSHA256 = '',
    [string]$HelperSHA256 = '',
    [string]$RequirementsSHA256 = '',
    [ValidateRange(1,10)][int]$RepeatIndex = 1
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'live-catalog.ps1')
. (Join-Path $PSScriptRoot 'orca-cli-lock.ps1')
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
. (Join-Path $PSScriptRoot 'test-configuration.ps1')
if (-not $RunnerExe) { $RunnerExe = Join-Path $taskRepo 'bin/steerrunner.exe' }
if (-not $ConfigHelper) { $ConfigHelper = Join-Path $taskRepo 'bin/test-config.exe' }
$taskAssessment = Get-SRTestConfiguration -RunnerExe $RunnerExe -Helper $ConfigHelper -Config $Config
foreach ($taskIdentity in @(@{expected=$ConfigurationSHA256;actual=$taskAssessment.source_digest;name='configuration'},@{expected=$RunnerSHA256;actual=$taskAssessment.runner_sha256;name='Runner'},@{expected=$HelperSHA256;actual=$taskAssessment.helper_sha256;name='helper'},@{expected=$RequirementsSHA256;actual=$taskAssessment.requirements_digest;name='requirements'})) {
    if ($taskIdentity.expected -and $taskIdentity.expected -cne $taskIdentity.actual) { throw ('Frozen batch artifact changed during assessment: ' + $taskIdentity.name) }
}
$taskAvailable = @($taskAssessment.cases | Where-Object {$_.scenario_id -ceq $Scenario -and $_.case_id -ceq $Case})
if ($taskAvailable.Count -ne 1 -or $taskAvailable[0].availability -ne 'available') { throw 'Selected case is unavailable; query -ListAvailability for its configuration reasons' }
$taskConfig = $taskAssessment.configuration
$taskRequired = $taskCase[0].requirements
$taskSelected = $taskConfig.configurations[$taskRequired.roles.primary.configuration]
$taskExecution = $taskSelected.execution | ConvertTo-Json -Depth 40 | ConvertFrom-Json -AsHashtable
$taskFixture = $taskConfig.fixture
if ($taskFixture.publication.provider -cne 'command' -or $taskFixture.publication.settings.base_url -cne 'https://github.com') { throw 'Dedicated live fixtures require the explicit GitHub command publication provider' }
$taskAudit = if ($taskScenario.driver -eq 'development-audited') { $taskSelected.workflow.audit_model } else { $null }
$taskOrca = ''; $taskEnvironment = ''; $taskLocalHelper = ''
if ($taskExecution.adapter -eq 'codex-app-server') {
    $taskCodex = $taskExecution.settings.executable
    if (-not [IO.Path]::IsPathFullyQualified($taskCodex) -or -not (Test-Path -LiteralPath $taskCodex -PathType Leaf)) { throw 'Configured native Codex executable is unavailable' }
    $taskVersion = (& $taskCodex --version | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $taskVersion -ne 'codex-cli 0.159.2') { throw 'Local native qualification requires Codex 0.159.2' }
} else {
    $taskOrca = $taskExecution.settings.transport.executable
    $taskEnvironment = if ($taskExecution.target.ContainsKey('environment')) { $taskExecution.target.environment } else { '' }
    if (-not [IO.Path]::IsPathFullyQualified($taskOrca) -or -not (Test-Path -LiteralPath $taskOrca -PathType Leaf)) { throw 'Configured Orca executable is unavailable' }
    $taskLaunch = $taskExecution.settings.agent_launch
    if ($taskExecution.target.kind -eq 'local') { $taskLocalHelper = $taskLaunch.runtime_executable }
    # Source hash is verified, never silently replaced by an installed build.
    if ($taskLocalHelper -and ((Get-FileHash -LiteralPath $taskLocalHelper -Algorithm SHA256).Hash.ToLowerInvariant() -cne $taskLaunch.runtime_sha256)) { throw 'Configured helper source hash changed' }
}
function Write-JSON([string]$Path, $Value) { [IO.File]::WriteAllText($Path, ($Value | ConvertTo-Json -Depth 60), [Text.UTF8Encoding]::new($false)) }
function Invoke-Git([string[]]$Arguments) {
    $taskOutput = & git -c core.longpaths=true @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) { throw ('Fixture Git failed: ' + ($Arguments -join ' ') + ': ' + ($taskOutput | Out-String).Trim()) }
    return ($taskOutput | Out-String).Trim()
}
if (-not $FeedbackQualification -and $taskConfig.ContainsKey('harness') -and $taskConfig.harness.ContainsKey('feedback_qualification')) { $FeedbackQualification = $taskConfig.harness.feedback_qualification }
if ($taskStability) { & (Join-Path $PSScriptRoot 'stability-preflight.ps1') -Config $Config -Scenario $Scenario -Case $Case -RunnerExe $RunnerExe -ConfigHelper $ConfigHelper -RequireAdmin:(Test-SRAdminCase $Scenario $Case) }
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
    $taskProbeCall = Invoke-SRLockedOrca -Executable $taskNode -Arguments @($taskCLI,'repo','list','--environment',$taskEnvironment,'--json')
    $taskProbe = $taskProbeCall.output | ConvertFrom-Json
    if ($taskProbeCall.exit_code -ne 0 -or -not $taskProbe.ok) { throw 'Private native Orca runtime failed preflight' }
    $taskExecution.settings.transport.executable = $taskHelper
    $taskExecution.settings.transport['args'] = @('audit','-manifest',(Join-Path $taskRoot 'manifest.json'),'--')
}
# A test-only forwarding CLI records actual local sends for all agents and can
# withhold one real business-send receipt. It never replaces Orca execution.
if ($taskScenario.target -eq 'orca-local') {
    $taskLocalAudit = Join-Path $taskRoot 'bin/local-audit.exe'
    Push-Location $taskRepo
    try { & go build -trimpath -o $taskLocalAudit ./tests/helpers/stability; if ($LASTEXITCODE -ne 0) { throw 'Local audit helper build failed' } } finally { Pop-Location }
    $taskExecution.settings.transport.executable = $taskLocalAudit
    $taskExecution.settings.transport['args'] = @('local-audit','-manifest',(Join-Path $taskRoot 'manifest.json'),'-orca',$taskOrca,'--')
}
$taskSourceRevision = $taskFixture.repository.base_revision
$taskRepository = Join-Path $taskRoot 'SteerlineTest'
$null = Invoke-Git @('clone','--quiet','--no-hardlinks',$taskFixture.repository.url,$taskRepository)
$null = Invoke-Git @('-C',$taskRepository,'checkout','-b',$taskBase,$taskSourceRevision)
$taskToolDir = Join-Path $taskRepository 'sr-smoke'
New-Item -ItemType Directory -Path $taskToolDir | Out-Null
$taskTemplates = Join-Path $taskRepo 'tests/fixtures/evidence-smoke'
$taskTemplateNames = @('spec.md','plan.md','check.ps1','publish.ps1','pr-list.ps1')
if ($taskScenario.level -eq 'L2') { $taskTemplateNames = @('spec.md','plan.md','check.ps1') }
foreach ($taskName in $taskTemplateNames) {
    $taskText = [IO.File]::ReadAllText((Join-Path $taskTemplates $taskName))
    # Windows PowerShell 5 requires a BOM for non-ASCII source literals.
    [IO.File]::WriteAllText((Join-Path $taskToolDir $taskName),$taskText,[Text.UTF8Encoding]::new($taskName.EndsWith('.ps1')))
}
if ($taskScenario.level -ne 'L2') { Write-JSON (Join-Path $taskToolDir 'publication.json') @{fixture_id=$taskID;base_branch=$taskBase} }
if ($taskStability) {
    foreach ($taskName in @('control.ps1','check.ps1','publish.ps1')) {
        $taskText = [IO.File]::ReadAllText((Join-Path $taskRepo ('tests/fixtures/evidence-stability/' + $taskName)))
        [IO.File]::WriteAllText((Join-Path $taskToolDir $taskName),$taskText,[Text.UTF8Encoding]::new($true))
    }
    $taskGate = if ($Scenario -eq 'orca-remote.publication-unknown') { 'publication' } elseif ($Scenario -eq 'orca-remote.caller-exit') { 'none' } else { 'implementation' }
    Write-JSON (Join-Path $taskToolDir 'stability.json') @{fixture_id=$taskID;case_id=$Case;gate=$taskGate;gate_timeout_seconds=600}
    [IO.File]::AppendAllText((Join-Path $taskRepository '.gitignore'),"`n.sr-stability-*/`n",[Text.UTF8Encoding]::new($false))
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
if ($taskScenario.level -eq 'L2') {
    # Preserve a real packed reference fixture; the worker must still commit
    # without root-level packed-refs.lock permission errors.
    $null = Invoke-Git @('-C',$taskRepository,'pack-refs','--all')
    if (-not (Test-Path -LiteralPath (Join-Path $taskRepository '.git/packed-refs') -PathType Leaf)) { throw 'L2 packed reference fixture is missing' }
}
$taskExe = Join-Path $taskRoot 'bin/steerrunner.exe'
$taskFrozenConfig = Join-Path $taskRoot 'bin/test-config.json'
Copy-Item -LiteralPath $Config -Destination $taskFrozenConfig
if ((Get-FileHash -LiteralPath $taskFrozenConfig -Algorithm SHA256).Hash.ToLowerInvariant() -cne $taskAssessment.source_digest) { throw 'Frozen test configuration changed during preparation' }
Copy-Item -LiteralPath $RunnerExe -Destination $taskExe
Copy-Item -LiteralPath ($RunnerExe + '.build.json') -Destination ($taskExe + '.build.json')
if ((Get-FileHash -LiteralPath $taskExe -Algorithm SHA256).Hash.ToLowerInvariant() -cne $taskAssessment.runner_sha256) { throw 'Selected Runner changed before fixture preparation' }
$taskBuild = Get-Content -LiteralPath ($taskExe + '.build.json') -Raw | ConvertFrom-Json
$taskSpec = Join-Path $taskToolDir 'spec.md'
$taskPlan = Join-Path $taskToolDir 'plan.md'
$taskPublication = $taskFixture.publication | ConvertTo-Json -Depth 40 | ConvertFrom-Json -AsHashtable
$taskPublication.base_branch = $taskBase
# Only the test-owned query instrumentation is replaced. Preserve configured
# provider, repository and authentication references in the public settings.
$taskPublication.settings.query = @{executable='powershell.exe';args=@('-NoProfile','-ExecutionPolicy','Bypass','-File',$taskQuery,'-URL','{pr_url}')}
$taskManifest = [ordered]@{
    schema='steerrunner-test/v1';id=$taskID;level=$taskScenario.level;scenario=$Scenario;driver=$taskScenario.driver;root=$taskRoot
    runner_exe=$taskExe;runner_sha256=$taskBuild.sha256.ToLowerInvariant();source_commit=$taskBuild.source_commit;source_dirty=$taskBuild.source_dirty
    repository_path=$taskRepository;source_revision=$taskSourceRevision;base_branch=$taskBase;base_revision=$taskBaseRevision
    spec_path=$taskSpec;spec_sha256=(Get-FileHash -LiteralPath $taskSpec -Algorithm SHA256).Hash.ToLowerInvariant()
    plan_path=$taskPlan;plan_sha256=(Get-FileHash -LiteralPath $taskPlan -Algorithm SHA256).Hash.ToLowerInvariant()
    execution=$taskExecution;publication=$taskPublication
    configuration_sha256=(Get-FileHash -LiteralPath $taskFrozenConfig -Algorithm SHA256).Hash.ToLowerInvariant();configuration_path=$taskFrozenConfig;workflow=$(if ($taskRequired.operation -eq 'submit') { $taskSelected.workflow } else { $null })
    created_at=[DateTime]::UtcNow.ToString('o')
    requirements_sha256=$taskAssessment.requirements_digest
}
if ($taskAudit) { $taskManifest.driver = 'development-audited'; $taskManifest['audit_model'] = $taskAudit }
if ($Scenario -eq 'orca-local.mixed') {
    $taskPeerSlot = $taskRequired.roles.secondary.configuration
    $taskPeer = $taskConfig.configurations[$taskPeerSlot].execution | ConvertTo-Json -Depth 40 | ConvertFrom-Json -AsHashtable
    $taskPeer.settings.transport = $taskExecution.settings.transport
    $taskManifest['peer_execution'] = $taskPeer
}
if ($taskStability) {
    function Frozen-Tool([string]$Path) { return @{path=$Path;sha256=(Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()} }
    $taskManifest.schema = 'steerrunner-test/v2'
    $taskManifest['case_id'] = $Case
    $taskManifest['repeat_index'] = $RepeatIndex
    $taskManifest['stability'] = @{helper=(Frozen-Tool $taskHelper);firewall=(Frozen-Tool $taskNetwork);native_orca=(Frozen-Tool $taskNode);cli_entry=(Frozen-Tool $taskCLI);runtime_files=(Frozen-Tool $taskRuntimeFiles);diagnostic_orca=$taskOrca;expected_outcome=$taskCase[0].outcome;timeout_minutes=$taskScenario.timeout_minutes}
    if ($taskConfig.harness.ContainsKey('admin_host')) { $taskManifest.stability['admin_host'] = $taskConfig.harness.admin_host }
}
# Persist the intended ref before its first external mutation; uncertain pushes
# retain this identity and are never retried under a replacement fixture name.
Write-JSON (Join-Path $taskRoot 'manifest.json') $taskManifest
$taskPreparation.status = 'pushing-base'
Write-JSON (Join-Path $taskRoot 'preparation.json') $taskPreparation
$null = Invoke-Git @('-C',$taskRepository,'push','origin',('HEAD:refs/heads/' + $taskBase))
if ($taskScenario.target -eq 'orca-environment') {
    $taskRuntime = & (Join-Path $PSScriptRoot 'prepare-native-orca.ps1') -Manifest (Join-Path $taskRoot 'manifest.json') -OrcaExe $taskOrca -Environment $taskEnvironment -Target orca_environment
    $taskManifest['remote_runtime_id'] = [string]$taskRuntime
    Write-JSON (Join-Path $taskRoot 'manifest.json') $taskManifest
} elseif ($taskScenario.target -eq 'orca-local') {
    $taskLocalPreparation = & (Join-Path $PSScriptRoot 'prepare-native-orca.ps1') -Manifest (Join-Path $taskRoot 'manifest.json') -OrcaExe $taskOrca -Target local -RepositoryPath $taskFixture.repository.local_path | Select-Object -Last 1 | ConvertFrom-Json
    $taskPreparation['orca_runtime_id'] = $taskLocalPreparation.runtime_id
    $taskPreparation['orca_repository_path'] = $taskLocalPreparation.repository_path
    $taskManifest['local_runtime_id'] = [string]$taskLocalPreparation.runtime_id
    $taskManifest['local_repository_path'] = [string]$taskLocalPreparation.repository_path
    Write-JSON (Join-Path $taskRoot 'manifest.json') $taskManifest
}
$taskPreparation.status = 'ready'
Write-JSON (Join-Path $taskRoot 'preparation.json') $taskPreparation
Write-Output (Join-Path $taskRoot 'manifest.json')
