//go:build !windows

package host

import (
	"context"
	"net"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func normalizeCase(value string) string { return value }
func unsupported() error {
	return contracts.Fail(3, "unsupported_platform", "The resident service currently supports Windows/amd64.")
}
func pipeName(string) (string, error)                    { return "", unsupported() }
func listenPipe(string) (net.Listener, error)            { return nil, unsupported() }
func dialPipe(context.Context, string) (net.Conn, error) { return nil, unsupported() }

type fileLock struct{}

func (*fileLock) Close() error                                    { return nil }
func tryLock(string, bool) (*fileLock, bool, error)               { return nil, false, unsupported() }
func writerBusy(string) (bool, error)                             { return false, unsupported() }
func replaceFile(string, string) error                            { return unsupported() }
func processFacts() (string, bool, error)                         { return "", false, unsupported() }
func startDetached(context.Context, string, string, string) error { return unsupported() }
func launchFacts(string, string) (string, bool, error)            { return "", false, unsupported() }
func completeBootstrap(string, string)                            {}
func bootstrapPath(dir string) string                             { return dir + "/service/bootstrap.json" }
