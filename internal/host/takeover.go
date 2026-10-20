package host

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/erranliu/SteerRunner/internal/application"
	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/resources"
	"github.com/erranliu/SteerRunner/internal/storage"
)

// Called with writer.lock held, before the first semantic marker upgrade.
// An unsupported aggregate must not be silently adopted by changing v3 to v4.
func qualifyLegacyRecovery(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "runner.db")); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	r, err := storage.OpenRecoveryReader(dir)
	if err != nil {
		return err
	}
	defer r.Close()
	format, err := r.StateFormat()
	if err != nil || format == storage.Format {
		return err
	}
	runs, err := r.Runs()
	if err != nil {
		return err
	}
	checks, err := r.Checks()
	if err != nil {
		return err
	}
	for _, item := range application.TakeoverFacts(runs, checks, resources.RecoveryCapabilities(), workflow.Catalog()) {
		if item.Status == "unsupported" {
			return contracts.Fail(3, "unsupported_schema", "Legacy state contains unsupported execution contracts; preserve it and inspect the takeover report before upgrading.")
		}
	}
	return nil
}

type takeoverCandidate struct {
	Recovery  contracts.RecoverySupport `json:"recovery"`
	Workflows []workflow.Profile        `json:"workflows"`
}

func candidateCapabilities(ctx context.Context, path string) (takeoverCandidate, string, *contracts.Fault) {
	var caps takeoverCandidate
	if !filepath.IsAbs(path) {
		return caps, "", contracts.Fail(2, "invalid_arguments", "Candidate must be an absolute executable path.")
	}
	sha, err := executableDigest(path)
	if err != nil {
		return caps, "", contracts.Fail(4, "service_unavailable", "Candidate executable cannot be verified.")
	}
	raw, err := process.New(15*time.Second).OutputCommand(ctx, b.LaunchCommand{Executable: path, Args: []string{"capabilities", "--offline", "--json"}, Timeout: 15 * time.Second})
	if err != nil {
		return caps, sha, contracts.Fail(3, "unsupported_capability", "Candidate offline recovery capabilities are unavailable.")
	}
	var response contracts.Response
	if contracts.DecodeStrict(raw, &response) != nil || !response.OK || response.Protocol != contracts.Protocol {
		return caps, sha, contracts.Fail(3, "unsupported_protocol", "Candidate returned an unsupported capability response.")
	}
	data, _ := json.Marshal(response.Data)
	if json.Unmarshal(data, &caps) != nil || caps.Recovery.WritableStateFormat != storage.Format || !slices.Contains(caps.Recovery.ReadableStateFormats, storage.Format) {
		return caps, sha, contracts.Fail(3, "unsupported_schema", "Candidate does not support current durable execution semantics.")
	}
	after, err := executableDigest(path)
	if err != nil || sha != after {
		return caps, sha, contracts.Fail(5, "directory_conflict", "Candidate changed during compatibility verification.")
	}
	return caps, sha, nil
}

func finishTakeover(dir, instance, sha string, items []contracts.TakeoverItem) contracts.TakeoverReport {
	result := contracts.TakeoverReport{Contract: "steerrunner-takeover/v1", StateDir: dir, InstanceID: instance, CandidateSHA256: sha, Compatible: true, Items: items}
	for _, item := range items {
		if item.Status != "compatible" && item.Status != "read_only" {
			result.Compatible = false
		}
	}
	return result
}

func VerifyTakeover(ctx context.Context, dir, candidate string) (contracts.TakeoverReport, *contracts.Fault) {
	var empty contracts.TakeoverReport
	if busy, err := writerBusy(dir); err == nil && busy {
		response, failure := Call(ctx, dir, Options{Command: "service verify-takeover", Candidate: candidate}, nil)
		if failure != nil {
			return empty, failure
		}
		if !response.OK {
			return empty, contracts.Fail(contracts.ExitCode(response), response.Error.Code, response.Error.Message)
		}
		raw, _ := json.Marshal(response.Data)
		if contracts.DecodeStrict(raw, &empty) != nil {
			return empty, contracts.Fail(4, "service_unavailable", "Invalid takeover report.")
		}
		return empty, nil
	}
	caps, sha, failure := candidateCapabilities(ctx, candidate)
	if failure != nil {
		return empty, failure
	}
	lock, busy, err := tryLock(filepath.Join(dir, "service", "writer.lock"), false)
	if err != nil {
		return empty, fault(err, contracts.Fail(4, "service_unavailable", "Original writer lease unavailable."))
	}
	if busy {
		return empty, contracts.Fail(5, "directory_conflict", "Writer changed during preflight.")
	}
	defer lock.Close()
	if _, err = os.Stat(bootstrapPath(dir)); !os.IsNotExist(err) {
		return empty, contracts.Fail(4, "startup_unverified", "Original startup is unresolved.")
	}
	reader, err := storage.OpenRecoveryReader(dir)
	if err != nil {
		return empty, fault(err, contracts.Fail(3, "unsupported_schema", "Recovery source format unavailable."))
	}
	defer reader.Close()
	runs, err := reader.Runs()
	if err != nil {
		return empty, contracts.Fail(3, "unsupported_schema", "Run records cannot be decoded.")
	}
	checks, err := reader.Checks()
	if err != nil {
		return empty, contracts.Fail(3, "unsupported_schema", "Check records cannot be decoded.")
	}
	return finishTakeover(dir, "", sha, reader.BindTakeoverRequests(application.TakeoverFacts(runs, checks, caps.Recovery, caps.Workflows))), nil
}
