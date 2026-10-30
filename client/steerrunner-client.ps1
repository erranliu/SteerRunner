param(
    [ValidateSet('Configure','Discover','Submit','Observe','Inspect','Reply','Resume','Stop','Reconcile','List','Viewer','Evidence')]
    [string]$Action = 'Configure',
    [string]$InputFile = '',
    [string]$ClientRoot = '',
    [string]$ProjectPath = ''
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2
$script:protocol = 'steerrunner-cli/v1'
$script:utf8 = New-Object Text.UTF8Encoding($false,$true)
$script:bundle = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
if (-not $ClientRoot) { $ClientRoot = Join-Path $env:LOCALAPPDATA 'SteerRunnerClient' }
$script:root = [IO.Path]::GetFullPath($ClientRoot)
if (-not $ProjectPath) { $ProjectPath = (Get-Location).Path }
$script:projectPath = [IO.Path]::GetFullPath($ProjectPath)
$script:deadline = [DateTime]::UtcNow.AddSeconds(60)

# PS 5.1 lacks strict JSON and ProcessStartInfo.ArgumentList. Keep these two
# mechanical details here; all product validation remains in the public CLI.
if (-not ('SteerClientWire' -as [type])) {
Add-Type -ReferencedAssemblies System.Web.Extensions -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Text.RegularExpressions;
using System.Web.Script.Serialization;
public static class SteerClientWire {
  static string s; static int p;
  static void WS(){while(p<s.Length && " \t\r\n".IndexOf(s[p])>=0)p++;}
  static string Str(){int start=p++;bool end=false;while(p<s.Length){char c=s[p++];if(c=='"'){end=true;break;}if(c<32)throw new Exception("JSON control");if(c=='\\'){if(p>=s.Length)throw new Exception("JSON escape");char e=s[p++];if(e=='u'){if(p+4>s.Length || !Regex.IsMatch(s.Substring(p,4),"^[0-9a-fA-F]{4}$"))throw new Exception("JSON unicode");p+=4;}else if("\"\\/bfnrt".IndexOf(e)<0)throw new Exception("JSON escape");}}if(!end)throw new Exception("JSON string");return new JavaScriptSerializer().Deserialize<string>(s.Substring(start,p-start));}
  static void Value(int depth){if(depth>100)throw new Exception("JSON depth");WS();if(p>=s.Length)throw new Exception("JSON missing");char c=s[p];if(c=='"'){Str();return;}if(c=='{'||c=='['){p++;WS();char close=c=='{'?'}':']';var keys=new HashSet<string>(StringComparer.Ordinal);if(p<s.Length&&s[p]==close){p++;return;}while(true){WS();if(c=='{'){if(p>=s.Length||s[p]!='"')throw new Exception("JSON key");string k=Str();if(!keys.Add(k))throw new Exception("JSON duplicate key");WS();if(p>=s.Length||s[p++]!=':')throw new Exception("JSON colon");}Value(depth+1);WS();if(p>=s.Length)throw new Exception("JSON close");char next=s[p++];if(next==close)return;if(next!=',')throw new Exception("JSON separator");}}int begin=p;while(p<s.Length&&" \t\r\n,]}".IndexOf(s[p])<0)p++;if(!Regex.IsMatch(s.Substring(begin,p-begin),@"\A(?:true|false|null|-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)\z"))throw new Exception("JSON token");}
  public static object Parse(string text){if(text.Length>8388608)throw new Exception("JSON size");s=text;p=0;Value(0);WS();if(p!=s.Length)throw new Exception("JSON trailing");var j=new JavaScriptSerializer();j.MaxJsonLength=8388608;j.RecursionLimit=110;return j.DeserializeObject(text);}
  public static string Quote(string v){return "\""+Regex.Replace(Regex.Replace(v,@"(\\*)""",m=>new string('\\',m.Groups[1].Length*2+1)+"\""),@"(\\+)$",m=>m.Value+m.Value)+"\"";}
}
'@
}
function Has($map,[string]$key) { return $null -ne $map -and @($map.Keys) -ccontains $key }
function Get($map,[string]$key,$fallback=$null) { if (Has $map $key) { return ,$map[$key] }; return ,$fallback }
function Fail([string]$code,[string]$message) { throw ($code+'|'+$message) }
function JSON($value) { return ConvertTo-Json -InputObject $value -Depth 100 -Compress }
function Parse([string]$text) { try { return ,([SteerClientWire]::Parse($text)) } catch { Fail 'invalid_json' 'Input must be one strict JSON object with no duplicate keys.' } }
function Read-JSON([string]$path) {
    if ((Get-Item -LiteralPath $path).Length -gt 8388608) { Fail 'invalid_json' 'Input file exceeds 8 MiB.' }
    return ,(Parse ([IO.File]::ReadAllText($path,$script:utf8)))
}
function Keys($value,[string[]]$allowed) {
    if ($value -isnot [Collections.IDictionary]) { Fail 'invalid_input' 'Expected a JSON object.' }
    foreach ($key in $value.Keys) { if ($allowed -cnotcontains $key) { Fail 'invalid_input' ('Unknown field: '+$key) } }
}
function Digest([string]$text) { $h=[Security.Cryptography.SHA256]::Create(); try { return ([BitConverter]::ToString($h.ComputeHash($script:utf8.GetBytes($text)))).Replace('-','').ToLowerInvariant() } finally { $h.Dispose() } }
function Atomic([string]$path,$value) {
    $dir=Split-Path -Parent $path; [IO.Directory]::CreateDirectory($dir) | Out-Null
    $temp=Join-Path $dir ([Guid]::NewGuid().ToString('N')+'.tmp')
    try { [IO.File]::WriteAllText($temp,(JSON $value),$script:utf8); if ([IO.File]::Exists($path)) { [IO.File]::Replace($temp,$path,[NullString]::Value) } else { [IO.File]::Move($temp,$path) } }
    finally { if ([IO.File]::Exists($temp)) { [IO.File]::Delete($temp) } }
}
function Lock([string]$path) {
    [IO.Directory]::CreateDirectory((Split-Path -Parent $path)) | Out-Null
    $until=[DateTime]::UtcNow.AddSeconds(2)
    do { try { return [IO.File]::Open($path,[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None) } catch [IO.IOException] { Start-Sleep -Milliseconds 50 } } while ([DateTime]::UtcNow -lt $until)
    Fail 'client_busy' 'Another client is updating this configuration or command.'
}
function Tool([string]$name) { $command=Get-Command $name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1; if ($command) { return $command.Source }; return $null }
function Process([string]$exe,[string[]]$argv,[string]$stdin='',[int]$seconds=30,[switch]$Discovery) {
    if (-not $exe) { return @{exit_code=4; text=''; unknown=$false} }
    if ($Discovery) { $seconds=[Math]::Min($seconds,[Math]::Max(0,[int]($script:deadline-[DateTime]::UtcNow).TotalSeconds)) }
    if ($seconds -le 0) { return @{exit_code=4;text='';unknown=$true} }
    $info=New-Object Diagnostics.ProcessStartInfo
    $info.FileName=$exe; $info.Arguments=(@($argv | ForEach-Object { [SteerClientWire]::Quote([string]$_) }) -join ' ')
    $info.UseShellExecute=$false; $info.CreateNoWindow=$true; $info.WorkingDirectory=$script:projectPath
    $info.RedirectStandardInput=$true; $info.RedirectStandardOutput=$true; $info.RedirectStandardError=$true
    $info.StandardOutputEncoding=$script:utf8; $info.StandardErrorEncoding=$script:utf8
    foreach ($name in @('ORCA_ENVIRONMENT','ORCA_PAIRING_CODE','ORCA_REMOTE_PAIRING')) { $info.EnvironmentVariables.Remove($name) }
    $p=New-Object Diagnostics.Process; $p.StartInfo=$info
    $clock=[Diagnostics.Stopwatch]::StartNew()
    try {
        if (-not $p.Start()) { return @{exit_code=4;text='';unknown=$false} }
        $out=$p.StandardOutput.ReadToEndAsync(); $err=$p.StandardError.ReadToEndAsync()
        # No shell pipe / platform default encoding is involved.
        $bytes=$script:utf8.GetBytes($stdin); $write=$p.StandardInput.BaseStream.WriteAsync($bytes,0,$bytes.Length)
        if (-not $write.Wait($seconds*1000)) { $p.Kill();$p.WaitForExit();return @{exit_code=6;text='';unknown=$true} }
        $null=$write.GetAwaiter().GetResult(); $p.StandardInput.Close()
        $remaining=[Math]::Max(0,$seconds*1000-$clock.ElapsedMilliseconds)
        if (-not $p.WaitForExit([int]$remaining)) { $p.Kill(); $p.WaitForExit(); return @{exit_code=6;text='';unknown=$true} }
        $text=$out.GetAwaiter().GetResult(); $null=$err.GetAwaiter().GetResult()
        return @{exit_code=$p.ExitCode;text=$text.Trim();unknown=$false}
    } catch { return @{exit_code=6;text='';unknown=$true} } finally { $p.Dispose() }
}
function Runner([string]$exe,[string[]]$argv,[string]$body='',[int]$seconds=35) {
    $r=Process $exe (@($argv)+@('--json')) $body $seconds
    try {
        $response=Parse $r.text
        $expectedID=$null
        if ($body -and $argv[0] -ne 'validate') { $request=Parse $body; $expectedID=Get $request 'request_id' }
        if ((Get $response 'protocol') -cne $script:protocol -or (Get $response 'ok') -isnot [bool] -or (Get $response 'request_id') -cne $expectedID) { throw 'invalid' }
        $r.response=$response
    }
    catch { $r.exit_code=6; $r.unknown=$true; $r.response=$null }
    return $r
}
function Require($r) { if ($r.exit_code -ne 0 -or -not $r.response -or -not $r.response.ok) { $code='runner_unavailable'; if ($r.response -and $r.response.error) { $code=$r.response.error.code }; Fail $code 'Runner rejected this operation; inspect the original CLI response or configuration.' }; return ,$r.response.data }
function Validate($config,[string]$kind,$value) { $r=Runner $config.runner_executable @('validate','--kind',$kind,'--input','-') (JSON $value); $null=Require $r }
function Load-Config {
    $path=Join-Path $script:root 'client.json'
    if (-not [IO.File]::Exists($path)) { return @{version=1;runner_executable=$null;state_dir=(Join-Path $env:LOCALAPPDATA 'SteerRunner');default_profile=$null;profiles=@{}} }
    $c=Read-JSON $path; Keys $c @('version','runner_executable','state_dir','default_profile','profiles')
    if ($c.version -ne 1 -or $c.profiles -isnot [Collections.IDictionary] -or -not [IO.Path]::IsPathRooted($c.state_dir)) { Fail 'invalid_configuration' 'Unsupported or damaged client configuration.' }
    return ,$c
}
function Resolve-Runner($config) {
    if ($config.runner_executable) { if (-not [IO.Path]::IsPathRooted($config.runner_executable) -or -not [IO.File]::Exists($config.runner_executable)) { Fail 'configured_runner_missing' 'The selected absolute Runner path is missing; no replacement was selected.' }; return $config.runner_executable }
    $bundle=Join-Path $script:bundle 'steerrunner.exe'; if ([IO.File]::Exists($bundle)) { return $bundle }; return Tool 'steerrunner.exe'
}
function Git([string[]]$commandArguments,[switch]$Discovery) { return Process (Tool 'git.exe') (@('-C',$script:projectPath)+$commandArguments) '' 10 -Discovery:$Discovery }
function Project {
    $r=Git @('rev-parse','--show-toplevel') -Discovery; if ($r.exit_code -ne 0) { return $null }
    $common=Git @('rev-parse','--path-format=absolute','--git-common-dir') -Discovery
    if ($common.exit_code -ne 0) { return $null }
    $identity=[IO.Path]::GetFullPath($common.text).TrimEnd('\','/').ToLowerInvariant()
    return @{root=([IO.Path]::GetFullPath($r.text));common_dir=$identity;key=(Digest $identity)}
}
function Project-File($project) { return Join-Path $script:root ('projects/'+$project.key+'.json') }
function Find-Codex {
    $candidates=@(Get-Command codex.exe -CommandType Application -All -ErrorAction SilentlyContinue | ForEach-Object { $_.Source })
    $shims=@(Get-Command codex,codex.cmd -All -ErrorAction SilentlyContinue)
    $roots=@(); foreach ($shim in $shims) { $roots+=Join-Path (Split-Path -Parent $shim.Source) 'node_modules/@openai/codex' }
    $roots+=Join-Path $env:APPDATA 'npm/node_modules/@openai/codex'
    foreach ($base in $roots) { foreach ($rel in @('vendor/x86_64-pc-windows-msvc/bin/codex.exe','node_modules/@openai/codex-win32-x64/vendor/x86_64-pc-windows-msvc/bin/codex.exe')) { $path=Join-Path $base $rel; if ([IO.File]::Exists($path)) { $candidates+=[IO.Path]::GetFullPath($path) } } }
    return @($candidates | Select-Object -Unique)
}
function GitHub-Publication([string]$hostName,[string]$repository,[string]$branch,[string]$gh) {
    $jq='{url:.html_url,repository:.base.repo.full_name,base_branch:.base.ref,head_repository:.head.repo.full_name,head_branch:.head.ref,head_revision:.head.sha,state:(if .merged then "merged" elif .state=="closed" then "closed_unmerged" else "open" end),merged:.merged,merged_at:.merged_at,merge_commit:.merge_commit_sha}'
    return @{provider='command';repository=$repository;base_branch=$branch;settings=@{base_url=('https://'+$hostName);query=@{executable=$gh;args=@('api','--hostname',$hostName,'repos/{repository}/pulls/{number}','--jq',$jq)}}}
}
function Native-Matches([string]$text,[string[]]$requirements) {
    # Interpret the selected Runner's declaration, never pin a client version.
    $actual=[regex]::Match($text,'\Acodex-cli ((?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*))(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\z')
    if (-not $actual.Success -or $requirements.Count -eq 0) { return $false }
    foreach ($identifier in $actual.Groups[2].Value.Split('.')) { if ($identifier -match '^0[0-9]+$') { return $false } }
    foreach ($requirement in $requirements) {
        $rule=[regex]::Match($requirement,'\Anative_codex_([0-9]+\.[0-9]+\.[0-9]+)(_or_newer)?_permissions\z')
        if (-not $rule.Success) { return $false }
        if (-not $rule.Groups[2].Success) { if ($text -cne ('codex-cli '+$rule.Groups[1].Value)) { return $false }; continue }
        $parts=$actual.Groups[1].Value.Split('.');$minimum=$rule.Groups[1].Value.Split('.');$comparison=0
        for ($index=0;$index -lt 3;$index++) {
            $comparison=$parts[$index].Length.CompareTo($minimum[$index].Length)
            if ($comparison -eq 0) { $comparison=[String]::CompareOrdinal($parts[$index],$minimum[$index]) }
            if ($comparison -ne 0) { break }
        }
        if ($comparison -lt 0 -or ($comparison -eq 0 -and $actual.Groups[2].Success)) { return $false }
    }
    return $true
}
function Discover($config) {
    $report=@{generated_at=[DateTime]::UtcNow.ToString('o');candidates=@();gaps=@();orca=@{installed=$false;environments=@()};project=$null}
    $exe=Resolve-Runner $config
    $caps=$null
    if ($exe) { $v=Runner $exe @('version') '' 5; $c=Runner $exe @('capabilities','--offline') '' 5; if ($v.exit_code -eq 0 -and $c.exit_code -eq 0) { $config.runner_executable=$exe; $caps=$c.response.data } else { $report.gaps+= 'runner_query_failed' } } else { $report.gaps+='runner_missing' }
    $natives=@(Find-Codex);$nativeSupported=$false
    foreach ($native in $natives) {
        if (-not $caps -or [DateTime]::UtcNow -ge $script:deadline) { break }
        $version=Process $native @('--version') '' 5 -Discovery
        $required=@($caps.capability_catalog.combinations | Where-Object { $_.adapter -eq 'codex-app-server' -and $_.workflow -and $_.workflow.id -eq 'development' } | ForEach-Object { $_.requirements } | Where-Object { $_ -match '^native_codex_(.+)_permissions$' } | Select-Object -Unique)
        $supported=$version.exit_code -eq 0 -and (Native-Matches $version.text $required)
        if ($supported) { $profile=@{execution=@{adapter='codex-app-server';target=@{kind='local'};settings=@{executable=$native;access_mode='sandbox'}};workflow=@{id='development';version=2}}; try { Validate $config 'run-configuration' $profile; $report.candidates+=@{id='local-codex';profile=$profile;connected=$true;facts=@('native_entry_found','static_configuration_valid','execution_readiness_unverified')};$nativeSupported=$true;break } catch { $report.gaps+='native_configuration_unsupported' } }
    }
    if (-not $nativeSupported) { $report.gaps+='native_codex_missing_or_version_unverified' }
    $orca=$null
    if ($env:ORCA_CLI_COMMAND) { $orca=$env:ORCA_CLI_COMMAND } elseif ($env:ORCA_DEV_REPO_ROOT) { $orca=Tool 'orca-dev' } else { $orca=Tool 'orca.exe' }
    if ($orca) {
        $report.orca.installed=$true
        $listing=Process $orca @('environment','list','--json') '' 5 -Discovery
        $local=Process $orca @('status','--json') '' 5 -Discovery
        $targets=@(@{id='orca-local';environment='';connected=$false})
        try { $lr=Parse $local.text; $targets[0].connected=($local.exit_code -eq 0 -and $lr.ok) } catch {}
        try {
            $envs=Parse $listing.text
            if ($listing.exit_code -ne 0 -or ((Has $envs 'ok') -and -not $envs.ok)) {
                $fault=Get $envs 'error';$faultCode=Get $fault 'code'
                if ($faultCode -is [string] -and $faultCode -match '^[a-zA-Z0-9_-]{1,80}$') { $report.orca.query_error=$faultCode }
                throw 'environment query failed'
            }
            $payload=Get $envs 'result' $envs
            if (-not (Has $payload 'environments')) { throw 'environment list missing' }
            $items=Get $payload 'environments' @()
            foreach ($entry in $items) { $id=Get $entry 'id'; if (-not $id) { continue }; $statusResult=Process $orca @('status','--environment',$id,'--json') '' 10 -Discovery; $connected=$false; try { $statusObject=Parse $statusResult.text; $connected=($statusResult.exit_code -eq 0 -and $statusObject.ok) } catch {}; $targets+=@{id=('orca-'+(Digest $id).Substring(0,12));environment=$id;connected=$connected}; $report.orca.environments+=@{id=$id;name=(Get $entry 'name' $id);connected=$connected} }
            if (@($items).Count -eq 0) { $report.gaps+='orca_no_remote_environment' }
        } catch { $report.gaps+='orca_environment_query_unverified' }
        if (-not $targets[0].connected) { $report.gaps+='orca_local_not_running_or_unreachable' }
        foreach ($target in $targets) {
            if (-not $caps) { continue }
            $kind='local';$targetObject=@{kind='local'};if ($target.environment) { $kind='orca_environment';$targetObject=@{kind=$kind;environment=$target.environment} }
            $profile=@{execution=@{adapter='orca-terminal';target=$targetObject;settings=@{agent='codex';access_mode='sandbox';transport=@{executable=$orca}}};workflow=@{id='development';version=2}}
            try { Validate $config 'run-configuration' $profile; $report.candidates+=@{id=$target.id;profile=$profile;connected=$target.connected;facts=@('static_configuration_valid','execution_host_tools_and_login_unverified','project_binding_required')} } catch { $report.gaps+='orca_configuration_unsupported' }
        }
    } else { $report.gaps+='orca_missing'; $report.orca.hint='要使用 SteerRunner 远程执行，请先安装 Orca，再配置远端 environment、执行主机工具/登录及项目，然后重新运行配置 skill。本机候选不受影响。' }
    $project=Project
    if ($project) { $report.project=$project }
    return $report
}
function Bind-Project($config,$project,$choices,$report) {
    if (-not $project) { return $null }
    $path=Project-File $project
    $binding=@{version=1;common_dir=$project.common_dir;profile=$config.default_profile;repository=@{};publication=$null}
    if ([IO.File]::Exists($path)) { $binding=Read-JSON $path; Keys $binding @('version','common_dir','profile','repository','publication'); if ($binding.version -ne 1 -or $binding.common_dir -cne $project.common_dir) { Fail 'project_identity_conflict' 'Saved project identity does not match this repository.' } }
    if (Has $choices 'project') { $update=$choices.project; Keys $update @('profile','repository','publication'); foreach ($k in $update.Keys) { $binding[$k]=$update[$k] } }
    $remotes=Git @('remote') -Discovery
    $names=@($remotes.text -split '[\r\n]+' | Where-Object { $_ })
    $remote=Get $binding.repository 'remote'
    if (-not $remote) { if ($names.Count -eq 1) { $remote=$names[0] } }
    if (-not $remote) { $report.gaps+='project_remote_ambiguous'; Atomic $path $binding; return $binding }
    $url=Git @('config','--get',('remote.'+$remote+'.url')) -Discovery
    if ($url.exit_code -ne 0 -or $url.text -match '(?i)(https?://[^/]*@|[?&](token|password)=)') { Fail 'unsafe_repository' 'Repository URL unavailable or contains embedded credentials.' }
    if ((Has $binding.repository 'url') -and $binding.repository.url -cne $url.text) { Fail 'project_identity_conflict' 'Repository remote changed; explicitly reconfigure its publication binding.' }
    $binding.repository.remote=$remote; $binding.repository.url=$url.text
    $profileId=Get $binding 'profile' $config.default_profile
    if (-not $profileId) { $profileId=$config.default_profile; $binding.profile=$profileId }
    if ($profileId -and (Has $config.profiles $profileId)) {
        $profile=$config.profiles[$profileId]
        if ($profile.execution.adapter -eq 'codex-app-server') { $binding.repository.local_path=$project.root }
        else {
            $commandArguments=@('repo','list','--json');if ($profile.execution.target.kind -eq 'orca_environment') { $commandArguments+=@('--environment',$profile.execution.target.environment) }
            $repos=Process $profile.execution.settings.transport.executable $commandArguments '' 10 -Discovery
            try {
                $payload=(Parse $repos.text).result; $matches=@()
                foreach ($repo in $payload.repos) {
                    $detailArgs=@('repo','show','--repo',('id:'+$repo.id),'--json');if ($profile.execution.target.kind -eq 'orca_environment') { $detailArgs+=@('--environment',$profile.execution.target.environment) }
                    $detail=Process $profile.execution.settings.transport.executable $detailArgs '' 10 -Discovery
                    try { $rp=(Parse $detail.text).result; $rp=Get $rp 'repo' $rp; $remoteURL=Get $rp 'remoteUrl' (Get $rp 'remote_url'); if (-not $remoteURL) { $identity=Get $rp 'gitRemoteIdentity';$remoteURL=Get $identity 'remoteUrl' }; if ($remoteURL -and $remoteURL.TrimEnd('/').Replace('.git','') -ceq $url.text.TrimEnd('/').Replace('.git','')) { $matches+= $repo.path } } catch {}
                }
                $savedPath=Get $binding.repository 'local_path'
                if ($savedPath) { if (@($matches | Where-Object { $_.Replace('\','/').TrimEnd('/') -ieq $savedPath.Replace('\','/').TrimEnd('/') }).Count -ne 1) { Fail 'orca_project_identity_unverified' 'The specified execution-host project could not be verified through Orca.' } }
                elseif ($matches.Count -eq 1) { $binding.repository.local_path=$matches[0] } else { $report.gaps+='orca_project_identity_unverified' }
            } catch { $report.gaps+='orca_project_query_failed' }
        }
    }
    $branch=Get $binding.repository 'base_branch'
    if (-not $branch) { $head=Git @('symbolic-ref',('--short'),('refs/remotes/'+$remote+'/HEAD')) -Discovery; if ($head.exit_code -eq 0 -and $head.text.StartsWith($remote+'/')) { $branch=$head.text.Substring($remote.Length+1) } }
    $gh=Tool 'gh.exe'
    if ($url.text -match '^https://([^/]+)/([^/]+/[^/]+?)(?:\.git)?$') {
        $hostName=$Matches[1];$repository=$Matches[2]
        if ($gh) {
            $auth=Process $gh @('auth','status','--hostname',$hostName) '' 5 -Discovery; if ($auth.exit_code -ne 0) { $report.gaps+='publication_login_unverified' }
            if (-not $branch) { $q=Process $gh @('api','--hostname',$hostName,('repos/'+$repository),'--jq','.default_branch') '' 5 -Discovery; if ($q.exit_code -eq 0 -and $q.text -notmatch '[\r\n]') { $branch=$q.text } }
            if ($branch -and -not $binding.publication -and $hostName -eq 'github.com') { $binding.publication=GitHub-Publication $hostName $repository $branch $gh }
        } else { $report.gaps+='gh_missing' }
    }
    if ($branch) { $binding.repository.base_branch=$branch } else { $report.gaps+='base_branch_required' }
    if (-not $binding.publication) { $report.gaps+='publication_required' }
    if ($config.runner_executable -and $profileId -and (Has $config.profiles $profileId) -and $binding.publication -and (Get $binding.repository 'base_branch') -and (Get $binding.repository 'local_path')) {
        $validationRepository=Parse (JSON $binding.repository);$validationRepository['base_revision']=('0'*40)
        $selection=$config.profiles[$profileId]
        Validate $config 'run' @{title='Validate project binding';repository=$validationRepository;documents=@{spec=@{path='spec.md'};plan=@{path='plan.md'}};execution=$selection.execution;workflow=$selection.workflow;publication=$binding.publication}
    }
    Atomic $path $binding
    return $binding
}
function Configure($choices,[switch]$Save) {
    Keys $choices @('runner_executable','state_dir','default_profile','profiles','project')
    $config=Load-Config
    foreach ($key in @('runner_executable','state_dir','default_profile','profiles')) { if (Has $choices $key) { $config[$key]=$choices[$key] } }
    if (-not [IO.Path]::IsPathRooted($config.state_dir)) { Fail 'invalid_configuration' 'state_dir must be absolute.' }
    $report=Discover $config
    if ($Save) {
        $lock=Lock (Join-Path $script:root 'configuration.lock')
        try {
            $current=Load-Config
            # Preserve explicit concurrent changes instead of overwriting them.
            foreach ($key in @('runner_executable','state_dir','default_profile','profiles')) { if (-not (Has $choices $key) -and $current[$key]) { $config[$key]=$current[$key] } }
            foreach ($candidate in $report.candidates) { if (-not (Has $config.profiles $candidate.id)) { $config.profiles[$candidate.id]=$candidate.profile } }
            if (-not $config.default_profile) {
                if (Has $config.profiles 'local-codex') { $config.default_profile='local-codex' }
                else { $eligible=@($report.candidates | Where-Object { $_.connected }); if ($eligible.Count -eq 1) { $config.default_profile=$eligible[0].id } elseif ($eligible.Count -gt 1) { $report.gaps+='default_profile_selection_required' } }
            }
            if ($config.runner_executable) { foreach ($id in $config.profiles.Keys) { Validate $config 'run-configuration' $config.profiles[$id] } }
            Atomic (Join-Path $script:root 'client.json') $config
            $projectBinding=Bind-Project $config $report.project $choices $report
            $report.project_binding=$projectBinding
            Atomic (Join-Path $script:root 'discovery.json') $report
        } finally { $lock.Dispose() }
    }
    return @{configuration=$config;discovery=$report;readiness='per_run_verification_required';note='Discovery is not execution or publication verification.'}
}
function Binding($config,[switch]$Start) {
    $cmd=@('service','status'); if ($Start) { $cmd=@('service','start') }
    $service=Require (Runner $config.runner_executable ($cmd+@('--state-dir',$config.state_dir)))
    if ($service.status -ne 'ready') { Fail 'service_unavailable' 'Original service is not ready; start or recover it explicitly.' }
    $binding=@{contract='steerrunner-service-binding/v1';protocol=$script:protocol;instance_id=$service.instance_id;state_dir=$service.state_dir;executable_sha256=$service.executable_sha256;version=$service.version;resource_digest=$service.resource_digest}
    $cap=Require (Runner $config.runner_executable @('capabilities','--state-dir',$config.state_dir,'--expected-binding',(JSON $binding)))
    if ($cap.capability_source.instance_id -cne $binding.instance_id) { Fail 'stale_instance' 'Service changed during binding.' }
    return $binding
}
function Identifier([string]$id) { if ($id -notmatch '^[a-zA-Z0-9][a-zA-Z0-9-]{0,95}$') { Fail 'invalid_identifier' 'Use a stable opaque alphanumeric/hyphen command identifier.' }; return $id }
function Record-Directory([string]$id) { return Join-Path $script:root ('submissions/'+(Identifier $id)) }
function Load-Record([string]$id) {
    $dir=Record-Directory $id; $meta=Read-JSON (Join-Path $dir 'meta.json'); $body=$script:utf8.GetString([IO.File]::ReadAllBytes((Join-Path $dir 'request.json')));$request=Parse $body
    if ($meta.request_digest -cne (Digest $body) -or $meta.request_id -cne $request.request_id -or $meta.request_id -cne $id) { Fail 'damaged_request_record' 'Original request record is damaged; do not rebuild or resubmit it.' }
    return @{directory=$dir;meta=$meta;request=$request;body=$body}
}
function Send-Saved($record,[switch]$Retry) {
    $meta=$record.meta; $config=@{runner_executable=$meta.runner_executable;state_dir=$meta.state_dir}
    $binding=Binding $config
    $queryCmd=@('inspect')
    $query=Runner $config.runner_executable ($queryCmd+@('--state-dir',$config.state_dir,'--request-id',$meta.request_id,'--expected-binding',(JSON $binding)))
    if ($query.exit_code -eq 0 -and $query.response.ok) { Atomic (Join-Path $record.directory 'receipt.json') $query.response; return @{operation_id=$meta.request_id;cli=$query;reconciled=$true} }
    if (-not $Retry) { return @{operation_id=$meta.request_id;cli=$query;reconciled=$false;note='Unknown results require reconciliation of the original ID; no new request was sent.'} }
    $commandArguments=@($meta.command -split ' ')+@('--state-dir',$config.state_dir,'--expected-binding',(JSON $binding),'--input','-')
    if ($meta.run_id) { $commandArguments+=@('--run-id',$meta.run_id) }
    $result=Runner $config.runner_executable $commandArguments $record.body
    if ($result.response) { Atomic (Join-Path $record.directory 'receipt.json') $result.response }
    return @{operation_id=$meta.request_id;cli=$result;reconciled=$false}
}
function Save-Command([string]$id,[string]$command,$config,$inputValue,[string]$runID,$binding,$project) {
    $dir=Record-Directory $id
    if ([IO.File]::Exists((Join-Path $dir 'meta.json')) -or [IO.File]::Exists((Join-Path $dir 'request.json'))) { Fail 'record_conflict' 'Command record already exists; reconcile it instead.' }
    $envelope=@{protocol=$script:protocol;request_id=$id;input=$inputValue}
    $meta=@{request_id=$id;command=$command;runner_executable=$config.runner_executable;state_dir=$binding.state_dir;run_id=$runID;project=$project;initial_binding=$binding;request_digest=(Digest (JSON $envelope))}
    Atomic (Join-Path $dir 'request.json') $envelope; Atomic (Join-Path $dir 'meta.json') $meta
    return Load-Record $id
}
function Submit($inputValue) {
    Keys $inputValue @('submission_id','title','spec','plan','base_revision','profile')
    $id=Identifier $inputValue.submission_id
    $lock=Lock (Join-Path (Record-Directory $id) 'command.lock')
    try {
        if ([IO.File]::Exists((Join-Path (Record-Directory $id) 'meta.json'))) { return Send-Saved (Load-Record $id) }
        $config=Load-Config; $config.runner_executable=Resolve-Runner $config
        $project=Project; if (-not $project) { Fail 'project_required' 'Submit from the intended Git repository.' }
        $path=Project-File $project
        if (-not [IO.File]::Exists($path)) { $null=Configure @{} -Save }
        if (-not [IO.File]::Exists($path)) { Fail 'configuration_required' 'Run configure to bind this project.' }
        $projectConfig=Read-JSON $path
        if ($projectConfig.common_dir -cne $project.common_dir) { Fail 'project_identity_conflict' 'Project identity changed.' }
        $profile=Get $inputValue 'profile' (Get $projectConfig 'profile' $config.default_profile)
        if (-not $profile -or -not (Has $config.profiles $profile)) { Fail 'configuration_required' 'Select an existing execution profile.' }
        if ($profile -cne $projectConfig.profile) { Fail 'project_binding_required' 'Bind this project to the requested target before submitting.' }
        $repository=$projectConfig.repository
        foreach ($key in @('url','remote','base_branch','local_path')) { if (-not (Get $repository $key)) { Fail 'configuration_required' ('Project requires '+$key) } }
        $remote=Git @('config','--get',('remote.'+$repository.remote+'.url'))
        if ($remote.exit_code -ne 0 -or $remote.text -cne $repository.url) { Fail 'project_identity_conflict' 'Remote no longer matches the saved publication target.' }
        $branchCheck=Git @('check-ref-format',('refs/heads/'+$repository.base_branch)); if ($branchCheck.exit_code -ne 0) { Fail 'invalid_input' 'Invalid target branch.' }
        $fetch=Git @('fetch','--no-tags',$repository.remote,('+refs/heads/'+$repository.base_branch+':refs/remotes/'+$repository.remote+'/'+$repository.base_branch))
        if ($fetch.exit_code -ne 0) { Fail 'base_refresh_failed' 'Cannot refresh the selected branch; no stale base was used.' }
        $base=Get $inputValue 'base_revision'
        if (-not $base) { $tip=Git @('rev-parse',('refs/remotes/'+$repository.remote+'/'+$repository.base_branch)); if ($tip.exit_code -ne 0) { Fail 'base_unavailable' 'Target commit is unavailable.' }; $base=$tip.text }
        if ($base -notmatch '^(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})$') { Fail 'invalid_input' 'Base must be a full commit.' }
        $documents=@{}
        foreach ($kind in @('spec','plan')) {
            $relative=[string](Get $inputValue $kind)
            if ([IO.Path]::IsPathRooted($relative)) { $prefix=$project.root.TrimEnd('\','/')+[IO.Path]::DirectorySeparatorChar; $full=[IO.Path]::GetFullPath($relative); if (-not $full.StartsWith($prefix,[StringComparison]::OrdinalIgnoreCase)) { Fail 'invalid_material' 'Material must belong to this repository.' }; $relative=$full.Substring($prefix.Length) }
            $relative=$relative.Replace([IO.Path]::DirectorySeparatorChar,'/')
            if (-not $relative -or $relative -match '(^/|:|(^|/)\.\.?(/|$))') { Fail 'invalid_material' 'Use a normalized repository-relative material path.' }
            $blob=Git @('rev-parse',($base+':'+$relative)); $working=Git @('hash-object',('--path='+$relative),'--',(Join-Path $project.root $relative))
            if ($blob.exit_code -ne 0 -or $working.exit_code -ne 0 -or $blob.text -cne $working.text) { Fail 'material_not_in_base' 'Spec/Plan is missing or differs from the pinned commit; publish the intended materials first.' }
            $documents[$kind]=@{path=$relative}
        }
        $repository.base_revision=$base
        $selection=$config.profiles[$profile]
        $run=@{title=$inputValue.title;repository=$repository;documents=$documents;workflow=$selection.workflow;execution=$selection.execution;publication=$projectConfig.publication}
        Validate $config 'run' $run
        $binding=Binding $config -Start
        $record=Save-Command $id 'submit' $config $run '' $binding $project.common_dir
        return Send-Saved $record -Retry
    } finally { $lock.Dispose() }
}
function Locate($inputValue) {
    if (Has $inputValue 'submission_id') {
        $record=Load-Record $inputValue.submission_id; $meta=$record.meta
        $config=@{runner_executable=$meta.runner_executable;state_dir=$meta.state_dir}
        $binding=Binding $config
        $result=Require (Runner $config.runner_executable @('inspect','--state-dir',$config.state_dir,'--request-id',$meta.request_id,'--expected-binding',(JSON $binding)))
        $run=Get $result 'run'; if (-not $run) { Fail 'run_unavailable' 'Original submission has no verified Run yet.' }
        return @{config=$config;binding=$binding;run=$run;project=$meta.project;record=$record}
    }
    if (-not (Get $inputValue 'run_id') -or -not (Get $inputValue 'state_dir')) { Fail 'target_required' 'Provide submission_id, or explicit state_dir and run_id.' }
    $config=Load-Config; $config.state_dir=$inputValue.state_dir; $config.runner_executable=Resolve-Runner $config
    $binding=Binding $config; $result=Require (Runner $config.runner_executable @('inspect','--state-dir',$config.state_dir,'--run-id',$inputValue.run_id,'--expected-binding',(JSON $binding)))
    return @{config=$config;binding=$binding;run=(Get $result 'run' $result);project=$null;record=$null}
}
function Existing($inputValue,[string]$actionName) {
    Keys $inputValue @('submission_id','state_dir','run_id','operation_id','text','question_id','expected_revision','after','evidence_command','artifact_id','relative_path')
    $collect=$actionName -eq 'Evidence' -and (Get $inputValue 'evidence_command') -eq 'artifacts collect'
    if ($actionName -in @('Reply','Resume','Stop') -or $collect) {
        $operation=Identifier $inputValue.operation_id
        $lock=Lock (Join-Path (Record-Directory $operation) 'command.lock')
        try {
            if ([IO.File]::Exists((Join-Path (Record-Directory $operation) 'meta.json'))) { return Send-Saved (Load-Record $operation) }
            $target=Locate $inputValue; $run=$target.run; $control='can_'+$actionName.ToLowerInvariant()
            if (-not $collect -and -not $run.controls[$control]) { Fail 'control_unavailable' 'Current Run does not allow this operation.' }
            if ((Has $inputValue 'expected_revision') -and $inputValue.expected_revision -ne $run.revision) { Fail 'stale_revision' 'Run changed; inspect current facts before providing new input.' }
            $body=@{expected_revision=$run.revision}
            if ($actionName -eq 'Reply') { $body.kind='feedback'; $body.text=$inputValue.text; $question=Get $run 'question'; if ($question) { $body.kind='answer'; $body.question_id=$question.id; if ((Has $inputValue 'question_id') -and $inputValue.question_id -cne $question.id) { Fail 'stale_question' 'The original question changed; do not repeat an old answer.' } } elseif (Has $inputValue 'question_id') { Fail 'stale_question' 'That question no longer exists; review the current Run before sending feedback.' } }
            $command=$actionName.ToLowerInvariant();if ($collect) { $command='artifacts collect';$body.relative_path=$inputValue.relative_path }
            $record=Save-Command $operation $command $target.config $body $run.run_id $target.binding $target.project
            return Send-Saved $record -Retry
        } finally { $lock.Dispose() }
    }
    $target=Locate $inputValue
    $cmd='observe';if ($actionName -eq 'Inspect') { $cmd='inspect' }
    if ($actionName -eq 'Evidence') { $cmd=Get $inputValue 'evidence_command' 'artifacts list';if ($cmd -notin @('artifacts discover','artifacts list','artifacts read','workspace-git')) { Fail 'invalid_input' 'Only public read-only evidence queries are allowed here.' } }
    $commandArguments=@($cmd -split ' ')+@('--state-dir',$target.config.state_dir,'--run-id',$target.run.run_id,'--expected-binding',(JSON $target.binding))
    if ($cmd -eq 'observe') {
        $cursor=Get $inputValue 'after'
        if (-not (Has $inputValue 'after') -and $target.record -and [IO.File]::Exists((Join-Path $target.record.directory 'cursor.json'))) { $cursor=(Read-JSON (Join-Path $target.record.directory 'cursor.json')).cursor }
        if ($cursor) { $commandArguments+=@('--after',$cursor) }
    }
    if ($cmd -eq 'artifacts read') { $commandArguments+=@('--artifact-id',$inputValue.artifact_id) }
    $result=Runner $target.config.runner_executable $commandArguments
    if ($cmd -eq 'observe' -and $result.response -and $result.response.ok -and $target.record) { Atomic (Join-Path $target.record.directory 'cursor.json') @{cursor=$result.response.data.event_page.next_cursor} }
    return @{cli=$result}
}

try {
    $inputValue=@{}; if ($InputFile) { $inputValue=Read-JSON ([IO.Path]::GetFullPath($InputFile)) }
    switch ($Action) {
        'Configure' { $data=Configure $inputValue -Save }
        'Discover' { $data=Configure $inputValue }
        'Submit' { $data=Submit $inputValue }
        'Reconcile' { Keys $inputValue @('operation_id','retry_same_request');$id=Identifier $inputValue.operation_id;$lock=Lock (Join-Path (Record-Directory $id) 'command.lock');try{$data=Send-Saved (Load-Record $id) -Retry:([bool](Get $inputValue 'retry_same_request' $false))}finally{$lock.Dispose()} }
        'List' { $data=@(); $path=Join-Path $script:root 'submissions'; if (Test-Path -LiteralPath $path) { foreach ($dir in Get-ChildItem -LiteralPath $path -Directory) { try { $record=Load-Record $dir.Name;$data+=@{operation_id=$record.meta.request_id;command=$record.meta.command;state_dir=$record.meta.state_dir;project=$record.meta.project;run_id=$record.meta.run_id} } catch { $data+=@{operation_id=$dir.Name;error='damaged_or_incomplete_record'} } } } }
        'Viewer' { $viewer=Join-Path $script:bundle 'steerrunner-viewer.exe'; if (-not [IO.File]::Exists($viewer)) { Fail 'viewer_missing' 'Viewer is not installed in this bundle.' }; Start-Process -FilePath $viewer -WindowStyle Normal | Out-Null; $data=@{opened=$true} }
        default { $data=Existing $inputValue $Action }
    }
    $exitCode=0; if ($data -is [Collections.IDictionary] -and (Has $data 'cli')) { $exitCode=$data.cli.exit_code }
    [Console]::OutputEncoding=New-Object Text.UTF8Encoding($false)
    [Console]::WriteLine((JSON @{ok=($exitCode -eq 0);action=$Action;data=$data;error=$null}));exit $exitCode
} catch {
    $code='client_error';$message='Client operation failed; original saved requests were retained.'
    if ($_.Exception.Message -match '^([a-z_]+)\|(.+)$') { $code=$Matches[1];$message=$Matches[2] }
    [Console]::OutputEncoding=New-Object Text.UTF8Encoding($false)
    [Console]::WriteLine((JSON @{ok=$false;action=$Action;data=$null;error=@{code=$code;message=$message}}));exit 2
}
