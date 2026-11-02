package orca

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
)

// UploadScopedFile writes finite chunks to one explicit workspace selector and
// relative-path scope. Its caller supplies the purpose and total-use budget.
func UploadScopedFile(ctx context.Context, environment, runtime, worktree string, scope FileScope, chunkBytes int, source io.Reader) error {
	if runtime == "" || worktree == "" || !scope.valid() || chunkBytes <= 0 || chunkBytes > 384<<10 || source == nil {
		return errors.New("file_scope_invalid")
	}
	socket, err := openStateConnection(ctx, environment, runtime)
	if err != nil {
		return err
	}
	defer socket.close()
	buffer := make([]byte, chunkBytes)
	appendChunk := false
	for {
		n, readErr := io.ReadFull(source, buffer)
		if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
			return readErr
		}
		if n == 0 {
			return nil
		}
		_, err = callState(ctx, socket, "files.writeBase64Chunk", map[string]any{"worktree": worktree, "relativePath": scope.RelativePath, "contentBase64": base64.StdEncoding.EncodeToString(buffer[:n]), "append": appendChunk, "expectedExecutionHostId": "local"})
		if err != nil {
			return err
		}
		appendChunk = true
		if readErr != nil {
			return nil
		}
	}
}
