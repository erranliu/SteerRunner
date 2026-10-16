package codexworker

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNativeViewerVerifiesLiveBindingWithReadsOnly(t *testing.T) {
	for _, scenario := range []string{"ready", "workspace_changed", "session_changed", "process_changed", "envelope_changed"} {
		t.Run(scenario, func(t *testing.T) {
			workspace := t.TempDir()
			session := "00000000-0000-0000-0000-000000000001"
			var mu sync.Mutex
			methods := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				for {
					var request struct {
						ID     json.RawMessage `json:"id"`
						Method string          `json:"method"`
					}
					if wsjson.Read(ctx, conn, &request) != nil {
						return
					}
					mu.Lock()
					methods = append(methods, request.Method)
					mu.Unlock()
					var result any
					switch request.Method {
					case "initialize":
						result = map[string]string{"userAgent": "fixture", "codexHome": workspace, "platformOs": "windows"}
					case "initialized":
						continue
					case "thread/read":
						id, cwd := session, workspace
						if scenario == "workspace_changed" {
							cwd += "-other"
						}
						if scenario == "session_changed" {
							id = "00000000-0000-0000-0000-000000000002"
						}
						result = map[string]any{"thread": map[string]string{"id": id, "cwd": cwd}}
					default:
						t.Error("viewer invoked execution method", request.Method)
						return
					}
					if wsjson.Write(ctx, conn, map[string]any{"id": request.ID, "result": result}) != nil {
						return
					}
				}
			}))
			defer server.Close()
			l := &ExecutionLifecycle{Backend: Backend{Config: Config{Executable: os.Args[0]}, Process: ProcessPort{Inspect: func(*b.ProcessIdentity) string {
				if scenario == "process_changed" {
					return "PID belongs to a different process"
				}
				return "matching process still exists"
			}}}}
			s := b.LifecycleSnapshot{Session: b.SessionContext{ProjectID: "project", ExecutionID: "execution", AttemptID: "attempt", WorkflowRunID: "run", SessionID: session}, ResourceGeneration: "resources", ConfigurationDigest: "config", Workspace: b.WorkspaceIntent{Path: workspace}}
			binding := AppServerBinding{Endpoint: "ws" + strings.TrimPrefix(server.URL, "http"), SessionID: session, Generation: "server"}
			envelope, err := NewAppServerBindingEnvelope(l.identity(s), binding)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "envelope_changed" {
				envelope.AttemptID = "old"
			}
			s.Implementation = []b.ImplementationDataEnvelope{envelope}
			s.Ref.External = b.ProcessIdentity{Kind: "os", PID: 123, Created: "fixed", Executable: "original.exe"}
			before := s.Clone()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command, err := l.NativeViewerCommand(ctx, s)
			if (err == nil) != (scenario == "ready") || !reflect.DeepEqual(before, s.Clone()) {
				t.Fatal("viewer changed state or accepted wrong binding", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if scenario == "process_changed" || scenario == "envelope_changed" {
				if len(methods) != 0 {
					t.Fatal("invalid binding connected", methods)
				}
				return
			}
			if !reflect.DeepEqual(methods, []string{"initialize", "initialized", "thread/read"}) {
				t.Fatal("viewer issued execution input", methods)
			}
			if err == nil && (command.Args[2] != binding.Endpoint || command.Args[5] != session || command.WorkingDirectory != workspace) {
				t.Fatal("viewer attached elsewhere", command)
			}
		})
	}
}

func TestNativeViewerAttachesWithoutPromptOrPolicyOverrides(t *testing.T) {
	binding := AppServerBinding{SessionID: "00000000-0000-0000-0000-000000000001", Endpoint: "ws://127.0.0.1:12345", Generation: "original"}
	workspace := t.TempDir()
	command, err := NativeViewerCommand(os.Args[0], workspace, binding)
	if err != nil {
		t.Fatal(err)
	}
	if len(command.Args) != 6 || command.Args[1] != "--remote" || command.Args[2] != binding.Endpoint || command.Args[4] != workspace || command.Args[5] != binding.SessionID || strings.Contains(strings.Join(command.Args, " "), "sandbox") {
		t.Fatalf("viewer starts input or overrides worker settings: %+v", command)
	}
	for _, endpoint := range []string{"", "ws://outside.example:12345", "ws://127.0.0.1:0"} {
		binding.Endpoint = endpoint
		if _, err := NativeViewerCommand(os.Args[0], workspace, binding); err == nil {
			t.Fatalf("invalid endpoint accepted: %q", endpoint)
		}
	}
}
