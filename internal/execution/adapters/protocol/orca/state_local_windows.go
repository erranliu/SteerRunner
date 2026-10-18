package orca

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

func dialLocalState(ctx context.Context, endpoint string) (net.Conn, error) {
	return winio.DialPipeContext(ctx, endpoint)
}
