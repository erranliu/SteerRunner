// Package resources contains the resources shipped inside the executable.
package resources

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"sort"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

var Version = "0.1.0-dev.1"
var SourceCommit = "development"

//go:embed assets/*
var assets embed.FS

func Read(name string) []byte {
	value, err := assets.ReadFile("assets/" + name)
	if err != nil {
		panic("missing embedded resource")
	}
	return value
}

func Digest() string {
	names, err := fs.Glob(assets, "assets/*")
	if err != nil {
		panic("invalid embedded resource glob")
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		body, err := assets.ReadFile(name)
		if err != nil {
			panic("unreadable embedded resource")
		}
		digest.Write([]byte(name + "\x00"))
		digest.Write(body)
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func VersionInfo() any {
	return map[string]any{
		"version": Version, "source_commit": SourceCommit,
		"protocols": []string{contracts.Protocol}, "resource_version": 1,
		"resource_digest": Digest(), "platform": "windows/amd64",
	}
}

func Capabilities() any {
	return map[string]any{
		"protocol": contracts.Protocol, "version": Version,
		"resource_version": 1, "resource_digest": Digest(),
		"execution":             map[string]any{"status": "unsupported", "reason": "Native execution extraction is pending its verified source handoff."},
		"workflow":              json.RawMessage(Read("development.json")),
		"configuration_schemas": json.RawMessage(Read("settings.schema.json")),
		"cli_schema":            json.RawMessage(Read("cli.schema.json")),
		"commands":              []string{"version", "service start", "service status", "service stop", "capabilities", "inspect"},
		"planned_commands":      []string{"submit", "events", "reply", "resume", "stop"},
	}
}
