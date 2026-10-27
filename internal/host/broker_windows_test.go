//go:build windows

package host

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestBrokerPreservesDisabledPrivilegesAndRejectsMissingRights(t *testing.T) {
	var original windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE, &original); err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	originalProfile, err := tokenProfile(original)
	if err != nil {
		t.Fatal(err)
	}
	// WMI may start the test with every present privilege enabled. Build the
	// disabled-right fixture on a private duplicate, never the process token.
	var caller windows.Token
	if err := windows.DuplicateTokenEx(original, windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE|windows.TOKEN_ADJUST_PRIVILEGES, nil, windows.SecurityImpersonation, windows.TokenPrimary, &caller); err != nil {
		t.Fatal(err)
	}
	defer caller.Close()
	initial, err := tokenPrivileges(caller)
	if err != nil || len(initial) < 2 {
		t.Fatalf("fixture requires two existing rights: %v", err)
	}
	hasDisabled := false
	for _, p := range initial {
		hasDisabled = hasDisabled || p.Attributes&windows.SE_PRIVILEGE_ENABLED == 0
	}
	if !hasDisabled {
		fixture := windows.Tokenprivileges{PrivilegeCount: 1, Privileges: [1]windows.LUIDAndAttributes{{Luid: initial[0].Luid, Attributes: 0}}}
		if err := windows.AdjustTokenPrivileges(caller, false, &fixture, 0, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	before, err := tokenProfile(caller)
	if err != nil {
		t.Fatal(err)
	}
	privileges, err := tokenPrivileges(caller)
	if err != nil {
		t.Fatal(err)
	}
	var selected *windows.LUIDAndAttributes
	for i := range privileges {
		if privileges[i].Attributes&windows.SE_PRIVILEGE_ENABLED == 0 {
			selected = &privileges[i]
			break
		}
	}
	if selected == nil {
		t.Fatal("private fixture did not establish a disabled privilege")
	}
	var child windows.Token
	if err := windows.DuplicateTokenEx(caller, windows.TOKEN_QUERY|windows.TOKEN_ADJUST_PRIVILEGES, nil, windows.SecurityImpersonation, windows.TokenPrimary, &child); err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	state := windows.Tokenprivileges{PrivilegeCount: 1, Privileges: [1]windows.LUIDAndAttributes{{Luid: selected.Luid, Attributes: windows.SE_PRIVILEGE_ENABLED}}}
	if err := windows.AdjustTokenPrivileges(child, false, &state, 0, nil, nil); err != nil {
		t.Fatal(err)
	}
	changed, err := tokenProfile(child)
	if err != nil || changed == before {
		t.Fatalf("fixture did not enable child privilege: %v", err)
	}
	if err := preserveTokenProfile(caller, child); err != nil {
		t.Fatal(err)
	}
	after, err := tokenProfile(child)
	if err != nil || after != before {
		t.Fatalf("caller profile was not restored: %v", err)
	}
	// A missing authorization right cannot be invented to satisfy identity.
	state.Privileges[0].Attributes = windows.SE_PRIVILEGE_REMOVED
	if err := windows.AdjustTokenPrivileges(child, false, &state, 0, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := preserveTokenProfile(caller, child); err == nil {
		t.Fatal("missing child privilege accepted")
	}
	t.Run("does-not-enable-child", func(t *testing.T) {
		var enabled *windows.LUIDAndAttributes
		for i := range privileges {
			if privileges[i].Attributes&windows.SE_PRIVILEGE_ENABLED != 0 {
				enabled = &privileges[i]
				break
			}
		}
		if enabled == nil {
			t.Skip("caller has no enabled privilege")
		}
		var reduced windows.Token
		if err := windows.DuplicateTokenEx(caller, windows.TOKEN_QUERY|windows.TOKEN_ADJUST_PRIVILEGES, nil, windows.SecurityImpersonation, windows.TokenPrimary, &reduced); err != nil {
			t.Fatal(err)
		}
		defer reduced.Close()
		state := windows.Tokenprivileges{PrivilegeCount: 1, Privileges: [1]windows.LUIDAndAttributes{{Luid: enabled.Luid, Attributes: 0}}}
		if err := windows.AdjustTokenPrivileges(reduced, false, &state, 0, nil, nil); err != nil {
			t.Fatal(err)
		}
		if err := preserveTokenProfile(caller, reduced); err == nil {
			t.Fatal("disabled original privilege accepted")
		}
		actual, err := tokenPrivileges(reduced)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range actual {
			if p.Luid == enabled.Luid && p.Attributes&windows.SE_PRIVILEGE_ENABLED != 0 {
				t.Fatal("child privilege was enabled")
			}
		}
	})
	unchanged, err := tokenProfile(caller)
	if err != nil || unchanged != before {
		t.Fatalf("caller token changed: %v", err)
	}
	processProfile, err := tokenProfile(original)
	if err != nil || processProfile != originalProfile {
		t.Fatalf("actual process token changed: %v", err)
	}
}
