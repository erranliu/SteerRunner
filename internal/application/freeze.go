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
	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/evidence"
	m "github.com/erranliu/SteerRunner/internal/execution/model"
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
	flowProfile, supported := workflow.Lookup(request.Workflow.ID, request.Workflow.Version)
	if !supported {
		return nil, contracts.Fail(3, "unsupported_capability", "Workflow interpreter unavailable.")
	}
	var err error
	id, attempt, generation := contracts.NewID(), contracts.NewID(), contracts.NewID()
	now := time.Now().UTC()
	r := &runstate.Record{ID: id, Request: request, Allowed: true, Status: "queued", Phase: "accepted", CreatedAt: now, UpdatedAt: now, ResourceDigest: resources.Digest(), CommandStates: map[string]string{}}
	if !workflow.Content(request.Workflow.ID) && request.Workflow.ID != workflow.SingleExecution {
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
	structured := false
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
		key, codec, payload = codex.ProfileCodecKey, codex.ProfileVersionForMode(settings.AccessMode), canonical(codex.ProfileConfig{AccessMode: settings.AccessMode, Executable: settings.Executable, Model: settings.Model, ReasoningEffort: settings.ReasoningEffort})
	} else {
		var settings contracts.OrcaSettings
		_ = json.Unmarshal(request.Execution.Settings, &settings)
		agent = settings.AgentName()
		audited := contracts.OrcaStructuredAudited(request.Execution, request.Workflow)
		development := audited || contracts.OrcaStructuredDevelopment(request.Execution, request.Workflow)
		structured = development || contracts.OrcaStructuredExecute(request.Execution, request.Workflow)
		if settings.Transport.Executable == "" {
			settings.Transport.Executable = "orca.exe"
		}
		if settings.Transport.WaitTimeout == "" {
			settings.Transport.WaitTimeout = "30s"
		}
		executable, err := exec.LookPath(settings.Transport.Executable)
		if err != nil {
			return nil, contracts.Fail(4, "environment_unavailable", "The configured Orca executable is unavailable.")
		}
		settings.Transport.Executable = executable
		if agent == "codex" && settings.AgentLaunch.Model != "" && settings.AgentLaunch.ReasoningEffort == "" {
			settings.AgentLaunch.ReasoningEffort = "medium"
		}
		profile := orca.Profile{Executable: executable, Args: settings.Transport.Args, WaitTimeout: settings.Transport.WaitTimeout, Environment: request.Execution.Target.Environment}
		target = profile.Target()
		if !structured {
			return nil, contracts.Fail(3, "unsupported_capability", "The selected Orca agent and workflow have no structured executor.")
		}
		settings.AccessMode = "full_access"
		r.Request.Execution.Settings = canonical(settings)
		kind, protocol = b.BackendOrca, b.ProtocolOrcaAgentSessionV1
		structuredProfile := orca.AgentSessionProfile{Transport: profile, Model: settings.AgentLaunch.Model, ReasoningEffort: settings.AgentLaunch.ReasoningEffort, AccessMode: settings.AccessMode, Development: development, Audited: audited}
		if audited {
			model := *request.Workflow.AuditModel
			if model.ReasoningEffort == "" {
				model.ReasoningEffort = settings.AgentLaunch.ReasoningEffort
			}
			r.Request.Workflow.AuditModel = &model
			r.ResultFileContract = contracts.ResultFileSnapshotContract
		}
		if agent == "pi" {
			r.ResultFileContract = contracts.ResultFileSnapshotContract
		}
		codec = orca.AgentSessionVersion
		if profile.Environment != "" {
			host, err := native.UsageHostID(profile.Environment)
			if err != nil {
				return nil, contracts.Fail(4, "environment_unavailable", "The selected paired Orca host identity is unavailable; no run was accepted.")
			}
			structuredProfile.PairingHostID = host
		}
		if development || profile.Environment != "" || agent == "pi" {
			codec = orca.AgentSessionCloseProfileVersion
		}
		key, payload = orca.AgentSessionProfileKey, canonical(structuredProfile)
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
	base := request.Repository.BaseRevision
	callerPrepared := request.Workflow.ProjectPolicySlots != nil || request.Workflow.ID == workflow.SingleExecution
	hasCommit := func(dir string) bool {
		kind, err := git.Output(ctx, dir, "--no-replace-objects", "cat-file", "-t", base)
		return err == nil && strings.TrimSpace(string(kind)) == "commit"
	}
	if !callerPrepared || !hasCommit(cache) {
		_, err := git.Output(ctx, cache, "fetch", "--no-tags", "--no-write-fetch-head", "--", request.Repository.Remote, "+refs/heads/"+request.Repository.BaseBranch+":"+baseRef)
		if err != nil && !callerPrepared {
			return nil, contracts.Fail(4, "environment_unavailable", "Cannot refresh the configured base branch; no run was accepted. "+redactDiagnostic(r, err.Error()))
		}
	}
	if !hasCommit(cache) {
		if _, err := git.Output(ctx, cache, "fetch", "--no-tags", "--no-write-fetch-head", "--", request.Repository.Remote, base); err != nil || !hasCommit(cache) {
			return nil, contracts.Fail(2, "invalid_input", "The pinned base commit is unavailable.")
		}
	}
	if !callerPrepared {
		if _, err := git.Output(ctx, cache, "merge-base", "--is-ancestor", base, baseRef); err != nil {
			return nil, contracts.Fail(2, "invalid_input", "Pinned base is not in the configured base branch.")
		}
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
		// A local clone copies branch heads. The fixed object may only be in
		// the cache, independently of the tracking ref used for publication.
		if _, refErr := git.Output(ctx, cache, "show-ref", "--verify", "--quiet", baseRef); refErr == nil {
			if _, err = git.Output(ctx, staging, "fetch", "--no-tags", "--no-write-fetch-head", "--", cache, "+"+baseRef+":"+baseRef); err != nil {
				return nil, err
			}
		}
		if !hasCommit(staging) {
			if _, err = git.Output(ctx, staging, "fetch", "--no-tags", "--no-write-fetch-head", "--", cache, base); err != nil || !hasCommit(staging) {
				return nil, contracts.Fail(4, "environment_unavailable", "Cannot copy the pinned base commit to the private Run repository.")
			}
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
	profile, err := b.NewImplementationDataEnvelope(b.ImplementationDataEnvelope{ImplementationKey: key, CodecVersion: codec, Purpose: b.ImplementationConfigPurpose, ProjectID: repo.CommonDir, ProfileID: id, AcceptedConfigVersion: r.InputDigest}, payload)
	if err != nil {
		return nil, err
	}
	r.Profile = []b.ImplementationDataEnvelope{profile}
	if structured {
		r.StopEvidenceScope = orca.AgentSessionStopScope
	}
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
	if request.Workflow.ID != workflow.SingleExecution {
		if err := freezeProjectPolicies(ctx, r); err != nil {
			return nil, err
		}
	}
	if request.Execution.Adapter == "codex-app-server" {
		var settings contracts.CodexSettings
		_ = json.Unmarshal(r.Request.Execution.Settings, &settings)
		r.Workflow.ExecutionModel = &workflow.ModelSettings{Model: settings.Model, ReasoningEffort: settings.ReasoningEffort}
	}
	if request.Execution.Adapter == "orca-terminal" {
		var settings contracts.OrcaSettings
		_ = json.Unmarshal(r.Request.Execution.Settings, &settings)
		r.Workflow.ExecutionModel = &workflow.ModelSettings{Model: settings.AgentLaunch.Model, ReasoningEffort: settings.AgentLaunch.ReasoningEffort}
	}
	if flowProfile.AuditModelRequired {
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
	if input.StepID == workflow.StepPRReady && r.DevelopmentMessage() && input.FeedbackDecisionID != "" {
		return queueRunMessage(r, input, feedback)
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
			switch state.ModelRole {
			case "execution":
			case "audit":
				model = r.Workflow.AuditModel
			default:
				return errors.New("workflow stage has an unknown model role")
			}
			if model == nil || state.ModelRole == "audit" && model.Model == "" {
				return errors.New("workflow stage lacks its configured model")
			}
			input.Model, input.ReasoningEffort = model.Model, model.ReasoningEffort
		}
		if state.Contract == "final" && workflow.Content(r.Request.Workflow.ID) {
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
		if !state.PlanDriven && state.Kind == "execute" && (state.Contract == "implementation" || state.Contract == "audit" || r.Request.Workflow.ID == workflow.SingleExecution) {
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
				} else if resultContract(r) == contracts.ResultFileSnapshotContract {
					text = workflow.ResultFileSnapshotInstruction
					if r.Request.Workflow.ID == "development" {
						text = workflow.DevelopmentResultFileSnapshotInstruction
					}
				}
				parts = append(parts, m.InputPart{Source: "result_file", Owner: m.RunnerInput, Text: "\n" + text})
			}
		}
		initial, _ := r.Workflow.State(r.Workflow.Config.Initial)
		if r.Snapshot.Ref.Protocol == b.ProtocolOrcaAgentSessionV1 && (state.PlanDriven || !initial.PlanDriven) && (r.Request.ResultFile != nil || r.Request.ProgressFile != nil) {
			bindings := map[string]string{}
			if r.Request.ResultFile != nil {
				bindings["STEERLINE_RESULT_FILE"] = workspace.ResultRelative(r.ID)
			}
			if r.Request.ProgressFile != nil {
				bindings["STEERLINE_PROGRESS_FILE"] = workspace.ProgressRelative(r.ID)
			}
			instruction := workflow.OrcaFileEnvironmentInstruction
			if state.PlanDriven {
				instruction = workflow.PlanDrivenOrcaFileEnvironment
			}
			parts = append(parts, m.InputPart{Source: "orca.file_environment", Owner: m.RunnerInput, Text: "\n" + instruction}, m.InputPart{Source: "orca.file_bindings", Owner: m.RunnerInput, Text: "\nRunner file bindings: " + string(canonical(bindings))})
		} else if state.PlanDriven && r.Request.Execution.Adapter == "codex-app-server" {
			if r.Request.ProgressFile != nil {
				parts = append(parts, m.InputPart{Source: "progress", Owner: m.RunnerInput, Text: "\n进度文件：STEERLINE_PROGRESS_FILE"})
			}
			if r.Request.ResultFile != nil {
				parts = append(parts, m.InputPart{Source: "result_file", Owner: m.RunnerInput, Text: "\n结果文件：STEERLINE_RESULT_FILE"})
			}
		}
		if r.Request.Execution.Adapter == "orca-terminal" && r.Snapshot.Ref.Protocol != b.ProtocolOrcaAgentSessionV1 {
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
	if state.Contract == "final" && workflow.Content(r.Request.Workflow.ID) {
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
	r.Snapshot.PendingTurn = &b.TurnInput{Budget: input.Budget.Clone(), StepID: input.StepID, InputVersion: input.Version, CommandID: input.CommandID, Prompt: input.Prompt, Purpose: "business", ReportToolPolicy: input.ReportToolPolicy, LinkedWorktreePaths: append([]string(nil), r.LinkedGitPaths...), Model: input.Model, ReasoningEffort: input.ReasoningEffort}
	r.Snapshot.Session.PendingCommandID = input.CommandID
	r.Snapshot.Session.PendingTurnID = ""
	r.Status, r.Phase = "running", "input_queued"
	r.Result = nil
	r.NativeStarted = false
	return nil
}

// queueRunMessage freezes only caller text, retaining the completed workflow and original resources.
func queueRunMessage(r *runstate.Record, input workflow.StageInput, text string) error {
	if text == "" {
		return errors.New("message text is required")
	}
	var err error
	input.Prompt, input.Budget, err = m.FreezeInput(m.InputPart{Source: "caller.message", Owner: m.CallerInput, Text: text})
	if err != nil {
		return err
	}
	input.CommandID, input.AttemptID, input.Version = contracts.NewID(), r.Snapshot.Session.AttemptID, uint64(len(r.Workflow.StageInputs)+1)
	if r.Workflow.ExecutionModel != nil {
		input.Model, input.ReasoningEffort = r.Workflow.ExecutionModel.Model, r.Workflow.ExecutionModel.ReasoningEffort
	}
	r.Workflow.StageInputs = append(r.Workflow.StageInputs, input)
	r.History = append(r.History, contracts.HistoryEntry{RequestCommandID: input.FeedbackDecisionID, CommandID: input.CommandID, SessionID: r.Snapshot.Session.SessionID, Role: "user", Text: text})
	// Acceptance already records the user entry, so bind it instead of duplicating it.
	for i := 0; i < len(r.History)-1; i++ {
		if h := &r.History[i]; h.Role == "user" && h.CommandID == "" && h.RequestCommandID == input.FeedbackDecisionID {
			h.CommandID, h.SessionID = input.CommandID, r.Snapshot.Session.SessionID
			r.History = r.History[:len(r.History)-1]
			break
		}
	}
	r.Snapshot.PendingTurn = &b.TurnInput{Budget: input.Budget.Clone(), StepID: input.StepID, InputVersion: input.Version, CommandID: input.CommandID, Prompt: input.Prompt, Purpose: "message", LinkedWorktreePaths: append([]string(nil), r.LinkedGitPaths...), Model: input.Model, ReasoningEffort: input.ReasoningEffort}
	r.Snapshot.Session.PendingCommandID, r.Snapshot.Session.PendingTurnID = input.CommandID, ""
	r.Status, r.Phase = "running", "input_queued"
	r.Result, r.LastFinal = nil, nil
	r.NativeStarted = false
	r.PreparationDeadline = time.Time{}
	return nil
}
