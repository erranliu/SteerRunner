package livetest

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type CommandAudit struct {
	Completed        int `json:"completed_commands"`
	PermissionErrors int `json:"permission_errors"`
}

// AuditCommandJournal checks actual command results, including stderr from
// commands which returned zero. Prompts and model commentary are not evidence.
func AuditCommandJournal(source io.Reader) (CommandAudit, error) {
	var audit CommandAudit
	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	for scanner.Scan() {
		var row struct {
			Method string `json:"method"`
			Params struct {
				Item struct {
					Type   string `json:"type"`
					Output string `json:"aggregatedOutput"`
				} `json:"item"`
			} `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &row) != nil {
			return audit, errors.New("invalid native command journal")
		}
		if row.Method != "item/completed" || row.Params.Item.Type != "commandExecution" {
			continue
		}
		audit.Completed++
		output := strings.ToLower(row.Params.Item.Output)
		for _, denied := range []string{"permission denied", "access is denied", "access denied", "unauthorizedaccessexception", "operation not permitted", "operation denied", "http 401", "http 403", "拒绝访问"} {
			if strings.Contains(output, denied) {
				audit.PermissionErrors++
				break
			}
		}
	}
	if scanner.Err() != nil {
		return audit, scanner.Err()
	}
	if audit.PermissionErrors > 0 {
		return audit, errors.New("native command output contains a permission failure, regardless of exit code")
	}
	return audit, nil
}
