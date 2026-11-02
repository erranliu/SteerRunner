//go:build windows

package application

import "golang.org/x/sys/windows"

func materialPathRedirected(path string) bool {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return true
	}
	attributes, err := windows.GetFileAttributes(name)
	return err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
