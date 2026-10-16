package orca

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

// UploadWorkspaceArtifact transfers a public installation artifact to a caller
// selected staging workspace. Each bounded chunk uses the paired host channel;
// it never opens a network listener or transports execution credentials.
func UploadWorkspaceArtifact(ctx context.Context, environment, runtime, worktree, relative string, source io.Reader) error {
	if !strings.HasPrefix(relative, ".sr-runtime-stage/") || strings.Contains(relative, "..") || strings.ContainsAny(relative, "\\\r\n\x00") {
		return errors.New("runtime artifact must stay inside its staging directory")
	}
	pairing, err := resolveStatePairing(environment)
	if err != nil {
		return err
	}
	socket, err := openStateSocket(ctx, pairing, runtime)
	if err != nil {
		return err
	}
	defer socket.conn.CloseNow()
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
		_, err = socket.call(ctx, "files.writeBase64Chunk", map[string]any{"worktree": worktree, "relativePath": relative, "contentBase64": base64.StdEncoding.EncodeToString(buffer[:n]), "append": appendChunk, "expectedExecutionHostId": "local"})
		if err != nil {
			return err
		}
		appendChunk = true
		if readErr != nil {
			return nil
		}
	}
}
