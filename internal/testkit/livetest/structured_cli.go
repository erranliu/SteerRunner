package livetest

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// ValidateStructuredCLI qualifies only CLI evidence. It never counts structured
// RPC inputs. Active snapshots may end with a pending call; after service exit
// every recorded call must have its original result, including failed calls.
func ValidateStructuredCLI(raw []byte, settled bool) error {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return errors.New("native CLI audit empty or truncated")
	}
	pending := map[string]string{}
	phases := map[string]int{}
	completed := 0
	for _, line := range bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'}) {
		var row struct {
			Kind, Phase string
			CallID      string `json:"call_id"`
			Args, Argv  []string
			Exit        *int
			Failed      *bool
		}
		if json.Unmarshal(line, &row) != nil {
			return errors.New("native CLI audit invalid")
		}
		args := row.Argv
		if row.Kind != "" {
			args = row.Args
		}
		if len(args) < 2 || (row.Kind != "" && row.Phase != "") {
			return errors.New("native CLI operation missing or ambiguous")
		}
		for _, arg := range args {
			if strings.Contains(arg, "--native-control") || strings.Contains(arg, "steerrunner-agent") || arg == "--configuration" {
				return errors.New("retired helper invoked")
			}
		}
		if args[0] == "terminal" && (args[1] == "send" || args[1] == "create") {
			return errors.New("retired terminal writer invoked")
		}
		encoded, _ := json.Marshal(args)
		key := string(encoded)
		switch {
		case row.Kind == "intent":
			if row.CallID == "" || pending[row.CallID] != "" {
				return errors.New("CLI intent identity missing or repeated")
			}
			pending[row.CallID] = key
		case row.Kind == "receipt":
			if row.CallID == "" || pending[row.CallID] != key || row.Failed == nil {
				return errors.New("CLI receipt missing its original intent")
			}
			delete(pending, row.CallID)
			completed++
		case row.Kind == "" && row.Phase == "dispatch":
			phases[key]++
		case row.Kind == "" && row.Phase == "result":
			if phases[key] <= 0 || row.Exit == nil {
				return errors.New("CLI result missing its original dispatch")
			}
			phases[key]--
			completed++
		default:
			return errors.New("native CLI journal row unrecognized")
		}
	}
	if completed == 0 {
		return errors.New("native CLI audit has no completed calls")
	}
	if settled {
		if len(pending) > 0 {
			return errors.New("native CLI audit has an unfinished call")
		}
		for _, count := range phases {
			if count != 0 {
				return errors.New("native CLI audit has an unfinished dispatch")
			}
		}
	}
	return nil
}
