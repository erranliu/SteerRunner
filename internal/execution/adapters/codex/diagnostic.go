package codexworker

import (
	"context"
	"errors"
	"strings"
)

// Keep native diagnostics finite: errors from executables/providers can contain
// arbitrary stdout, paths or credential-bearing URLs and are never published.
func EnvironmentDiagnostic(err error) string {
	if err == nil {
		return ""
	}
	for _, stage := range []string{"original_native_turn_unsettled"} {
		if strings.Contains(err.Error(), stage) {
			return stage
		}
	}
	for _, stage := range []string{"authentication_api_binding_invalid", "authentication_api_binding_missing", "authentication_api_binding_mismatch", "authentication_api_directory_unverified", "authentication_api_directory_redirected", "authentication_api_directory_unavailable", "authentication_api_directory_conflict", "authentication_api_directory_identity_changed", "authentication_api_socket_identity_changed", "authentication_api_socket_unverified", "authentication_api_socket_unavailable", "authentication_api_config_unavailable"} {
		if strings.Contains(err.Error(), stage) {
			return stage
		}
	}
	for _, stage := range []string{"authentication_broker_unavailable", "sandbox_readiness_evidence_unavailable", "native_version_unsupported", "native_executable_unavailable", "native_effective_permissions_unverified", "authentication_host_gh_unavailable", "authentication_host_login_unavailable", "authentication_expired_or_unavailable", "authentication_api_socket_path_too_long", "sandbox_identity", "tool_environment", "git_admin_read", "git_admin_write", "remote_identity", "git_transport_authentication", "github_identity_trust", "github_identity_transport", "github_identity_rejected", "github_identity", "github_repository", "remote_git_ca_file", "remote_git_ca_chain", "remote_git_tls", "remote_git_trust", "remote_git_transport", "remote_git_rejected", "remote_git"} {
		if strings.Contains(err.Error(), stage) {
			return stage
		}
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "deadline exceeded") {
		return "io_timeout_unverified"
	}
	return "preparation_unverified"
}
