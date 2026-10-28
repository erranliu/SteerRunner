//go:build !windows

package process

func CheckCommandLine(string, []string) error { return nil }
func CommandLineUnits(string, []string) int   { return 0 }
