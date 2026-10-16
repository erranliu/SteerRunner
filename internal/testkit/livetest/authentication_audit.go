package livetest

import (
	"encoding/json"
	"errors"
)

// CheckBrokerCleanup independently catches refusals hidden by shell output
// redirection or a later successful command. Missing counters are not zero.
func CheckBrokerCleanup(body []byte) error {
	var fact struct {
		Revoked bool `json:"revocation_confirmed"`
		Broker  *struct {
			Closed      bool    `json:"closed"`
			Denied      *uint64 `json:"denied_requests"`
			SocketClean bool    `json:"socket_cleanup_confirmed"`
		} `json:"broker"`
	}
	if json.Unmarshal(body, &fact) != nil || !fact.Revoked || fact.Broker == nil || fact.Broker.Denied == nil || *fact.Broker.Denied != 0 || !fact.Broker.Closed || !fact.Broker.SocketClean {
		return errors.New("L2 requires settled authentication with zero broker refusals")
	}
	return nil
}
