package livetest

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/erranliu/SteerRunner/internal/contracts"
)

//go:embed cases.json
var caseFiles embed.FS

func RegistryDigest() string {
	raw, _ := caseFiles.ReadFile("cases.json")
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

type Case struct {
	ID           string        `json:"id"`
	Outcome      string        `json:"outcome"`
	Test         string        `json:"test,omitempty"`
	Requirements *Requirements `json:"requirements,omitempty"`
}

type RoleRequirement struct {
	Configuration   string `json:"configuration"`
	Adapter         string `json:"adapter"`
	TargetKind      string `json:"target_kind"`
	Agent           string `json:"agent"`
	AccessMode      string `json:"access_mode"`
	Workflow        string `json:"workflow,omitempty"`
	DifferentModels bool   `json:"different_models,omitempty"`
}
type Requirements struct {
	Roles       map[string]RoleRequirement `json:"roles"`
	Operation   string                     `json:"operation"`
	Publication bool                       `json:"publication"`
	Admin       bool                       `json:"admin"`
}
type Scenario struct {
	ID             string `json:"id"`
	Level          string `json:"level"`
	Driver         string `json:"driver"`
	Target         string `json:"target"`
	Agent          string `json:"agent,omitempty"`
	TimeoutMinutes int    `json:"timeout_minutes"`
	Cases          []Case `json:"cases"`
}

func Scenarios() []Scenario {
	raw, _ := caseFiles.ReadFile("cases.json")
	var v struct {
		Schema    string     `json:"schema"`
		Scenarios []Scenario `json:"scenarios"`
	}
	if err := contracts.DecodeStrict(raw, &v); err != nil {
		panic(err)
	}
	return v.Scenarios
}
func Lookup(scenario, caseID string) (Scenario, Case, error) {
	for _, s := range Scenarios() {
		if s.ID == scenario {
			for _, c := range s.Cases {
				if c.ID == caseID {
					return s, c, nil
				}
			}
		}
	}
	return Scenario{}, Case{}, fmt.Errorf("unregistered live case %s/%s", scenario, caseID)
}

type Assertion struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Evidence string `json:"evidence"`
}
type Cleanup struct {
	NativeStopped       bool `json:"native_stopped"`
	ServiceStopped      bool `json:"service_stopped"`
	FacilitiesRestored  bool `json:"facilities_restored"`
	DriverNativeCleanup bool `json:"driver_native_cleanup"`
}
type StabilityResult struct {
	Scenario        string      `json:"scenario"`
	CaseID          string      `json:"case_id"`
	RepeatIndex     int         `json:"repeat_index"`
	Stage           string      `json:"stage"`
	RunID           string      `json:"run_id"`
	PRURL           string      `json:"pr_url,omitempty"`
	MergeCommit     string      `json:"merge_commit,omitempty"`
	TestOutcome     string      `json:"test_outcome"`
	BusinessOutcome string      `json:"business_outcome"`
	FaultsHit       int         `json:"faults_hit"`
	AuditComplete   bool        `json:"audit_complete"`
	Assertions      []Assertion `json:"assertions"`
	Cleanup         Cleanup     `json:"cleanup"`
}

func (r StabilityResult) Validate(m Manifest) error {
	_, c, err := Lookup(m.Scenario, m.CaseID)
	if err != nil {
		return err
	}
	if r.Scenario != m.Scenario || r.CaseID != m.CaseID || r.RepeatIndex != m.RepeatIndex || r.RunID == "" || r.TestOutcome != "passed" || r.BusinessOutcome != c.Outcome || r.FaultsHit < 1 || !r.AuditComplete || len(r.Assertions) < 3 {
		return errors.New("incomplete or mismatched stability result")
	}
	seen := map[string]bool{}
	for _, a := range r.Assertions {
		if !a.Passed || a.Name == "" || a.Evidence == "" || seen[a.Name] {
			return errors.New("failed, duplicate or unproven assertion")
		}
		seen[a.Name] = true
	}
	for _, name := range []string{"fault-hit", "original-identity", "no-duplicate-inputs"} {
		if !seen[name] {
			return fmt.Errorf("missing assertion %s", name)
		}
	}
	if !r.Cleanup.NativeStopped || !r.Cleanup.ServiceStopped || !r.Cleanup.FacilitiesRestored {
		return errors.New("unconfirmed cleanup")
	}
	if c.Outcome == "merged" && (r.MergeCommit == "" || r.PRURL == "") {
		return errors.New("missing verified merge")
	}
	if c.Outcome != "merged" && (r.MergeCommit != "" || r.PRURL != "") {
		return errors.New("unexpected publication in stop case")
	}
	return nil
}
