//go:build !windows

package orca

import (
	"context"
	"errors"
	"net"
)

func dialLocalState(context.Context, string) (net.Conn, error) {
	return nil, errors.New("local Orca IPC requires Windows")
}
