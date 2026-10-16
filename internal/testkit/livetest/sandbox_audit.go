package livetest

import (
	"github.com/erranliu/SteerRunner/internal/execution/nativeaudit"
	"io"
)

type SandboxAudit = nativeaudit.SandboxAudit

func AuditSandboxHistory(source io.Reader, session, workspace string) (SandboxAudit, error) {
	return nativeaudit.AuditSandboxHistory(source, session, workspace)
}
