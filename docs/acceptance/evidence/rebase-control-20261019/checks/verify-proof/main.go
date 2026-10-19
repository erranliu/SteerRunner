package main

import (
	"encoding/json"
	"fmt"
	codex "github.com/erranliu/SteerRunner/internal/execution/adapters/codex"
	transport "github.com/erranliu/SteerRunner/internal/execution/adapters/orcatransport"
	"github.com/erranliu/SteerRunner/internal/storage"
	"os"
	"path/filepath"
)

func read(path string, value any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(raw, value); err != nil {
		panic(err)
	}
}
func write(path string, value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		panic(err)
	}
}

func main() {
	for _, arg := range os.Args[1:] {
		dir, err := filepath.Abs(arg)
		if err != nil {
			panic(err)
		}
		var ready codex.RemoteReadiness
		var signed json.RawMessage
		mode := "workflow"
		if _, err = os.Stat(filepath.Join(dir, "remote-authentication-cleanup.json")); err == nil {
			var input struct {
				Authentication struct {
					Proof  codex.RemoteAuthenticationCleanup `json:"proof"`
					Signed json.RawMessage                   `json:"signed_native_exit"`
				} `json:"authentication"`
			}
			read(filepath.Join(dir, "remote-authentication-cleanup.json"), &input)
			ready = input.Authentication.Proof.Readiness
			signed, err = os.ReadFile(filepath.Join(dir, "native-exit.json"))
			if err != nil {
				panic(err)
			}
		} else {
			mode = "offline-probe"
			read(filepath.Join(dir, "native-readiness.json"), &ready)
			signed, err = os.ReadFile(filepath.Join(dir, "native-exit.json"))
			if err != nil {
				panic(err)
			}
			store, e := storage.OpenReader(filepath.Join(dir, "state"))
			if e != nil {
				panic(e)
			}
			records, e := store.Runs()
			if e != nil {
				panic(e)
			}
			if len(records) != 1 {
				panic("probe Run count differs")
			}
			record := records[0]
			var state transport.State
			for _, envelope := range record.Snapshot.Implementation {
				if envelope.ImplementationKey == transport.StateKey {
					if e = json.Unmarshal(envelope.Payload, &state); e != nil {
						panic(e)
					}
				}
			}
			proof, actor, e := transport.ReadAuthenticationStopEvidence(filepath.Join(dir, "state"), record.Snapshot, state)
			if e != nil {
				panic(e)
			}
			write(filepath.Join(dir, "native-control-proof.json"), map[string]any{"run_id": record.ID, "snapshot_ref": record.Snapshot.Ref, "control": actor, "authentication": proof, "zero_business_turns": len(state.Turns) == 0})
			if e = store.Close(); e != nil {
				panic(e)
			}
		}
		if err = codex.VerifyRemoteExit(signed, ready); err != nil {
			panic(err)
		}
		var envelope struct {
			Body codex.RemoteExit `json:"body"`
		}
		if err = json.Unmarshal(signed, &envelope); err != nil {
			panic(err)
		}
		proof := map[string]any{"signature_verified": true, "mode": mode, "owner": ready.Owner, "native_process": ready.Process, "helper_sha256": ready.RuntimeSHA256, "native_transport": ready.NativeTransport, "writers_absent": envelope.Body.WritersAbsent, "authentication_closed": envelope.Body.AuthenticationClosed, "tui": envelope.Body.TUI, "closed_at": envelope.Body.ClosedAt}
		write(filepath.Join(dir, "native-signature-proof.json"), proof)
		fmt.Println(dir + ": original signature, native tree and authentication verified")
	}
}
