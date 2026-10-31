package codexworker

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/execution/process"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const RemoteAuthenticationPrefix = "SR_REMOTE_AUTHENTICATION_STOP:"
const RemoteAuthenticationFailurePrefix = "SR_REMOTE_AUTHENTICATION_FAILURE:"

// RemoteAuthenticationDiagnostic exports only a finite category, never an OS
// path, provider response, command output or credential-bearing error message.
func RemoteAuthenticationDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	for _, code := range []string{"remote_authentication_binding_invalid", "remote_authentication_helper_mismatch", "remote_authentication_configuration_mismatch", "remote_authentication_original_owner_absent", "remote_authentication_original_owner_changed", "remote_authentication_original_owner_unavailable", "remote_authentication_stop_conflict", "remote_authentication_stop_request_unverified", "remote_authentication_cleanup_unverified", "remote_authentication_stop_deadline", "remote_authentication_stop_canceled", "remote_authentication_stop_permission_denied", "remote_authentication_stop_unverified"} {
		if strings.Contains(err.Error(), code) {
			return code
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "remote_authentication_stop_deadline"
	}
	if errors.Is(err, context.Canceled) {
		return "remote_authentication_stop_canceled"
	}
	if errors.Is(err, os.ErrPermission) {
		return "remote_authentication_stop_permission_denied"
	}
	return "remote_authentication_stop_unverified"
}

type RemoteAuthenticationQuery struct {
	Workspace string `json:"workspace"`
	Session   string `json:"session"`
}
type RemoteAuthenticationCleanup struct {
	Readiness     RemoteReadiness `json:"readiness"`
	Revoked       bool            `json:"revocation_confirmed"`
	BrokerClosed  bool            `json:"broker_closed"`
	SocketCleaned bool            `json:"socket_cleanup_confirmed"`
	Mode          string          `json:"mode"`
}
type remoteAuthenticationSignal struct {
	Session             string `json:"session"`
	ConfigurationDigest string `json:"configuration_digest"`
}

func readRemoteAuthenticationCleanup(dir string, ready RemoteReadiness) (RemoteAuthenticationCleanup, error) {
	out := RemoteAuthenticationCleanup{Readiness: ready}
	raw, err := os.ReadFile(filepath.Join(dir, "authentication-cleanup.json"))
	if err != nil {
		return out, err
	}
	var fact struct {
		Session       string `json:"session_id"`
		Configuration string `json:"configuration_digest"`
		Revoked       bool   `json:"revocation_confirmed"`
		Broker        struct {
			Closed bool   `json:"closed"`
			Socket bool   `json:"socket_cleanup_confirmed"`
			Mode   string `json:"mode"`
		} `json:"broker"`
	}
	if json.Unmarshal(raw, &fact) != nil || fact.Session != ready.SessionID || fact.Configuration != ready.ConfigurationDigest || !fact.Revoked || !fact.Broker.Closed || !fact.Broker.Socket || fact.Broker.Mode != "host-gh-repository-broker" {
		return out, errors.New("remote_authentication_cleanup_unverified")
	}
	out.Revoked, out.BrokerClosed, out.SocketCleaned, out.Mode = fact.Revoked, fact.Broker.Closed, fact.Broker.Socket, fact.Broker.Mode
	return out, nil
}

// The request and reply are public facts under the protected runtime directory.
// Only the original helper owns credentials. The auxiliary actor never receives
// a key/token, starts a model, repeats business input, or adopts another worker.
func RevokeRemoteAuthentication(ctx context.Context, q RemoteAuthenticationQuery) (RemoteAuthenticationCleanup, error) {
	var ready RemoteReadiness
	dir := RemoteRuntimeDirectory(q.Workspace)
	raw, err := os.ReadFile(filepath.Join(dir, "ready.json"))
	if err != nil || json.Unmarshal(raw, &ready) != nil || !ready.Verified || ready.NativeTransport != RemoteNativeTransport || !SupportedDevelopmentPolicy(ready.Contract) || !ValidNativeID(ready.SessionID) || !ready.AuthenticationControl || !sameWorkspace(ready.Workspace, q.Workspace) || ready.SessionID != q.Session {
		return RemoteAuthenticationCleanup{}, errors.New("remote_authentication_binding_invalid")
	}
	self, err := RuntimeBinarySHA256()
	if err != nil || self != ready.RuntimeSHA256 {
		return RemoteAuthenticationCleanup{}, errors.New("remote_authentication_helper_mismatch")
	}
	var cfg RemoteAgentConfig
	raw, err = os.ReadFile(filepath.Join(dir, "configuration.json"))
	if err != nil || contracts.DecodeStrict(raw, &cfg) != nil || cfg.RepositoryURL != ready.Repository || RemoteConfigurationDigest(cfg) != ready.ConfigurationDigest {
		return RemoteAuthenticationCleanup{}, errors.New("remote_authentication_configuration_mismatch")
	}
	if proof, e := readRemoteAuthenticationCleanup(dir, ready); e == nil {
		return proof, nil
	}
	if probe := process.Probe(&ready.Process); probe.State != "matches" {
		code := "remote_authentication_original_owner_unavailable"
		if probe.State == "absent" {
			code = "remote_authentication_original_owner_absent"
		}
		if probe.State == "pid-reused" {
			code = "remote_authentication_original_owner_changed"
		}
		return RemoteAuthenticationCleanup{}, errors.New(code)
	}
	signal, _ := json.Marshal(remoteAuthenticationSignal{Session: ready.SessionID, ConfigurationDigest: ready.ConfigurationDigest})
	path := filepath.Join(dir, "authentication-stop.json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		original, e := os.ReadFile(path)
		if e != nil || string(original) != string(signal) {
			return RemoteAuthenticationCleanup{}, errors.New("remote_authentication_stop_conflict")
		}
	} else if err != nil {
		return RemoteAuthenticationCleanup{}, err
	} else {
		_, err = file.Write(signal)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return RemoteAuthenticationCleanup{}, errors.New("remote_authentication_stop_request_unverified")
		}
	}
	for {
		if proof, e := readRemoteAuthenticationCleanup(dir, ready); e == nil {
			return proof, nil
		}
		select {
		case <-ctx.Done():
			return RemoteAuthenticationCleanup{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func watchRemoteAuthentication(ctx context.Context, dir string, ready RemoteReadiness, auth *AuthenticationRuntime) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			raw, err := os.ReadFile(filepath.Join(dir, "authentication-stop.json"))
			if err != nil {
				continue
			}
			var signal remoteAuthenticationSignal
			if json.Unmarshal(raw, &signal) != nil || signal.Session != ready.SessionID || signal.ConfigurationDigest != ready.ConfigurationDigest {
				continue
			}
			auth.Close()
			return
		}
	}
}
