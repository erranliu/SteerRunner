package contracts

import (
	"encoding/hex"
	"path/filepath"
)

const ServiceBindingContract = "steerrunner-service-binding/v1"

// ExpectedServiceBinding is checked at resident admission before any business
// handler, external query or durable acceptance. It grants no implicit start.
type ExpectedServiceBinding struct {
	Contract         string `json:"contract"`
	Protocol         string `json:"protocol"`
	InstanceID       string `json:"instance_id"`
	StateDir         string `json:"state_dir"`
	ExecutableSHA256 string `json:"executable_sha256"`
	Version          string `json:"version"`
	ResourceDigest   string `json:"resource_digest"`
}

func (b ExpectedServiceBinding) Valid() bool {
	sha, err := hex.DecodeString(b.ExecutableSHA256)
	resource, resourceErr := hex.DecodeString(b.ResourceDigest)
	return b.Contract == ServiceBindingContract && b.Protocol == Protocol && ValidRequestID(b.InstanceID) && filepath.IsAbs(b.StateDir) && ValidRequestID(b.StateDir) && ValidRequestID(b.Version) && err == nil && len(sha) == 32 && resourceErr == nil && len(resource) == 32
}

func ServiceBindingCommands() []string {
	return []string{"capabilities", "progress-file", "result-file", "run-usage", "run-usage-events", "estimate-run-cost", "submit", "reply", "resume", "stop", "inspect", "observe", "last-message", "events", "environment-check start", "environment-check stop", "environment-check inspect", "open-native-session", "artifacts discover", "artifacts collect", "artifacts list", "artifacts read", "workspace-git", "message", "interrupt", "close", "history"}
}

func ServiceBindingCommand(command string) bool {
	for _, covered := range ServiceBindingCommands() {
		if command == covered {
			return true
		}
	}
	return false
}
