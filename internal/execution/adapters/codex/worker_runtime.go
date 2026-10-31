package codexworker

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/execution/hostenv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type workerRuntime struct {
	Shell     string
	Temp      string
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
	env = hostenv.Development(env)
	if r.OrcaHooks {
		value := environmentValue(env, "NO_PROXY")
		for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
			if !strings.Contains(","+value+",", ","+host+",") {
				if value != "" {
					value += ","
				}
				value += host
			}
		}
		env = setEnvironmentValues(env, map[string]string{"NO_PROXY": value})
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

	return setEnvironmentValues(env, values), nil
}

const WorkerInstructionFormat = "shell=%q,login=false。临时文件使用本次Run指定临时目录并同次清理，禁写认证/Git管理根。不得输出环境/凭据、提权、登录、改认证或权限。仅精确暂存任务路径，检查非空暂存diff且仅含任务；串行暂存/检查/提交，非零即停。commit不用管道，紧接命令保存并打印其$LASTEXITCODE，再查状态。失败或未知（含认证过期）保留原分支/PR结果并停止对账；未知发布不重做。"

func (r *workerRuntime) instructions() string {
	return fmt.Sprintf(WorkerInstructionFormat, r.Shell)
}

func (r *workerRuntime) configArgs(env []string) ([]string, error) {
	names := strings.Fields(hostenv.ToolNames)
	names = append(names, "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_*", "GIT_CONFIG_VALUE_*")
	if r.OrcaHooks {
		names = append(names, strings.Fields("ORCA_AGENT_HOOK_ENDPOINT ORCA_AGENT_HOOK_PORT ORCA_AGENT_HOOK_TOKEN ORCA_AGENT_HOOK_TRANSPORT ORCA_AGENT_HOOK_ENV ORCA_AGENT_HOOK_NODE ORCA_AGENT_HOOK_VERSION ORCA_PANE_KEY ORCA_AGENT_PANE ORCA_AGENT_LAUNCH_TOKEN ORCA_AGENT_LAUNCH ORCA_TAB_ID ORCA_WORKTREE_ID ORCA_USER_DATA_PATH")...)
	}
	raw, _ := json.Marshal(names)
	args := []string{"--config", `shell_environment_policy.inherit="all"`, "--config", "shell_environment_policy.ignore_default_excludes=true", "--config", "shell_environment_policy.include_only=" + string(raw)}
	// Values remain in process environment, including arbitrary Git credential headers.
	return args, nil
}
