package orca

import (
	"context"
	"encoding/json"
	"errors"
)

// One reader owns each connection. Queries/uploads do not share the live feed.
type stateConnection interface {
	request(context.Context, string, any) (string, error)
	next(context.Context, string) (json.RawMessage, error)
	close() error
}

func openStateConnection(ctx context.Context, environment, runtime string) (stateConnection, error) {
	if runtime == "" {
		return nil, errors.New("missing original Orca runtime")
	}
	if environment == "" {
		return openLocalState(ctx, runtime)
	}
	pairing, err := resolveStatePairing(environment)
	if err != nil {
		return nil, err
	}
	return openStateSocket(ctx, pairing, runtime)
}

func callState(ctx context.Context, conn stateConnection, method string, params any) (json.RawMessage, error) {
	id, err := conn.request(ctx, method, params)
	if err != nil {
		return nil, err
	}
	return conn.next(ctx, id)
}

func stateResponse(data []byte, requestID, runtime string) (json.RawMessage, error) {
	var response struct {
		ID        string                     `json:"id"`
		OK        bool                       `json:"ok"`
		Result    json.RawMessage            `json:"result"`
		Error     *StateRPCError             `json:"error"`
		Keepalive bool                       `json:"_keepalive"`
		Meta      struct{ RuntimeID string } `json:"_meta"`
	}
	if json.Unmarshal(data, &response) != nil {
		return nil, errors.New("invalid Orca state envelope")
	}
	if response.Keepalive {
		return nil, nil
	}
	if response.ID != requestID || response.Meta.RuntimeID != runtime {
		return nil, stateIdentityError("request/runtime identity changed")
	}
	if !response.OK {
		if response.Error != nil {
			return nil, response.Error
		}
		return nil, errors.New("Orca state request was rejected")
	}
	if len(response.Result) == 0 {
		return nil, errors.New("missing Orca state result")
	}
	return response.Result, nil
}
