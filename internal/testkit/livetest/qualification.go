package livetest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

// A batch case record binds the existing feedback coverage to the exact Runner
// build. The native entry point also compares the frozen driver hash.
func ValidateFeedbackQualification(proof Tool, runner string) error {
	if !filepath.IsAbs(proof.Path) {
		return errors.New("feedback qualification must be an absolute case record")
	}
	body, err := os.ReadFile(proof.Path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != proof.SHA256 {
		return errors.New("feedback qualification changed")
	}
	var record struct {
		Scenario string `json:"scenario"`
		Case     string `json:"case_id"`
		Status   string `json:"status"`
		Identity struct {
			Runner string `json:"runner_sha256"`
			Driver string `json:"driver_sha256"`
		} `json:"identity"`
		Evidence string `json:"evidence"`
	}
	if json.Unmarshal(body, &record) != nil || record.Scenario != "local-codex.feedback" || record.Case != "default" || record.Status != "passed" || record.Identity.Runner != runner || len(record.Identity.Driver) != 64 || !filepath.IsAbs(record.Evidence) {
		return errors.New("feedback qualification lacks matching passed case/build")
	}
	batch, err := os.ReadFile(filepath.Join(filepath.Dir(proof.Path), "summary.json"))
	if err != nil {
		return err
	}
	var summary struct {
		Status  string `json:"status"`
		Results []struct {
			Scenario string `json:"scenario"`
			Case     string `json:"case_id"`
			Status   string `json:"status"`
			Evidence string `json:"evidence"`
		} `json:"results"`
	}
	if json.Unmarshal(batch, &summary) != nil || summary.Status != "passed" {
		return errors.New("feedback qualification batch has not passed")
	}
	matched := 0
	for _, result := range summary.Results {
		if result.Scenario == record.Scenario && result.Case == record.Case && result.Status == "passed" && result.Evidence == record.Evidence {
			matched++
		}
	}
	if matched != 1 {
		return errors.New("feedback qualification lacks unique coordinator-confirmed case")
	}
	result, err := os.ReadFile(filepath.Join(record.Evidence, "scenario.json"))
	if err != nil {
		return err
	}
	var scenario struct {
		Passed         bool   `json:"passed"`
		Stopped        bool   `json:"run_stopped"`
		ServiceStopped bool   `json:"service_stopped"`
		Feedback       string `json:"feedback_head"`
		Merge          string `json:"merge_commit"`
	}
	if json.Unmarshal(result, &scenario) != nil || !scenario.Passed || !scenario.Stopped || !scenario.ServiceStopped || scenario.Feedback == "" || scenario.Merge == "" {
		return errors.New("feedback qualification lacks actual feedback, merge or cleanup")
	}
	return nil
}

func ValidateFeedbackExecution(proof Tool, execution contracts.Execution) error {
	body, err := os.ReadFile(proof.Path)
	if err != nil {
		return err
	}
	var record struct {
		Manifest string `json:"manifest"`
		Identity struct {
			Manifest string `json:"manifest_sha256"`
		} `json:"identity"`
	}
	if json.Unmarshal(body, &record) != nil || !filepath.IsAbs(record.Manifest) {
		return errors.New("feedback qualification lacks frozen execution manifest")
	}
	frozen, err := os.ReadFile(record.Manifest)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(frozen)
	if hex.EncodeToString(sum[:]) != record.Identity.Manifest {
		return errors.New("feedback qualification manifest changed")
	}
	var original Manifest
	if contracts.DecodeStrict(frozen, &original) != nil || original.Driver != "development-feedback" || original.Scenario != "local-codex.feedback" || original.Level != "L3" {
		return errors.New("feedback qualification used another workflow")
	}
	var before, after any
	if json.Unmarshal(original.Execution.Settings, &before) != nil || json.Unmarshal(execution.Settings, &after) != nil || original.Execution.Adapter != execution.Adapter || original.Execution.Target != execution.Target || !reflect.DeepEqual(before, after) {
		return errors.New("core smoke execution configuration differs from qualified feedback")
	}
	return nil
}
