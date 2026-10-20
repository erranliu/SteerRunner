package application

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/erranliu/SteerRunner/internal/contracts"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/runstate"
	"github.com/erranliu/SteerRunner/internal/storage"
)

func artifactEngine(t *testing.T) (*Engine, *worker, string) {
	t.Helper()
	e, w := auditEngine(t)
	r := e.runs["run"].Clone()
	s := &r.Snapshot
	s.Workspace = s.WorkspacePlan
	s.Ref = b.ExecutionRef{RunID: r.ID, WorkspacePath: s.Workspace.Path, WorkspaceBranch: s.Workspace.Branch, BaseCommit: s.Workspace.BaseCommit}
	s.Session.WorkflowRunID, s.Session.ExecutionID, s.Session.ProjectID = r.ID, r.ID, r.RepositoryCommon
	r.WorkspaceFact = &b.WorkspaceInspection{Repository: r.RepositoryPath, Worktree: s.Workspace.Path, Branch: s.Workspace.Branch, Base: s.Workspace.BaseCommit}
	if err := e.commitLocked(r, nil); err != nil {
		t.Fatal(err)
	}
	w.port = nil // Evidence must never require an execution port.
	root := filepath.Join(s.Workspace.Path, filepath.FromSlash(contracts.ArtifactDirectory))
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	return e, w, root
}

func artifactRequest(e *Engine, id, path string) *contracts.Request {
	raw, _ := json.Marshal(contracts.CollectArtifactInput{ExpectedRevision: e.runs["run"].Revision, RelativePath: path})
	return &contracts.Request{Protocol: contracts.Protocol, RequestID: id, Input: raw}
}

func TestR2RunArtifactLimitsAreInclusiveAndRefusalsAreAtomic(t *testing.T) {
	for _, boundary := range []string{"count", "bytes"} {
		t.Run(boundary, func(t *testing.T) {
			e, w, root := artifactEngine(t)
			r := e.runs["run"].Clone()
			if boundary == "count" {
				for i := 0; i < contracts.MaxRunArtifacts-1; i++ {
					r.Artifacts = append(r.Artifacts, runstate.Artifact{Artifact: contracts.Artifact{RunID: r.ID, ArtifactID: fmt.Sprint(i)}})
				}
			} else {
				for i := 0; i < contracts.MaxRunArtifactBytes/contracts.MaxArtifactBytes-1; i++ {
					r.Artifacts = append(r.Artifacts, runstate.Artifact{Artifact: contracts.Artifact{RunID: r.ID, ArtifactID: fmt.Sprint(i), SizeBytes: contracts.MaxArtifactBytes}, Content: make([]byte, contracts.MaxArtifactBytes)})
				}
			}
			if err := e.commitLocked(r, nil); err != nil {
				t.Fatal(err)
			}
			data := []byte{}
			if boundary == "bytes" {
				data = bytes.Repeat([]byte{'x'}, contracts.MaxArtifactBytes)
			}
			if err := os.WriteFile(filepath.Join(root, "limit"), data, 0600); err != nil {
				t.Fatal(err)
			}
			accepted := e.HandleEvidence("artifacts collect", "run", "", artifactRequest(e, "at-limit", "limit"))
			if !accepted.OK {
				t.Fatal("exact limit refused", accepted)
			}
			if err := os.WriteFile(filepath.Join(root, "extra"), []byte{1}, 0600); err != nil {
				t.Fatal(err)
			}
			before := canonical(e.runs["run"])
			request := artifactRequest(e, "over-limit", "extra")
			rejected := e.HandleEvidence("artifacts collect", "run", "", request)
			if rejected.OK || rejected.Error.Code != "artifact_limit_exceeded" || !bytes.Equal(before, canonical(e.runs["run"])) || len(w.wake) != 0 {
				t.Fatal("limit refusal changed original run", rejected)
			}
			runs, err := e.store.Runs()
			if err != nil || len(runs) != 1 || !bytes.Equal(before, canonical(runs[0])) {
				t.Fatal("limit refusal partially committed", err)
			}
		})
	}
}

func TestR2CollectCommitFailureAndRestartReplay(t *testing.T) {
	e, w, root := artifactEngine(t)
	data := []byte("original\x00bytes\xff")
	path := filepath.Join(root, "report")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(e.dir, "runner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TRIGGER fail_artifact BEFORE INSERT ON events WHEN json_extract(NEW.body,'$.type')='artifact_collected' BEGIN SELECT RAISE(ABORT,'injected artifact failure'); END;`); err != nil {
		t.Fatal(err)
	}
	request := artifactRequest(e, "original-collect", "report")
	before := canonical(e.runs["run"])
	response := e.HandleEvidence("artifacts collect", "run", "", request)
	if response.OK || response.Error.Code != "request_outcome_unknown" || !bytes.Equal(before, canonical(e.runs["run"])) {
		t.Fatal("failed transaction changed memory", response)
	}
	if _, found, err := e.store.Original(request.RequestID, storage.RequestDigest("artifacts collect", "run", *request)); found || err != nil {
		t.Fatal("rollback retained a command receipt", found, err)
	}
	runs, err := e.store.Runs()
	if err != nil || len(runs) != 1 || !bytes.Equal(before, canonical(runs[0])) {
		t.Fatal("rollback retained artifact or revision", err)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_artifact`); err != nil {
		t.Fatal(err)
	}
	accepted := e.HandleEvidence("artifacts collect", "run", "", request)
	if !accepted.OK || len(e.runs["run"].Artifacts) != 1 || len(w.wake) != 0 {
		t.Fatal("original request did not settle exactly once", accepted)
	}
	artifact := e.runs["run"].Artifacts[0]
	original := e.HandleEvidence("artifacts read", "run", artifact.ArtifactID, nil)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// Reopen the actual database, not the original engine's in-memory record.
	if err := e.store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(e.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	restarted, err := New(context.Background(), e.dir, store, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.cancel()
	replay := restarted.HandleEvidence("artifacts collect", "run", "", request)
	var first, second any
	_ = json.Unmarshal(canonical(accepted), &first)
	_ = json.Unmarshal(canonical(replay), &second)
	if !reflect.DeepEqual(first, second) || len(restarted.runs["run"].Artifacts) != 1 || !bytes.Equal(canonical(original), canonical(restarted.HandleEvidence("artifacts read", "run", artifact.ArtifactID, nil))) {
		t.Fatal("restart changed immutable bytes or original receipt", replay)
	}
	changed := *request
	changed.Input = json.RawMessage(fmt.Sprintf(`{"expected_revision":%d,"relative_path":"other"}`, restarted.runs["run"].Revision))
	if response := restarted.HandleEvidence("artifacts collect", "run", "", &changed); response.OK || response.Error.Code != "request_conflict" {
		t.Fatal("changed request reused original receipt", response)
	}
}
