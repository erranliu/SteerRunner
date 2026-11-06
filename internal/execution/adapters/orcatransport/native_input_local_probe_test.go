package orcatransport

import (
	"encoding/json"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"

	"testing"
)

// Match the existing originalWorktreeInventory contract: an unrelated omitted
// paired host does not own terminals bound to this exact local runtime.
func localProbeEmptyCensus(result native.Result, runtime string) bool {
	var v struct {
		TotalCount *int
		Truncated  *bool
		HostScope  *struct{ HostIDs, OmittedHostIDs *[]string }
		Terminals  *[]native.Terminal
	}
	if result.CallUncertain || result.ErrorCode != "" || result.RuntimeID != runtime || json.Unmarshal(result.Payload, &v) != nil || v.TotalCount == nil || *v.TotalCount != 0 || v.Truncated == nil || *v.Truncated || v.Terminals == nil || len(*v.Terminals) != 0 || v.HostScope == nil || v.HostScope.HostIDs == nil || v.HostScope.OmittedHostIDs == nil {
		return false
	}
	seen := map[string]bool{}
	for _, id := range *v.HostScope.HostIDs {
		if id == "" || seen[id] {
			return false
		}
		seen[id] = true
	}
	if !seen["local"] {
		return false
	}
	omitted := map[string]bool{}
	for _, id := range *v.HostScope.OmittedHostIDs {
		if id == "" || seen[id] || omitted[id] {
			return false
		}
		omitted[id] = true
	}
	return true
}

func TestLocalNativeProbeCensusKeepsOriginalHostScope(t *testing.T) {
	for _, fault := range []string{"none", "foreign-runtime", "omitted-local", "no-local", "duplicate-host", "duplicate-omission", "truncated", "nonempty", "missing-count"} {
		t.Run(fault, func(t *testing.T) {
			r := native.Result{RuntimeID: "original"}
			hosts, omitted := []string{"local"}, []string{"runtime:offline-peer"}
			count := any(0)
			rows := []any{}
			truncated := false
			switch fault {
			case "foreign-runtime":
				r.RuntimeID = "replacement"
			case "omitted-local":
				omitted = append(omitted, "local")
			case "no-local":
				hosts = []string{"other"}
			case "duplicate-host":
				hosts = append(hosts, "local")
			case "duplicate-omission":
				omitted = append(omitted, omitted[0])
			case "truncated":
				truncated = true
			case "nonempty":
				count = 1
				rows = append(rows, map[string]string{"handle": "foreign"})
			case "missing-count":
				count = nil
			}
			r.Payload, _ = json.Marshal(map[string]any{"terminals": rows, "totalCount": count, "truncated": truncated, "hostScope": map[string]any{"hostIds": hosts, "omittedHostIds": omitted}})
			if localProbeEmptyCensus(r, "original") != (fault == "none") {
				t.Fatal("incorrect original-host inventory result")
			}
		})
	}
}
