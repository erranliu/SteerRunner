package orca

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var catalogCapabilities = []string{"agent-session.structured.v1", "agent-session.structured.registered-agents.v1"}
var catalogAgentID = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
var ErrCatalogUnsupported = errors.New("Orca directory query unsupported")

type CatalogEnvironment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type CatalogEffort struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
type CatalogModel struct {
	ID            string `json:"id"`
	Label         string `json:"label"`
	Provider      string
	Default       bool            `json:"isDefault"`
	DefaultEffort string          `json:"defaultEffort"`
	Efforts       []CatalogEffort `json:"efforts"`
}
type Catalog struct {
	Version   string
	RuntimeID string
	Protocol  int
	Agents    []string
	Models    []CatalogModel
	Origin    string
	FetchedAt *time.Time
	Listing   bool
}

func CatalogEnvironments(ctx context.Context) ([]CatalogEnvironment, error) {
	raw, err := readCatalogMetadata("orca-environments.json")
	if os.IsNotExist(err) {
		return []CatalogEnvironment{}, ctx.Err()
	}

	var store struct {
		Version      int                  `json:"version"`
		Environments []CatalogEnvironment `json:"environments"`
	}
	if err != nil || json.Unmarshal(raw, &store) != nil || store.Version != 1 || store.Environments == nil {
		return nil, errors.New("invalid Orca environment directory")
	}
	seen := map[string]bool{}
	for _, e := range store.Environments {
		if e.ID == "" || seen[e.ID] {
			return nil, errors.New("ambiguous Orca environment directory")
		}
		seen[e.ID] = true
	}
	return store.Environments, ctx.Err()
}

// QueryCatalog owns only this query's connection. No execution resources are created.
func QueryCatalog(ctx context.Context, environment, kind, agent string) (Catalog, error) {
	var call func(context.Context, string, any) (json.RawMessage, error)
	var closeConnection func() error
	var runtimeID func() string
	pairedDevice := ""
	if environment == "" {
		metadata, err := readCatalogMetadata("orca-runtime.json")
		var runtime struct {
			ID         string `json:"runtimeId"`
			Token      string `json:"authToken"`
			Transports []struct {
				Kind     string `json:"kind"`
				Endpoint string `json:"endpoint"`
			} `json:"transports"`
		}
		if err != nil || json.Unmarshal(metadata, &runtime) != nil || runtime.ID == "" || runtime.Token == "" {
			return Catalog{}, errors.New("Orca metadata unavailable")
		}
		endpoints := 0
		for _, transport := range runtime.Transports {
			if transport.Kind != "named-pipe" {
				continue
			}
			endpoint := transport.Endpoint
			if !strings.HasPrefix(endpoint, `\\.\pipe\`) || len(strings.TrimPrefix(endpoint, `\\.\pipe\`)) == 0 || strings.ContainsAny(strings.TrimPrefix(endpoint, `\\.\pipe\`), "\\/\x00\r\n") {
				return Catalog{}, errors.New("Orca metadata pipe invalid")
			}
			endpoints++
		}
		if endpoints != 1 {
			return Catalog{}, errors.New("Orca metadata pipe unavailable")
		}
		id := runtime.ID
		if err != nil {
			return Catalog{}, err
		}
		connection, err := openLocalStateMode(ctx, id, true)
		if err != nil {
			return Catalog{}, err
		}
		call = func(ctx context.Context, method string, params any) (json.RawMessage, error) {
			return callState(ctx, connection, method, params)
		}
		closeConnection = connection.close
		runtimeID = func() string { return connection.runtime }
	} else {
		metadata, err := readCatalogMetadata("orca-environments.json")
		if err != nil {
			return Catalog{}, err
		}
		pairing, err := decodeStatePairing(metadata, environment)
		if err != nil {
			return Catalog{}, err
		}
		connection, err := openStateSocketMode(ctx, pairing, "", catalogCapabilities, true)
		if err != nil {
			return Catalog{}, err
		}
		call = func(ctx context.Context, method string, params any) (json.RawMessage, error) {
			id, err := connection.requestWithCapabilities(ctx, method, params, catalogCapabilities, false)
			if err != nil {
				return nil, err
			}
			return connection.next(ctx, id)
		}
		closeConnection = connection.close
		runtimeID = func() string { return connection.runtime }
		pairedDevice = pairing.PairedDevice
	}
	defer closeConnection()
	result, err := readCatalog(ctx, call, pairedDevice, kind, agent)
	result.RuntimeID = runtimeID()
	return result, err
}

func readCatalog(ctx context.Context, call func(context.Context, string, any) (json.RawMessage, error), pairedDevice, kind, agent string) (Catalog, error) {
	out := Catalog{Origin: "registered", Agents: []string{}}
	raw, err := call(ctx, "status.get", map[string]any{})
	if err != nil {
		return out, err
	}
	var status struct {
		Capabilities []string `json:"capabilities"`
		Protocol     int      `json:"runtimeProtocolVersion"`
		Minimum      int      `json:"minCompatibleRuntimeClientVersion"`
		Version      string   `json:"appVersion"`
		PairedDevice string   `json:"pairedDeviceId"`
	}
	if json.Unmarshal(raw, &status) != nil || status.Protocol != 3 || status.Minimum > 3 {
		return out, ErrCatalogUnsupported
	}
	if pairedDevice != "" && status.PairedDevice != pairedDevice {
		return out, errors.New("Orca directory identity changed")
	}
	out.Version, out.Protocol = status.Version, status.Protocol
	for _, required := range catalogCapabilities {
		found := false
		for _, capability := range status.Capabilities {
			if capability == required {
				found = true
			}
		}
		if !found {
			return out, ErrCatalogUnsupported
		}
	}
	raw, err = call(ctx, "agentSession.agents", map[string]any{})
	if err != nil {
		return out, err
	}
	var agents struct {
		Agents []struct {
			ID string `json:"agent"`
		} `json:"agents"`
	}
	if json.Unmarshal(raw, &agents) != nil || agents.Agents == nil {
		return out, errors.New("incomplete Orca agents")
	}
	seen := map[string]bool{}
	for _, row := range agents.Agents {
		if !catalogAgentID.MatchString(row.ID) || seen[row.ID] {
			return out, errors.New("invalid Orca agents")
		}
		seen[row.ID] = true
		out.Agents = append(out.Agents, row.ID)
	}
	if kind == "agents" {
		return out, nil
	}
	if !seen[agent] {
		return out, ErrCatalogUnsupported
	}
	params := map[string]any{"agent": agent}
	type modelListing struct {
		Origin    string         `json:"origin"`
		FetchedAt int64          `json:"fetchedAt"`
		Listing   bool           `json:"listingInProgress"`
		Models    []CatalogModel `json:"models"`
	}
	var listing modelListing
	for attempt := 0; attempt < 2; attempt++ {
		raw, err = call(ctx, "agentSession.modelCatalog", params)
		if err != nil {
			return out, err
		}
		listing = modelListing{}
		if json.Unmarshal(raw, &listing) != nil {
			return out, errors.New("invalid Orca catalog")
		}
		if !listing.Listing {
			break
		}
		params["waitForListing"] = true
	}
	out.Origin = listing.Origin
	if out.Origin == "" {
		out.Origin = "unknown"
	}
	if listing.FetchedAt > 0 {
		value := time.UnixMilli(listing.FetchedAt).UTC()
		out.FetchedAt = &value
	}
	out.Listing = listing.Listing
	if listing.Models == nil && !listing.Listing {
		return out, errors.New("incomplete Orca models")
	}
	out.Models = []CatalogModel{}
	seen = map[string]bool{}
	for _, row := range listing.Models {
		if row.ID == "" || seen[row.ID] {
			return out, errors.New("invalid Orca model identity")
		}
		seen[row.ID] = true
		efforts := map[string]bool{}
		for _, effort := range row.Efforts {
			if effort.Value == "" || efforts[effort.Value] {
				return out, errors.New("invalid Orca effort")
			}
			efforts[effort.Value] = true
		}
		if row.Efforts == nil {
			row.Efforts = []CatalogEffort{}
		}
		if agent == "pi" {
			provider, model, ok := strings.Cut(row.ID, "/")
			if ok && catalogAgentID.MatchString(provider) && model != "" {
				row.Provider = provider
			}
		}
		out.Models = append(out.Models, row)
	}
	return out, nil
}

func readCatalogMetadata(name string) ([]byte, error) {
	dir, err := stateUserDataDir()
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(dir) {
		return nil, errors.New("Orca metadata directory must be absolute")
	}
	file := filepath.Join(dir, name)
	resolved, err := filepath.EvalSymlinks(file)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(file)) {
		return nil, errors.New("Orca metadata link unsupported")
	}
	info, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, errors.New("Orca metadata unavailable")
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		return nil, errors.New("Orca metadata exceeds limit")
	}
	return raw, nil
}
