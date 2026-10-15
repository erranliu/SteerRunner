package operation

import (
	"bytes"
	"encoding/json"
	"errors"
	b "github.com/erranliu/SteerRunner/internal/execution/backend"
	"github.com/erranliu/SteerRunner/internal/execution/reliable"
)

type SavedIntent struct {
	Request       []byte
	PayloadDigest string
}

func ValidateSavedIntent(saved *SavedIntent, action b.LifecycleAction, input b.LifecycleInput) error {
	if saved == nil {
		return errors.New("lifecycle operation is missing")
	}
	value, err := InputValue(action, input)
	if err != nil {
		return err
	}
	wire, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if !bytes.Equal(saved.Request, wire) || saved.PayloadDigest != reliable.Digest(wire) {
		return errors.New("lifecycle operation input differs from its original durable intent")
	}
	return nil
}
