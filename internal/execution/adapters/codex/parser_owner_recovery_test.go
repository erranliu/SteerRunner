package codexworker

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable/operation"
)

func TestExecutionOwnerParserReconciliationReadsOnlyOriginalParser(t *testing.T) {
	for _, outcome := range []string{"completed", "pending", "missing"} {
		t.Run(outcome, func(t *testing.T) {
			launched := 0
			l := ExecutionLifecycle{setup: b.LifecycleSetup{LaunchExecutionOwner: func(context.Context, string) error { launched++; return nil }}, Backend: Backend{Config: Config{ServerDir: t.TempDir()}, Process: ProcessPort{Inspect: func(*b.ProcessIdentity) string { return "matching process still exists" }}}}
			dir := l.ownerDirectory()
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			for name, value := range map[string]any{ExecutionOwnerConfigFile: ownerConfig{Token: "original", RuntimeSHA256: "original-sha"}, "owner-ready.json": ownerReady{Identity: ExecutionOwnerIdentity{Protocol: ExecutionOwnerProtocol, Directory: dir, RuntimeSHA256: "original-sha"}, Endpoint: listener.Addr().String()}} {
				if err := os.WriteFile(filepath.Join(dir, name), encode(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			original := ownerTestRequest(t)
			intent, err := operation.New("run", original.Snapshot, b.LifecycleCandidate{Action: b.LifecycleParseResult, Input: b.LifecycleInput{Parse: &b.ParseInput{ResultID: "result", Payload: "{}", Generation: "parser", Deadline: time.Now().UTC().Add(time.Minute)}}}, "parse", "parse-key", time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			seen := make(chan ownerQuery, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				var query ownerQuery
				if json.NewDecoder(conn).Decode(&query) != nil {
					return
				}
				seen <- query
				response := ownerResponse{Receipt: b.LifecycleReceipt{Operation: query.Request.Operation, Disposition: b.OperationConfirmed, State: b.LifecycleStateFacts{Stopped: true, WritersAbsent: true}, ParseEvidence: &b.ParseEvidence{Status: "completed", Content: `{"outcome":"done"}`}}}
				if outcome == "pending" {
					response = ownerResponse{Pending: true}
				}
				if outcome == "missing" {
					response = ownerResponse{Error: "original parser was not dispatched; no replay"}
				}
				_ = json.NewEncoder(conn).Encode(response)
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			receipt, observer, err := l.Reconcile(ctx, intent.Request)
			if observer != nil || launched != 0 || receipt.TurnEvidence != nil {
				t.Fatal("parser reconciliation created a substitute or adopted business history")
			}
			if outcome == "completed" {
				if err != nil || receipt.ParseEvidence == nil || receipt.ParseEvidence.Status != "completed" {
					t.Fatal("original parser receipt not recovered", err, receipt)
				}
			} else if err == nil || outcome == "missing" && !strings.Contains(err.Error(), "no replay") {
				t.Fatal("unknown parser became a successful business turn", err, receipt)
			}
			select {
			case q := <-seen:
				if q.Method != "receipt" || q.Request.Operation.ID != intent.Request.Operation.ID || q.Request.Input.Parse.Payload != "{}" {
					t.Fatal("reconciliation did not preserve original parser request", q.Method)
				}
			case <-ctx.Done():
				t.Fatal("original owner was not queried")
			}
		})
	}
}
