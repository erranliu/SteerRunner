package codexworker

import "github.com/erranliu/SteerRunner/internal/codexcommand"

func NativeExecutable(command string) (string, error) { return codexcommand.NativeExecutable(command) }
func isNativeCodexExecutable(path string) bool        { return codexcommand.IsNativeCodexExecutable(path) }
