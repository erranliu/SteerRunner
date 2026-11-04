package resources

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
)

func TestOrcaPublishedContractsMatchAdmissionAndStopScopes(t *testing.T) {
	raw, err := json.Marshal(Capabilities())
	if err != nil {
		t.Fatal(err)
	}
	var published struct {
		Contracts struct {
			Native struct {
				Agent      string
				Workflow   contracts.Workflow
				AccessMode string   `json:"access_mode"`
				Targets    []string `json:"target_kinds"`
				Scope      string   `json:"stop_evidence_scope"`
				Model      string   `json:"actual_model"`
				Audit      string   `json:"workspace_audit"`
			} `json:"native"`
			Managed struct {
				Scope string `json:"codex_stop_evidence_scope"`
			} `json:"retained_managed_local"`
		} `json:"orca_execution_contracts"`
		Stops map[string]json.RawMessage `json:"stop_evidence"`
	}
	if err := json.Unmarshal(raw, &published); err != nil {
		t.Fatal(err)
	}
	n := published.Contracts.Native
	if len(n.Targets) != 2 || n.Model != "unverified_launch_preference_only" || n.Audit != "unreported" {
		t.Fatal("native publication gained model/audit assurance", n)
	}
	for _, target := range n.Targets {
		t.Run(target, func(t *testing.T) {
			p := orca.Profile{NativeTerminal: true}
			execution := contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: target}, Settings: marshal(map[string]any{"agent": n.Agent, "access_mode": n.AccessMode})}
			if target == "orca_environment" {
				execution.Target.Environment = "explicit-environment"
				p.Environment = execution.Target.Environment
			}
			if contracts.ValidateRunConfiguration(marshal(contracts.RunConfiguration{Execution: execution, Workflow: n.Workflow})) != nil || !Catalog().Declares(execution, &n.Workflow, "submit") {
				t.Fatal("published native configuration is not admitted")
			}
			if scope := orca.StopScope(p, n.Agent); scope != n.Scope || len(published.Stops[scope]) == 0 {
				t.Fatal("published native scope differs from production", scope, n.Scope)
			}
		})
	}
	managed := orca.StopScope(orca.Profile{RuntimeExecutable: "explicit-helper.exe"}, "codex")
	if published.Contracts.Managed.Scope != managed || managed == n.Scope || len(published.Stops[managed]) == 0 {
		t.Fatal("managed roots were silently relabeled native")
	}
}

func TestStructuredExecuteCatalogMatchesNarrowAdmission(t *testing.T) {
	for _, target := range []contracts.Target{{Kind: "local"}, {Kind: "orca_environment", Environment: "id:configured-remote"}} {
		execution := contracts.Execution{Adapter: "orca-terminal", Target: contracts.Target{Kind: "local"}, Settings: marshal(map[string]any{"access_mode": "full_access"})}
		execution.Target = target
		selection := contracts.Workflow{ID: "execute", Version: 1, PromptOverrides: map[string]string{"execute": "one\r\n\r\n中文😀  \n"}}
		if fault := contracts.ValidateRunConfiguration(marshal(contracts.RunConfiguration{Execution: execution, Workflow: selection})); fault != nil {
			t.Fatal(fault)
		}
		catalog := Catalog()
		if !catalog.Declares(execution, &selection, "submit") || !catalog.Declares(execution, &selection, "stop") || catalog.Declares(execution, &selection, "reply") || catalog.Declares(execution, &selection, "resume") {
			t.Fatal("structured execute discovery differs from its operations")
		}
		for _, entry := range catalog.Combinations {
			if entry.Adapter == "orca-terminal" && entry.TargetKind == target.Kind && entry.Agent == "codex" && entry.Workflow.ID == "execute" {
				local := target.Kind == "local"
				if slices.Contains(entry.Requirements, "session_scoped_close_acknowledgement") != local || slices.Contains(entry.Requirements, "resource_settlement_unverified") == local {
					t.Fatal(entry)
				}
			}
		}
		// Other workflows retain their own executor configuration and input rules.
		execution.Settings = marshal(map[string]any{"agent": "pi", "access_mode": "full_access"})
		if fault := contracts.ValidateRunConfiguration(marshal(contracts.RunConfiguration{Execution: execution, Workflow: selection})); fault == nil {
			t.Fatal("unmigrated Pi execute admission changed")
		}
	}
}
