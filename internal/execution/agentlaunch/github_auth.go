package agentlaunch

import (
	"net/url"
	"regexp"
	"strings"
)

var gitHubRepositoryPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func gitHubRepository(repository string) bool {
	u, err := url.Parse(repository)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	return len(parts) == 2 && gitHubRepositoryPart.MatchString(parts[0]) && gitHubRepositoryPart.MatchString(parts[1])
}

// Resolve the existing login only on the execution host. The credential travels
// in this native process's environment, never the Orca request, command argv,
// Git URL, config file, or a deployed helper. Git's exact repository header
// avoids starting an MSYS credential shell inside the Windows AppContainer.
// This retains the existing account's privileges; it is not a scoped broker.
func gitHubAuthenticationLaunch(repository, command string) string {
	lines := []string{
		`if($env:GIT_CONFIG_COUNT -or $env:GIT_CONFIG_PARAMETERS) { throw 'Existing process Git configuration conflicts with native authentication' }`,
		`foreach($srName in [Environment]::GetEnvironmentVariables().Keys) { if($srName -match '^(GIT_TRACE|GIT_CURL_VERBOSE|GH_DEBUG)' -and [Environment]::GetEnvironmentVariable($srName)) { throw 'Credential transport cannot run with inherited authentication tracing' } }`,
		`$srAuthNames=@('GH_HOST','GH_TOKEN','GIT_CONFIG_COUNT','GIT_CONFIG_KEY_0','GIT_CONFIG_VALUE_0','GH_PROMPT_DISABLED','GIT_TERMINAL_PROMPT'); $srAuthBefore=@{}; foreach($srName in $srAuthNames) { $srAuthBefore[$srName]=[Environment]::GetEnvironmentVariable($srName) }`,
		`$ErrorActionPreference='Continue'; $srCredential=(& gh auth token --hostname github.com 2>$null | Out-String).Trim(); $srCredentialExit=$LASTEXITCODE; $ErrorActionPreference='Stop'; if($srCredentialExit -ne 0 -or -not $srCredential -or $srCredential -match '[\r\n]') { throw 'Existing execution-host gh login unavailable; no agent input was sent' }`,
		`try { $env:GH_HOST='github.com'; $env:GH_TOKEN=$srCredential; $env:GH_PROMPT_DISABLED='1'; $env:GIT_TERMINAL_PROMPT='0'; $env:GIT_CONFIG_COUNT='1'; $env:GIT_CONFIG_KEY_0=` + quote("http."+repository+".extraHeader") + `; $env:GIT_CONFIG_VALUE_0='Authorization: Basic '+[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes(('x-access-token:'+$srCredential)))`,
		`$srAllowed=@('PATH','PATHEXT','SystemRoot','SystemDrive','COMSPEC','WINDIR','TEMP','TMP','USERPROFILE','HOME','APPDATA','LOCALAPPDATA','ProgramFiles','ProgramFiles(x86)','ProgramData','GH_HOST','GH_TOKEN','GH_PROMPT_DISABLED','GH_CONFIG_DIR','GIT_TERMINAL_PROMPT','GIT_CONFIG_COUNT','GIT_CONFIG_KEY_0','GIT_CONFIG_VALUE_0','HTTPS_PROXY','HTTP_PROXY','NO_PROXY','SSL_CERT_FILE','GIT_SSL_CAINFO','ORCA_AGENT_HOOK_ENDPOINT','ORCA_AGENT_HOOK_PORT','ORCA_AGENT_HOOK_TOKEN','ORCA_AGENT_HOOK_TRANSPORT','ORCA_AGENT_HOOK_ENV','ORCA_AGENT_HOOK_NODE','ORCA_AGENT_HOOK_VERSION','ORCA_PANE_KEY','ORCA_AGENT_PANE','ORCA_AGENT_LAUNCH_TOKEN','ORCA_AGENT_LAUNCH','ORCA_TAB_ID','ORCA_WORKTREE_ID','ORCA_USER_DATA_PATH')`,
		`$srAllowed+=@('STEERLINE_PROGRESS_FILE','STEERLINE_RESULT_FILE'); $srPolicy='shell_environment_policy.include_only=['+(@($srAllowed | ForEach-Object { "'"+$_+"'" }) -join ',')+']'; $srNativeAuthenticationArgs=@('--config',"shell_environment_policy.inherit='all'",'--config','shell_environment_policy.ignore_default_excludes=true','--config',$srPolicy)`,
		command + ` } finally { foreach($srName in $srAuthNames) { [Environment]::SetEnvironmentVariable($srName,$srAuthBefore[$srName]) }; $srCredential=$null }`,
	}
	return "& { " + strings.Join(lines, "; ") + " }"
}
