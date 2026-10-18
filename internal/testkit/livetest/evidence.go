package livetest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func equalSHA(a, b string) bool { return a != "" && strings.EqualFold(a, b) }

// ValidateEvidence independently checks the journals used by the result, rather
// than trusting a caller-supplied passed/audit_complete flag.
func ValidateEvidence(m Manifest, r StabilityResult) error {
	if err := r.Validate(m); err != nil {
		return err
	}
	root := filepath.Join(m.Root, "evidence")
	for _, name := range []string{"faults.jsonl", "native-calls.jsonl", "caller-commands.jsonl", "source-identity.json", "cleanup.json", "result.json", "pr-list.json"} {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || info.Size() == 0 {
			return errors.New("required stability evidence missing: " + name)
		}
	}
	raw, _ := os.ReadFile(filepath.Join(root, "native-calls.jsonl"))
	facts, err := AnalyzeNativeAudit(raw, m.RemoteRuntimeID, ReceiptAuditPolicy(m))
	if err != nil || !facts.Complete || facts.Creations != 1 {
		return errors.New("native mutation receipts incomplete or duplicated")
	}
	if facts.Inspections > 0 || facts.InitialTerminals > 0 {
		proofRaw, proofErr := os.ReadFile(filepath.Join(root, "native-resource-audit.json"))
		var proof struct {
			Verified    bool `json:"verified"`
			Creations   int  `json:"business_creations"`
			Prompts     int  `json:"prompt_sends"`
			Inspections int  `json:"inspections"`
			Closed      int  `json:"closed_inspections"`
			Initial     int  `json:"initial_terminal_creations"`
		}
		if proofErr != nil || json.Unmarshal(proofRaw, &proof) != nil || !proof.Verified || proof.Creations != facts.Creations || proof.Initial != facts.InitialTerminals || proof.Prompts != len(facts.Prompts) || proof.Inspections != facts.Inspections || proof.Closed != facts.ClosedInspections || proof.Closed != proof.Inspections {
			return errors.New("readonly inspector closure evidence incomplete or changed")
		}
	}
	seen := map[string]bool{}
	for _, prompt := range facts.Prompts {
		if seen[prompt] {
			return errors.New("duplicate native input in evidence")
		}
		seen[prompt] = true
	}
	raw, _ = os.ReadFile(filepath.Join(root, "cleanup.json"))
	var cleanup Cleanup
	if json.Unmarshal(raw, &cleanup) != nil || cleanup != r.Cleanup {
		return errors.New("cleanup evidence differs from result")
	}
	raw, _ = os.ReadFile(filepath.Join(root, "result.json"))
	var exit struct {
		Exit  int `json:"exit_code"`
		Count int `json:"scenario_count"`
	}
	if json.Unmarshal(raw, &exit) != nil || exit.Exit != 0 || exit.Count != 1 {
		return errors.New("harness did not execute exactly one passing scenario")
	}
	raw, _ = os.ReadFile(filepath.Join(root, "source-identity.json"))
	var identity struct {
		Manifest string `json:"manifest_sha256"`
		Runner   string `json:"runner_sha256"`
		Driver   string `json:"driver_sha256"`
		Helper   string `json:"helper_sha256"`
		Runtime  string `json:"runtime_sha256"`
		Case     string `json:"case_id"`
		Repeat   int    `json:"repeat_index"`
	}
	if json.Unmarshal(raw, &identity) != nil {
		return errors.New("source identity invalid")
	}
	body, err := os.ReadFile(filepath.Join(m.Root, "manifest.json"))
	sum := sha256.Sum256(body)
	if err != nil || !equalSHA(identity.Manifest, hex.EncodeToString(sum[:])) || !equalSHA(identity.Runner, m.RunnerSHA256) || !equalSHA(identity.Helper, m.Stability.Helper.SHA256) || !equalSHA(identity.Runtime, m.Stability.RuntimeFiles.SHA256) || identity.Case != m.CaseID || identity.Repeat != m.RepeatIndex {
		return errors.New("source identity differs from frozen fixture")
	}
	body, err = os.ReadFile(filepath.Join(m.Root, "bin", "native.test.exe"))
	sum = sha256.Sum256(body)
	if err != nil || !equalSHA(identity.Driver, hex.EncodeToString(sum[:])) {
		return errors.New("driver differs from recorded executable")
	}
	required := []string{}
	switch m.Scenario {
	case "orca-remote.caller-exit":
		required = []string{"submit-replay-original", "single-native-publication"}
	case "orca-remote.connection-reconnect":
		required = []string{"network-preflight", "direct-state-blocked", "native-cli-blocked", "original-service-unchanged", "normal-reconnect-no-resume", "single-native-publication"}
		if m.CaseID == "pre-send-disconnect" {
			required = []string{"network-preflight", "publication-pre-send-gate", "pre-send-connection-retried", "pre-send-authority-retained", "original-service-unchanged", "normal-reconnect-no-resume", "single-native-publication"}
			if err := validatePreSendEvidence(m); err != nil {
				return err
			}
		}
	case "orca-remote.service-restart":
		required = []string{"service-replaced", "resume-required-before-authorization", "no-input-after-revocation-or-before-resume", "single-native-publication"}
	case "orca-remote.stop-revocation":
		required = []string{"no-unexpected-publication", "no-input-after-revocation-or-before-resume"}
		if m.CaseID == "unreachable" {
			required = append(required, "revocation-survives-restart", "revoked-feedback-rejected", "stale-revoked-write-rejected", "unreachable-revocation-window")
		}
	case "orca-remote.publication-unknown":
		required = []string{"network-preflight", "real-publication-before-gap", "completion-gap-unverified", "original-publication-binding", "single-native-publication"}
		if m.CaseID == "query-unavailable" {
			required = append(required, "query-failure-no-false-ready")
		}
	}
	assertions := map[string]bool{}
	if m.Scenario == "orca-remote.connection-reconnect" && m.CaseID == "active-disconnect" || m.Scenario == "orca-remote.publication-unknown" || strings.HasSuffix(m.CaseID, "boundary") || m.CaseID == "unreachable" {
		required = append(required, "failed-observation-retried")
		proofRaw, proofErr := os.ReadFile(filepath.Join(root, "network-observation-proof.json"))
		var proof ConnectionProof
		blockedRaw, blockedErr := os.ReadFile(filepath.Join(root, "network-blocked.json"))
		var blocked struct {
			At time.Time `json:"at"`
		}
		auditRaw, auditErr := os.ReadFile(filepath.Join(root, "native-calls.jsonl"))
		if proofErr != nil || auditErr != nil || blockedErr != nil || json.Unmarshal(blockedRaw, &blocked) != nil || blocked.At.IsZero() || json.Unmarshal(proofRaw, &proof) != nil || proof.StartedAt.IsZero() || proof.StartedAt.Before(blocked.At) || proof.ObservedAt.Sub(proof.StartedAt) < time.Minute || !FailedObservationRetried(auditRaw, proof.Terminal, proof.StartedAt) {
			return errors.New("network failure/retry window lacks physical observation evidence")
		}
	}
	for _, a := range r.Assertions {
		assertions[a.Name] = a.Passed
	}
	for _, name := range required {
		if !assertions[name] {
			return errors.New("case assertion missing from evidence: " + name)
		}
	}
	raw, _ = os.ReadFile(filepath.Join(root, "pr-list.json"))
	var prs []struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(raw, &prs) != nil {
		return errors.New("actual PR list evidence invalid")
	}
	if r.BusinessOutcome == "merged" {
		if len(prs) != 1 || prs[0].URL != r.PRURL {
			return errors.New("actual PR list differs from result")
		}
		raw, err = os.ReadFile(filepath.Join(root, "merged-run.json"))
		var v contracts.RunView
		if err != nil || json.Unmarshal(raw, &v) != nil || v.PR == nil || !v.PR.Merged || v.PR.URL != r.PRURL || v.PR.Repository != Repository || v.PR.BaseBranch != m.BaseBranch || v.PR.MergeCommit == nil || *v.PR.MergeCommit != r.MergeCommit || v.PR.MergedAt == nil {
			return errors.New("verified merge evidence missing or mismatched")
		}
	} else if len(prs) != 0 {
		return errors.New("unexpected actual PR in stop evidence")
	}
	return nil
}
