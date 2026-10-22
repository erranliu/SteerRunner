package orca

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"path/filepath"
	"strings"
)

type UsageFileChannel struct {
	socket stateConnection
	HostID string
}
type UsageSession struct {
	Agent     string `json:"agent"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	FilePath  string `json:"filePath"`
	CodexHome string `json:"codexHome"`
}
type UsageGroup struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ParentPath    *string `json:"parentPath"`
	ParentGroupID *string `json:"parentGroupId"`
	ConnectionID  *string `json:"connectionId"`
}
type UsageFolder struct {
	ID            string          `json:"id"`
	GroupID       string          `json:"projectGroupId"`
	Name          string          `json:"name"`
	Path          string          `json:"folderPath"`
	LinkedTask    json.RawMessage `json:"linkedTask"`
	LinkedItems   json.RawMessage `json:"linkedItems"`
	DiffComments  json.RawMessage `json:"diffComments"`
	ConnectionID  *string         `json:"connectionId"`
	LinkedContext json.RawMessage `json:"linkedTaskSourceContext"`
	Comment       string          `json:"comment"`
	Archived      bool            `json:"isArchived"`
	Pinned        bool            `json:"isPinned"`
}
type UsageStat struct {
	Size        int64    `json:"size"`
	IsDirectory bool     `json:"isDirectory"`
	IsSymlink   bool     `json:"isSymlink"`
	Mtime       *float64 `json:"mtime"`
}

// VerifySource checks explicit link metadata for every path component. Missing
// link facts are not treated as false. Only directory metadata is requested.
func (c *UsageFileChannel) VerifySource(ctx context.Context, root, file string) error {
	absolute := filepath.Join(root, file)
	volume := filepath.VolumeName(absolute)
	if len(volume) != 2 || volume[1] != ':' || !filepath.IsAbs(absolute) || !safeUsageRelative(file) {
		return errors.New("SOURCE_PATH_UNVERIFIED")
	}
	current := volume + string(filepath.Separator)
	components := strings.Split(strings.TrimPrefix(absolute, current), string(filepath.Separator))
	for i, name := range components {
		raw, err := callState(ctx, c.socket, "files.browseServerDir", map[string]any{"path": current})
		if err != nil {
			return err
		}
		var result struct {
			Resolved string `json:"resolvedPath"`
			Entries  []struct {
				Name      string `json:"name"`
				Directory *bool  `json:"isDirectory"`
				Link      *bool  `json:"isSymlink"`
			} `json:"entries"`
		}
		if json.Unmarshal(raw, &result) != nil || !strings.EqualFold(filepath.Clean(result.Resolved), filepath.Clean(current)) {
			return errors.New("SOURCE_PATH_UNVERIFIED")
		}
		found := 0
		for _, entry := range result.Entries {
			if !strings.EqualFold(entry.Name, name) {
				continue
			}
			found++
			if entry.Directory == nil || entry.Link == nil || *entry.Link || *entry.Directory != (i < len(components)-1) {
				return errors.New("SOURCE_PATH_UNVERIFIED")
			}
		}
		if found != 1 {
			return errors.New("SOURCE_PATH_UNVERIFIED")
		}
		current = filepath.Join(current, name)
	}
	return nil
}

// UsageHostID exposes only the paired public-key fingerprint, never its token.
func UsageHostID(environment string) (string, error) {
	p, err := resolveStatePairing(environment)
	if err != nil {
		return "", err
	}
	key, err := base64.StdEncoding.DecodeString(p.PublicKey)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(key)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}
func OpenUsageFiles(ctx context.Context, environment, runtime, expectedHost string) (*UsageFileChannel, error) {
	host := "local"
	var err error
	var socket stateConnection
	if environment != "" {
		pairing, e := resolveStatePairing(environment)
		if e != nil {
			return nil, e
		}
		key, e := base64.StdEncoding.DecodeString(pairing.PublicKey)
		if e != nil {
			return nil, e
		}
		digest := sha256.Sum256(key)
		host = "sha256:" + hex.EncodeToString(digest[:])
		if runtime == "" {
			return nil, errors.New("IDENTITY_UNVERIFIED")
		}
		if host != expectedHost {
			return nil, errors.New("IDENTITY_CHANGED")
		}
		socket, err = openStateSocket(ctx, pairing, runtime)
	} else {
		socket, err = openStateConnection(ctx, environment, runtime)
	}
	if err != nil {
		return nil, err
	}
	return &UsageFileChannel{socket: socket, HostID: host}, nil
}
func (c *UsageFileChannel) Close() error { return c.socket.close() }

// Existing endpoints return unpaginated lists. If a host advertises a partial
// or inconsistent inventory, it cannot prove unique ownership or safe deletion.
type usageInventoryBoundary struct {
	Truncated  bool   `json:"truncated"`
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor"`
	TotalCount *int   `json:"totalCount"`
}

func (b usageInventoryBoundary) complete(count int) bool {
	return !b.Truncated && !b.HasMore && b.NextCursor == "" && (b.TotalCount == nil || *b.TotalCount == count)
}

func (c *UsageFileChannel) Sessions(ctx context.Context) ([]UsageSession, error) {
	raw, err := callState(ctx, c.socket, "aiVault.listSessions", map[string]any{"limit": 2000})
	if err != nil {
		return nil, err
	}
	var r struct {
		Sessions []UsageSession `json:"sessions"`
		usageInventoryBoundary
	}
	err = json.Unmarshal(raw, &r)
	if r.Sessions == nil && err == nil {
		err = errors.New("missing session list")
	}
	if err == nil && !r.complete(len(r.Sessions)) {
		err = errors.New("SESSION_AMBIGUOUS")
	}
	return r.Sessions, err
}
func (c *UsageFileChannel) Groups(ctx context.Context) ([]UsageGroup, error) {
	raw, err := callState(ctx, c.socket, "projectGroup.list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var r struct {
		Groups []UsageGroup `json:"groups"`
		usageInventoryBoundary
	}
	err = json.Unmarshal(raw, &r)
	if (r.Groups == nil || !r.complete(len(r.Groups))) && err == nil {
		err = errors.New("missing group list")
	}
	return r.Groups, err
}
func (c *UsageFileChannel) Folders(ctx context.Context) ([]UsageFolder, error) {
	raw, err := callState(ctx, c.socket, "folderWorkspace.list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var r struct {
		Folders []UsageFolder `json:"folderWorkspaces"`
		usageInventoryBoundary
	}
	err = json.Unmarshal(raw, &r)
	if (r.Folders == nil || !r.complete(len(r.Folders))) && err == nil {
		err = errors.New("missing folder list")
	}
	return r.Folders, err
}

// GroupUnused checks the other reference type in projectGroup.delete's cascade.
// repo.list is an unpaginated authoritative list; folder and child-group checks
// remain separate and must also succeed immediately before deletion.
func (c *UsageFileChannel) GroupUnused(ctx context.Context, group string) (bool, error) {
	raw, err := callState(ctx, c.socket, "repo.list", nil)
	if err != nil {
		return false, err
	}
	var result struct {
		Repos []struct {
			Group *string `json:"projectGroupId"`
		} `json:"repos"`
		usageInventoryBoundary
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return false, err
	}
	if result.Repos == nil || !result.complete(len(result.Repos)) {
		return false, errors.New("incomplete repository inventory")
	}
	for _, repo := range result.Repos {
		if repo.Group != nil && *repo.Group == group {
			return false, nil
		}
	}
	return true, nil
}
func (c *UsageFileChannel) CreateGroup(ctx context.Context, name string) (UsageGroup, error) {
	var r struct {
		Group UsageGroup `json:"group"`
	}
	raw, err := callState(ctx, c.socket, "projectGroup.create", map[string]any{"name": name, "createdFrom": "manual"})
	if err == nil {
		err = json.Unmarshal(raw, &r)
	}
	if err == nil && r.Group.ID == "" {
		err = errors.New("group identity missing")
	}
	return r.Group, err
}
func (c *UsageFileChannel) CreateFolder(ctx context.Context, group, name, root string) (UsageFolder, error) {
	var r struct {
		Folder UsageFolder `json:"folderWorkspace"`
	}
	raw, err := callState(ctx, c.socket, "folderWorkspace.create", map[string]any{"projectGroupId": group, "name": name, "folderPath": root})
	if err == nil {
		err = json.Unmarshal(raw, &r)
	}
	if err == nil && r.Folder.ID == "" {
		err = errors.New("folder identity missing")
	}
	return r.Folder, err
}
func (c *UsageFileChannel) DeleteFolder(ctx context.Context, id string) error {
	_, err := callState(ctx, c.socket, "folderWorkspace.delete", map[string]any{"folderWorkspaceId": id})
	return err
}
func (c *UsageFileChannel) DeleteGroup(ctx context.Context, id string) error {
	_, err := callState(ctx, c.socket, "projectGroup.delete", map[string]any{"groupId": id})
	return err
}
func safeUsageRelative(s string) bool {
	return s != "" && path.Base(s) == s && !strings.ContainsAny(s, "\\/:\x00\r\n") && s != "." && s != ".."
}
func (c *UsageFileChannel) Stat(ctx context.Context, folder, file string) (UsageStat, error) {
	var stat UsageStat
	if !safeUsageRelative(file) {
		return stat, errors.New("invalid usage filename")
	}
	raw, err := callState(ctx, c.socket, "files.stat", map[string]any{"worktree": "folder:" + folder, "relativePath": file})
	if err == nil {
		err = json.Unmarshal(raw, &stat)
	}
	return stat, err
}
func (c *UsageFileChannel) ReadChunk(ctx context.Context, folder, file string, offset int64, length int) ([]byte, error) {
	if !safeUsageRelative(file) || offset < 0 || length < 1 || length > 512<<10 {
		return nil, errors.New("invalid usage read bound")
	}
	raw, err := callState(ctx, c.socket, "files.readChunk", map[string]any{"worktree": "folder:" + folder, "relativePath": file, "offset": offset, "length": length})
	if err != nil {
		return nil, err
	}
	var r struct {
		Content string `json:"contentBase64"`
		Bytes   *int   `json:"bytesRead"`
	}
	if json.Unmarshal(raw, &r) != nil || r.Bytes == nil || *r.Bytes < 0 || *r.Bytes > length {
		return nil, errors.New("invalid usage chunk")
	}
	body, err := base64.StdEncoding.DecodeString(r.Content)
	if err != nil || len(body) != *r.Bytes {
		return nil, errors.New("invalid usage bytes")
	}
	return body, nil
}
func (c *UsageFileChannel) SelectorRevoked(ctx context.Context, folder, file string) bool {
	_, err := c.Stat(ctx, folder, file)
	var rpc *StateRPCError
	return errors.As(err, &rpc) && rpc.Code == "selector_not_found"
}
