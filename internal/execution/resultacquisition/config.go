package resultacquisition

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf8"
)

const ConfigSchema = "steerrunner-result-acquisition/v1"
const ParserProfile = "codex-parser/v2:0.159.2:cli:memory:provided-text"

type ParserConfig struct {
	MaxConcurrent   int    `json:"max_concurrent,omitempty"`
	Enabled         bool   `json:"enabled"`
	Executable      string `json:"executable"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	CodexHomeEnv    string `json:"codex_home_env"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	MaxInputBytes   int    `json:"max_input_bytes"`
	MaxOutputBytes  int    `json:"max_output_bytes"`
}
type Config struct {
	Schema      string       `json:"schema"`
	CodexParser ParserConfig `json:"codex_parser"`
}
type FrozenConfig struct {
	Config     Config `json:"config"`
	FileDigest string `json:"file_digest"`
}

func DefaultConfig() Config {
	return Config{Schema: ConfigSchema, CodexParser: ParserConfig{Executable: "codex.exe", TimeoutSeconds: 120, MaxInputBytes: 131072, MaxOutputBytes: 32768}}
}

func LoadConfig(stateDir string) (FrozenConfig, error) {
	c := DefaultConfig()
	data, err := os.ReadFile(filepath.Join(stateDir, "service", "result-acquisition.json"))
	if os.IsNotExist(err) {
		data, _ = json.Marshal(c)
		return FrozenConfig{c, Digest(data)}, nil
	}
	if err != nil {
		return FrozenConfig{}, err
	}
	if !utf8.Valid(data) || len(data) > 16384 {
		return FrozenConfig{}, errors.New("result acquisition configuration exceeds bound")
	}
	fields := resultFields{"schema": nil, "codex_parser": {"enabled": nil, "executable": nil, "model": nil, "reasoning_effort": nil, "codex_home_env": nil, "timeout_seconds": nil, "max_input_bytes": nil, "max_output_bytes": nil, "max_concurrent": nil}}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	if err := uniqueJSON(dec, fields); err != nil {
		return FrozenConfig{}, errors.New("ambiguous result acquisition configuration")
	}
	if _, err := dec.Token(); err != io.EOF {
		return FrozenConfig{}, errors.New("trailing configuration")
	}
	if !exactConfigKeys(data, fields) {
		return FrozenConfig{}, errors.New("unknown or null configuration field")
	}
	dec = json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return FrozenConfig{}, errors.New("invalid result acquisition configuration")
	}
	if err := c.Validate(); err != nil {
		return FrozenConfig{}, err
	}
	return FrozenConfig{c, Digest(data)}, nil
}

// Validate checks the persisted contract without rereading mutable service
// configuration, credentials, provider availability or model defaults.
func (c Config) Validate() error {
	p := c.CodexParser
	if p.MaxConcurrent < 0 || c.Schema != ConfigSchema || p.Executable == "" || strings.ContainsAny(p.Executable+p.Model+p.ReasoningEffort, "\x00\r\n") || strings.TrimSpace(p.Model) != p.Model || p.TimeoutSeconds < 10 || p.TimeoutSeconds > 300 || p.MaxInputBytes < 16384 || p.MaxInputBytes > 262144 || p.MaxOutputBytes < 4096 || p.MaxOutputBytes > 65536 {
		return errors.New("unsupported or out of range result acquisition configuration")
	}
	if p.CodexHomeEnv != "" && !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`).MatchString(p.CodexHomeEnv) {
		return errors.New("invalid native home environment reference")
	}
	switch p.ReasoningEffort {
	case "", "minimal", "low", "medium", "high", "xhigh", "max", "ultra":
	default:
		return errors.New("unsupported parser effort")
	}
	return nil
}
func exactConfigKeys(data []byte, fields resultFields) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal(data, &object) != nil || object == nil {
		return false
	}
	for key, value := range object {
		nested, ok := fields[key]
		if !ok || string(value) == "null" {
			return false
		}
		if nested != nil && !exactConfigKeys(value, nested) {
			return false
		}
	}
	return true
}

// ParserSupport describes the built-in profile, not availability of any host,
// authentication, or model. Qualification is lazy at the actual owner boundary.
func ParserSupport() (bool, string) {
	if runtime.GOOS != "windows" {
		return false, "native parser profile is qualified only on Windows"
	}
	return true, "codex-parser/v1: CLI read-only/never, internal tool disabling and event rejection, disposable process; not an empty-tool guarantee; no execution owner or recovery"
}
