package application

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/evidence"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/publication"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/execution/workspace"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

var repositoryLocks sync.Map

// Keep linked-worktree Git administration paths comfortably below native Git's
// Windows buffer limits. The full request identities remain in accepted input;
// a cache hit still verifies the exact remote and every frozen commit.
func repositoryCache(dir string, repository contracts.Repository) string {
	return filepath.Join(dir, "r", compactDirectoryKey(repository.URL+"\x00"+repository.BaseRevision+"\x00"+repository.LocalPath))
}

// A case-insensitive 26-character encoding retains 128 bits without the
// additional hex expansion in native Git's fixed-size administration buffers.
func compactDirectoryKey(identity string) string {
	key := sha256.Sum256([]byte(identity))
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(key[:16]))
}

func executionWorkspace(dir, id string) string {
	return filepath.Join(dir, "workspaces", compactDirectoryKey(id))
}

func gitHost() workspace.Git {
	return workspace.Git{Command: b.LaunchCommand{Executable: "git", Args: []string{"-c", "core.longpaths=true"}}, Timeout: 30 * time.Second, Start: process.New(30 * time.Second).StartManaged}
}
func canonical(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
func providerFor(r *runstate.Record) (*publication.Client, error) {
	var base string
	var refs []string
	var query b.Command
	if r.Request.Publication.Provider == "gitea" {
		var settings contracts.GiteaSettings
		if err := contracts.DecodeStrict(r.Request.Publication.Settings, &settings); err != nil {
			return nil, err
		}
		base, refs = settings.BaseURL, settings.AuthenticationRefs
	} else {
		var settings contracts.CommandProviderSettings
		if err := contracts.DecodeStrict(r.Request.Publication.Settings, &settings); err != nil {
			return nil, err
		}
		base, refs = settings.BaseURL, settings.AuthenticationRefs
		query = b.Command{Executable: settings.Query.Executable, Args: settings.Query.Args}
	}
	config := publication.Config{Kind: r.Request.Publication.Provider, BaseURL: base, Repository: r.Request.Publication.Repository, Query: query}
	if len(refs) > 0 {
		config.TokenEnv = refs[0]
	}
	settings := evidence.PublicationSettings{Provider: evidence.PRProviderConfig(config), Git: b.Command{Executable: "git", Args: []string{"-c", "core.longpaths=true"}}, Remote: r.Request.Repository.Remote, RemoteURL: r.Request.Repository.URL, BaseBranch: r.Request.Repository.BaseBranch, AuthenticationRefs: refs}
	if err := publication.ValidateSettings(settings); err != nil {
		return nil, err
	}
	if err := config.ValidateRemote(r.Request.Repository.URL); err != nil {
		return nil, err
	}
	return publication.NewClient(config, commandHost{}, 30*time.Second), nil
}

type commandHost struct{}

func (commandHost) Output(ctx context.Context, command b.Command, args ...string) ([]byte, error) {
	return process.New(30*time.Second).OutputCommand(ctx, b.LaunchCommand{Executable: command.Executable, Args: append(append([]string{}, command.Args...), args...)})
}

func freeze(ctx context.Context, dir string, input contracts.Request) (*runstate.Record, error) {
	var request contracts.RunRequest
	if err := contracts.DecodeStrict(input.Input, &request); err != nil {
		return nil, err
	}
	if request.Execution.Adapter == "orca-terminal" && request.Repository.LocalPath == "" {
		return nil, contracts.Fail(2, "invalid_input", "Orca execution requires repository.local_path for its registered project identity.")
	}
	id, attempt, generation := contracts.NewID(), contracts.NewID(), contracts.NewID()
	now := time.Now().UTC()
	r := &runstate.Record{ID: id, Request: request, Allowed: true, Status: "queued", Phase: "accepted", CreatedAt: now, UpdatedAt: now, ResourceDigest: resources.Digest(), CommandStates: map[string]string{}}
	if _, err := providerFor(r); err != nil {
		return nil, contracts.Fail(2, "invalid_input", "Publication configuration does not match the fixed repository or non-secret schema.")
	}
	target := b.ExecutionTarget{ID: "local", ExecutionHost: "local"}
	kind, protocol := b.BackendLocal, b.ProtocolLocalCodexAppServer
	var key string
	var codec int
	var payload []byte
	if request.Execution.Adapter == "codex-app-server" {
		var settings contracts.CodexSettings
		_ = json.Unmarshal(request.Execution.Settings, &settings)
		if settings.Executable == "" {
			settings.Executable = "codex.exe"
		}
		if settings.ReasoningEffort == "" {
			settings.ReasoningEffort = "medium"
		}
		executable, err := codex.NativeExecutable(settings.Executable)
		if err != nil {
			return nil, contracts.Fail(4, "environment_unavailable", "The configured native Codex executable is unavailable.")
		}
		settings.Executable = executable
		r.Request.Execution.Settings = canonical(settings)
		key, codec, payload = codex.ProfileCodecKey, codex.ProfileCodecVersion, canonical(codex.ProfileConfig{Executable: settings.Executable, Model: settings.Model, ReasoningEffort: settings.ReasoningEffort})
	} else {
		var settings contracts.OrcaSettings
		_ = json.Unmarshal(request.Execution.Settings, &settings)
		if settings.Transport.Executable == "" {
			settings.Transport.Executable = "orca.exe"
		}
		if settings.Transport.WaitTimeout == "" {
			settings.Transport.WaitTimeout = "30s"
		}
		if settings.AgentLaunch.Executable == "" {
			settings.AgentLaunch.Executable = "codex"
		}
		executable, err := exec.LookPath(settings.Transport.Executable)
		if err != nil {
			return nil, contracts.Fail(4, "environment_unavailable", "The configured Orca executable is unavailable.")
		}
		settings.Transport.Executable = executable
		profile := orca.Profile{Executable: executable, Args: settings.Transport.Args, WaitTimeout: settings.Transport.WaitTimeout, Environment: request.Execution.Target.Environment}
		target = profile.Target()
		kind, protocol = b.BackendOrca, b.ProtocolOrcaTerminalV3
		launch := agentlaunch.Config{Executable: settings.AgentLaunch.Executable, Args: settings.AgentLaunch.Args, Model: settings.AgentLaunch.Model, ReasoningEffort: settings.AgentLaunch.ReasoningEffort}
		r.LaunchCommand, err = launch.Command("codex")
		if err != nil {
			return nil, contracts.Fail(2, "invalid_input", "Invalid frozen agent launch configuration.")
		}
		r.Request.Execution.Settings = canonical(settings)
		key, codec, payload = orca.ProfileKey, orca.ProfileVersion, canonical(profile)
	}
	r.InputDigest = reliable.Digest(canonical(r.Request))
	git := gitHost()
	cache := repositoryCache(dir, request.Repository)
	entry, _ := repositoryLocks.LoadOrStore(cache, &sync.Mutex{})
	cacheLock := entry.(*sync.Mutex)
	cacheLock.Lock()
	defer cacheLock.Unlock()
	if _, err := os.Stat(cache); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cache), 0700); err != nil {
			return nil, err
		}
		source := request.Repository.URL
		if request.Repository.LocalPath != "" {
			if !filepath.IsAbs(request.Repository.LocalPath) {
				return nil, contracts.Fail(2, "invalid_input", "repository.local_path must be absolute.")
			}
			source = request.Repository.LocalPath
			remote, err := git.Output(ctx, source, "config", "--get", "remote."+request.Repository.Remote+".url")
			if err != nil || strings.TrimSpace(string(remote)) != request.Repository.URL {
				return nil, contracts.Fail(2, "invalid_input", "The local repository remote differs from the frozen URL.")
			}
		}
		staging, err := os.MkdirTemp(filepath.Dir(cache), ".clone-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(staging) // Only the private directory created above.
		if _, err := git.Output(ctx, dir, "clone", "--no-checkout", "--no-hardlinks", "--origin", request.Repository.Remote, "--", source, staging); err != nil {
			return nil, contracts.Fail(4, "environment_unavailable", "Cannot prepare the repository cache; no run was accepted. "+redactDiagnostic(r, err.Error()))
		}
		if _, err := git.Output(ctx, staging, "remote", "set-url", request.Repository.Remote, request.Repository.URL); err != nil {
			return nil, err
		}
		if err := os.Rename(staging, cache); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	remote, err := git.Output(ctx, cache, "config", "--get", "remote."+request.Repository.Remote+".url")
	if err != nil || strings.TrimSpace(string(remote)) != request.Repository.URL {
		return nil, contracts.Fail(5, "repository_conflict", "Repository cache identity does not match.")
	}
	commits := []string{request.Repository.BaseRevision}
	for _, doc := range append([]contracts.Document{request.Documents.Spec, request.Documents.Plan}, request.Documents.References...) {
		if doc.Kind == "git" {
			commits = append(commits, doc.Commit)
		}
	}
	for _, revision := range commits {
		if _, err := git.Output(ctx, cache, "cat-file", "-e", revision+"^{commit}"); err != nil {
			if _, err := git.Output(ctx, cache, "fetch", "--no-tags", request.Repository.Remote, revision); err != nil {
				return nil, contracts.Fail(2, "invalid_input", "A pinned material/base commit is unavailable.")
			}
		}
	}
	if _, err := git.Output(ctx, cache, "check-ref-format", "refs/heads/"+request.Repository.BaseBranch); err != nil {
		return nil, contracts.Fail(2, "invalid_input", "Invalid base branch.")
	}
	if _, err := git.Output(ctx, cache, "merge-base", "--is-ancestor", request.Repository.BaseRevision, "refs/remotes/"+request.Repository.Remote+"/"+request.Repository.BaseBranch); err != nil {
		return nil, contracts.Fail(2, "invalid_input", "Pinned base is not in the configured base branch.")
	}
	repo, err := workspace.ResolveRepository(ctx, git, cache)
	if err != nil {
		return nil, err
	}
	r.RepositoryPath, r.RepositoryCommon = repo.Path, repo.CommonDir
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: key, CodecVersion: codec, Purpose: b.ImplementationConfigPurpose, ProjectID: repo.CommonDir, ProfileID: id, AcceptedConfigVersion: r.InputDigest}, payload)
	if err != nil {
		return nil, err
	}
	r.Profile = []b.ImplementationDataEnvelope{profile}
	seen := map[string]bool{}
	for _, doc := range append([]contracts.Document{request.Documents.Spec, request.Documents.Plan}, request.Documents.References...) {
		if seen[strings.ToLower(doc.Path)] {
			return nil, contracts.Fail(2, "invalid_input", "Material paths must be distinct.")
		}
		seen[strings.ToLower(doc.Path)] = true
		var content []byte
		if doc.Kind == "inline" {
			content = []byte(*doc.Content)
		} else {
			content, err = git.Output(ctx, cache, "show", doc.Commit+":"+doc.Path)
			if err != nil {
				return nil, contracts.Fail(2, "invalid_input", "A pinned material path is missing.")
			}
		}
		if !utf8.Valid(content) || len(content) > contracts.MaxRequestBytes {
			return nil, contracts.Fail(2, "invalid_input", "Material must be bounded UTF-8 text.")
		}
		path := filepath.Join(dir, "inputs", id, filepath.FromSlash(doc.Path))
		if err := writeFixed(path, content); err != nil {
			return nil, err
		}
		r.Materials = append(r.Materials, runstate.Material{MaterialView: contracts.MaterialView{Kind: doc.Kind, Path: doc.Path, Commit: doc.Commit, Digest: reliable.Digest(content), SnapshotPath: path}, Content: string(content)})
	}
	_ = json.Unmarshal(resources.Read("prompts.json"), &r.PromptBodies)
	for step, body := range request.Workflow.PromptOverrides {
		r.PromptBodies[step] = body
	}
	if request.Execution.Adapter == "orca-terminal" {
		// The current Windows Orca CLI accepts prompt text in argv. Reserve
		// space for escaping, source result and fixed protocol instructions.
		units := len(utf16.Encode([]rune(string(canonical(r.Materials)))))
		for _, body := range r.PromptBodies {
			units += len(utf16.Encode([]rune(body)))
		}
		if units > 12000 {
			return nil, contracts.Fail(3, "unsupported_capability", "Frozen Orca materials and prompt bodies exceed the current CLI transport limit (12000 UTF-16 units).")
		}
	}
	branch := "codex/runner-" + id
	worktree := executionWorkspace(dir, id)
	r.Snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{Version: 1, Backend: kind, Protocol: protocol, Target: target, Agent: b.AgentSpec{Name: "codex"}, RunID: id, AttemptID: attempt, WorkspacePath: worktree, WorkspaceBranch: branch, BaseCommit: request.Repository.BaseRevision}, Session: b.SessionContext{ProjectID: repo.CommonDir, WorkflowRunID: id, ExecutionID: id, AttemptID: attempt, Agent: b.AgentSpec{Name: "codex"}, Target: target, AttemptNumber: 1}, ConfigurationDigest: r.InputDigest, ResourceGeneration: generation, WorkspacePlan: b.WorkspaceIntent{Path: worktree, Branch: branch, BaseCommit: request.Repository.BaseRevision}}
	r.Workflow = workflow.Snapshot{StepID: workflow.StepImplement}
	return r, nil
}

func writeFixed(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if prior, err := os.ReadFile(path); err == nil {
		if string(prior) != string(content) {
			return errors.New("frozen material changed")
		}
		return nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = file.Write(content)
	if err == nil {
		err = file.Sync()
	}
	return errors.Join(err, file.Close())
}
func materialize(r *runstate.Record) error {
	if r.Request.Execution.Adapter == "orca-terminal" {
		for i := range r.Materials {
			r.Materials[i].ExecutionLocation = "prompt:" + r.Materials[i].Path
		}
		return nil
	}
	gitfile, err := os.ReadFile(filepath.Join(r.Snapshot.Ref.WorkspacePath, ".git"))
	if err != nil {
		return err
	}
	admin, ok := strings.CutPrefix(strings.TrimSpace(string(gitfile)), "gitdir: ")
	if !ok || !filepath.IsAbs(admin) {
		return errors.New("prepared worktree lacks its owned Git admin directory")
	}
	for i := range r.Materials {
		m := &r.Materials[i]
		path := filepath.Join(admin, "steerrunner-inputs", filepath.FromSlash(m.Path))
		if err := writeFixed(path, []byte(m.Content)); err != nil {
			return err
		}
		m.SnapshotPath = path
		m.ExecutionLocation = path
	}
	return nil
}
func queueInput(r *runstate.Record, input workflow.StageInput, feedback string) error {
	input.CommandID, input.AttemptID, input.Version = contracts.NewID(), r.Snapshot.Session.AttemptID, uint64(len(r.Workflow.StageInputs)+1)
	if len(r.Materials) < 2 {
		return fmt.Errorf("frozen material manifest incomplete")
	}
	refs := []workflow.FileReference{}
	location := func(m runstate.Material) string {
		if m.ExecutionLocation != "" {
			return m.ExecutionLocation
		}
		return m.SnapshotPath
	}
	for _, m := range r.Materials[2:] {
		refs = append(refs, workflow.FileReference{Path: location(m)})
	}
	// Use explicit JSON names for the fixed material references consumed by Workflow.
	input.MaterialFacts = canonical(map[string]any{"spec": map[string]string{"path": location(r.Materials[0])}, "plan": map[string]string{"path": location(r.Materials[1])}, "references": refs})
	prompt, err := workflow.RenderInputWithBody(r.Workflow, input, r.PromptBodies[input.StepID])
	if err != nil {
		return err
	}
	if feedback != "" {
		prompt += "\nUser feedback for the original session:\n" + feedback
	}
	if r.Request.Execution.Adapter == "orca-terminal" && input.StepID == workflow.StepImplement {
		attached := []map[string]string{}
		for _, m := range r.Materials {
			attached = append(attached, map[string]string{"location": location(m), "path": m.Path, "commit": m.Commit, "sha256": m.Digest, "content": m.Content})
		}
		prompt += "\nFrozen Spec, Plan and references are attached in this input. Read the exact content below; prompt: locations identify these attached documents.\n" + string(canonical(attached))
	}
	prompt += workflow.PublicationInstruction(input.StepID, canonical(r.Request.Publication), r.Request.Repository.Remote, r.Request.Repository.BaseBranch)
	input.Prompt = prompt
	r.Workflow.StepID = input.StepID
	r.Workflow.StageInputs = append(r.Workflow.StageInputs, input)
	r.Snapshot.PendingTurn = &b.TurnInput{StepID: input.StepID, InputVersion: input.Version, CommandID: input.CommandID, Prompt: prompt, Purpose: "business", ReportToolPolicy: input.ReportToolPolicy, LinkedWorktreeArgs: append([]string(nil), r.LinkedWriteArgs...)}
	r.Snapshot.Session.PendingCommandID = input.CommandID
	r.Snapshot.Session.PendingTurnID = ""
	r.Status, r.Phase = "running", "input_queued"
	r.Result = nil
	r.NativeStarted = false
	return nil
}
