//go:build windows

package process

import (
	"strings"
	"testing"
)

func TestCommandLineTransportBound(t *testing.T) {
	// x + space + argument + NUL. These are actual escaped Windows argv units.
	if err := CheckCommandLine("x", []string{strings.Repeat("a", 32764)}); err != nil {
		t.Fatal(err)
	}
	if err := CheckCommandLine("x", []string{strings.Repeat("a", 32765)}); err == nil {
		t.Fatal("32768 units admitted")
	}
	if CommandLineUnits("x", []string{"😀"}) != 5 || CommandLineUnits("x", []string{"a b"}) != 8 {
		t.Fatal("non-BMP/quoting not counted")
	}
}
