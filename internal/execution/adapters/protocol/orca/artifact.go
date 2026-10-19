package orca

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

// ReadStopArtifact reads one bounded public exit envelope through the original
// runtime channel. Authenticity belongs to the helper's frozen signing key,
// never to this worker-writable path. No terminal, service or model is started.
func ReadStopArtifact(ctx context.Context, environment, runtime, worktree, relative string) ([]byte, error) {
	if !strings.HasPrefix(relative, ".sr-runtime-stage/native-stop/") || !strings.HasSuffix(relative, "/exit.json") || strings.Contains(relative, "..") || strings.ContainsAny(relative, "\\\r\n\x00") || worktree == "" {
		return nil, errors.New("native exit artifact path invalid")
	}
	socket, err := openStateConnection(ctx, environment, runtime)
	if err != nil {
		return nil, err
	}
	defer socket.close()
	raw, err := callState(ctx, socket, "files.readChunk", map[string]any{"worktree": "id:" + worktree, "relativePath": relative, "offset": 0, "length": 65536})
	if err != nil {
		return nil, err
	}
	return decodeStopArtifact(raw)
}

func decodeStopArtifact(raw []byte) ([]byte, error) {
	var result struct {
		Content string `json:"contentBase64"`
		Bytes   *int   `json:"bytesRead"`
		EOF     *bool  `json:"eof"`
	}
	if json.Unmarshal(raw, &result) != nil || result.Bytes == nil || result.EOF == nil || !*result.EOF || *result.Bytes <= 0 || *result.Bytes > 65536 || len(result.Content) > 87384 {
		return nil, errors.New("native exit artifact is incomplete or oversized")
	}
	body, err := base64.StdEncoding.DecodeString(result.Content)
	if err != nil || len(body) != *result.Bytes {
		return nil, errors.New("native exit artifact byte count differs")
	}
	return body, nil
}

// UploadWorkspaceArtifact transfers a public installation artifact to a caller
// selected staging workspace. Each bounded chunk uses the selected host channel;
// it never opens a network listener or transports execution credentials.
func UploadWorkspaceArtifact(ctx context.Context, environment, runtime, worktree, relative string, source io.Reader) error {
	if !strings.HasPrefix(relative, ".sr-runtime-stage/") || strings.Contains(relative, "..") || strings.ContainsAny(relative, "\\\r\n\x00") {
		return errors.New("runtime artifact must stay inside its staging directory")
	}
	socket, err := openStateConnection(ctx, environment, runtime)
	if err != nil {
		return err
	}
	defer socket.close()
	buffer := make([]byte, 384<<10)
	appendChunk := false
	for {
		n, readErr := io.ReadFull(source, buffer)
		if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
			return readErr
		}
		if n == 0 {
			return nil
		}
		_, err = callState(ctx, socket, "files.writeBase64Chunk", map[string]any{"worktree": worktree, "relativePath": relative, "contentBase64": base64.StdEncoding.EncodeToString(buffer[:n]), "append": appendChunk, "expectedExecutionHostId": "local"})
		if err != nil {
			return err
		}
		appendChunk = true
		if readErr != nil {
			return nil
		}
	}
}
