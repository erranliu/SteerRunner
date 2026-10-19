package codexworker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/process"
)

const RemoteTurnPrefix = "SR_REMOTE_ORIGINAL_TURN:"
const RemoteTurnSource = "codex.app-server.original-turn.v1"

type RemoteTurnQuery struct {
	Workspace   string `json:"workspace"`
	Session     string `json:"session"`
	InputDigest string `json:"input_digest"`
}

type RemoteTurnReceipt struct {
	InspectionFailed    bool            `json:"inspection_failed,omitempty"`
	ReaderWritersAbsent bool            `json:"reader_writers_absent"`
	Source              string          `json:"source"`
	Readiness           RemoteReadiness `json:"readiness"`
	InputDigest         string          `json:"input_digest"`
	NativeTurnID        string          `json:"native_turn_id,omitempty"`
	UserItemID          string          `json:"user_item_id,omitempty"`
	Started             bool            `json:"started"`
	Completed           bool            `json:"completed"`
	Status              string          `json:"status,omitempty"`
	FinalItemID         string          `json:"final_item_id,omitempty"`
	FinalText           string          `json:"final_text,omitempty"`
	FinalDigest         string          `json:"final_digest,omitempty"`
}

func InspectRemoteTurn(ctx context.Context, q RemoteTurnQuery) (out RemoteTurnReceipt, result error) {
	out.Source, out.InputDigest = RemoteTurnSource, q.InputDigest
	if !filepath.IsAbs(q.Workspace) || !ValidNativeID(q.Session) || len(q.InputDigest) != 64 || strings.Trim(q.InputDigest, "0123456789abcdef") != "" {
		return out, errors.New("original_turn_query_invalid")
	}
	dir := RemoteRuntimeDirectory(q.Workspace)
	raw, err := os.ReadFile(filepath.Join(dir, "ready.json"))
	if err != nil {
		return out, errors.New("original_turn_preparation_missing")
	}
	var ready RemoteReadiness
	if json.Unmarshal(raw, &ready) != nil || !ready.Verified || ready.NativeTransport != RemoteNativeTransport || !sameWorkspace(ready.Workspace, q.Workspace) || ready.SessionID != q.Session {
		return out, errors.New("original_turn_preparation_mismatch")
	}
	self, err := RuntimeBinarySHA256()
	if err != nil || self != ready.RuntimeSHA256 {
		return out, errors.New("original_turn_runtime_mismatch")
	}
	out.Readiness = ready
	var owned b.StopHandle
	var reader *HistoryClient
	// Failure to read history is separate from resource settlement. Preserve
	// the latter even when the original business server exits during this query.
	defer func() {
		if reader != nil {
			reader.Close()
			out.ReaderWritersAbsent = reader.WritersAbsent()
		} else {
			out.ReaderWritersAbsent = settleRemoteTree(owned)
		}
		if !out.ReaderWritersAbsent {
			result = errors.Join(result, errors.New("original_turn_reader_tree_unverified"))
		}
		out.InspectionFailed = result != nil
	}()
	if process.Probe(&ready.Process).State != "matches" {
		return out, errors.New("original_turn_process_unverified")
	}
	raw, err = os.ReadFile(filepath.Join(dir, "configuration.json"))
	var cfg RemoteAgentConfig
	if err != nil || json.Unmarshal(raw, &cfg) != nil || RemoteConfigurationDigest(cfg) != ready.ConfigurationDigest || cfg.RepositoryURL != ready.Repository {
		return out, errors.New("original_turn_configuration_mismatch")
	}
	exe, err := NativeExecutable(cfg.Executable)
	if err != nil {
		return out, errors.New("original_turn_native_unavailable")
	}
	reader, err = OpenHistory(ctx, b.Command{Executable: exe}, q.Workspace, "", func(cmd *exec.Cmd) (b.StopHandle, error) {
		handle, e := process.New(30 * time.Second).StartManaged(cmd)
		if e == nil {
			owned = handle
		}
		return handle, e
	})
	if err != nil {
		return out, errors.New("original_turn_reader_unavailable")
	}
	thread, err := reader.ReadThread(ctx, q.Session, q.Workspace)
	if err != nil {
		return out, errors.New("original_turn_history_unverified")
	}
	if process.Probe(&ready.Process).State != "matches" {
		return out, errors.New("original_turn_process_unverified")
	}
	out, err = MatchRemoteTurn(thread, q.InputDigest)
	out.Readiness = ready
	return out, err
}

// MatchRemoteTurn uses the complete native user item, not a terminal preview or
// model assertion. A unique wire command marker prevents identical report
// prompts in later workflow steps from selecting an earlier turn.
func MatchRemoteTurn(thread HistoryThread, inputDigest string) (RemoteTurnReceipt, error) {
	out := RemoteTurnReceipt{Source: RemoteTurnSource, InputDigest: inputDigest}
	matches := 0
	for _, turn := range thread.Turns {
		userID := ""
		for _, raw := range turn.Items {
			var header struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(raw, &header) != nil {
				return out, errors.New("original_turn_item_invalid")
			}
			if header.Type != "userMessage" {
				continue
			}
			var item struct {
				Type    string `json:"type"`
				ID      string `json:"id"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			}
			if json.Unmarshal(raw, &item) != nil {
				return out, errors.New("original_turn_item_invalid")
			}
			if item.Type != "userMessage" {
				continue
			}
			if matches > 0 {
				return out, errors.New("original_turn_followed_by_another_input")
			}
			if len(item.Content) != 1 || (item.Content[0].Type != "text" && item.Content[0].Type != "inputText") {
				continue
			}
			if fmt.Sprintf("%x", sha256.Sum256([]byte(item.Content[0].Text))) != inputDigest {
				continue
			}
			if userID != "" || item.ID == "" {
				return out, errors.New("original_turn_input_ambiguous")
			}
			userID = item.ID
		}
		if userID == "" {
			continue
		}
		matches++
		if matches > 1 || !ValidNativeID(turn.ID) {
			return out, errors.New("original_turn_input_ambiguous")
		}
		out.NativeTurnID, out.UserItemID, out.Started, out.Status = turn.ID, userID, true, turn.Status
		out.Completed = turn.CompletedAt != nil && *turn.CompletedAt > 0 && (turn.Status == "completed" || turn.Status == "failed" || turn.Status == "interrupted")
		finals := 0
		for _, raw := range turn.Items {
			var item struct {
				Type     string  `json:"type"`
				ID       string  `json:"id"`
				Text     *string `json:"text"`
				Phase    string  `json:"phase"`
				Delivery string  `json:"delivery"`
			}
			_ = json.Unmarshal(raw, &item)
			if item.Type != "agentMessage" || item.Phase != "final_answer" || item.Delivery == "async" {
				continue
			}
			finals++
			if finals > 1 || item.ID == "" || item.Text == nil || strings.TrimSpace(*item.Text) == "" || len(*item.Text) > 128<<10 {
				return out, errors.New("original_turn_final_unverified")
			}
			out.FinalItemID, out.FinalText = item.ID, *item.Text
			out.FinalDigest = fmt.Sprintf("%x", sha256.Sum256([]byte(*item.Text)))
		}
	}
	return out, nil
}
