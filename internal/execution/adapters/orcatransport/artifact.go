package orcatransport

import (
	"context"
	"errors"
	"io"
	"strings"

	native "github.com/erranliu/SteerRunner/internal/execution/adapters/protocol/orca"
)

// UploadWorkspaceArtifact retains the execution-owned staging policy while
// delegating the original bounded transport to Orca's file capability.
func UploadWorkspaceArtifact(ctx context.Context, environment, runtime, worktree, relative string, source io.Reader) error {
	if !strings.HasPrefix(relative, ".sr-runtime-stage/") || strings.Contains(relative, "..") || strings.ContainsAny(relative, "\\\r\n\x00") {
		return errors.New("runtime artifact must stay inside its staging directory")
	}
	return native.UploadScopedFile(ctx, environment, runtime, worktree, native.FileScope{Root: ".sr-runtime-stage", RelativePath: relative}, 384<<10, source)
}
