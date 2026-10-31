package livetest

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
)

// A bare Enter is a submission effect, never an independently exempt send.
// Retain its mutation/receipt accounting and require one accepted original-pane
// bracketed paste followed by exactly one accepted Enter on that same terminal.
func auditNativePasteSubmissions(calls []nativeAuditCall, runtime string, p NativeReceiptAuditPolicy) (map[string]bool, bool, error) {
	submits := map[string]bool{}
	var paste, enter *nativeAuditCall
	accepted := false
	original := ""
	started := false
	for _, c := range calls {
		if len(c.Args) < 2 || c.Args[0] != "terminal" || c.Args[1] != "send" {
			continue
		}
		if c.Kind == "intent" {
			if !started {
				started = true
				original = auditArg(c.Args, "--terminal")
				continue
			}
			text := auditArg(c.Args, "--text")
			if text != "" {
				if paste != nil {
					return nil, false, errors.New("native paste has an unresolved submission")
				}
				if !strings.ContainsAny(text, "\x1b\x00") {
					continue
				}
				const begin, end = "\x1b[200~", "\x1b[201~"
				if !strings.HasPrefix(text, begin) || !strings.HasSuffix(text, end) || len(text) <= len(begin)+len(end) {
					return nil, false, errors.New("invalid native bracketed paste")
				}
				body := text[len(begin) : len(text)-len(end)]
				if strings.ContainsAny(body, "\x1b\x00") || !nativePasteSendFlags(c.Args, original, p.Environment, false) {
					return nil, false, errors.New("native paste target or flags changed")
				}
				copy := c
				paste = &copy
				accepted = false
			} else {
				if paste == nil || !accepted || enter != nil || !nativePasteSendFlags(c.Args, original, p.Environment, true) {
					return nil, false, errors.New("native Enter lacks one accepted original paste")
				}
				copy := c
				enter = &copy
				submits[c.ID] = true
			}
		} else if c.Kind == "receipt" {
			if paste != nil && c.ID == paste.ID {
				if !reflect.DeepEqual(c.Args, paste.Args) || !nativePasteReceipt(c, runtime, original, len(auditArg(paste.Args, "--text"))) {
					return nil, false, errors.New("native paste effect receipt unverified")
				}
				accepted = true
			} else if enter != nil && c.ID == enter.ID {
				if !reflect.DeepEqual(c.Args, enter.Args) || !nativePasteReceipt(c, runtime, original, 1) {
					return nil, false, errors.New("native Enter effect receipt unverified")
				}
				paste, enter = nil, nil
				accepted = false
			}
		}
	}
	return submits, paste != nil || enter != nil, nil
}

func nativePasteSendFlags(args []string, terminal, environment string, enter bool) bool {
	if terminal == "" {
		return false
	}
	wanted := map[string]string{"--terminal": terminal, "--json": ""}
	if environment != "" {
		wanted["--environment"] = environment
	}
	if enter {
		wanted["--enter"] = ""
	} else {
		wanted["--text"] = auditArg(args, "--text")
	}
	seen := map[string]bool{}
	for i := 2; i < len(args); i++ {
		key := args[i]
		value, ok := wanted[key]
		if !ok || seen[key] {
			return false
		}
		seen[key] = true
		if key == "--json" || key == "--enter" {
			continue
		}
		i++
		if i >= len(args) || args[i] != value {
			return false
		}
	}
	return len(seen) == len(wanted)
}
func nativePasteReceipt(c nativeAuditCall, runtime, terminal string, size int) bool {
	var r struct {
		OK   bool
		Meta struct {
			RuntimeID string `json:"runtimeId"`
		} `json:"_meta"`
		Result struct {
			Send struct {
				Handle       string
				Accepted     *bool
				BytesWritten int
			}
		}
	}
	return !c.Failed && json.Unmarshal(c.Response, &r) == nil && r.OK && r.Meta.RuntimeID == runtime && r.Result.Send.Handle == terminal && r.Result.Send.Accepted != nil && *r.Result.Send.Accepted && r.Result.Send.BytesWritten == size
}
