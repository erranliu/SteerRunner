package orca

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type localState struct {
	conn         net.Conn
	reader       *bufio.Reader
	token        string
	runtime      string
	capabilities []string
}

func openLocalState(ctx context.Context, runtime string) (*localState, error) {
	dir, err := stateUserDataDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "orca-runtime.json"))
	var metadata struct {
		RuntimeID  string                            `json:"runtimeId"`
		AuthToken  string                            `json:"authToken"`
		Transports []struct{ Kind, Endpoint string } `json:"transports"`
		Transport  *struct{ Kind, Endpoint string }  `json:"transport"`
	}
	if err != nil || json.Unmarshal(data, &metadata) != nil || metadata.RuntimeID == "" || metadata.AuthToken == "" {
		return nil, errors.New("local Orca IPC metadata/authentication is unavailable")
	}
	if metadata.RuntimeID != runtime {
		return nil, stateIdentityError("local bootstrap runtime changed")
	}
	endpoint := ""
	if metadata.Transports != nil {
		for _, transport := range metadata.Transports {
			if transport.Kind == "named-pipe" {
				endpoint = transport.Endpoint
				break
			}
		}
	} else if metadata.Transport != nil && metadata.Transport.Kind == "named-pipe" {
		endpoint = metadata.Transport.Endpoint
	}
	if !strings.HasPrefix(endpoint, `\\.\pipe\`) {
		return nil, errors.New("local Orca named pipe is unavailable")
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	conn, err := dialLocalState(dialCtx, endpoint)
	cancel()
	if err != nil {
		return nil, &StateConnectionError{Cause: err}
	}
	s := &localState{conn: conn, reader: bufio.NewReaderSize(conn, 64<<10), token: metadata.AuthToken, runtime: runtime}
	status, err := callState(ctx, s, "status.get", nil)
	if err != nil {
		_ = s.close()
		return nil, err
	}
	s.capabilities = localRuntimeCapabilities(status)
	return s, nil
}

func localRuntimeCapabilities(status json.RawMessage) []string {
	// RPC status.get is flat; CLI status nests these facts under runtime.
	var advertised struct {
		Capabilities []string `json:"capabilities"`
	}
	if json.Unmarshal(status, &advertised) != nil {
		return nil
	}
	return advertised.Capabilities
}

func (s *localState) close() error { return s.conn.Close() }

func (s *localState) request(ctx context.Context, method string, params any) (string, error) {
	return s.requestWithCapabilities(ctx, method, params, []string{"agent-session.session-boundary.v1"}, false)
}

// The ordinary state reader retains its original capabilities and read-only
// behavior. Structured mutations consume their durable gate at the real write.
func (s *localState) requestWithCapabilities(ctx context.Context, method string, params any, capabilities []string, mutation bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	requestID := hex.EncodeToString(id[:])
	data, err := json.Marshal(map[string]any{"id": requestID, "authToken": s.token, "method": method, "params": params,
		"clientCapabilities": capabilities})
	if err != nil {
		return "", err
	}
	stop := context.AfterFunc(ctx, func() { _ = s.close() })
	defer stop()
	if mutation {
		if err := CheckpointDispatch(ctx); err != nil {
			return "", err
		}
	}
	data = append(data, '\n')
	for len(data) > 0 {
		n, err := s.conn.Write(data)
		if err != nil {
			return "", &StateConnectionError{Cause: err}
		}
		data = data[n:]
	}
	return requestID, nil
}

func (s *localState) next(ctx context.Context, id string) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = s.close() })
	defer stop()
	for {
		var frame []byte
		for {
			part, err := s.reader.ReadSlice('\n')
			if len(frame)+len(part) > 4<<20 {
				_ = s.close()
				return nil, errors.New("Orca IPC state frame exceeds limit")
			}
			frame = append(frame, part...)
			if errors.Is(err, bufio.ErrBufferFull) {
				continue
			}
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, &StateConnectionError{Cause: err}
			}
			break
		}
		if len(strings.TrimSpace(string(frame))) == 0 {
			continue
		}
		result, err := stateResponse(frame, id, s.runtime)
		if err != nil || result != nil {
			return result, err
		}
	}
}
