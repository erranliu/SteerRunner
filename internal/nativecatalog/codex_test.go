package nativecatalog

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("SL_DISCOVERY_CHILD") == "1" && len(os.Args) > 1 && os.Args[1] == "app-server" {
		nativeFixture()
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func nativeFixture() {
	if log := os.Getenv("SL_DISCOVERY_CWD"); log != "" {
		cwd, _ := os.Getwd()
		_ = os.WriteFile(log, []byte(cwd), 0600)
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var q struct {
			ID     *int           `json:"id"`
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		json.Unmarshal(scanner.Bytes(), &q)
		if log := os.Getenv("SL_DISCOVERY_LOG"); log != "" {
			f, e := os.OpenFile(log, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if e == nil {
				f.WriteString(q.Method + "\n")
				f.Close()
			}
		}
		if q.ID == nil {
			continue
		}
		var result any
		switch q.Method {
		case "initialize":
			result = map[string]string{"userAgent": "fixture/1"}
		case "model/list":
			if q.Params["includeHidden"] != false {
				os.Exit(3)
			}
			if os.Getenv("SL_DISCOVERY_MODE") == "slow" {
				time.Sleep(100 * time.Millisecond)
			}
			switch os.Getenv("SL_DISCOVERY_MODE") {
			case "hang":
				time.Sleep(time.Minute)
			case "oversize":
				os.Stdout.WriteString(strings.Repeat("x", (1<<20)+1) + "\n")
				continue
			case "disconnect":
				os.Exit(0)
			case "invalid":
				os.Stdout.WriteString("not-json\n")
				continue
			}
			cursor := ""
			if os.Getenv("SL_DISCOVERY_MODE") == "cycle" || q.Params["cursor"] == nil {
				cursor = "page2"
			}
			id := "first"
			if q.Params["cursor"] != nil {
				id = "second"
			}
			efforts := []any{map[string]string{"reasoningEffort": "high", "description": "High"}}
			switch os.Getenv("SL_DISCOVERY_MODE") {
			case "duplicate-model":
				id = "first" // The second page repeats the first page's identity.
			case "empty-model":
				id = ""
			case "duplicate-effort":
				efforts = append(efforts, map[string]string{"reasoningEffort": "high", "description": "Other label"})
			case "empty-effort":
				efforts = []any{map[string]string{"reasoningEffort": "", "description": "Empty"}}
			}
			result = map[string]any{"data": []any{
				map[string]any{"id": id, "model": id, "displayName": id, "isDefault": q.Params["cursor"] == nil, "supportedReasoningEfforts": efforts, "defaultReasoningEffort": "high"},
				// Hidden rows must never leak into the public catalog, even if their
				// identities would be invalid for a visible model.
				map[string]any{"id": "", "model": "", "hidden": true},
			}, "nextCursor": cursor}
		default:
			os.Exit(3)
		}
		encoder.Encode(map[string]any{"id": *q.ID, "result": result})
	}
}

func TestCodexCatalog(t *testing.T) {
	t.Setenv("SL_DISCOVERY_CHILD", "1")
	exe, _ := os.Executable()
	log := filepath.Join(t.TempDir(), "methods")
	t.Setenv("SL_DISCOVERY_LOG", log)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	models, version, err := codexModels(ctx, exe)
	if err != nil || len(models) != 2 || version != "fixture/1" || models[1].ID != "second" || models[0].Efforts[0].Value != "high" || !models[0].Default || models[0].DefaultEffort != "high" || models[1].Default {
		t.Fatal(models, version, err)
	}
	methods, _ := os.ReadFile(log)
	if string(methods) != "initialize\ninitialized\nmodel/list\nmodel/list\n" {
		t.Fatal("unexpected native methods", string(methods))
	}
	for _, mode := range []string{"cycle", "invalid", "hang", "oversize", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("SL_DISCOVERY_MODE", mode)
			ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
			defer cancel()
			started := time.Now()
			_, _, err := codexModels(ctx, exe)
			if err == nil || time.Since(started) > 6*time.Second {
				t.Fatal("invalid query or cleanup succeeded", err)
			}
		})
	}
}

func TestCodexCatalogRejectsInvalidIdentities(t *testing.T) {
	t.Setenv("SL_DISCOVERY_CHILD", "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"duplicate-model", "empty-model", "duplicate-effort", "empty-effort"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("SL_DISCOVERY_MODE", mode)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			models, _, err := codexModels(ctx, exe)
			if err == nil || ctx.Err() != nil || len(models) != 0 {
				t.Fatal("invalid identity must be rejected before the deadline", models, err, ctx.Err())
			}
			want := "duplicate or empty model identities"
			if strings.Contains(mode, "effort") {
				want = "invalid effort catalog"
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatal("fixture did not reach catalog validation", err)
			}
		})
	}
}
