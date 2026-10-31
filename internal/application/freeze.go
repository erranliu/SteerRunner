package application

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/execution/agentlaunch"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/evidence"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/publication"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
	ra "github.com/erranliu/SteerRunner/internal/execution/resultacquisition"
	"github.com/erranliu/SteerRunner/internal/execution/terminaltools"
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
)

var repositoryLocks sync.Map

// Keep linked-worktree Git administration paths comfortably below native Git's
// Windows buffer limits. The full request identities remain in accepted input;
// a cache hit still verifies the exact remote and frozen base commit.
func repositoryCache(dir string, repository contracts.Repository) string {
	return filepath.Join(dir, "r", compactDirectoryKey(repository.URL+"\x00"+repository.BaseRevision+"\x00"+repository.LocalPath))
}

// Local workers need to create root-level Git lockfiles (including commit's
// AUTO_MERGE cleanup). Their common directory must therefore belong to one
// Run, rather than granting a worker write access to a shared repository cache.
func privateRunRepository(dir string, repository contracts.Repository, id string) string {
	return filepath.Join(dir, "r", compactDirectoryKey(repository.URL+"\x00"+repository.BaseRevision+"\x00"+repository.LocalPath+"\x00run\x00"+id))
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
	return providerForInput(r.Request.Repository, r.Request.Publication)
}
func providerForInput(repositoryInput contracts.Repository, publicationInput contracts.Publication) (*publication.Client, error) {
	var base string
	var refs []string
	var query b.Command
	if publicationInput.Provider == "gitea" {
		var settings contracts.GiteaSettings
		if err := contracts.DecodeStrict(publicationInput.Settings, &settings); err != nil {
			return nil, err
		}
		base, refs = settings.BaseURL, settings.AuthenticationRefs
	} else {
		var settings contracts.CommandProviderSettings
		if err := contracts.DecodeStrict(publicationInput.Settings, &settings); err != nil {
			return nil, err
		}
		base, refs = settings.BaseURL, settings.AuthenticationRefs
		query = b.Command{Executable: settings.Query.Executable, Args: settings.Query.Args}
	}
	config := publication.Config{Kind: publicationInput.Provider, BaseURL: base, Repository: publicationInput.Repository, Query: query}
	if len(refs) > 0 {
		config.TokenEnv = refs[0]
	}
	settings := evidence.PublicationSettings{Provider: evidence.PRProviderConfig(config), Git: b.Command{Executable: "git", Args: []string{"-c", "core.longpaths=true"}}, Remote: repositoryInput.Remote, RemoteURL: repositoryInput.URL, BaseBranch: repositoryInput.BaseBranch, AuthenticationRefs: refs}
	if err := publication.ValidateSettings(settings); err != nil {
		return nil, err
	}
	if err := config.ValidateRemote(repositoryInput.URL); err != nil {
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
	if err := contracts.ValidateProgress(request); err != nil {
		return nil, err
	}
	if err := contracts.ValidateResultFile(request); err != nil {
		return nil, err
	}
	if workflow.Content(request.Workflow.ID) {
		if err := contracts.ValidateRun(input.Input); err != nil {
			return nil, err
		}
	} else if err := contracts.ValidateRunConfiguration(canonical(contracts.RunConfiguration{Execution: request.Execution, Workflow: request.Workflow})); err != nil {
		return nil, err
	}
	if request.Execution.Adapter == "orca-terminal" && request.Repository.LocalPath == "" {
		return nil, contracts.Fail(2, "invalid_input", "Orca execution requires repository.local_path for its registered project identity.")
	}
	var acquisitionConfig ra.FrozenConfig
	var err error
	flowProfile, supported := workflow.Lookup(request.Workflow.ID, request.Workflow.Version)
	if !supported {
		return nil, contracts.Fail(3, "unsupported_capability", "Workflow interpreter unavailable.")
	}
	if flowProfile.RequiresResultAcquisition() {
		acquisitionConfig, err = ra.LoadConfig(dir)
		if err != nil {
			return nil, contracts.Fail(2, "invalid_input", err.Error())
		}
	}
	id, attempt, generation := contracts.NewID(), contracts.NewID(), contracts.NewID()
	now := time.Now().UTC()
	r := &runstate.Record{AcquisitionConfig: acquisitionConfig, ID: id, Request: request, Allowed: true, Status: "queued", Phase: "accepted", CreatedAt: now, UpdatedAt: now, ResourceDigest: resources.Digest(), CommandStates: map[string]string{}}
	if !workflow.Content(request.Workflow.ID) {
		if _, err := providerFor(r); err != nil {
			return nil, contracts.Fail(2, "invalid_input", "Publication configuration does not match the fixed repository or non-secret schema.")
		}
	}
	target := b.ExecutionTarget{ID: "local", ExecutionHost: "local"}
	kind, protocol := b.BackendLocal, b.ProtocolLocalCodexAppServer
	agent := "codex"
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
		if request.Workflow.AuditModel != nil {
			model := *request.Workflow.AuditModel
			if model.ReasoningEffort == "" {
				model.ReasoningEffort = settings.ReasoningEffort
			}
			r.Request.Workflow.AuditModel = &model
		}
		key, codec, payload = codex.ProfileCodecKey, codex.ProfileCodecVersion, canonical(codex.ProfileConfig{AccessMode: settings.AccessMode, Executable: settings.Executable, Model: settings.Model, ReasoningEffort: settings.ReasoningEffort})
	} else {
		var settings contracts.OrcaSettings
		_ = json.Unmarshal(request.Execution.Settings, &settings)
		agent = settings.AgentName()
		if settings.Transport.Executable == "" {
			settings.Transport.Executable = "orca.exe"
		}
		if settings.Transport.WaitTimeout == "" {
			settings.Transport.WaitTimeout = "30s"
		}
		if settings.AgentLaunch.Executable == "" && agent == "codex" {
			settings.AgentLaunch.Executable = "codex"
		}
		if settings.AgentLaunch.Executable == "" && (agent == "pi" || agent == "dsh") {
			settings.AgentLaunch.Executable = agent + ".cmd"
		}
		executable, err := exec.LookPath(settings.Transport.Executable)
		if err != nil {
			return nil, contracts.Fail(4, "environment_unavailable", "The configured Orca executable is unavailable.")
		}
		settings.Transport.Executable = executable
		if agent == "codex" && settings.AgentLaunch.Model != "" && settings.AgentLaunch.ReasoningEffort == "" {
			settings.AgentLaunch.ReasoningEffort = "medium"
		}
		if (agent == "pi" || agent == "dsh") && settings.AgentLaunch.RuntimeExecutable != "" {
			helper, helperErr := exec.LookPath(settings.AgentLaunch.RuntimeExecutable)
			if helperErr == nil {
				helper, helperErr = filepath.Abs(helper)
			}
			if helperErr != nil {
				return nil, contracts.Fail(4, "environment_unavailable", "The configured native terminal helper is unavailable.")
			}
			settings.AgentLaunch.RuntimeExecutable = helper
		}
		profile := orca.Profile{Executable: executable, Args: settings.Transport.Args, WaitTimeout: settings.Transport.WaitTimeout, Environment: request.Execution.Target.Environment}
		target = profile.Target()
		kind, protocol = b.BackendOrca, b.ProtocolOrcaTerminalV3
		launch := agentlaunch.Config{AccessMode: settings.AccessMode, Executable: settings.AgentLaunch.Executable, Args: settings.AgentLaunch.Args, Model: settings.AgentLaunch.Model, ReasoningEffort: settings.AgentLaunch.ReasoningEffort}
		if request.Execution.Target.Kind == "orca_environment" {
			launch.RepositoryURL, launch.RepositoryRemote = request.Repository.URL, request.Repository.Remote
		}
		if request.Execution.Target.Kind == "local" {
			launch.RuntimeExecutable, launch.RepositoryURL = settings.AgentLaunch.RuntimeExecutable, request.Repository.URL
			if request.Workflow.AuditModel != nil {
				model := *request.Workflow.AuditModel
				if model.ReasoningEffort == "" {
					model.ReasoningEffort = settings.AgentLaunch.ReasoningEffort
				}
				r.Request.Workflow.AuditModel = &model
				launch.AuditModel = &model
			}
			profile.RuntimeExecutable = launch.RuntimeExecutable
			profile.RuntimeSHA256 = settings.AgentLaunch.RuntimeSHA256
		}
		if launch.RuntimeExecutable != "" && agent == "codex" {
			profile.RuntimeConfigurationDigest = codex.RemoteConfigurationDigest(launch.RemoteConfig())
		}
		if agent == "codex" || settings.AgentLaunch.Executable != "" && agent != "pi" && agent != "dsh" {
			r.LaunchCommand, err = launch.Command(agent)
			if err == nil && agent == "codex" {
				_, r.LaunchInput, err = m.FreezeInput(launch.InitializationParts()...)
			}
		}
		if err != nil {
			return nil, contracts.Fail(2, "invalid_input", "Invalid frozen agent launch configuration.")
		}
		r.Request.Execution.Settings = canonical(settings)
		key, codec, payload = orca.ProfileKey, orca.ProfileVersion, canonical(profile)
		if profile.Environment != "" {
			codec = orca.NativeRemoteProfileVersion
		}
	}
	r.InputDigest = reliable.Digest(canonical(r.Request))
	if workflow.Content(request.Workflow.ID) {
		return freezeContent(ctx, dir, r, attempt, generation, payload)
	}
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
	if _, err := git.Output(ctx, cache, "check-ref-format", "refs/heads/"+request.Repository.BaseBranch); err != nil {
		return nil, contracts.Fail(2, "invalid_input", "Invalid base branch.")
	}
	baseRef := "refs/remotes/" + request.Repository.Remote + "/" + request.Repository.BaseBranch
	// A local clone copies the caller's branch heads, which can lag its fetched
	// remote refs. Warm caches also retain old heads. Refresh the exact target
	// before checking ancestry without changing the caller's pinned revision.
	if _, err := git.Output(ctx, cache, "fetch", "--no-tags", "--no-write-fetch-head", "--", request.Repository.Remote, "+refs/heads/"+request.Repository.BaseBranch+":"+baseRef); err != nil {
		return nil, contracts.Fail(4, "environment_unavailable", "Cannot refresh the configured base branch; no run was accepted. "+redactDiagnostic(r, err.Error()))
	}
	if _, err := git.Output(ctx, cache, "cat-file", "-e", request.Repository.BaseRevision+"^{commit}"); err != nil {
		if _, err := git.Output(ctx, cache, "fetch", "--no-tags", request.Repository.Remote, request.Repository.BaseRevision); err != nil {
			return nil, contracts.Fail(2, "invalid_input", "The pinned base commit is unavailable.")
		}
	}
	if _, err := git.Output(ctx, cache, "merge-base", "--is-ancestor", request.Repository.BaseRevision, baseRef); err != nil {
		return nil, contracts.Fail(2, "invalid_input", "Pinned base is not in the configured base branch.")
	}
	if request.Execution.Adapter == "codex-app-server" {
		private := privateRunRepository(dir, request.Repository, id)
		staging, err := os.MkdirTemp(filepath.Dir(private), ".run-clone-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(staging) // Only the private directory allocated above.
		if _, err = git.Output(ctx, dir, "clone", "--no-checkout", "--no-hardlinks", "--dissociate", "--origin", request.Repository.Remote, "--", cache, staging); err != nil {
			return nil, contracts.Fail(4, "environment_unavailable", "Cannot prepare the private Run repository; no run was accepted. "+redactDiagnostic(r, err.Error()))
		}
		if _, err = git.Output(ctx, staging, "remote", "set-url", request.Repository.Remote, request.Repository.URL); err != nil {
			return nil, err
		}
		// Local clone copies branch heads, not the refreshed tracking ref. Keep
		// the exact validated base visible in the worker's independent clone.
		if _, err = git.Output(ctx, staging, "fetch", "--no-tags", "--no-write-fetch-head", "--", cache, "+"+baseRef+":"+baseRef); err != nil {
			return nil, err
		}
		if err = os.Rename(staging, private); err != nil {
			return nil, err
		}
		cache = private
	}
	repo, err := workspace.ResolveRepository(ctx, git, cache)
	if err != nil {
		return nil, err
	}
	r.RepositoryPath, r.RepositoryCommon = repo.Path, repo.CommonDir
	if agent == "pi" || agent == "dsh" {
		var settings contracts.OrcaSettings
		_ = json.Unmarshal(r.Request.Execution.Settings, &settings)
		var p orca.Profile
		_ = json.Unmarshal(payload, &p)
		if settings.AgentLaunch.RuntimeExecutable != "" {
			metadataBytes, e := process.New(30*time.Second).OutputCommand(ctx, b.LaunchCommand{Executable: settings.AgentLaunch.RuntimeExecutable, Args: []string{"--terminal-metadata"}})
			var metadata orca.TerminalMetadata
			if e != nil || len(metadataBytes) > 4096 || contracts.DecodeStrict(metadataBytes, &metadata) != nil || metadata.Version != 4 || metadata.Contract != terminaltools.Contract || len(metadata.RuntimeSHA256) != 64 || len(metadata.BundleSHA256) != 64 || len(metadata.PiSHA256) != 64 || len(metadata.DSHSHA256) != 64 || settings.AgentLaunch.RuntimeSHA256 != "" && metadata.RuntimeSHA256 != settings.AgentLaunch.RuntimeSHA256 {
				return nil, contracts.Fail(4, "environment_unavailable", "The selected local terminal helper metadata or frozen build does not match.")
			}
			settings.AgentLaunch.RuntimeSHA256 = metadata.RuntimeSHA256
			p.RuntimeSHA256 = metadata.RuntimeSHA256
			r.Request.Execution.Settings = canonical(settings)
			r.InputDigest = reliable.Digest(canonical(r.Request))
			// The transport adds a short digest directory bound to the full Run,
			// attempt and generation. Repeating the Run ID here can exceed the
			// native socket path limit even with a short state directory.
			c := orca.TerminalConfig{ReceiptVersion: 3, Agent: agent, ProjectID: repo.CommonDir, ExecutionID: id, AttemptID: attempt, Generation: generation, ConfigurationDigest: r.InputDigest, SessionID: contracts.NewID(), Executable: settings.AgentLaunch.Executable, StateDirectory: filepath.Join(dir, "terminal-agents", id, attempt), AuthenticationState: dir, RepositoryURL: request.Repository.URL, AccessMode: settings.AccessMode, Model: settings.AgentLaunch.Model, ReasoningEffort: settings.AgentLaunch.ReasoningEffort}
			c.BundleSHA256 = metadata.BundleSHA256
			c.EvidenceSHA256 = metadata.PiSHA256
			if agent == "dsh" {
				c.EvidenceSHA256 = metadata.DSHSHA256
			}
			// Pi session IDs use its native UUID spelling; the Run remains a hex ID.
			c.SessionID = c.SessionID[:8] + "-" + c.SessionID[8:12] + "-" + c.SessionID[12:16] + "-" + c.SessionID[16:20] + "-" + c.SessionID[20:]
			codec = orca.FullAccessTerminalProfileVersion
			p.Terminal = &c
			r.LaunchCommand = c.Command(settings.AgentLaunch.RuntimeExecutable)
		}
		payload = canonical(p)
	}
	if r.Request.Execution.Adapter == "orca-terminal" && (r.Request.ProgressFile != nil || r.Request.ResultFile != nil || r.LaunchCommand != "") {
		progressRun := ""
		if r.Request.ProgressFile != nil {
			progressRun = r.ID
		}
		resultRun := ""
		if r.Request.ResultFile != nil {
			resultRun = r.ID
		}
		r.LaunchCommand = agentlaunch.FileCommands(r.LaunchCommand, progressRun, resultRun)
	}
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
		// Development documents belong to the execution worktree. The caller
		// supplies their paths; admission does not read or prepare their files.
		r.Materials = append(r.Materials, runstate.Material{MaterialView: contracts.MaterialView{Kind: "worktree", Path: doc.Path, Commit: request.Repository.BaseRevision, ExecutionLocation: doc.Path}})
	}
	r.PromptBodies = map[string]string{}
	for _, state := range flowProfile.States {
		if state.PromptKey != "" {
			r.PromptBodies[state.PromptKey] = state.Prompt
		}
	}
	for step, body := range request.Workflow.PromptOverrides {
		r.PromptBodies[step] = body
	}
	branch := "codex/runner-" + id
	worktree := executionWorkspace(dir, id)
	namePrefix := request.NamePrefix
	if namePrefix == "" {
		namePrefix = "sr"
	}
	r.Snapshot = b.LifecycleSnapshot{Ref: b.ExecutionRef{Version: 1, Backend: kind, Protocol: protocol, Target: target, Agent: b.AgentSpec{Name: agent}, RunID: id, AttemptID: attempt, WorkspacePath: worktree, WorkspaceBranch: branch, BaseCommit: request.Repository.BaseRevision}, Session: b.SessionContext{ProjectID: repo.CommonDir, NamePrefix: namePrefix, WorkflowRunID: id, ExecutionID: id, AttemptID: attempt, Agent: b.AgentSpec{Name: agent}, Target: target, AttemptNumber: 1}, ConfigurationDigest: r.InputDigest, ResourceGeneration: generation, WorkspacePlan: b.WorkspaceIntent{Path: worktree, Branch: branch, BaseCommit: request.Repository.BaseRevision}}
	r.Workflow = workflow.Snapshot{StepID: flowProfile.Initial, Config: &flowProfile}
	if err := freezeProjectPolicies(ctx, r); err != nil {
		return nil, err
	}
	if request.Execution.Adapter == "codex-app-server" {
		var settings contracts.CodexSettings
		_ = json.Unmarshal(r.Request.Execution.Settings, &settings)
		r.Workflow.ExecutionModel = &workflow.ModelSettings{Model: settings.Model, ReasoningEffort: settings.ReasoningEffort}
	}
	if flowProfile.AuditModelRequired {
		settings, modelErr := contracts.ExecutionModel(r.Request.Execution)
		if modelErr != nil {
			return nil, modelErr
		}
		r.Workflow.Config = &flowProfile
		r.Workflow.ExecutionModel = &workflow.ModelSettings{Model: settings.Model, ReasoningEffort: settings.ReasoningEffort}
		model := *r.Request.Workflow.AuditModel
		r.Workflow.AuditModel = &model
	}
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
func queueInput(r *runstate.Record, input workflow.StageInput, feedback string) error {
	if err := validateWorkflow(r); err != nil {
		return err
	}
	state, defined := r.Workflow.State(input.StepID)
	if state.Contract == "publication" && !workflow.AuditsPassed(r.Workflow) {
		return errors.New("publication requires current audits")
	}
	if !defined || state.Kind != "execute" && state.Kind != "report" {
		return errors.New("no automatic input for this step")
	}
	if state.Contract == "audit" && state.Kind == "execute" && !workflow.AuditSource(r.Workflow, input) {
		return errors.New("audit lacks its current successful implementation report")
	}
	// A restored, unsent input already contains its complete frozen prompt,
	// including any user answer. Only fresh stages need rendering and models.
	if input.Prompt != "" && feedback != "" {
		return errors.New("cannot append feedback to an already frozen input")
	}
	if err := workflow.VerifyPrerequisites(r.Workflow, input, state); err != nil {
		return err
	}
	if state.Kind == "report" && input.ReportToolPolicy == "" {
		return errors.New("originating adapter report policy unavailable")
	}
	if state.Kind == "report" && input.Prompt == "" {
		original := workflow.Input(r.Workflow, input.SourceCommandID)
		acquired := workflow.Acquired(r.Workflow, input.SourceResultID)
		if original == nil || acquired == nil || acquired.Source.CommandID != input.SourceCommandID || acquired.Source.TurnID != input.SourceTurnID || acquired.Source.InputVersion != input.SourceVersion {
			return errors.New("report lacks frozen original result binding")
		}
		originalState, _ := r.Workflow.State(original.StepID)
		if originalState.Kind != "execute" || originalState.Report != input.StepID || originalState.ReportLimit != 1 {
			return errors.New("report is not authorized by original state definition")
		}

		for _, prior := range r.Workflow.StageInputs {
			ps, _ := r.Workflow.State(prior.StepID)
			if ps.Kind == "report" && prior.SourceCommandID == input.SourceCommandID {
				return errors.New("original source report budget exhausted")
			}
		}
	}
	if input.Prompt == "" {
		var parts []m.InputPart
		var err error
		if r.Workflow.ExecutionModel != nil {
			model := r.Workflow.ExecutionModel
			if state, _ := r.Workflow.State(input.StepID); state.ModelRole == "audit" {
				model = r.Workflow.AuditModel
			}
			input.Model, input.ReasoningEffort = model.Model, model.ReasoningEffort
		}
		if state.Contract == "final" {
			parts, err = contentParts(r, feedback)
			if err != nil {
				return err
			}
		} else {
			refs := []workflow.FileReference{}
			for _, doc := range r.Request.Documents.References {
				refs = append(refs, workflow.FileReference{Path: doc.Path})
			}
			// Use explicit JSON names for the fixed material references consumed by Workflow.
			input.MaterialFacts = canonical(map[string]any{"spec": map[string]string{"path": r.Request.Documents.Spec.Path}, "plan": map[string]string{"path": r.Request.Documents.Plan.Path}, "references": refs})
			_, caller := r.Request.Workflow.PromptOverrides[state.PromptKey]
			parts, err = workflow.InputParts(r.Workflow, input, r.PromptBodies[state.PromptKey], caller)
			if err != nil {
				return err
			}
			if feedback != "" {
				parts = append(parts, m.InputPart{Source: "feedback.label", Owner: m.RunnerInput, Text: workflow.FeedbackLabel}, m.InputPart{Source: "caller.feedback", Owner: m.CallerInput, Text: feedback})
			}
			parts = append(parts, m.InputPart{Source: "publication.target", Owner: m.RunnerInput, Text: workflow.PublicationInstruction(state, canonical(r.Request.Publication), r.Request.Repository.Remote, r.Request.Repository.BaseBranch)})
			input.SuccessOutput = state.SuccessOutput
		}
		if state.Kind == "execute" && (state.Contract == "implementation" || state.Contract == "audit") {
			if r.Request.ProgressFile != nil {
				text := workflow.ProgressInstruction
				if priorFileInstruction(r, "progress") {
					text = workflow.ContinueProgressInstruction
				}
				parts = append(parts, m.InputPart{Source: "progress", Owner: m.RunnerInput, Text: "\n" + text})
			}
			if r.Request.ResultFile != nil {
				text := workflow.ResultFileInstruction
				if priorFileInstruction(r, "result_file") {
					text = workflow.ContinueResultInstruction
				}
				parts = append(parts, m.InputPart{Source: "result_file", Owner: m.RunnerInput, Text: "\n" + text})
			}
		}
		if r.Request.Execution.Adapter == "orca-terminal" {
			// Preserve the frozen logical body; native normalization/framing owns
			// the final send budget. Validate the normalized body early as well.
			_, _, err = m.FreezeInput(m.NativeInputParts(parts)...)
			if err == nil {
				input.Prompt, input.Budget, err = m.DescribeInput(parts...)
			}
		} else {
			input.Prompt, input.Budget, err = m.FreezeInput(parts...)
		}
		if err != nil {
			return err
		}
	} else {
		if err := ensureStageBudget(r, &input); err != nil {
			return err
		}
	}
	input.CommandID, input.AttemptID, input.Version = contracts.NewID(), r.Snapshot.Session.AttemptID, uint64(len(r.Workflow.StageInputs)+1)
	if state.Kind == "execute" && state.Contract == "implementation" {
		r.Workflow.ImplementationResultID = ""
		r.Workflow.AuditResultID = ""
		r.Workflow.PublicationResultID = ""
		r.Workflow.Audits = nil
	}
	// A newly authorized stage gets its own preparation budget. Time spent in
	// the previous business turn must not expire preparation of the next one.
	// Keep the initial workspace budget, and never reset unresolved intents.
	if len(r.Workflow.StageInputs) > 0 {
		r.PreparationDeadline = time.Time{}
	}
	r.Workflow.StepID = input.StepID
	r.Workflow.StageInputs = append(r.Workflow.StageInputs, input)
	if state.Contract == "final" {
		r.ContentResult = nil
		text := feedback
		if text == "" {
			text, _, _ = r.Request.ContentInput()
		}
		requestCommand := input.FeedbackDecisionID
		if requestCommand == "" {
			requestCommand = r.SubmitCommandID
		}
		bound := false
		for i := range r.History {
			entry := &r.History[i]
			if entry.Role == "user" && entry.CommandID == "" && entry.RequestCommandID == requestCommand {
				entry.CommandID, entry.SessionID = input.CommandID, r.Snapshot.Session.SessionID
				bound = true
				break
			}
		}
		if !bound {
			r.History = append(r.History, contracts.HistoryEntry{RequestCommandID: requestCommand, CommandID: input.CommandID, SessionID: r.Snapshot.Session.SessionID, Role: "user", Text: text})
		}
	}
	r.Snapshot.PendingTurn = &b.TurnInput{Budget: input.Budget.Clone(), StepID: input.StepID, InputVersion: input.Version, CommandID: input.CommandID, Prompt: input.Prompt, Purpose: "business", ReportToolPolicy: input.ReportToolPolicy, LinkedWorktreeArgs: append([]string(nil), r.LinkedWriteArgs...), Model: input.Model, ReasoningEffort: input.ReasoningEffort}
	r.Snapshot.Session.PendingCommandID = input.CommandID
	r.Snapshot.Session.PendingTurnID = ""
	r.Status, r.Phase = "running", "input_queued"
	r.Result = nil
	r.NativeStarted = false
	return nil
}
