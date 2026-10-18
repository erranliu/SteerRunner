param(
    [Parameter(Mandatory)][string]$Workspace,
    [Parameter(Mandatory)][string]$EvidenceDirectory,
    [string]$OrcaExecutable = 'orca',
    [ValidateRange(5, 120)][int]$TimeoutSeconds = 90
)
$ErrorActionPreference = 'Stop'
foreach ($taskRouteName in @('ORCA_ENVIRONMENT','ORCA_PAIRING_CODE','ORCA_REMOTE_PAIRING')) {
    if ([Environment]::GetEnvironmentVariable($taskRouteName)) { throw "Local P0 refuses inherited Orca routing: $taskRouteName" }
}
$Workspace = (Resolve-Path -LiteralPath $Workspace).Path
$taskTemporaryRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\', '/')
if (-not ([IO.Path]::GetDirectoryName($Workspace)).Equals($taskTemporaryRoot, [StringComparison]::OrdinalIgnoreCase) -or
    [IO.Path]::GetFileName($Workspace) -notmatch '^steerrunner-orca-pi-p0-[a-f0-9]{32}$') {
    throw 'Use an isolated steerrunner-orca-pi-p0-<UUID> repository directly below the user temporary directory.'
}
$taskRemotes = @(& git -C $Workspace remote)
if ($LASTEXITCODE -ne 0 -or $taskRemotes.Count) { throw 'P0 requires an isolated local repository without remotes.' }
$EvidenceDirectory = [IO.Path]::GetFullPath($EvidenceDirectory)
if (Test-Path -LiteralPath $EvidenceDirectory) { throw 'Use a fresh evidence directory; never overwrite an unknown probe.' }
New-Item -ItemType Directory -Path $EvidenceDirectory | Out-Null
$taskExtension = Join-Path $PSScriptRoot '../internal/execution/adapters/orcatransport/extensions/pi-evidence.mjs'
$taskExtensionTest = Join-Path $PSScriptRoot '../internal/execution/adapters/orcatransport/extensions/pi-evidence.test.mjs'
& node --test $taskExtensionTest 1> (Join-Path $EvidenceDirectory 'extension-tests.log') 2>&1
if ($LASTEXITCODE -ne 0) { throw 'Pi extension preflight failed; no Orca resource created.' }
$taskPrivateExtension = Join-Path $EvidenceDirectory 'pi-evidence.mjs'
Copy-Item -LiteralPath (Join-Path $PSScriptRoot '../internal/execution/adapters/orcatransport/extensions/receipt.mjs') -Destination (Join-Path $EvidenceDirectory 'receipt.mjs')
Copy-Item -LiteralPath $taskExtension -Destination $taskPrivateExtension
$taskExtensionDigest = (Get-FileHash -LiteralPath $taskPrivateExtension -Algorithm SHA256).Hash.ToLowerInvariant()
$taskSession = [guid]::NewGuid().ToString()
$taskRun = [guid]::NewGuid().ToString('N')
$taskConfig = Join-Path $EvidenceDirectory 'config.json'
@{ version = 1; run_id = $taskRun; session_id = $taskSession; cwd = $Workspace; directory = $EvidenceDirectory } |
    ConvertTo-Json -Compress | Set-Content -LiteralPath $taskConfig -Encoding utf8NoBOM

function Invoke-ProbeOrca([string[]]$Arguments) {
    $taskOutput = & $OrcaExecutable @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Orca failed for $($Arguments[0]) $($Arguments[1]): $taskOutput" }
    $taskResult = $taskOutput | ConvertFrom-Json
    if (-not $taskResult.ok) { throw "Orca rejected $($Arguments[0]) $($Arguments[1])" }
    return $taskResult
}
function Read-ProbeReceipt([string]$Name, [string]$Stage) {
    $taskUntil = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    do {
        $taskFile = Join-Path $EvidenceDirectory ($Name + '.json')
        if (Test-Path -LiteralPath $taskFile) {
            $taskInfo = Get-Item -LiteralPath $taskFile
            if ($taskInfo.Length -gt 65536 -or ($taskInfo.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Receipt exceeds its bound or was redirected.' }
            $taskStream = [IO.File]::OpenRead($taskFile)
            try {
                if ($taskStream.Length -gt 65536) { throw 'Receipt exceeds its read bound.' }
                $taskBytes = [byte[]]::new([int]$taskStream.Length)
                $taskStream.ReadExactly($taskBytes)
                $taskFact = [Text.UTF8Encoding]::new($false, $true).GetString($taskBytes) | ConvertFrom-Json
            } finally { $taskStream.Dispose() }
            if ($taskFact.version -ne 1 -or $taskFact.source -ne 'pi-extension-p0' -or
                $taskFact.run_id -ne $taskRun -or $taskFact.session_id -ne $taskSession -or
                $taskFact.source_sha256 -ne $taskExtensionDigest -or $taskFact.mode -ne 'tui' -or
                $taskFact.tools.Count -ne 0 -or -not $taskFact.instance_id -or
                ($taskInstanceID -and $taskFact.instance_id -ne $taskInstanceID) -or
                ($taskPID -and $taskFact.pid -ne $taskPID)) { throw 'Original Run/session/extension/process binding changed.' }
            if ($taskFact.stage -eq 'unverified') { throw "Probe became unverified: $($taskFact.failure)" }
            if ($taskFact.stage -eq $Stage) { return $taskFact }
        }
        Start-Sleep -Milliseconds 200
    } while ([DateTime]::UtcNow -lt $taskUntil)
    throw "Original probe receipt timed out: $Name / $Stage; no input was resent."
}
function Get-ProbeHash([string]$Value) {
    return [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($Value))).ToLowerInvariant()
}
function Quote-ProbeLiteral([string]$Value) { return "'" + $Value.Replace("'", "''") + "'" }
function Write-ProbeIntent([string]$Name, $Value) {
    $taskBytes = [Text.Encoding]::UTF8.GetBytes(($Value | ConvertTo-Json -Compress -Depth 8))
    $taskFile = [IO.File]::Open((Join-Path $EvidenceDirectory ($Name + '.json')), [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
    try { $taskFile.Write($taskBytes); $taskFile.Flush($true) } finally { $taskFile.Dispose() }
}
function Get-ProbeGlobalConfigHashes {
    $taskHashes = @{}
    $taskAgentDirectory = if ($env:PI_CODING_AGENT_DIR) { $env:PI_CODING_AGENT_DIR } else { Join-Path $env:USERPROFILE '.pi/agent' }
    foreach ($taskName in @('settings.json','models.json','auth.json')) {
        $taskPath = Join-Path $taskAgentDirectory $taskName
        $taskHashes[$taskName] = if (Test-Path -LiteralPath $taskPath) { (Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash } else { 'absent' }
    }
    return $taskHashes
}

$taskHandle = $null
$taskPID = $null
$taskInstanceID = $null
$taskGlobalConfigBefore = Get-ProbeGlobalConfigHashes
$taskSummary = [ordered]@{ status = 'failed'; production_enabled = $false; run_id = $taskRun; session_id = $taskSession; source_sha256 = $taskExtensionDigest; cases = @('extension_preflight') }
try {
    $taskPi = (Get-Command pi.cmd -ErrorAction Stop).Source
    $taskPiVersion = (& $taskPi --version | Out-String).Trim()
    $taskStatus = Invoke-ProbeOrca -Arguments @('status','--json')
    if ($taskPiVersion -ne '0.87.1' -or $taskStatus.result.runtime.appVersion -ne '1.4.224' -or $taskStatus.result.target.kind -ne 'local') {
        throw 'This P0 probe is qualified only against Pi 0.87.1 / Orca 1.4.224; review native APIs for the actual versions first.'
    }
    $taskSummary.versions = @{ pi=$taskPiVersion; orca=$taskStatus.result.runtime.appVersion; runtime_id=$taskStatus.result.runtime.runtimeId }
    # No auth values are copied or printed. The original model access is inherited.
    # The no-discovery flags select exactly this managed extension and no tools.
    $taskCommand = '$env:PI_OFFLINE=''1''; $env:PI_TELEMETRY=''0''; & ' + (Quote-ProbeLiteral $taskPi) +
        ' --offline --thinking off --no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-themes --no-approve' +
        ' --session-id ' + (Quote-ProbeLiteral $taskSession) +
        ' --session-dir ' + (Quote-ProbeLiteral (Join-Path $EvidenceDirectory 'sessions')) +
        ' -e ' + (Quote-ProbeLiteral $taskPrivateExtension) +
        ' --steerrunner-evidence-config ' + (Quote-ProbeLiteral $taskConfig)
    Write-ProbeIntent 'create-intent' @{ run_id=$taskRun; source_sha256=$taskExtensionDigest; workspace=$Workspace; command=$taskCommand }
    $taskSummary.terminal_create_attempted = $true
    $taskCreate = Invoke-ProbeOrca -Arguments @('terminal','create','--worktree',('path:' + $Workspace),'--shell','powershell.exe','--title',('Runner Pi P0 ' + $taskRun.Substring(0,8)),'--command',$taskCommand,'--json')
    $taskCreate | ConvertTo-Json -Depth 30 | Set-Content (Join-Path $EvidenceDirectory 'terminal-create.json')
    $taskHandle = $taskCreate.result.terminal.handle
    if (-not $taskHandle) { $taskHandle = $taskCreate.result.handle }
    if (-not $taskHandle) { throw 'Terminal create did not return a handle; inspect the original create record, do not create again.' }
    $taskReady = Read-ProbeReceipt 'readiness' 'ready'
    $taskPID = $taskReady.pid
    $taskInstanceID = $taskReady.instance_id
    if ($taskReady.mode -ne 'tui' -or $taskReady.tools.Count -ne 0 -or $taskReady.session_id -ne $taskSession) { throw 'Native TUI/tool/session binding failed.' }
    $taskProcess = Get-CimInstance Win32_Process -Filter "ProcessId=$taskPID"
    if (-not $taskProcess) { throw 'Original Pi process not found.' }
    $taskSummary.process = @{ pid=$taskPID; created_at=$taskProcess.CreationDate.ToUniversalTime().ToString('o') }
    $taskWait = Invoke-ProbeOrca -Arguments @('terminal','wait','--terminal',$taskHandle,'--for','tui-idle','--timeout-ms','20000','--json')
    $taskWait | ConvertTo-Json -Depth 30 | Set-Content (Join-Path $EvidenceDirectory 'terminal-ready.json')
    if (-not $taskWait.result.wait.satisfied) { throw 'Orca did not confirm the original TUI was ready; no input sent.' }
    $taskSummary.cases += 'native_tui_no_tools'
    $taskPrompt = 'Reply with exactly STEERRUNNER_PI_P0_OK. Do not add other text.'
    $taskPromptHash = Get-ProbeHash $taskPrompt
    $taskFirstWire = $null
    $taskFirstRequest = $null
    $taskFirstCommand = $null
    foreach ($taskIndex in 1..2) {
        Read-ProbeReceipt 'readiness' 'ready' | Out-Null
        $taskCommandID = [guid]::NewGuid().ToString('N')
        $taskRequest = [guid]::NewGuid().ToString()
        $taskWire = "[SteerRunner command=$taskCommandID input=$taskPromptHash]`n$taskPrompt"
        $taskSendArgs = @('terminal','send','--terminal',$taskHandle,'--text',$taskWire,'--enter','--wait-submit','1','--retry-request',$taskRequest,'--json')
        Write-ProbeIntent "input-intent-$taskIndex" @{ command_id=$taskCommandID; request_id=$taskRequest; terminal=$taskHandle; wire=$taskWire; wire_sha256=(Get-ProbeHash $taskWire) }
        Invoke-ProbeOrca -Arguments @('terminal','read','--terminal',$taskHandle,'--limit','1000','--json') | Out-Null
        $taskSend = Invoke-ProbeOrca -Arguments $taskSendArgs
        $taskSend | ConvertTo-Json -Depth 30 | Set-Content (Join-Path $EvidenceDirectory "send-$taskIndex.json")
        # Deliberately discard the first client response for decision-making, then
        # query exactly the original durable request. This must not inject again.
        $taskReplay = Invoke-ProbeOrca -Arguments $taskSendArgs
        $taskReplay | ConvertTo-Json -Depth 30 | Set-Content (Join-Path $EvidenceDirectory "replay-$taskIndex.json")
        if (-not $taskReplay.result.mutation.replayed -or
            $taskReplay.result.send.prompt.requestId -ne $taskRequest -or
            $taskReplay.result.send.prompt.processIncarnation -ne $taskSend.result.send.prompt.processIncarnation) {
            throw 'Orca did not reconcile the original request/incarnation.'
        }
        $taskFinal = Read-ProbeReceipt $taskCommandID 'settled'
        if ($taskFinal.outcome -ne 'completed' -or $taskFinal.stop_reason -ne 'stop' -or
            $taskFinal.reply -ne 'STEERRUNNER_PI_P0_OK' -or $taskFinal.wire_sha256 -ne (Get-ProbeHash $taskWire) -or
            -not $taskFinal.native_input_id -or -not $taskFinal.native_reply_id) {
            $taskSummary.native_outcome = $taskFinal.outcome
            $taskSummary.native_stop_reason = $taskFinal.stop_reason
            $taskSummary.error_kind = $taskFinal.error_kind
            throw 'Native final evidence failed; no production input authorized.'
        }
        # A fresh reader process only reads the original file, never the TUI.
        $taskFinalPath = Join-Path $EvidenceDirectory ($taskCommandID + '.json')
        $taskHashBefore = (Get-FileHash -LiteralPath $taskFinalPath).Hash
        $taskChildCheck = '$v = Get-Content -LiteralPath ' + (Quote-ProbeLiteral $taskFinalPath) + ' -Raw | ConvertFrom-Json; if ($v.stage -ne ''settled'') { exit 1 }; $v.native_input_id'
        $taskReread = & pwsh -NoProfile -Command $taskChildCheck
        if ($LASTEXITCODE -ne 0 -or $taskReread -ne $taskFinal.native_input_id) { throw 'Fresh reader could not recover original evidence.' }
        if ((Get-FileHash -LiteralPath $taskFinalPath).Hash -ne $taskHashBefore) { throw 'Read changed the original result.' }
        if ($taskIndex -eq 1) { $taskFirstWire=$taskWire; $taskFirstCommand=$taskCommandID }
        $taskSummary.cases += "unique_input_settled_and_readonly_recovery_$taskIndex"
    }
    # A new request reusing an already executed command is ambiguous. The
    # extension refuses it and preserves the already frozen result bytes.
    $taskFirstPath = Join-Path $EvidenceDirectory ($taskFirstCommand + '.json')
    $taskFrozen = (Get-FileHash -LiteralPath $taskFirstPath).Hash
    $taskDuplicateRequest = [guid]::NewGuid().ToString()
    Write-ProbeIntent 'duplicate-intent' @{ command_id=$taskFirstCommand; request_id=$taskDuplicateRequest; terminal=$taskHandle; wire=$taskFirstWire }
    Invoke-ProbeOrca -Arguments @('terminal','send','--terminal',$taskHandle,'--text',$taskFirstWire,'--enter','--retry-request',$taskDuplicateRequest,'--json') | Out-Null
    $taskUntil = [DateTime]::UtcNow.AddSeconds(10)
    do {
        $taskInvalid = Get-Content (Join-Path $EvidenceDirectory 'readiness.json') -Raw | ConvertFrom-Json
        if ($taskInvalid.stage -eq 'unverified') { break }
        Start-Sleep -Milliseconds 200
    } while ([DateTime]::UtcNow -lt $taskUntil)
    if ($taskInvalid.failure -ne 'unassociated_or_duplicate_input' -or (Get-FileHash $taskFirstPath).Hash -ne $taskFrozen) { throw 'Duplicate input was not fenced or mutated a frozen result.' }
    $taskSummary.cases += 'duplicate_command_fenced_frozen_result_unchanged'
    $taskRebind = Invoke-ProbeOrca -Arguments @('terminal','show','--terminal',$taskHandle,'--json')
    $taskRebind | ConvertTo-Json -Depth 30 | Set-Content (Join-Path $EvidenceDirectory 'terminal-rebind.json')
    if ($taskRebind._meta.runtimeId -ne $taskCreate._meta.runtimeId -or
        $taskRebind.result.terminal.incarnationId -ne $taskCreate.result.terminal.incarnationId -or
        $taskRebind.result.terminal.worktreeId -ne $taskCreate.result.terminal.worktreeId) {
        throw 'Original Orca runtime/terminal incarnation/workspace changed.'
    }
    $taskOriginalAgain = Get-CimInstance Win32_Process -Filter "ProcessId=$taskPID"
    if (-not $taskOriginalAgain -or $taskOriginalAgain.CreationDate -ne $taskProcess.CreationDate) { throw 'Original TUI writer changed during readonly reattachment.' }
    $taskSummary.cases += 'original_live_writer_identity_rechecked'
    $taskSummary.status = 'passed_probe'
} catch {
    $taskSummary.error = $_.Exception.Message
} finally {
    if ($taskSummary.terminal_create_attempted -and -not $taskHandle) {
        $taskSummary.status='cleanup_unverified'
        $taskSummary.cleanup_error='Original terminal create outcome is unknown. Reconcile the recorded Run title and runtime before another probe; no input was sent.'
    }
    if ($taskHandle) {
        try {
            $taskClose = Invoke-ProbeOrca -Arguments @('terminal','close','--terminal',$taskHandle,'--json')
            $taskClose | ConvertTo-Json -Depth 30 | Set-Content (Join-Path $EvidenceDirectory 'terminal-close.json')
            if ($taskPID) {
                $taskUntil = [DateTime]::UtcNow.AddSeconds(10)
                while ((Get-Process -Id $taskPID -ErrorAction SilentlyContinue) -and [DateTime]::UtcNow -lt $taskUntil) { Start-Sleep -Milliseconds 200 }
                if (Get-Process -Id $taskPID -ErrorAction SilentlyContinue) { throw 'Original no-tool Pi writer still exists after close.' }
                $taskSummary.cases += 'original_no_tool_writer_exited'
            }
        } catch { $taskSummary.status='cleanup_unverified'; $taskSummary.cleanup_error=$_.Exception.Message }
    }
    $taskGlobalConfigAfter = Get-ProbeGlobalConfigHashes
    $taskSummary.global_config_unchanged = @($taskGlobalConfigBefore.Keys | Where-Object { $taskGlobalConfigBefore[$_] -ne $taskGlobalConfigAfter[$_] }).Count -eq 0
    if (-not $taskSummary.global_config_unchanged) { $taskSummary.status='failed'; $taskSummary.config_error='Pi global configuration changed during the probe; no automatic overwrite was attempted.' }
    $taskSummary | ConvertTo-Json -Depth 10 | Set-Content (Join-Path $EvidenceDirectory 'summary.json')
}
$taskSummary | ConvertTo-Json -Depth 10
if ($taskSummary.status -ne 'passed_probe') { exit 1 }
