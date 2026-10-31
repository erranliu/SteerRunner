package application

import (
	"encoding/json"
	"github.com/erranliu/SteerRunner/internal/contracts"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	orca "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/execution/workflow"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"strings"
)

const retiredAccessDiagnostic = "Development sandbox/implicit permissions are retired. This Run is read-only; only exact original-resource stop is supported. An existing native execution may still be running."

// Classification reads frozen data only. It never normalizes the old request,
// changes durable authorization or treats retirement as resource settlement.
func retiredDevelopment(r *runstate.Record) bool {
	if r == nil || workflow.Content(r.Request.Workflow.ID) || (r.Request.Workflow.ID != "development" && r.Request.Workflow.ID != "development-audited") {
		return false
	}
	var settings struct {
		AccessMode string `json:"access_mode"`
	}
	if json.Unmarshal(r.Request.Execution.Settings, &settings) != nil || settings.AccessMode != "full_access" {
		return true
	}
	for _, envelope := range r.Profile {
		switch envelope.ImplementationKey {
		case "codex.worker-profile":
			if envelope.CodecVersion != codex.ProfileCodecVersion {
				return true
			}
			var profile struct {
				AccessMode string `json:"access_mode"`
			}
			if json.Unmarshal(envelope.Payload, &profile) != nil || profile.AccessMode != "full_access" {
				return true
			}
		case "orca.terminal-profile":
			if envelope.CodecVersion != orca.ProfileVersion && envelope.CodecVersion != orca.NativeRemoteProfileVersion && envelope.CodecVersion != orca.FullAccessTerminalProfileVersion {
				return true
			}
			var profile struct {
				Terminal *struct {
					AccessMode string `json:"access_mode"`
				} `json:"terminal"`
			}
			if json.Unmarshal(envelope.Payload, &profile) != nil || profile.Terminal != nil && profile.Terminal.AccessMode != "full_access" {
				return true
			}
		}
	}
	if resource := r.Snapshot.PolicyResources; resource != nil {
		var policy struct {
			Contract string `json:"contract"`
			Settings struct {
				Args []string `json:"args"`
			} `json:"settings"`
		}
		if json.Unmarshal(resource.Payload, &policy) != nil {
			return true
		}
		if resource.ImplementationKey != codex.PolicyResourcesCodecKey || resource.CodecVersion != 5 || !codex.SupportedDevelopmentPolicy(policy.Contract) {
			return true
		}
		mode := ""
		for _, arg := range policy.Settings.Args {
			if strings.HasPrefix(arg, "sandbox_mode=") {
				if mode != "" {
					return true
				}
				mode = strings.TrimPrefix(arg, "sandbox_mode=")
			}
		}
		if mode != "danger-full-access" {
			return true
		}
	}
	return false
}

func readOnlyRun(r *runstate.Record) bool {
	if r == nil {
		return false
	}
	return retiredDevelopment(r) || orca.LegacyRemoteProfile(r.Profile)
}

func readOnlyStatus(r *runstate.Record) string {
	if retiredDevelopment(r) {
		var mode struct {
			AccessMode string `json:"access_mode"`
		}
		if json.Unmarshal(r.Request.Execution.Settings, &mode) != nil || mode.AccessMode != "" && mode.AccessMode != "sandbox" {
			return "unsupported"
		}
	}
	return "read_only"
}

func readOnlyProjection(r *runstate.Record) *runstate.Record {
	if !readOnlyRun(r) {
		return r
	}
	// Only top-level presentation fields change. A shallow copy also keeps
	// malformed historical envelopes inspectable without re-marshaling them.
	copy := *r
	view := &copy
	view.Allowed, view.ResumeRequired = false, true
	view.Diagnostic = retiredAccessDiagnostic
	if readOnlyStatus(r) == "unsupported" {
		view.Diagnostic = "Frozen development permission data is inconsistent or unsupported; only persisted queries and supported original-resource stop are available."
	}
	return view
}

func publicRunView(r *runstate.Record) contracts.RunView {
	v := readOnlyProjection(r).View()
	if readOnlyRun(r) {
		v.Controls = contracts.Controls{CanStop: r.StopStatus != "stopped", Reason: v.Diagnostic}
	}
	return v
}
