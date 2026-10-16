package codexworker

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type workerRuntime struct {
	Shell     string
	Temp      string
	Auth      *AuthenticationRuntime
	OrcaHooks bool
}

// The temp directory is a child of the already frozen linked-worktree admin
// root. It needs no additional writable root and cannot enter the candidate.
func prepareWorkerRuntime(worktree string, linkedArgs []string) (*workerRuntime, error) {
	roots, err := WritableRoots(linkedArgs)
	if err != nil || (len(roots) != 4 && len(roots) != 5) {
		return nil, errors.Join(err, errors.New("worker runtime needs fixed linked Git roots"))
	}
	if len(roots) == 5 {
		verified, verifyErr := PrivateLinkedWorktreeWriteArgs(roots[0], roots[1], roots[2], roots[3])
		if verifyErr != nil || !strings.EqualFold(filepath.Clean(verified[len(verified)-1]), filepath.Clean(roots[4])) {
			return nil, errors.New("worker private Git common directory differs from its fixed roots")
		}
	}
	gitfile, err := os.ReadFile(filepath.Join(worktree, ".git"))
	if err != nil {
		return nil, err
	}
	admin, ok := strings.CutPrefix(strings.TrimSpace(string(gitfile)), "gitdir: ")
	if !ok {
		return nil, errors.New("worker runtime requires a linked worktree")
	}
	if !filepath.IsAbs(admin) {
		admin = filepath.Join(worktree, admin)
	}
	admin = filepath.Clean(admin)
	if !filepath.IsAbs(roots[3]) || !strings.EqualFold(admin, filepath.Clean(roots[3])) {
		return nil, errors.New("worker temp admin differs from the frozen writable root")
	}
	adminInfo, err := os.Lstat(admin)
	if err != nil || !adminInfo.IsDir() || adminInfo.Mode()&os.ModeSymlink != 0 {
		return nil, errors.Join(err, errors.New("worker admin is not a plain directory"))
	}
	real, err := filepath.EvalSymlinks(admin)
	if err != nil || !strings.EqualFold(filepath.Clean(real), admin) {
		return nil, errors.Join(err, errors.New("worker admin was redirected"))
	}
	temp := filepath.Join(admin, "steerline-temp")
	if err = os.MkdirAll(temp, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(temp)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.Join(err, errors.New("worker temp is not a plain directory"))
	}
	real, err = filepath.EvalSymlinks(temp)
	if err != nil || !strings.EqualFold(filepath.Clean(real), temp) {
		return nil, errors.Join(err, errors.New("worker temp directory was redirected"))
	}
	shell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if !filepath.IsAbs(shell) || !isNativeCodexExecutable(shell) {
		return nil, errors.New("native Windows PowerShell executable is unavailable")
	}
	return &workerRuntime{Shell: shell, Temp: temp}, nil
}

func environmentValue(env []string, name string) string {
	for i := len(env) - 1; i >= 0; i-- {
		key, value, _ := strings.Cut(env[i], "=")
		if strings.EqualFold(key, name) {
			return value
		}
	}
	return ""
}

func setEnvironmentValues(env []string, values map[string]string) []string {
	result := make([]string, 0, len(env)+len(values))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		replaced := false
		for name := range values {
			if strings.EqualFold(key, name) {
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, entry)
		}
	}
	for name, value := range values {
		result = append(result, name+"="+value)
	}
	return result
}

func (r *workerRuntime) environment(env []string) ([]string, error) {
	if r.Auth != nil {
		var err error
		env, err = r.Auth.environment(env)
		if err != nil {
			return nil, err
		}
	}
	paths := []string{filepath.Dir(r.Shell)}
	for _, path := range filepath.SplitList(environmentValue(env, "PATH")) {
		// WindowsApps execution aliases are zero-length reparse files. Codex's
		// shell discovery must not select one as the worker's PowerShell.
		alias := filepath.Join(path, "pwsh.exe")
		if info, err := os.Stat(alias); err == nil && (info.Size() == 0 || !isNativeCodexExecutable(alias)) {
			continue
		}
		paths = append(paths, path)
	}
	count := 0
	if value := environmentValue(env, "GIT_CONFIG_COUNT"); value != "" {
		var err error
		count, err = strconv.Atoi(value)
		if err != nil || count < 0 || count > 64 {
			return nil, errors.New("invalid inherited Git process configuration count")
		}
		for i := 0; i < count; i++ {
			if environmentValue(env, fmt.Sprintf("GIT_CONFIG_KEY_%d", i)) == "" {
				return nil, errors.New("inherited Git process configuration is incomplete")
			}
		}
	}
	values := map[string]string{
		"PATH": strings.Join(paths, string(os.PathListSeparator)), "TEMP": r.Temp, "TMP": r.Temp,
		"GIT_CONFIG_COUNT":                          strconv.Itoa(count + 3),
		fmt.Sprintf("GIT_CONFIG_KEY_%d", count):     "maintenance.auto",
		fmt.Sprintf("GIT_CONFIG_VALUE_%d", count):   "false",
		fmt.Sprintf("GIT_CONFIG_KEY_%d", count+1):   "gc.auto",
		fmt.Sprintf("GIT_CONFIG_VALUE_%d", count+1): "0",
		fmt.Sprintf("GIT_CONFIG_KEY_%d", count+2):   "core.longpaths",
		fmt.Sprintf("GIT_CONFIG_VALUE_%d", count+2): "true",
	}
	if r.Auth != nil {
		// Git's helper launcher invokes MSYS sh, which cannot create its named
		// objects in MXC. Send a URL-scoped HTTP header from process environment
		// instead. Never pin this secret value with a native config argv.
		header := "AUTHORIZATION: basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+environmentValue(env, "GH_TOKEN")))
		entries := [][2]string{{"credential.helper", ""}, {"http." + r.Auth.remoteURL + ".extraheader", header}, {"credential.interactive", "false"}}
		for i, entry := range entries {
			values[fmt.Sprintf("GIT_CONFIG_KEY_%d", count+3+i)] = entry[0]
			values[fmt.Sprintf("GIT_CONFIG_VALUE_%d", count+3+i)] = entry[1]
		}
		values["GIT_CONFIG_COUNT"] = strconv.Itoa(count + 6)
	}
	return setEnvironmentValues(env, values), nil
}

func (r *workerRuntime) instructions() string {
	return fmt.Sprintf("SteerRunner Windows tool setup: use shell %q and login=false. MXC owns each tool AppContainer temporary directory. Keep the fixed workspace and Git roots. Git maintenance is disabled. The trusted execution runtime owns repository authentication; model-generated code can read its process environment. Do not dump authentication or the environment. Never request elevated or sandbox-external execution, repair authentication, run login, or change permission settings. For EVERY git commit, first stage ONLY the exact intended task paths with git add -- <exact task paths>, immediately capture its own exit code and stop if it is nonzero. Verify git diff --cached --name-only is nonempty and contains all and only the intended task paths before invoking git commit. Run staging, staged-diff verification and commit sequentially, never in parallel. Then preserve and print git commit\u0027s own exit code in that same tool call immediately after git commit, before git status or any other command. In PowerShell use: git commit <task arguments>; $srCommitExit=$LASTEXITCODE; Write-Output ('SR_COMMIT_EXIT='+$srCommitExit); if($srCommitExit -ne 0){exit $srCommitExit}. Do not put git commit in a pipeline. Git 2.55 compatibility: an exact shared .git/packed-refs.lock permission warning can come from deleting the absent AUTO_MERGE ref AFTER a successful commit. This warning alone does not make that commit fail. If the captured commit exit is zero, verify HEAD advanced on the assigned branch to the intended commit, its parent and changed paths match the task, the worktree is clean, and no AUTO_MERGE or pending merge/cherry-pick/revert state exists; report the successful commit together with this precise warning and continue. Never retry that commit or repair the denied cleanup. A nonzero or unknown commit exit, any OTHER denied operation, authentication expiry, sandbox access failure or unverified Git state is an environment blocker: preserve the exact failure and original branch/PR outcome for reconciliation. Do not repeat an uncertain publication.", r.Shell)
}

func (r *workerRuntime) configArgs(env []string) ([]string, error) {
	count, err := strconv.Atoi(environmentValue(env, "GIT_CONFIG_COUNT"))
	if r.Auth != nil {
		count -= 3
	}
	if err != nil || count < 3 || environmentValue(env, fmt.Sprintf("GIT_CONFIG_KEY_%d", count-3)) != "maintenance.auto" || environmentValue(env, fmt.Sprintf("GIT_CONFIG_KEY_%d", count-2)) != "gc.auto" || environmentValue(env, fmt.Sprintf("GIT_CONFIG_KEY_%d", count-1)) != "core.longpaths" {
		return nil, errors.New("worker native environment lacks its fixed Git configuration")
	}
	values := [][2]string{{"PATH", environmentValue(env, "PATH")}, {"GIT_CONFIG_COUNT", strconv.Itoa(count)},
		{fmt.Sprintf("GIT_CONFIG_KEY_%d", count-3), "maintenance.auto"}, {fmt.Sprintf("GIT_CONFIG_VALUE_%d", count-3), "false"},
		{fmt.Sprintf("GIT_CONFIG_KEY_%d", count-2), "gc.auto"}, {fmt.Sprintf("GIT_CONFIG_VALUE_%d", count-2), "0"},
		{fmt.Sprintf("GIT_CONFIG_KEY_%d", count-1), "core.longpaths"}, {fmt.Sprintf("GIT_CONFIG_VALUE_%d", count-1), "true"}}
	args := make([]string, 0, len(values)*2)
	for _, entry := range values {
		args = append(args, "--config", "shell_environment_policy.set."+entry[0]+"="+strconv.Quote(entry[1]))
	}
	if r.Auth != nil {
		// Values are inherited from the process environment, never argv. The
		// explicit allowlist prevents the App key and unrelated caller secrets
		// from reaching shell tools even when native defaults exclude *TOKEN*.
		args = append(args, "--config", `shell_environment_policy.inherit="all"`, "--config", "shell_environment_policy.ignore_default_excludes=true", "--config", `shell_environment_policy.include_only=["PATH","PATHEXT","SystemRoot","SystemDrive","COMSPEC","WINDIR","TEMP","TMP","USERPROFILE","HOME","APPDATA","LOCALAPPDATA","ProgramFiles","ProgramFiles(x86)","ProgramData","GH_HOST","GH_TOKEN","GH_PROMPT_DISABLED","GH_CONFIG_DIR","GIT_TERMINAL_PROMPT","GIT_CONFIG_*","HTTPS_PROXY","HTTP_PROXY","NO_PROXY","SSL_CERT_FILE","GIT_SSL_CAINFO"]`)
		if r.OrcaHooks {
			// Native Codex command hooks use this same environment policy. Keep
			// only the existing pane-scoped hook transport and identity; never
			// admit remote pairing credentials or the whole ORCA_* namespace.
			const hookNames = `,"ORCA_AGENT_HOOK_ENDPOINT","ORCA_AGENT_HOOK_PORT","ORCA_AGENT_HOOK_TOKEN","ORCA_AGENT_HOOK_TRANSPORT","ORCA_AGENT_HOOK_ENV","ORCA_AGENT_HOOK_NODE","ORCA_AGENT_HOOK_VERSION","ORCA_PANE_KEY","ORCA_AGENT_PANE","ORCA_AGENT_LAUNCH_TOKEN","ORCA_AGENT_LAUNCH","ORCA_TAB_ID","ORCA_WORKTREE_ID","ORCA_USER_DATA_PATH"`
			args[len(args)-1] = strings.TrimSuffix(args[len(args)-1], "]") + hookNames + "]"
		}
		for i := count; i < count+3; i++ {
			for _, prefix := range []string{"GIT_CONFIG_KEY_", "GIT_CONFIG_VALUE_"} {
				if i == count+1 && prefix == "GIT_CONFIG_VALUE_" {
					continue
				} // secret: environment only
				name := fmt.Sprintf("%s%d", prefix, i)
				args = append(args, "--config", "shell_environment_policy.set."+name+"="+strconv.Quote(environmentValue(env, name)))
			}
		}
		args = append(args, "--config", "shell_environment_policy.set.GIT_CONFIG_COUNT="+strconv.Quote(strconv.Itoa(count+3)))
		if broker := r.Auth.broker; broker != nil {
			if broker.socketLease != nil {
				if err := broker.socketLease.verify(); err != nil {
					return nil, err
				}
				args = append(args, "--add-dir", broker.socketLease.plan.Directory)
			}
			// Public transport values belong to tool policy, not the app-server's
			// own model/network environment. No machine-wide proxy or trust change.
			noProxy := ""
			if r.OrcaHooks {
				// Native status hooks must reach Orca's loopback listener rather
				// than the repository-only GitHub authentication proxy.
				noProxy = "127.0.0.1,localhost,::1"
			}
			for _, entry := range [][2]string{{"HTTPS_PROXY", broker.address}, {"HTTP_PROXY", broker.address}, {"NO_PROXY", noProxy}, {"SSL_CERT_FILE", broker.caPath}, {"GIT_SSL_CAINFO", broker.caPath}, {"GH_CONFIG_DIR", broker.configDir}} {
				args = append(args, "--config", "shell_environment_policy.set."+entry[0]+"="+strconv.Quote(entry[1]))
			}
		}
	}
	return args, nil
}
