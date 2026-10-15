package livetest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManifestRejectsChangedInputsAndUnsafeTargets(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := func(name string) (string, string) {
		path := filepath.Join(root, name)
		body := []byte(name)
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		return path, hex.EncodeToString(sum[:])
	}
	exe, exeSHA := file("runner.exe")
	spec, specSHA := file("spec.md")
	plan, planSHA := file("plan.md")
	repo := filepath.Join(root, "SteerlineTest")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	m := Manifest{Schema: "steerrunner-test/v1", ID: "012345abcdef", Level: "L2", Scenario: "local-codex.smoke", Driver: "development-smoke", Root: root, RunnerExe: exe, RunnerSHA256: exeSHA, SourceCommit: strings40, RepositoryPath: repo, SourceRevision: strings40, BaseBranch: "codex/srtest/012345abcdef/base", BaseRevision: strings40, SpecPath: spec, SpecSHA256: specSHA, PlanPath: plan, PlanSHA256: planSHA, Execution: contracts.Execution{Adapter: "codex-app-server", Target: contracts.Target{Kind: "local"}, Settings: json.RawMessage(`{}`)}, Publication: contracts.Publication{Repository: Repository, BaseBranch: "codex/srtest/012345abcdef/base", Settings: json.RawMessage(`{}`)}, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	wire, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Manifest
	if err := contracts.DecodeStrict(wire, &decoded); err != nil {
		t.Fatal("manifest wire cannot be strictly decoded", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	audited := m
	audited.Driver = "development-audited"
	audited.AuditModel = &workflow.ModelSettings{Model: "reviewer", ReasoningEffort: "high"}
	audited.Execution.Settings = json.RawMessage(`{"model":"executor","reasoning_effort":"medium"}`)
	if err := audited.Validate(); err != nil {
		t.Fatal("valid audited fixture rejected", err)
	}
	for _, mutate := range []func(*Manifest){
		func(x *Manifest) { x.AuditModel = nil },
		func(x *Manifest) { x.Execution.Settings = json.RawMessage(`{}`) },
		func(x *Manifest) { x.AuditModel = &workflow.ModelSettings{Model: "executor"} },
		func(x *Manifest) { x.Driver = "development-smoke" },
		func(x *Manifest) { x.Execution.Adapter = "orca-terminal" },
	} {
		x := audited
		mutate(&x)
		if x.Validate() == nil {
			t.Fatal("invalid audited fixture was accepted", x)
		}
	}
	for _, mutate := range []func(*Manifest){func(x *Manifest) { x.BaseBranch = "main" }, func(x *Manifest) { x.Publication.Repository = "other/repo" }, func(x *Manifest) { x.Level = "L1" }, func(x *Manifest) { x.RunnerSHA256 = "wrong" }, func(x *Manifest) { x.SpecPath = filepath.Dir(root) }, func(x *Manifest) {
		x.Level = "L3"
		x.Execution.Adapter = "orca-terminal"
		x.Execution.Target = contracts.Target{Kind: "orca_environment", Environment: "remote"}
	}} {
		x := m
		mutate(&x)
		if x.Validate() == nil {
			t.Fatal("invalid live fixture was accepted", x)
		}
	}
	if err := os.WriteFile(spec, []byte("changed after preparation"), 0600); err != nil {
		t.Fatal(err)
	}
	if m.Validate() == nil {
		t.Fatal("changed frozen Spec was accepted")
	}
}

const strings40 = "0123456789012345678901234567890123456789"

func TestArtifactRejectsStaleAndForgedEvidence(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	valid := "# Evidence sample 101\n- sample_id: 101\n- generated_at: " + now.Format(time.RFC3339) + "\n- 模拟任务\n- 未修改业务代码\n"
	if _, err := ValidateArtifact([]byte(valid), now, now); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{valid + "extra\n", valid[:len(valid)-1], "done", valid} {
		from := now
		if body == valid {
			from = now.Add(time.Second)
		}
		if _, err := ValidateArtifact([]byte(body), from, now.Add(time.Minute)); err == nil {
			t.Fatal("accepted invalid or stale artifact", body)
		}
	}
}

func TestMergeGuardRejectsForeignAndUnverifiedPR(t *testing.T) {
	m := Manifest{BaseBranch: "codex/srtest/012345abcdef/base"}
	v := contracts.RunView{Status: "pr_ready", Execution: contracts.ExecutionView{Branch: "worker", SessionID: "session", AttemptID: "attempt", TurnID: "turn"}, PR: &contracts.PRFact{Repository: Repository, HeadRepository: Repository, BaseBranch: m.BaseBranch, HeadBranch: "worker", HeadRevision: "0123456789012345678901234567890123456789", Number: 12, URL: "https://github.com/erranliu/SteerlineTest/pull/12", State: "open"}, Final: &contracts.FinalView{Verified: true, Source: "native", ItemID: "item", SessionID: "session", AttemptID: "attempt", CommandID: "command", TurnID: "turn"}}
	if err := CheckPR(m, v); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*contracts.RunView){func(x *contracts.RunView) { x.PR.BaseBranch = "main" }, func(x *contracts.RunView) { x.PR.HeadRepository = "other/repo" }, func(x *contracts.RunView) { x.PR.HeadBranch = "other" }, func(x *contracts.RunView) { x.PR.QueryError = "offline" }, func(x *contracts.RunView) { x.PR.HeadRevision = "" }, func(x *contracts.RunView) { x.PR.URL = "https://github.com/other/repo/pull/12" }, func(x *contracts.RunView) { x.Final.Verified = false }, func(x *contracts.RunView) { x.Final.SessionID = "replacement" }, func(x *contracts.RunView) { x.Status = "running" }} {
		x := v
		p := *v.PR
		f := *v.Final
		x.PR = &p
		x.Final = &f
		mutate(&x)
		if CheckPR(m, x) == nil {
			t.Fatal("unsafe merge was accepted", x)
		}
	}
	m.RemoteRuntimeID = "remote"
	if CheckPR(m, v) == nil {
		t.Fatal("accepted local final as remote evidence")
	}
}
