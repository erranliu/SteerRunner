package codexworker

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"io"
	"path/filepath"
	"strings"
	"time"
)

const RemoteControlReadyPrefix = "SR_NATIVE_CONTROL_READY:"
const RemoteControlPrefix = "SR_NATIVE_CONTROL:"

// RemoteControlSetup is public resource binding, never credentials or a command.
// The Record owns the resource; this actor only reads original native facts.
type RemoteControlSetup struct {
	Binding       string `json:"binding"`
	Workspace     string `json:"workspace"`
	RuntimeSHA256 string `json:"runtime_sha256"`
}

type RemoteControlRequest struct {
	ID          string                     `json:"id"`
	Binding     string                     `json:"binding"`
	Method      string                     `json:"method"`
	Session     string                     `json:"session"`
	InputDigest string                     `json:"input_digest,omitempty"`
	Audit       *RemoteWorkspaceAuditQuery `json:"workspace_audit,omitempty"`
}

type RemoteControlResponse struct {
	ID            string                `json:"id"`
	Binding       string                `json:"binding"`
	RequestDigest string                `json:"request_digest"`
	Error         string                `json:"error,omitempty"`
	Turn          *RemoteTurnReceipt    `json:"turn,omitempty"`
	Workspace     *RemoteWorkspaceAudit `json:"workspace_audit,omitempty"`
}

func RemoteControlDigest(request RemoteControlRequest) string {
	raw, _ := json.Marshal(request)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func (q RemoteControlRequest) Validate(setup RemoteControlSetup) error {
	if !b.StableExecutionID(q.ID) || q.Binding != setup.Binding || !ValidNativeID(q.Session) {
		return errors.New("native_control_request_binding_invalid")
	}
	switch q.Method {
	case "original-turn":
		if q.Audit != nil || len(q.InputDigest) != 64 || strings.Trim(q.InputDigest, "0123456789abcdef") != "" {
			return errors.New("native_control_input_invalid")
		}
	case "workspace-audit":
		if q.InputDigest != "" || q.Audit == nil || q.Audit.Validate(setup.Workspace) != nil {
			return errors.New("native_control_input_invalid")
		}
	default:
		return errors.New("native_control_method_invalid")
	}
	return nil
}

// ServeRemoteControl has only finite original-turn, workspace and stop queries. Its one-response cache
// repairs transport loss while this same actor lives; it is not a task journal.
func ServeRemoteControl(ctx context.Context, setup RemoteControlSetup, input io.Reader, output io.Writer) error {
	self, err := RuntimeBinarySHA256()
	if err != nil || self != setup.RuntimeSHA256 {
		return errors.New("native_control_runtime_mismatch")
	}
	return serveRemoteControl(ctx, setup, input, output, func(ctx context.Context, q RemoteControlRequest) RemoteControlResponse {
		r := RemoteControlResponse{}
		if q.Method == "original-turn" {
			value, e := InspectRemoteTurn(ctx, RemoteTurnQuery{Workspace: setup.Workspace, Session: q.Session, InputDigest: q.InputDigest})
			// Resource settlement remains evidence even when history failed.
			r.Turn = &value
			if e != nil {
				r.Error = "original_turn_unverified"
			}
		} else if q.Method == "workspace-audit" {
			value, e := InspectRemoteWorkspaceAudit(ctx, setup, q.Session, *q.Audit)
			r.Workspace = &value
			if e != nil {
				r.Error = "workspace_audit_unverified"
			}
		}

		return r
	})
}

func serveRemoteControl(ctx context.Context, setup RemoteControlSetup, input io.Reader, output io.Writer, call func(context.Context, RemoteControlRequest) RemoteControlResponse) error {
	if len(setup.Binding) != 64 || strings.Trim(setup.Binding, "0123456789abcdef") != "" || len(setup.RuntimeSHA256) != 64 || strings.Trim(setup.RuntimeSHA256, "0123456789abcdef") != "" || !filepath.IsAbs(setup.Workspace) {
		return errors.New("native_control_setup_invalid")
	}
	emit := func(prefix string, value any) error {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(output, prefix+base64.RawURLEncoding.EncodeToString(raw))
		return err
	}
	if err := emit(RemoteControlReadyPrefix, setup); err != nil {
		return err
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 8192)
	var previous RemoteControlResponse
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, err := base64.RawURLEncoding.DecodeString(scanner.Text())
		var request RemoteControlRequest
		if err != nil || b.DecodeStrictJSON(raw, &request, "trailing native control request") != nil || request.Validate(setup) != nil {
			return errors.New("native_control_request_invalid")
		}
		digest := RemoteControlDigest(request)
		if previous.ID == request.ID {
			if previous.RequestDigest != digest {
				return errors.New("native_control_request_conflict")
			}
		} else {
			callCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
			previous = call(callCtx, request)
			cancel()
			previous.ID, previous.Binding, previous.RequestDigest = request.ID, setup.Binding, digest
		}
		if err := emit(RemoteControlPrefix, previous); err != nil {
			return err
		}
	}
	return scanner.Err()
}
